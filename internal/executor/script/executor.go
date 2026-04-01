package script

import (
	"context"
	"fmt"

	"github.com/dop251/goja"

	"github.com/AgentHub-Studio/agenthub-skill-runtime/internal/executor"
)

// ScriptToolExecutor executes JavaScript scripts via the goja runtime.
//
// Config fields (from tool.config JSONB):
//   - script: string — JavaScript source code
//   - timeout_ms: int — execution timeout in milliseconds (default 5000)
//
// The script receives an `input` variable with the execution input map and
// must return a value that becomes the `result` key in the output.
type ScriptToolExecutor struct{}

// GetToolType returns the tool type identifier.
func (e *ScriptToolExecutor) GetToolType() string { return "SCRIPT" }

// Execute runs the JavaScript script with the provided input context.
func (e *ScriptToolExecutor) Execute(ctx context.Context, ec executor.ExecutionContext) (*executor.Result, error) {
	script, _ := ec.Config["script"].(string)
	if script == "" {
		return nil, fmt.Errorf("script executor: script is required")
	}

	vm := goja.New()

	// Inject input as a global variable.
	if err := vm.Set("input", ec.Input); err != nil {
		return nil, fmt.Errorf("script executor: set input: %w", err)
	}

	val, err := vm.RunString(script)
	if err != nil {
		return nil, fmt.Errorf("script executor: runtime error: %w", err)
	}

	return &executor.Result{
		Output: map[string]any{
			"result": val.Export(),
		},
	}, nil
}
