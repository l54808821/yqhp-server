package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"yqhp/workflow-engine/internal/executor"
	"yqhp/workflow-engine/pkg/logger"
	"yqhp/workflow-engine/pkg/types"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"
)

// MCPToolWrapper 将 MCP 工具包装为 executor.Tool，通过 mcp-go client 直连 MCP Server
type MCPToolWrapper struct {
	def        *types.ToolDefinition
	mcpClient  *client.Client
	toolName   string
	serverName string
	timeout    int
}

func (t *MCPToolWrapper) Definition() *types.ToolDefinition {
	return t.def
}

func (t *MCPToolWrapper) Execute(ctx context.Context, arguments string, execCtx *executor.ExecutionContext) (*types.ToolResult, error) {
	var argsMap map[string]interface{}
	if arguments != "" {
		if err := json.Unmarshal([]byte(arguments), &argsMap); err != nil {
			return types.NewErrorResult(fmt.Sprintf("参数解析失败: %v", err)), nil
		}
	}

	callReq := mcp.CallToolRequest{}
	callReq.Params.Name = t.toolName
	callReq.Params.Arguments = argsMap

	if t.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(t.timeout)*time.Second)
		defer cancel()
	}

	result, err := t.mcpClient.CallTool(ctx, callReq)
	if err != nil {
		return types.NewErrorResult(fmt.Sprintf("MCP 工具调用失败 [%s/%s]: %v", t.serverName, t.toolName, err)), nil
	}

	content := extractTextContent(result)
	return &types.ToolResult{
		Content: content,
		IsError: result.IsError,
	}, nil
}

func extractTextContent(result *mcp.CallToolResult) string {
	var content string
	for _, c := range result.Content {
		if tc, ok := c.(mcp.TextContent); ok {
			if content != "" {
				content += "\n"
			}
			content += tc.Text
		}
	}
	if content == "" && len(result.Content) > 0 {
		if data, err := json.Marshal(result.Content); err == nil {
			content = string(data)
		}
	}
	return content
}

// connectMCPServer 根据配置连接 MCP Server 并返回 client
func connectMCPServer(ctx context.Context, cfg *MCPServerConfig) (*client.Client, error) {
	var c *client.Client
	var err error

	switch cfg.Transport {
	case "stdio":
		if cfg.Command == "" {
			return nil, fmt.Errorf("MCP Server %q: stdio 传输需要 command", cfg.Name)
		}
		env := envMapToSlice(cfg.Env)
		c, err = client.NewStdioMCPClient(cfg.Command, env, cfg.Args...)
		if err != nil {
			return nil, fmt.Errorf("MCP Server %q: 创建 stdio client 失败: %w", cfg.Name, err)
		}
	case "sse":
		if cfg.URL == "" {
			return nil, fmt.Errorf("MCP Server %q: sse 传输需要 url", cfg.Name)
		}
		var opts []transport.ClientOption
		if len(cfg.Headers) > 0 {
			opts = append(opts, transport.WithHeaders(cfg.Headers))
		}
		c, err = client.NewSSEMCPClient(cfg.URL, opts...)
		if err != nil {
			return nil, fmt.Errorf("MCP Server %q: 创建 SSE client 失败: %w", cfg.Name, err)
		}
		if err := c.Start(context.Background()); err != nil {
			return nil, fmt.Errorf("MCP Server %q: 启动 SSE 连接失败: %w", cfg.Name, err)
		}
	case "streamable-http":
		if cfg.URL == "" {
			return nil, fmt.Errorf("MCP Server %q: streamable-http 传输需要 url", cfg.Name)
		}
		var opts []transport.StreamableHTTPCOption
		if len(cfg.Headers) > 0 {
			opts = append(opts, transport.WithHTTPHeaders(cfg.Headers))
		}
		c, err = client.NewStreamableHttpClient(cfg.URL, opts...)
		if err != nil {
			return nil, fmt.Errorf("MCP Server %q: 创建 streamable-http client 失败: %w", cfg.Name, err)
		}
	default:
		return nil, fmt.Errorf("MCP Server %q: 不支持的传输方式 %q", cfg.Name, cfg.Transport)
	}

	initReq := mcp.InitializeRequest{}
	initReq.Params.ClientInfo = mcp.Implementation{
		Name:    "yqhp-workflow-engine",
		Version: "1.0.0",
	}
	initReq.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION

	if _, err := c.Initialize(ctx, initReq); err != nil {
		_ = c.Close()
		return nil, fmt.Errorf("MCP Server %q: 初始化失败: %w", cfg.Name, err)
	}

	return c, nil
}

// loadMCPTools 连接 MCP Server 并加载所有工具，返回 executor.Tool 列表和 client（需要在执行完成后关闭）
func loadMCPTools(ctx context.Context, cfg *MCPServerConfig) ([]executor.Tool, *client.Client, error) {
	c, err := connectMCPServer(ctx, cfg)
	if err != nil {
		return nil, nil, err
	}

	toolsResult, err := c.ListTools(ctx, mcp.ListToolsRequest{})
	if err != nil {
		_ = c.Close()
		return nil, nil, fmt.Errorf("MCP Server %q: 获取工具列表失败: %w", cfg.Name, err)
	}

	tools := make([]executor.Tool, 0, len(toolsResult.Tools))
	for _, tool := range toolsResult.Tools {
		schemaBytes, err := json.Marshal(tool.InputSchema)
		if err != nil {
			logger.Warn("[MCPTool] MCP Server %q: 工具 %q schema 序列化失败: %v", cfg.Name, tool.Name, err)
			continue
		}

		wrapper := &MCPToolWrapper{
			def: &types.ToolDefinition{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  schemaBytes,
			},
			mcpClient:  c,
			toolName:   tool.Name,
			serverName: cfg.Name,
			timeout:    cfg.Timeout,
		}
		tools = append(tools, wrapper)
	}

	logger.Debug("[MCPTool] MCP Server %q (%s): 加载了 %d 个工具", cfg.Name, cfg.Transport, len(tools))
	return tools, c, nil
}

func envMapToSlice(env map[string]string) []string {
	if len(env) == 0 {
		return nil
	}
	result := make([]string, 0, len(env))
	for k, v := range env {
		result = append(result, k+"="+v)
	}
	return result
}
