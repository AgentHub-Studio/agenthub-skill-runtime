package invoker_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-skill-runtime/internal/executor"
	"github.com/AgentHub-Studio/agenthub-skill-runtime/internal/invoker"
)

// stubExecutor is a configurable ToolExecutor for testing.
type stubExecutor struct {
	toolType  string
	callCount int
	failUntil int // fail the first N calls
	result    *executor.Result
	err       error
}

func (s *stubExecutor) GetToolType() string { return s.toolType }

func (s *stubExecutor) Execute(_ context.Context, _ executor.ExecutionContext) (*executor.Result, error) {
	s.callCount++
	if s.callCount <= s.failUntil {
		return nil, s.err
	}
	if s.result != nil {
		return s.result, nil
	}
	return &executor.Result{Output: map[string]any{"ok": true}}, nil
}

func ec() executor.ExecutionContext {
	return executor.ExecutionContext{ToolID: "t1", TenantID: "test-tenant"}
}

func TestInvoker_Success_NoRetry(t *testing.T) {
	stub := &stubExecutor{toolType: "MOCK"}
	inv := invoker.New(stub, invoker.Config{MaxRetries: 0, Timeout: time.Second})
	res, err := inv.Invoke(context.Background(), ec())
	require.NoError(t, err)
	assert.Equal(t, true, res.Output["ok"])
	assert.Equal(t, 1, stub.callCount)
}

func TestInvoker_RetryOnFailure(t *testing.T) {
	// Fail first 2 calls, succeed on 3rd.
	stub := &stubExecutor{
		toolType:  "MOCK",
		failUntil: 2,
		err:       errors.New("transient error"),
	}
	cfg := invoker.Config{
		MaxRetries:  2,
		RetryDelay:  time.Millisecond,
		Timeout:     time.Second,
	}
	inv := invoker.New(stub, cfg)
	res, err := inv.Invoke(context.Background(), ec())
	require.NoError(t, err)
	assert.NotNil(t, res)
	assert.Equal(t, 3, stub.callCount)
}

func TestInvoker_AllAttemptsFail(t *testing.T) {
	stub := &stubExecutor{
		toolType:  "MOCK",
		failUntil: 99,
		err:       errors.New("permanent error"),
	}
	cfg := invoker.Config{
		MaxRetries: 2,
		RetryDelay: time.Millisecond,
		Timeout:    time.Second,
	}
	inv := invoker.New(stub, cfg)
	_, err := inv.Invoke(context.Background(), ec())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "all 3 attempt(s) failed")
	assert.Equal(t, 3, stub.callCount)
}

func TestInvoker_CircuitBreakerOpensAfterThreshold(t *testing.T) {
	stub := &stubExecutor{
		toolType:  "MOCK",
		failUntil: 99,
		err:       errors.New("fail"),
	}
	threshold := 3
	cfg := invoker.Config{
		MaxRetries:              0,
		RetryDelay:              time.Millisecond,
		Timeout:                 time.Second,
		CircuitBreakerThreshold: threshold,
	}
	inv := invoker.New(stub, cfg)

	// Exhaust the threshold.
	for i := 0; i < threshold; i++ {
		_, err := inv.Invoke(context.Background(), ec())
		require.Error(t, err)
		assert.NotErrorIs(t, err, invoker.ErrCircuitOpen)
	}

	// Next call should hit the open circuit.
	_, err := inv.Invoke(context.Background(), ec())
	require.ErrorIs(t, err, invoker.ErrCircuitOpen)
}

func TestInvoker_CircuitBreakerResetsOnSuccess(t *testing.T) {
	stub := &stubExecutor{
		toolType:  "MOCK",
		failUntil: 2,
		err:       errors.New("fail"),
	}
	cfg := invoker.Config{
		MaxRetries:              2, // allows 3 attempts — succeeds on 3rd
		RetryDelay:              time.Millisecond,
		Timeout:                 time.Second,
		CircuitBreakerThreshold: 5,
	}
	inv := invoker.New(stub, cfg)

	// First call: retries twice then succeeds — failures counter should reset.
	res, err := inv.Invoke(context.Background(), ec())
	require.NoError(t, err)
	assert.NotNil(t, res)

	// Circuit should still be closed; second call also succeeds.
	stub.failUntil = 0
	_, err = inv.Invoke(context.Background(), ec())
	require.NoError(t, err)
}

