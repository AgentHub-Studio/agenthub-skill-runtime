package script

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/dop251/goja"

	"github.com/AgentHub-Studio/agenthub-skill-runtime/internal/executor"
)

const defaultTimeoutMS = 5000

// ScriptToolExecutor executes JavaScript scripts via the goja runtime.
//
// Config fields (from tool.config JSONB):
//   - script: string — JavaScript source code
//   - timeout_ms: int — execution timeout in milliseconds (default 5000)
//
// The script receives an `input` variable with the execution input map and
// must return a value that becomes the `result` key in the output.
// console.log output is captured and returned under the "logs" key.
type ScriptToolExecutor struct{}

// GetToolType returns the tool type identifier.
func (e *ScriptToolExecutor) GetToolType() string { return "SCRIPT" }

// Execute runs the JavaScript script with the provided input context.
func (e *ScriptToolExecutor) Execute(ctx context.Context, ec executor.ExecutionContext) (*executor.Result, error) {
	scriptSrc, _ := ec.Config["script"].(string)
	if scriptSrc == "" {
		return nil, fmt.Errorf("script executor: script is required")
	}

	timeoutMS := defaultTimeoutMS
	if v, ok := ec.Config["timeout_ms"].(int); ok && v > 0 {
		timeoutMS = v
	}

	vm := goja.New()

	// Capture console.log output.
	var logBuf bytes.Buffer
	console := vm.NewObject()
	_ = console.Set("log", func(call goja.FunctionCall) goja.Value {
		for i, arg := range call.Arguments {
			if i > 0 {
				logBuf.WriteByte(' ')
			}
			logBuf.WriteString(arg.String())
		}
		logBuf.WriteByte('\n')
		return goja.Undefined()
	})
	_ = vm.Set("console", console)

	// Inject input as a global variable.
	if err := vm.Set("input", ec.Input); err != nil {
		return nil, fmt.Errorf("script executor: set input: %w", err)
	}

	// Enforce timeout: interrupt the VM after timeoutMS milliseconds.
	timer := time.AfterFunc(time.Duration(timeoutMS)*time.Millisecond, func() {
		vm.Interrupt("execution timeout exceeded")
	})
	defer timer.Stop()

	// Also respect context cancellation.
	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			vm.Interrupt("context cancelled")
		case <-done:
		}
	}()

	val, err := vm.RunString(scriptSrc)
	close(done)
	if err != nil {
		return nil, fmt.Errorf("script executor: runtime error: %w", err)
	}

	out := map[string]any{
		"result": val.Export(),
	}
	if logBuf.Len() > 0 {
		out["logs"] = logBuf.String()
	}

	return &executor.Result{Output: out}, nil
}
