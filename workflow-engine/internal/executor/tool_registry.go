package executor

import (
	"fmt"
	"sync"

	"yqhp/workflow-engine/pkg/types"
)

// ToolRegistry 工具注册表，管理和查询已注册的工具。
// 使用 sync.RWMutex 保证并发安全。
type ToolRegistry struct {
	tools map[string]Tool
	mu    sync.RWMutex
}

// NewToolRegistry 创建一个新的工具注册表。
func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{
		tools: make(map[string]Tool),
	}
}

// Register 注册一个工具到注册表。
// 如果工具名称已存在，返回名称冲突错误。
func (r *ToolRegistry) Register(tool Tool) error {
	if tool == nil {
		return fmt.Errorf("不能注册空工具")
	}

	def := tool.Definition()
	if def == nil || def.Name == "" {
		return fmt.Errorf("工具定义不能为空且名称不能为空")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.tools[def.Name]; exists {
		return fmt.Errorf("工具名称已注册: %s", def.Name)
	}

	r.tools[def.Name] = tool
	return nil
}

// Get 按名称获取工具。
// 返回工具实例和是否存在的标志。
func (r *ToolRegistry) Get(name string) (Tool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	tool, exists := r.tools[name]
	return tool, exists
}

// List 返回所有已注册工具的定义列表。
func (r *ToolRegistry) List() []*types.ToolDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()

	defs := make([]*types.ToolDefinition, 0, len(r.tools))
	for _, tool := range r.tools {
		defs = append(defs, tool.Definition())
	}
	return defs
}

// Has 检查指定名称的工具是否已注册。
func (r *ToolRegistry) Has(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, exists := r.tools[name]
	return exists
}

// DefaultToolRegistry 全局默认工具注册表。
var DefaultToolRegistry = NewToolRegistry()

// RegisterTool 在默认工具注册表中注册工具。
func RegisterTool(tool Tool) error {
	return DefaultToolRegistry.Register(tool)
}

// GetTool 从默认工具注册表获取工具。
func GetTool(name string) (Tool, bool) {
	return DefaultToolRegistry.Get(name)
}

// ListTools 返回默认工具注册表中所有工具的定义。
func ListTools() []*types.ToolDefinition {
	return DefaultToolRegistry.List()
}

// HasTool 检查默认工具注册表中是否存在指定名称的工具。
func HasTool(name string) bool {
	return DefaultToolRegistry.Has(name)
}
