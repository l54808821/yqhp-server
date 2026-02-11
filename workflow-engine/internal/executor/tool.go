package executor

import (
	"context"

	"yqhp/workflow-engine/pkg/types"
)

// Tool 工具执行接口
type Tool interface {
	// Definition 返回工具的定义信息
	Definition() *types.ToolDefinition

	// Execute 执行工具调用
	Execute(ctx context.Context, arguments string, execCtx *ExecutionContext) (*types.ToolResult, error)
}
