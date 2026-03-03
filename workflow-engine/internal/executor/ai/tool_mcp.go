package ai

import (
	"context"
	"fmt"

	"yqhp/workflow-engine/internal/executor"
	"yqhp/workflow-engine/pkg/types"
)

// MCPToolWrapper 将 MCP 远程工具包装为统一 Tool 接口
type MCPToolWrapper struct {
	def      *types.ToolDefinition
	client   *executor.MCPRemoteClient
	serverID int64
}

func NewMCPToolWrapper(def *types.ToolDefinition, client *executor.MCPRemoteClient, serverID int64) *MCPToolWrapper {
	return &MCPToolWrapper{
		def:      def,
		client:   client,
		serverID: serverID,
	}
}

func (t *MCPToolWrapper) Definition() *types.ToolDefinition {
	return t.def
}

func (t *MCPToolWrapper) Execute(ctx context.Context, arguments string, execCtx *executor.ExecutionContext) (*types.ToolResult, error) {
	result, err := t.client.CallTool(ctx, t.serverID, t.def.Name, arguments)
	if err != nil {
		return types.NewErrorResult(fmt.Sprintf("MCP 工具调用失败: %v", err)), nil
	}
	return result, nil
}
