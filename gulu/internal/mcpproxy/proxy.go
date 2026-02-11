package mcpproxy

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"yqhp/workflow-engine/pkg/types"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
)

// MCPServerConnConfig MCP 服务器连接配置
type MCPServerConnConfig struct {
	Transport string            `json:"transport"` // "stdio" | "sse"
	Command   string            `json:"command,omitempty"`
	Args      []string          `json:"args,omitempty"`
	URL       string            `json:"url,omitempty"`
	Env       map[string]string `json:"env,omitempty"`
	Timeout   int               `json:"timeout,omitempty"` // 超时秒数
}

// ServerStatus MCP 服务器连接状态
type ServerStatus struct {
	ServerID  int64  `json:"server_id"`
	Connected bool   `json:"connected"`
	ToolCount int    `json:"tool_count"`
	Error     string `json:"error,omitempty"`
}

// mcpConnection 内部连接结构
type mcpConnection struct {
	client    *client.Client
	config    MCPServerConnConfig
	toolCount int
	err       string
}

// MCPProxyService MCP 代理服务
type MCPProxyService struct {
	connections map[int64]*mcpConnection
	mu          sync.RWMutex
}

// NewMCPProxyService 创建 MCP 代理服务实例
func NewMCPProxyService() *MCPProxyService {
	return &MCPProxyService{
		connections: make(map[int64]*mcpConnection),
	}
}

// ConnectServer 连接指定的 MCP 服务器
func (s *MCPProxyService) ConnectServer(ctx context.Context, serverID int64, config MCPServerConnConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 如果已有连接，先关闭
	if conn, ok := s.connections[serverID]; ok {
		_ = conn.client.Close()
		delete(s.connections, serverID)
	}

	c, err := s.createClient(config)
	if err != nil {
		return fmt.Errorf("创建 MCP 客户端失败: %w", err)
	}

	// 初始化 MCP 连接
	if err := s.initializeClient(ctx, c); err != nil {
		_ = c.Close()
		return fmt.Errorf("初始化 MCP 连接失败: %w", err)
	}

	s.connections[serverID] = &mcpConnection{
		client: c,
		config: config,
	}

	log.Printf("[MCPProxy] 已连接 MCP 服务器 %d (transport=%s)", serverID, config.Transport)
	return nil
}

// createClient 根据传输方式创建 MCP 客户端
func (s *MCPProxyService) createClient(config MCPServerConnConfig) (*client.Client, error) {
	switch config.Transport {
	case "stdio":
		if config.Command == "" {
			return nil, fmt.Errorf("stdio 传输方式需要指定 command")
		}
		env := envMapToSlice(config.Env)
		return client.NewStdioMCPClient(config.Command, env, config.Args...)
	case "sse":
		if config.URL == "" {
			return nil, fmt.Errorf("sse 传输方式需要指定 url")
		}
		c, err := client.NewSSEMCPClient(config.URL)
		if err != nil {
			return nil, err
		}
		// SSE 客户端需要手动启动
		if err := c.Start(context.Background()); err != nil {
			return nil, fmt.Errorf("启动 SSE 连接失败: %w", err)
		}
		return c, nil
	default:
		return nil, fmt.Errorf("不支持的传输方式: %s", config.Transport)
	}
}

// initializeClient 初始化 MCP 客户端连接
func (s *MCPProxyService) initializeClient(ctx context.Context, c *client.Client) error {
	initReq := mcp.InitializeRequest{}
	initReq.Params.ClientInfo = mcp.Implementation{
		Name:    "gulu-mcp-proxy",
		Version: "1.0.0",
	}
	initReq.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION

	_, err := c.Initialize(ctx, initReq)
	return err
}

// GetTools 获取指定 MCP 服务器的工具列表
func (s *MCPProxyService) GetTools(ctx context.Context, serverID int64) ([]*types.ToolDefinition, error) {
	conn, err := s.getConnection(serverID)
	if err != nil {
		return nil, err
	}

	toolsResult, err := conn.client.ListTools(ctx, mcp.ListToolsRequest{})
	if err != nil {
		return nil, fmt.Errorf("获取工具列表失败: %w", err)
	}

	defs := make([]*types.ToolDefinition, 0, len(toolsResult.Tools))
	for _, tool := range toolsResult.Tools {
		td, err := convertMCPTool(tool)
		if err != nil {
			log.Printf("[MCPProxy] 转换工具 %s 失败: %v", tool.Name, err)
			continue
		}
		defs = append(defs, td)
	}

	// 更新工具数量缓存
	s.mu.Lock()
	if c, ok := s.connections[serverID]; ok {
		c.toolCount = len(defs)
		c.err = ""
	}
	s.mu.Unlock()

	return defs, nil
}

