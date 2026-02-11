package executor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"yqhp/workflow-engine/pkg/types"
)

// MCPRemoteClient 通过 HTTP 调用 MCP Proxy Service 的客户端
type MCPRemoteClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewMCPRemoteClient 创建 MCP 远程客户端
func NewMCPRemoteClient(baseURL string) *MCPRemoteClient {
	return &MCPRemoteClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// proxyResponse 代理服务的标准响应包装
type proxyResponse struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

// getToolsData GetTools 响应的 data 部分
type getToolsData struct {
	Tools []*types.ToolDefinition `json:"tools"`
}

// GetTools 获取指定 MCP 服务器的工具列表
func (c *MCPRemoteClient) GetTools(ctx context.Context, serverID int64) ([]*types.ToolDefinition, error) {
	reqBody, err := json.Marshal(map[string]int64{"server_id": serverID})
	if err != nil {
		return nil, fmt.Errorf("序列化请求失败: %w", err)
	}

	data, err := c.doRequest(ctx, "/api/mcp-proxy/tools", reqBody)
	if err != nil {
		return nil, fmt.Errorf("获取 MCP 工具列表失败: %w", err)
	}

	var toolsData getToolsData
	if err := json.Unmarshal(data, &toolsData); err != nil {
		return nil, fmt.Errorf("解析工具列表响应失败: %w", err)
	}

	return toolsData.Tools, nil
}

// CallTool 调用指定 MCP 服务器的工具
func (c *MCPRemoteClient) CallTool(ctx context.Context, serverID int64, toolName, arguments string) (*types.ToolResult, error) {
	reqBody, err := json.Marshal(map[string]interface{}{
		"server_id": serverID,
		"tool_name": toolName,
		"arguments": arguments,
	})
	if err != nil {
		return nil, fmt.Errorf("序列化请求失败: %w", err)
	}

	data, err := c.doRequest(ctx, "/api/mcp-proxy/call-tool", reqBody)
	if err != nil {
		return &types.ToolResult{
			Content: fmt.Sprintf("MCP 工具调用失败: %v", err),
			IsError: true,
		}, nil
	}

	var result types.ToolResult
	if err := json.Unmarshal(data, &result); err != nil {
		return &types.ToolResult{
			Content: fmt.Sprintf("解析工具调用响应失败: %v", err),
			IsError: true,
		}, nil
	}

	return &result, nil
}

// doRequest 发送 POST 请求到 MCP Proxy Service 并解析标准响应
func (c *MCPRemoteClient) doRequest(ctx context.Context, path string, body []byte) (json.RawMessage, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP 请求失败: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP 状态码 %d: %s", resp.StatusCode, string(respBody))
	}

	var proxyResp proxyResponse
	if err := json.Unmarshal(respBody, &proxyResp); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	if proxyResp.Code != 0 {
		return nil, fmt.Errorf("MCP Proxy 错误 (code=%d): %s", proxyResp.Code, proxyResp.Message)
	}

	return proxyResp.Data, nil
}
