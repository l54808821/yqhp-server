package execution

import (
	"fmt"
	"sync"

	"github.com/grafana/k6/workflow-engine/pkg/types"
)

// Registry manages execution mode instances.
type Registry struct {
	modes map[types.ExecutionMode]func() Mode
	mu    sync.RWMutex
}

// NewRegistry creates a new execution mode registry with default modes.
func NewRegistry() *Registry {
	r := &Registry{
		modes: make(map[types.ExecutionMode]func() Mode),
	}

	// Register default modes
	r.Register(types.ModeConstantVUs, func() Mode { return NewConstantVUsMode() })
	r.Register(types.ModeRampingVUs, func() Mode { return NewRampingVUsMode() })
	r.Register(types.ModeConstantArrivalRate, func() Mode { return NewConstantArrivalRateMode() })
	r.Register(types.ModeRampingArrivalRate, func() Mode { return NewRampingArrivalRateMode() })
	r.Register(types.ModePerVUIterations, func() Mode { return NewPerVUIterationsMode() })
	r.Register(types.ModeSharedIterations, func() Mode { return NewSharedIterationsMode() })
	r.Register(types.ModeExternally, func() Mode { return NewExternallyControlledMode() })

	return r
}

// Register registers a mode factory.
func (r *Registry) Register(mode types.ExecutionMode, factory func() Mode) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.modes[mode] = factory
}

// Get returns a new instance of the specified mode.
func (r *Registry) Get(mode types.ExecutionMode) (Mode, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	factory, ok := r.modes[mode]
	if !ok {
		return nil, fmt.Errorf("unknown execution mode: %s", mode)
	}

	return factory(), nil
}

// GetOrDefault returns a new instance of the specified mode, or constant-vus if empty.
func (r *Registry) GetOrDefault(mode types.ExecutionMode) (Mode, error) {
	if mode == "" {
		mode = types.ModeConstantVUs
	}
	return r.Get(mode)
}

// List returns all registered mode names.
func (r *Registry) List() []types.ExecutionMode {
	r.mu.RLock()
	defer r.mu.RUnlock()

	modes := make([]types.ExecutionMode, 0, len(r.modes))
	for mode := range r.modes {
		modes = append(modes, mode)
	}
	return modes
}

// DefaultRegistry is the default execution mode registry.
var DefaultRegistry = NewRegistry()

// GetMode returns a new instance of the specified mode from the default registry.
func GetMode(mode types.ExecutionMode) (Mode, error) {
	return DefaultRegistry.Get(mode)
}

// GetModeOrDefault returns a new instance of the specified mode from the default registry,
// or constant-vus if empty.
func GetModeOrDefault(mode types.ExecutionMode) (Mode, error) {
	return DefaultRegistry.GetOrDefault(mode)
}
