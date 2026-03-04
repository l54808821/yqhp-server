package executor

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"yqhp/workflow-engine/pkg/logger"
	"yqhp/workflow-engine/pkg/types"
)

// ToolRegistry 工具注册表，管理和查询已注册的工具。
type ToolRegistry struct {
	tools map[string]Tool
	mu    sync.RWMutex
}

func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{
		tools: make(map[string]Tool),
	}
}

// Register 注册工具，同名覆盖（与 picoclaw 一致，方便扩展）
func (r *ToolRegistry) Register(tool Tool) {
	if tool == nil {
		return
	}
	def := tool.Definition()
	if def == nil || def.Name == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tools[def.Name] = tool
}

func (r *ToolRegistry) Get(name string) (Tool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	tool, exists := r.tools[name]
	return tool, exists
}

func (r *ToolRegistry) Has(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, exists := r.tools[name]
	return exists
}

// SortedNames 返回排序后的工具名，保证 KV cache 稳定性
func (r *ToolRegistry) SortedNames() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// List 返回所有已注册工具的定义列表（按名称排序）
func (r *ToolRegistry) List() []*types.ToolDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		names = append(names, name)
	}
	sort.Strings(names)

	defs := make([]*types.ToolDefinition, 0, len(names))
	for _, name := range names {
		defs = append(defs, r.tools[name].Definition())
	}
	return defs
}

// Count 返回已注册工具数量
func (r *ToolRegistry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.tools)
}

// Clone 创建注册表的浅拷贝，用于多 Agent 场景下基于共享注册表扩展
func (r *ToolRegistry) Clone() *ToolRegistry {
	r.mu.RLock()
	defer r.mu.RUnlock()
	newReg := NewToolRegistry()
	for name, tool := range r.tools {
		newReg.tools[name] = tool
	}
	return newReg
}

// Execute 执行工具并返回结果
func (r *ToolRegistry) Execute(ctx context.Context, name string, arguments string, execCtx *ExecutionContext) (*types.ToolResult, error) {
	tool, ok := r.Get(name)
	if !ok {
		logger.Debug("[ToolRegistry] 未找到工具: %s", name)
		return types.NewErrorResult(fmt.Sprintf("未知工具: %s", name)), nil
	}
	logger.Debug("[ToolRegistry] 分发工具调用: %s", name)
	return tool.Execute(ctx, arguments, execCtx)
}

// DefaultToolRegistry 全局默认工具注册表
var DefaultToolRegistry = NewToolRegistry()

func RegisterTool(tool Tool) {
	DefaultToolRegistry.Register(tool)
}

func GetTool(name string) (Tool, bool) {
	return DefaultToolRegistry.Get(name)
}

func ListTools() []*types.ToolDefinition {
	return DefaultToolRegistry.List()
}

func HasTool(name string) bool {
	return DefaultToolRegistry.Has(name)
}
