// Package invoker wraps ToolExecutor calls with retry, timeout, and basic
// circuit-breaker semantics.
package invoker

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/AgentHub-Studio/agenthub-skill-runtime/internal/executor"
)

// Config holds tuning parameters for the ToolInvoker.
type Config struct {
	// MaxRetries is the number of additional attempts after the first failure
	// (0 = no retries).
	MaxRetries int

	// RetryDelay is the base wait duration for the first retry interval.
	// Subsequent intervals are multiplied by RetryDelayMultiplier.
	RetryDelay time.Duration

	// RetryDelayMultiplier is the exponential growth factor applied to RetryDelay
	// between successive retries (e.g. 2.0 = doubles each time).
	// Values <= 1.0 disable the multiplier (constant delay).
	RetryDelayMultiplier float64

	// MaxRetryDelay caps the computed exponential delay.
	// 0 means no cap beyond RetryDelay.
	MaxRetryDelay time.Duration

	// Timeout is the per-attempt deadline (0 = no timeout beyond ctx).
	Timeout time.Duration

	// CircuitBreakerThreshold is the number of consecutive failures before the
	// circuit opens. 0 disables the circuit breaker.
	CircuitBreakerThreshold int
}

// DefaultConfig returns sensible defaults with exponential backoff.
func DefaultConfig() Config {
	return Config{
		MaxRetries:              2,
		RetryDelay:              200 * time.Millisecond,
		RetryDelayMultiplier:    2.0,
		MaxRetryDelay:           30 * time.Second,
		Timeout:                 30 * time.Second,
		CircuitBreakerThreshold: 5,
	}
}

// RetryDelayForAttempt returns the backoff duration for the given attempt index (0-based).
// It applies exponential growth capped at MaxRetryDelay.
func (c Config) RetryDelayForAttempt(attempt int) time.Duration {
	if attempt == 0 || c.RetryDelayMultiplier <= 1.0 {
		return c.RetryDelay
	}
	delay := float64(c.RetryDelay)
	for i := 0; i < attempt; i++ {
		delay *= c.RetryDelayMultiplier
	}
	d := time.Duration(delay)
	if c.MaxRetryDelay > 0 && d > c.MaxRetryDelay {
		return c.MaxRetryDelay
	}
	return d
}

// ErrCircuitOpen is returned when the circuit breaker is open.
var ErrCircuitOpen = errors.New("invoker: circuit breaker open")

// ToolInvoker wraps a ToolExecutor with retry, timeout, and circuit-breaker logic.
type ToolInvoker struct {
	exec    executor.ToolExecutor
	cfg     Config
	failures atomic.Int64
}

// New creates a ToolInvoker for the given executor using cfg.
func New(exec executor.ToolExecutor, cfg Config) *ToolInvoker {
	return &ToolInvoker{exec: exec, cfg: cfg}
}

// Invoke executes the tool, applying timeout per attempt and retrying on
// transient errors. Returns ErrCircuitOpen when the circuit breaker trips.
// Sets LatencyMs on the returned Result to reflect total wall-clock time.
func (inv *ToolInvoker) Invoke(ctx context.Context, ec executor.ExecutionContext) (*executor.Result, error) {
	if inv.cfg.CircuitBreakerThreshold > 0 &&
		inv.failures.Load() >= int64(inv.cfg.CircuitBreakerThreshold) {
		return nil, ErrCircuitOpen
	}

	start := time.Now()

	var (
		result *executor.Result
		err    error
	)

	attempts := inv.cfg.MaxRetries + 1
	for attempt := range attempts {
		result, err = inv.attemptOnce(ctx, ec)
		if err == nil {
			inv.failures.Store(0) // reset on success
			result.LatencyMs = time.Since(start).Milliseconds()
			return result, nil
		}

		if attempt < inv.cfg.MaxRetries {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(inv.cfg.RetryDelayForAttempt(attempt)):
			}
		}
	}

	inv.failures.Add(1)
	return nil, fmt.Errorf("invoker: all %d attempt(s) failed: %w", attempts, err)
}

// attemptOnce performs a single execution with an optional per-attempt timeout.
func (inv *ToolInvoker) attemptOnce(ctx context.Context, ec executor.ExecutionContext) (*executor.Result, error) {
	if inv.cfg.Timeout <= 0 {
		return inv.exec.Execute(ctx, ec)
	}

	attemptCtx, cancel := context.WithTimeout(ctx, inv.cfg.Timeout)
	defer cancel()
	return inv.exec.Execute(attemptCtx, ec)
}
