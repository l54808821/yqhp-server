package executor

import (
	"context"
	"fmt"
	"sync"
)

// Registry manages executor registration and lookup.
type Registry struct {
	executors map[string]Executor
	mu        sync.RWMutex
}

// NewRegistry creates a new executor registry.
func NewRegistry() *Registry {
	return &Registry{
		executors: make(map[string]Executor),
	}
}

// Register registers an executor for a given type.
// Returns an error if an executor is already registered for the type.
func (r *Registry) Register(executor Executor) error {
	if executor == nil {
		return fmt.Errorf("cannot register nil executor")
	}

	execType := executor.Type()
	if execType == "" {
		return fmt.Errorf("executor type cannot be empty")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.executors[execType]; exists {
		return fmt.Errorf("executor already registered for type: %s", execType)
	}

	r.executors[execType] = executor
	return nil
}

// MustRegister registers an executor and panics on error.
func (r *Registry) MustRegister(executor Executor) {
	if err := r.Register(executor); err != nil {
		panic(err)
	}
}

// Unregister removes an executor for a given type.
func (r *Registry) Unregister(execType string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.executors, execType)
}

// Get retrieves an executor by type.
// Returns nil if no executor is registered for the type.
func (r *Registry) Get(execType string) Executor {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.executors[execType]
}

// GetOrError retrieves an executor by type or returns an error.
func (r *Registry) GetOrError(execType string) (Executor, error) {
	executor := r.Get(execType)
	if executor == nil {
		return nil, NewExecutorNotFoundError(execType)
	}
	return executor, nil
}

// Has checks if an executor is registered for a given type.
func (r *Registry) Has(execType string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, exists := r.executors[execType]
	return exists
}

// Types returns all registered executor types.
func (r *Registry) Types() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	types := make([]string, 0, len(r.executors))
	for t := range r.executors {
		types = append(types, t)
	}
	return types
}

// Count returns the number of registered executors.
func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.executors)
}

// InitAll initializes all registered executors.
func (r *Registry) InitAll(ctx context.Context, configs map[string]map[string]any) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for execType, executor := range r.executors {
		config := configs[execType]
		if config == nil {
			config = make(map[string]any)
		}
		if err := executor.Init(ctx, config); err != nil {
			return fmt.Errorf("failed to initialize executor %s: %w", execType, err)
		}
	}
	return nil
}

// CleanupAll cleans up all registered executors.
func (r *Registry) CleanupAll(ctx context.Context) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var lastErr error
	for execType, executor := range r.executors {
		if err := executor.Cleanup(ctx); err != nil {
			lastErr = fmt.Errorf("failed to cleanup executor %s: %w", execType, err)
		}
	}
	return lastErr
}

// DefaultRegistry is the global default executor registry.
var DefaultRegistry = NewRegistry()

// Register registers an executor in the default registry.
func Register(executor Executor) error {
	return DefaultRegistry.Register(executor)
}

// MustRegister registers an executor in the default registry and panics on error.
func MustRegister(executor Executor) {
	DefaultRegistry.MustRegister(executor)
}

// Get retrieves an executor from the default registry.
func Get(execType string) Executor {
	return DefaultRegistry.Get(execType)
}

// GetOrError retrieves an executor from the default registry or returns an error.
func GetOrError(execType string) (Executor, error) {
	return DefaultRegistry.GetOrError(execType)
}
