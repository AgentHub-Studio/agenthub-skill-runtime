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

	// RetryDelay is the base wait duration between retries.
	RetryDelay time.Duration

	// Timeout is the per-attempt deadline (0 = no timeout beyond ctx).
	Timeout time.Duration

	// CircuitBreakerThreshold is the number of consecutive failures before the
	// circuit opens. 0 disables the circuit breaker.
	CircuitBreakerThreshold int
}

// DefaultConfig returns sensible defaults.
func DefaultConfig() Config {
	return Config{
		MaxRetries:              2,
		RetryDelay:              200 * time.Millisecond,
		Timeout:                 30 * time.Second,
		CircuitBreakerThreshold: 5,
	}
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
			case <-time.After(inv.cfg.RetryDelay):
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
