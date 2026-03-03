package mcpserver

import (
	"context"
	"fmt"
	"log"

	"yqhp/gulu/internal/config"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

var sseServer *server.SSEServer

// Start 启动内置 MCP Server（SSE 传输）
func Start(cfg *config.MCPServerConfig) error {
	if !cfg.Enabled {
		log.Println("[MCPServer] 内置 MCP Server 未启用")
		return nil
	}

	name := cfg.Name
	if name == "" {
		name = "gulu-builtin-mcp"
	}

	s := server.NewMCPServer(name, "1.0.0",
		server.WithToolCapabilities(false),
	)

	registerTools(s)

	addr := fmt.Sprintf(":%d", cfg.Port)
	sseServer = server.NewSSEServer(s,
		server.WithBaseURL(fmt.Sprintf("http://127.0.0.1:%d", cfg.Port)),
	)

	go func() {
		log.Printf("[MCPServer] 内置 MCP Server 启动在 http://127.0.0.1:%d/sse", cfg.Port)
		if err := sseServer.Start(addr); err != nil {
			log.Printf("[MCPServer] MCP Server 启动失败: %v", err)
		}
	}()

	return nil
}

// Stop 停止内置 MCP Server
func Stop() {
	if sseServer != nil {
		log.Println("[MCPServer] 正在关闭内置 MCP Server...")
		if err := sseServer.Shutdown(context.Background()); err != nil {
			log.Printf("[MCPServer] 关闭 MCP Server 失败: %v", err)
		}
	}
}

// registerTools 注册所有内置工具
func registerTools(s *server.MCPServer) {
	s.AddTool(
		mcp.NewTool("list_workflows",
			mcp.WithDescription("查询工作流列表，可按名称和状态筛选"),
			mcp.WithNumber("project_id", mcp.Required(), mcp.Description("项目ID")),
			mcp.WithString("name", mcp.Description("按名称模糊搜索")),
			mcp.WithNumber("page", mcp.Description("页码，默认1")),
			mcp.WithNumber("page_size", mcp.Description("每页条数，默认10")),
		),
		handleListWorkflows,
	)

	s.AddTool(
		mcp.NewTool("get_workflow",
			mcp.WithDescription("根据ID获取工作流详情，包括名称、描述、版本、状态和定义等"),
			mcp.WithNumber("workflow_id", mcp.Required(), mcp.Description("工作流ID")),
		),
		handleGetWorkflow,
	)

	s.AddTool(
		mcp.NewTool("list_execution_records",
			mcp.WithDescription("查询执行记录列表，可按项目、工作流、状态筛选"),
			mcp.WithNumber("project_id", mcp.Description("项目ID")),
			mcp.WithNumber("source_id", mcp.Description("工作流ID（source_id）")),
			mcp.WithString("status", mcp.Description("执行状态: pending/running/completed/failed/stopped")),
			mcp.WithNumber("page", mcp.Description("页码，默认1")),
			mcp.WithNumber("page_size", mcp.Description("每页条数，默认10")),
		),
		handleListExecutionRecords,
	)

	s.AddTool(
		mcp.NewTool("list_projects",
			mcp.WithDescription("查询当前可用的项目列表"),
			mcp.WithNumber("page", mcp.Description("页码，默认1")),
			mcp.WithNumber("page_size", mcp.Description("每页条数，默认10")),
		),
		handleListProjects,
	)
}
