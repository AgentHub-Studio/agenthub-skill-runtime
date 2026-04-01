package executor_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-skill-runtime/internal/executor"
)

// fakeExecutor is a ToolExecutor stub for testing.
type fakeExecutor struct {
	toolType string
}

func (f *fakeExecutor) GetToolType() string { return f.toolType }
func (f *fakeExecutor) Execute(_ context.Context, _ executor.ExecutionContext) (*executor.Result, error) {
	return &executor.Result{Output: map[string]any{"ok": true}}, nil
}

func TestRegistry_RegisterAndGet_Success(t *testing.T) {
	r := executor.NewRegistry()
	r.Register(&fakeExecutor{toolType: "HTTP"})

	got, err := r.Get("HTTP")

	require.NoError(t, err)
	assert.Equal(t, "HTTP", got.GetToolType())
}

func TestRegistry_Get_NotFound(t *testing.T) {
	r := executor.NewRegistry()

	_, err := r.Get("UNKNOWN")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "no executor registered for type")
}

func TestRegistry_Types_ReturnsAll(t *testing.T) {
	r := executor.NewRegistry()
	r.Register(&fakeExecutor{toolType: "HTTP"})
	r.Register(&fakeExecutor{toolType: "SQL"})

	types := r.Types()

	assert.Len(t, types, 2)
	assert.ElementsMatch(t, []string{"HTTP", "SQL"}, types)
}

func TestRegistry_Register_Replaces(t *testing.T) {
	r := executor.NewRegistry()
	r.Register(&fakeExecutor{toolType: "HTTP"})
	r.Register(&fakeExecutor{toolType: "HTTP"}) // replace

	types := r.Types()
	assert.Len(t, types, 1)
}
