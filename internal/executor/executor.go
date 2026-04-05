package executor

import "context"

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