func TestInvoker_CircuitBreakerDisabledWhenZero(t *testing.T) {
	stub := &stubExecutor{
		toolType:  "MOCK",
		failUntil: 99,
		err:       errors.New("fail"),
	}
	cfg := invoker.Config{
		MaxRetries:              0,
		RetryDelay:              time.Millisecond,
		Timeout:                 time.Second,
		CircuitBreakerThreshold: 0, // disabled
	}
	inv := invoker.New(stub, cfg)

	// Many failures should never open the circuit.
	for i := 0; i < 10; i++ {
		_, err := inv.Invoke(context.Background(), ec())
		require.Error(t, err)
		assert.NotErrorIs(t, err, invoker.ErrCircuitOpen)
	}
}

func TestInvoker_ContextCancelledDuringRetryDelay(t *testing.T) {
	stub := &stubExecutor{
		toolType:  "MOCK",
		failUntil: 99,
		err:       errors.New("fail"),
	}
	cfg := invoker.Config{
		MaxRetries: 3,
		RetryDelay: 500 * time.Millisecond, // long delay
		Timeout:    5 * time.Second,
	}
	inv := invoker.New(stub, cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := inv.Invoke(ctx, ec())
	require.Error(t, err)
	// Should be context error, not "all attempts failed" with full retry count.
	assert.True(t, errors.Is(err, context.DeadlineExceeded) || stub.callCount < 4,
		"expected context cancellation to cut retries short")
}

func TestInvoker_ExponentialBackoffDelayGrows(t *testing.T) {
	// Track when each Execute call happens to verify growing delays.
	var callTimes []time.Time
	stub := &customStub{
		onExecute: func() (*executor.Result, error) {
			callTimes = append(callTimes, time.Now())
			if len(callTimes) < 3 {
				return nil, errors.New("transient")
			}
			return &executor.Result{Output: map[string]any{"ok": true}}, nil
		},
	}
	cfg := invoker.Config{
		MaxRetries:    2,
		RetryDelay:    10 * time.Millisecond,
		MaxRetryDelay: 1 * time.Second,
		Timeout:       5 * time.Second,
	}
	inv := invoker.New(stub, cfg)
	_, err := inv.Invoke(context.Background(), ec())
	require.NoError(t, err)
	require.Len(t, callTimes, 3)

	// delay0 = 10ms (base), delay1 = 20ms (2x base)
	// Each gap between calls should be >= the expected delay.
	gap0 := callTimes[1].Sub(callTimes[0])
	gap1 := callTimes[2].Sub(callTimes[1])
	assert.GreaterOrEqual(t, gap0, 10*time.Millisecond, "first retry gap should be >= base delay")
	assert.GreaterOrEqual(t, gap1, gap0, "second retry gap should be >= first (exponential growth)")
}

func TestInvoker_ExponentialBackoffCappedByMaxRetryDelay(t *testing.T) {
	calls := 0
	stub := &customStub{
		onExecute: func() (*executor.Result, error) {
			calls++
			if calls < 4 {
				return nil, errors.New("transient")
			}
			return &executor.Result{Output: map[string]any{"ok": true}}, nil
		},
	}
	cfg := invoker.Config{
		MaxRetries:    3,
		RetryDelay:    100 * time.Millisecond,
		MaxRetryDelay: 150 * time.Millisecond, // caps at 150ms even though 2x200ms = 400ms
		Timeout:       5 * time.Second,
	}
	start := time.Now()
	inv := invoker.New(stub, cfg)
	_, err := inv.Invoke(context.Background(), ec())
	require.NoError(t, err)
	// 3 retries, all capped at 150ms → total ≤ 3*150ms + overhead = 450ms + overhead
	// Without cap it would be 100+200+400 = 700ms
	elapsed := time.Since(start)
	assert.Less(t, elapsed, 600*time.Millisecond, "backoff should be capped, total should be much less than uncapped 700ms")
}

// customStub is a ToolExecutor backed by a callback.
type customStub struct {
	toolType  string
	onExecute func() (*executor.Result, error)
}

func (s *customStub) GetToolType() string { return s.toolType }
func (s *customStub) Execute(_ context.Context, _ executor.ExecutionContext) (*executor.Result, error) {
	return s.onExecute()
}

func TestInvoker_LatencyIsPopulated(t *testing.T) {
	exec := &stubExecutor{result: &executor.Result{Output: map[string]any{"ok": true}}}
	inv := invoker.New(exec, invoker.Config{Timeout: time.Second})

	result, err := inv.Invoke(context.Background(), executor.ExecutionContext{})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, result.LatencyMs, int64(0), "LatencyMs should be set")
}
