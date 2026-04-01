package executor

import "fmt"

// Registry maps tool types to their ToolExecutor implementations.
type Registry struct {
	executors map[string]ToolExecutor
}

// NewRegistry creates an empty Registry.
func NewRegistry() *Registry {
	return &Registry{executors: make(map[string]ToolExecutor)}
}

// Register adds a ToolExecutor to the registry.
// If an executor for the same type already exists it is replaced.
func (r *Registry) Register(e ToolExecutor) {
	r.executors[e.GetToolType()] = e
}

// Get returns the executor registered for toolType.
func (r *Registry) Get(toolType string) (ToolExecutor, error) {
	e, ok := r.executors[toolType]
	if !ok {
		return nil, fmt.Errorf("executor: no executor registered for type %q", toolType)
	}
	return e, nil
}

// Types returns a slice of all registered tool type identifiers.
func (r *Registry) Types() []string {
	types := make([]string, 0, len(r.executors))
	for t := range r.executors {
		types = append(types, t)
	}
	return types
}
