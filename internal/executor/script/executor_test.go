package script_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-skill-runtime/internal/executor"
	"github.com/AgentHub-Studio/agenthub-skill-runtime/internal/executor/script"
)

func TestScriptExecutor_GetToolType(t *testing.T) {
	e := &script.ScriptToolExecutor{}
	assert.Equal(t, "SCRIPT", e.GetToolType())
}

func TestScriptExecutor_ReturnsValue(t *testing.T) {
	e := &script.ScriptToolExecutor{}
	res, err := e.Execute(context.Background(), executor.ExecutionContext{
		Config: map[string]any{"script": `42`},
	})
	require.NoError(t, err)
	// goja exports int64 for integer literals
	assert.EqualValues(t, 42, res.Output["result"])
}

func TestScriptExecutor_AccessesInput(t *testing.T) {
	e := &script.ScriptToolExecutor{}
	res, err := e.Execute(context.Background(), executor.ExecutionContext{
		Config: map[string]any{"script": `input.name + " world"`},
		Input:  map[string]any{"name": "hello"},
	})
	require.NoError(t, err)
	assert.Equal(t, "hello world", res.Output["result"])
}

func TestScriptExecutor_ArithmeticOnInput(t *testing.T) {
	e := &script.ScriptToolExecutor{}
	res, err := e.Execute(context.Background(), executor.ExecutionContext{
		Config: map[string]any{"script": `input.x * input.y`},
		Input:  map[string]any{"x": 6, "y": 7},
	})
	require.NoError(t, err)
	assert.EqualValues(t, 42, res.Output["result"])
}

func TestScriptExecutor_ReturnsObject(t *testing.T) {
	e := &script.ScriptToolExecutor{}
	res, err := e.Execute(context.Background(), executor.ExecutionContext{
		Config: map[string]any{"script": `({key: "value", count: 1})`},
	})
	require.NoError(t, err)
	obj, ok := res.Output["result"].(map[string]any)
	require.True(t, ok, "expected map result, got %T", res.Output["result"])
	assert.Equal(t, "value", obj["key"])
}

func TestScriptExecutor_MissingScript(t *testing.T) {
	e := &script.ScriptToolExecutor{}
	_, err := e.Execute(context.Background(), executor.ExecutionContext{
		Config: map[string]any{},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "script is required")
}

func TestScriptExecutor_SyntaxError(t *testing.T) {
	e := &script.ScriptToolExecutor{}
	_, err := e.Execute(context.Background(), executor.ExecutionContext{
		Config: map[string]any{"script": `{{{{invalid javascript`},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "runtime error")
}

func TestScriptExecutor_RuntimeError(t *testing.T) {
	e := &script.ScriptToolExecutor{}
	_, err := e.Execute(context.Background(), executor.ExecutionContext{
		Config: map[string]any{"script": `throw new Error("boom")`},
	})
	require.Error(t, err)
}

func TestScriptExecutor_Timeout(t *testing.T) {
	e := &script.ScriptToolExecutor{}
	_, err := e.Execute(context.Background(), executor.ExecutionContext{
		Config: map[string]any{
			"script":     `while(true){}`,
			"timeout_ms": 50,
		},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "runtime error")
}

func TestScriptExecutor_ConsoleLog(t *testing.T) {
	e := &script.ScriptToolExecutor{}
	res, err := e.Execute(context.Background(), executor.ExecutionContext{
		Config: map[string]any{"script": `console.log("hello", "world"); "done"`},
	})
	require.NoError(t, err)
	assert.Equal(t, "done", res.Output["result"])
	assert.Contains(t, res.Output["logs"], "hello world")
}

func TestScriptExecutor_NoLogsKey_WhenNoConsoleLog(t *testing.T) {
	e := &script.ScriptToolExecutor{}
	res, err := e.Execute(context.Background(), executor.ExecutionContext{
		Config: map[string]any{"script": `1 + 1`},
	})
	require.NoError(t, err)
	_, hasLogs := res.Output["logs"]
	assert.False(t, hasLogs, "logs key should not be present when console.log is not called")
}

func TestScriptExecutor_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	e := &script.ScriptToolExecutor{}
	_, err := e.Execute(ctx, executor.ExecutionContext{
		Config: map[string]any{
			"script":     `while(true){}`,
			"timeout_ms": 5000,
		},
	})
	require.Error(t, err)
}