// CallTool 调用指定 MCP 服务器的工具
func (s *MCPProxyService) CallTool(ctx context.Context, serverID int64, toolName, arguments string) (*types.ToolResult, error) {
	conn, err := s.getConnection(serverID)
	if err != nil {
		return &types.ToolResult{
			Content: fmt.Sprintf("MCP 服务器连接错误: %v", err),
			IsError: true,
		}, nil
	}

	// 解析参数 JSON
	var argsMap map[string]interface{}
	if arguments != "" {
		if err := json.Unmarshal([]byte(arguments), &argsMap); err != nil {
			return &types.ToolResult{
				Content: fmt.Sprintf("参数 JSON 解析失败: %v", err),
				IsError: true,
			}, nil
		}
	}

	// 构建调用请求
	callReq := mcp.CallToolRequest{}
	callReq.Params.Name = toolName
	callReq.Params.Arguments = argsMap

	// 应用超时
	if conn.config.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(conn.config.Timeout)*time.Second)
		defer cancel()
	}

	result, err := conn.client.CallTool(ctx, callReq)
	if err != nil {
		return &types.ToolResult{
			Content: fmt.Sprintf("工具调用失败: %v", err),
			IsError: true,
		}, nil
	}

	return convertCallToolResult(result), nil
}

// GetServerStatus 获取 MCP 服务器连接状态
func (s *MCPProxyService) GetServerStatus(serverID int64) *ServerStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()

	conn, ok := s.connections[serverID]
	if !ok {
		return &ServerStatus{
			ServerID:  serverID,
			Connected: false,
		}
	}

	return &ServerStatus{
		ServerID:  serverID,
		Connected: true,
		ToolCount: conn.toolCount,
		Error:     conn.err,
	}
}

// DisconnectServer 断开指定 MCP 服务器连接
func (s *MCPProxyService) DisconnectServer(serverID int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	conn, ok := s.connections[serverID]
	if !ok {
		return fmt.Errorf("MCP 服务器 %d 未连接", serverID)
	}

	err := conn.client.Close()
	delete(s.connections, serverID)

	log.Printf("[MCPProxy] 已断开 MCP 服务器 %d", serverID)
	return err
}

// Close 关闭所有连接
func (s *MCPProxyService) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var lastErr error
	for id, conn := range s.connections {
		if err := conn.client.Close(); err != nil {
			log.Printf("[MCPProxy] 关闭 MCP 服务器 %d 连接失败: %v", id, err)
			lastErr = err
		}
	}
	s.connections = make(map[int64]*mcpConnection)

	log.Printf("[MCPProxy] 已关闭所有 MCP 连接")
	return lastErr
}

// getConnection 获取指定服务器的连接（读锁）
func (s *MCPProxyService) getConnection(serverID int64) (*mcpConnection, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	conn, ok := s.connections[serverID]
	if !ok {
		return nil, fmt.Errorf("MCP 服务器 %d 未连接", serverID)
	}
	return conn, nil
}

// convertMCPTool 将 MCP Tool 转换为 ToolDefinition
func convertMCPTool(tool mcp.Tool) (*types.ToolDefinition, error) {
	// 将 InputSchema 序列化为 JSON
	schemaBytes, err := json.Marshal(tool.InputSchema)
	if err != nil {
		return nil, fmt.Errorf("序列化 InputSchema 失败: %w", err)
	}

	return &types.ToolDefinition{
		Name:        tool.Name,
		Description: tool.Description,
		Parameters:  schemaBytes,
	}, nil
}

// convertCallToolResult 将 MCP CallToolResult 转换为 ToolResult
func convertCallToolResult(result *mcp.CallToolResult) *types.ToolResult {
	// 提取文本内容
	var content string
	for _, c := range result.Content {
		if tc, ok := c.(mcp.TextContent); ok {
			if content != "" {
				content += "\n"
			}
			content += tc.Text
		}
	}

	// 如果没有文本内容，尝试 JSON 序列化整个 Content
	if content == "" && len(result.Content) > 0 {
		if data, err := json.Marshal(result.Content); err == nil {
			content = string(data)
		}
	}

	return &types.ToolResult{
		Content: content,
		IsError: result.IsError,
	}
}

// envMapToSlice 将环境变量 map 转换为 KEY=VALUE 切片
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
