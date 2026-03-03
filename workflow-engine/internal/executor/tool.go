package executor

import (
	"context"

	"yqhp/workflow-engine/pkg/types"
)

// Tool 工具执行接口
type Tool interface {
	Definition() *types.ToolDefinition
	Execute(ctx context.Context, arguments string, execCtx *ExecutionContext) (*types.ToolResult, error)
}

// ContextualTool 可选接口：工具需要感知当前会话上下文（如 stepID、callback）
type ContextualTool interface {
	Tool
	SetContext(stepID string, callback types.AICallback)
}

// AsyncTool 可选接口：异步工具完成后通过回调通知
type AsyncTool interface {
	Tool
	SetAsyncCallback(cb func(ctx context.Context, result *types.ToolResult))
}
