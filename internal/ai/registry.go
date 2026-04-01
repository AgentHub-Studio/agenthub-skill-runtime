package ai

import (
	"fmt"
	"sort"
	"sync"
)

// ProviderRegistry holds configured LLM providers keyed by name.
type ProviderRegistry struct {
	mu        sync.RWMutex
	providers map[string]ChatModel
	defaults  map[string]ChatModel // keyed by provider type ("openai", "anthropic", etc.)
}

// NewProviderRegistry creates an empty ProviderRegistry.
func NewProviderRegistry() *ProviderRegistry {
	return &ProviderRegistry{
		providers: make(map[string]ChatModel),
		defaults:  make(map[string]ChatModel),
	}
}

// Register adds a provider under the given name key.
// The first provider registered for a given provider type becomes the default for that type.
func (r *ProviderRegistry) Register(name string, model ChatModel) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.providers[name] = model

	// Register as default for its provider type if not already set.
	providerType := model.GetProviderName()
	if _, exists := r.defaults[providerType]; !exists {
		r.defaults[providerType] = model
	}
}

// Get returns the provider registered under name.
func (r *ProviderRegistry) Get(name string) (ChatModel, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	m, ok := r.providers[name]
	if !ok {
		return nil, fmt.Errorf("ai: provider %q not registered", name)
	}
	return m, nil
}

// GetDefault returns the default provider for the given provider type (e.g., "openai").
func (r *ProviderRegistry) GetDefault(providerType string) (ChatModel, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	m, ok := r.defaults[providerType]
	if !ok {
		return nil, fmt.Errorf("ai: no default provider registered for type %q", providerType)
	}
	return m, nil
}

// Available returns all registered provider name keys in sorted order.
func (r *ProviderRegistry) Available() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.providers))
	for name := range r.providers {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
