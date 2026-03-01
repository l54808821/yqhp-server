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
		return fmt.Errorf("不能注册空执行器")
	}

	execType := executor.Type()
	if execType == "" {
		return fmt.Errorf("执行器类型不能为空")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.executors[execType]; exists {
		return fmt.Errorf("执行器类型已注册: %s", execType)
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
			return fmt.Errorf("初始化执行器 %s 失败: %w", execType, err)
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
			lastErr = fmt.Errorf("清理执行器 %s 失败: %w", execType, err)
		}
	}
	return lastErr
}

// RegisterAlias 为已注册的执行器类型创建别名。
// 别名类型将指向目标类型的执行器实例。
func (r *Registry) RegisterAlias(aliasType, targetType string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if target, exists := r.executors[targetType]; exists {
		r.executors[aliasType] = target
	}
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

// RegisterAlias 在默认注册表中为已注册的执行器创建别名。
func RegisterAlias(aliasType, targetType string) {
	DefaultRegistry.RegisterAlias(aliasType, targetType)
}

// Get 从默认注册表获取执行器。
func Get(execType string) Executor {
	return DefaultRegistry.Get(execType)
}

// GetOrError 从默认注册表获取执行器，如果不存在则返回错误。
func GetOrError(execType string) (Executor, error) {
	return DefaultRegistry.GetOrError(execType)
}
