package execution

import (
	"fmt"
	"sync"

	"yqhp/workflow-engine/pkg/types"
)

// Registry 管理执行模式实例。
type Registry struct {
	modes map[types.ExecutionMode]func() Mode
	mu    sync.RWMutex
}

// NewRegistry 创建一个带有默认模式的新执行模式注册表。
func NewRegistry() *Registry {
	r := &Registry{
		modes: make(map[types.ExecutionMode]func() Mode),
	}

	// 注册默认模式
	r.Register(types.ModeConstantVUs, func() Mode { return NewConstantVUsMode() })
	r.Register(types.ModeRampingVUs, func() Mode { return NewRampingVUsMode() })
	r.Register(types.ModeConstantArrivalRate, func() Mode { return NewConstantArrivalRateMode() })
	r.Register(types.ModeRampingArrivalRate, func() Mode { return NewRampingArrivalRateMode() })
	r.Register(types.ModePerVUIterations, func() Mode { return NewPerVUIterationsMode() })
	r.Register(types.ModeSharedIterations, func() Mode { return NewSharedIterationsMode() })
	r.Register(types.ModeExternally, func() Mode { return NewExternallyControlledMode() })

	return r
}

// Register 注册模式工厂。
func (r *Registry) Register(mode types.ExecutionMode, factory func() Mode) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.modes[mode] = factory
}

// Get 返回指定模式的新实例。
func (r *Registry) Get(mode types.ExecutionMode) (Mode, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	factory, ok := r.modes[mode]
	if !ok {
		return nil, fmt.Errorf("未知的执行模式: %s", mode)
	}

	return factory(), nil
}

// GetOrDefault 返回指定模式的新实例，如果为空则返回 constant-vus。
func (r *Registry) GetOrDefault(mode types.ExecutionMode) (Mode, error) {
	if mode == "" {
		mode = types.ModeConstantVUs
	}
	return r.Get(mode)
}

// List 返回所有已注册的模式名称。
func (r *Registry) List() []types.ExecutionMode {
	r.mu.RLock()
	defer r.mu.RUnlock()

	modes := make([]types.ExecutionMode, 0, len(r.modes))
	for mode := range r.modes {
		modes = append(modes, mode)
	}
	return modes
}

// DefaultRegistry 是默认的执行模式注册表。
var DefaultRegistry = NewRegistry()

// GetMode 从默认注册表返回指定模式的新实例。
func GetMode(mode types.ExecutionMode) (Mode, error) {
	return DefaultRegistry.Get(mode)
}

// GetModeOrDefault 从默认注册表返回指定模式的新实例，
// 如果为空则返回 constant-vus。
func GetModeOrDefault(mode types.ExecutionMode) (Mode, error) {
	return DefaultRegistry.GetOrDefault(mode)
}
