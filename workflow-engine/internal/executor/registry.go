package executor

import (
	"context"
	"fmt"
	"sync"
)

// Registry 管理执行器的注册和查找。
type Registry struct {
	executors map[string]Executor
	mu        sync.RWMutex
}

// NewRegistry 创建一个新的执行器注册表。
func NewRegistry() *Registry {
	return &Registry{
		executors: make(map[string]Executor),
	}
}

// Register 为给定类型注册执行器。
// 如果该类型已注册执行器，则返回错误。
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

// MustRegister 注册执行器，如果出错则 panic。
func (r *Registry) MustRegister(executor Executor) {
	if err := r.Register(executor); err != nil {
		panic(err)
	}
}

// Unregister 移除给定类型的执行器。
func (r *Registry) Unregister(execType string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.executors, execType)
}

// Get 按类型获取执行器。
// 如果该类型没有注册执行器，则返回 nil。
func (r *Registry) Get(execType string) Executor {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.executors[execType]
}

// GetOrError 按类型获取执行器，如果不存在则返回错误。
func (r *Registry) GetOrError(execType string) (Executor, error) {
	executor := r.Get(execType)
	if executor == nil {
		return nil, NewExecutorNotFoundError(execType)
	}
	return executor, nil
}

// Has 检查给定类型是否已注册执行器。
func (r *Registry) Has(execType string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, exists := r.executors[execType]
	return exists
}

// Types 返回所有已注册的执行器类型。
func (r *Registry) Types() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	types := make([]string, 0, len(r.executors))
	for t := range r.executors {
		types = append(types, t)
	}
	return types
}

// Count 返回已注册执行器的数量。
func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.executors)
}

// InitAll 初始化所有已注册的执行器。
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

// CleanupAll 清理所有已注册的执行器。
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

// DefaultRegistry 是全局默认执行器注册表。
var DefaultRegistry = NewRegistry()

// Register 在默认注册表中注册执行器。
func Register(executor Executor) error {
	return DefaultRegistry.Register(executor)
}

// MustRegister 在默认注册表中注册执行器，如果出错则 panic。
func MustRegister(executor Executor) {
	DefaultRegistry.MustRegister(executor)
}

// Get 从默认注册表获取执行器。
func Get(execType string) Executor {
	return DefaultRegistry.Get(execType)
}

// GetOrError 从默认注册表获取执行器，如果不存在则返回错误。
func GetOrError(execType string) (Executor, error) {
	return DefaultRegistry.GetOrError(execType)
}
