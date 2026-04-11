package executor

import (
	"context"
	"errors"
	"fmt"
)

// PermanentError wraps an error that must not be retried — e.g. invalid
// configuration, missing required fields, or unsupported operations.
// The invoker skips backoff and fails immediately on the first attempt.
type PermanentError struct {
	Cause error
}

func (e *PermanentError) Error() string { return e.Cause.Error() }
func (e *PermanentError) Unwrap() error { return e.Cause }

// Permanent wraps cause as a PermanentError.
func Permanent(cause error) error { return &PermanentError{Cause: cause} }

// Permanentf creates a PermanentError from a format string.
func Permanentf(format string, args ...any) error {
	return &PermanentError{Cause: fmt.Errorf(format, args...)}
}

// IsPermanent reports whether err is (or wraps) a PermanentError.
func IsPermanent(err error) bool {
	var p *PermanentError
	return errors.As(err, &p)
}

// ExecutionContext holds the input data and metadata for a tool execution.
type ExecutionContext struct {
	// ToolID is the unique identifier of the tool to execute.
	ToolID string
	// SkillSlug is the slug of the skill that resolved this execution (may be empty for direct tool calls).
	SkillSlug string
	// TenantID is the tenant context extracted from the JWT issuer.
	TenantID string
	// CallerToken is the raw Bearer JWT of the caller, forwarded when useCallerToken is true.
	CallerToken string
	// Input holds the caller-supplied input parameters for the tool.
	Input map[string]any
	// Config holds the tool configuration loaded from the database (JSONB).
	Config map[string]any
}

// Result holds the output of a tool execution.
type Result struct {
	// Output contains the tool's response payload.
	Output map[string]any
	// Error describes the failure reason when execution was unsuccessful.
	Error string
	// LatencyMs is the total execution time in milliseconds, set by ToolInvoker.
	LatencyMs int64
}

// ToolExecutor executes a specific type of tool.
type ToolExecutor interface {
	// GetToolType returns the type identifier (e.g., "HTTP", "SQL", "DOCUMENT_SEARCH").
	GetToolType() string
	// Execute runs the tool and returns the result.
	Execute(ctx context.Context, ec ExecutionContext) (*Result, error)
}
