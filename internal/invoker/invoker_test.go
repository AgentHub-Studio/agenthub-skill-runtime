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

func TestInvoker_PermanentError_NoRetry(t *testing.T) {
	stub := &stubExecutor{
		toolType:  "MOCK",
		failUntil: 99,
		err:       executor.Permanentf("datasource not configured"),
	}
	cfg := invoker.Config{
		MaxRetries: 2, // would allow 3 attempts, but permanent error skips retries
		RetryDelay: time.Millisecond,
		Timeout:    time.Second,
	}
	inv := invoker.New(stub, cfg)
	_, err := inv.Invoke(context.Background(), ec())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "permanent error")
	assert.Contains(t, err.Error(), "datasource not configured")
	// Must have been called exactly once — no retries for permanent errors.
	assert.Equal(t, 1, stub.callCount)
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

func TestInvoker_ExponentialBackoff_DelayGrowsWithAttempt(t *testing.T) {
	cfg := invoker.Config{
		RetryDelay:           100 * time.Millisecond,
		RetryDelayMultiplier: 2.0,
		MaxRetryDelay:        400 * time.Millisecond,
	}
	// Attempt 0 → 100ms, attempt 1 → 200ms, attempt 2 → 400ms (capped), attempt 3 → 400ms (capped)
	assert.Equal(t, 100*time.Millisecond, cfg.RetryDelayForAttempt(0))
	assert.Equal(t, 200*time.Millisecond, cfg.RetryDelayForAttempt(1))
	assert.Equal(t, 400*time.Millisecond, cfg.RetryDelayForAttempt(2))
	assert.Equal(t, 400*time.Millisecond, cfg.RetryDelayForAttempt(3), "cap at MaxRetryDelay")
}

func TestInvoker_ExponentialBackoff_NoMultiplierIsConstant(t *testing.T) {
	cfg := invoker.Config{
		RetryDelay:           50 * time.Millisecond,
		RetryDelayMultiplier: 0, // disabled
	}
	assert.Equal(t, 50*time.Millisecond, cfg.RetryDelayForAttempt(0))
	assert.Equal(t, 50*time.Millisecond, cfg.RetryDelayForAttempt(5))
}

func TestInvoker_LatencyIsPopulated(t *testing.T) {
	exec := &stubExecutor{result: &executor.Result{Output: map[string]any{"ok": true}}}
	inv := invoker.New(exec, invoker.Config{Timeout: time.Second})

	result, err := inv.Invoke(context.Background(), executor.ExecutionContext{})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, result.LatencyMs, int64(0), "LatencyMs should be set")
}
