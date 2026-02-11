package handler

import (
	"fmt"
	"strconv"

	"yqhp/common/response"
	"yqhp/gulu/internal/logic"
	"yqhp/gulu/internal/mcpproxy"

	"github.com/gofiber/fiber/v2"
)

// MCPProxyHandler MCP 代理服务 HTTP Handler
type MCPProxyHandler struct {
	proxyService *mcpproxy.MCPProxyService
}

// NewMCPProxyHandler 创建 MCP 代理 Handler
func NewMCPProxyHandler(proxyService *mcpproxy.MCPProxyService) *MCPProxyHandler {
	return &MCPProxyHandler{proxyService: proxyService}
}

// serverIDRequest 通用的 server_id 请求体
type serverIDRequest struct {
	ServerID int64 `json:"server_id"`
}

// callToolRequest 调用工具请求体
type callToolRequest struct {
	ServerID  int64  `json:"server_id"`
	ToolName  string `json:"tool_name"`
	Arguments string `json:"arguments"`
}

// GetTools 获取 MCP 服务器工具列表
// POST /api/mcp-proxy/tools
func (h *MCPProxyHandler) GetTools(c *fiber.Ctx) error {
	var req serverIDRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}
	if req.ServerID <= 0 {
		return response.Error(c, "无效的服务器ID")
	}

	ctx := c.UserContext()

	// 自动连接：如果服务器未连接，先查询配置并连接
	if err := h.ensureConnected(c, req.ServerID); err != nil {
		return response.Error(c, err.Error())
	}

	tools, err := h.proxyService.GetTools(ctx, req.ServerID)
	if err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, fiber.Map{"tools": tools})
}

// CallTool 调用 MCP 工具
// POST /api/mcp-proxy/call-tool
func (h *MCPProxyHandler) CallTool(c *fiber.Ctx) error {
	var req callToolRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}
	if req.ServerID <= 0 {
		return response.Error(c, "无效的服务器ID")
	}
	if req.ToolName == "" {
		return response.Error(c, "工具名称不能为空")
	}

	ctx := c.UserContext()

	// 自动连接
	if err := h.ensureConnected(c, req.ServerID); err != nil {
		return response.Error(c, err.Error())
	}

	result, err := h.proxyService.CallTool(ctx, req.ServerID, req.ToolName, req.Arguments)
	if err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, result)
}

// GetStatus 获取 MCP 服务器连接状态
// GET /api/mcp-proxy/status/:serverId
func (h *MCPProxyHandler) GetStatus(c *fiber.Ctx) error {
	serverID, err := strconv.ParseInt(c.Params("serverId"), 10, 64)
	if err != nil {
		return response.Error(c, "无效的服务器ID")
	}

	status := h.proxyService.GetServerStatus(serverID)
	return response.Success(c, status)
}

// Connect 连接 MCP 服务器
// POST /api/mcp-proxy/connect
func (h *MCPProxyHandler) Connect(c *fiber.Ctx) error {
	var req serverIDRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}
	if req.ServerID <= 0 {
		return response.Error(c, "无效的服务器ID")
	}

	if err := h.ensureConnected(c, req.ServerID); err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, nil)
}

// Disconnect 断开 MCP 服务器连接
// POST /api/mcp-proxy/disconnect
func (h *MCPProxyHandler) Disconnect(c *fiber.Ctx) error {
	var req serverIDRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}
	if req.ServerID <= 0 {
		return response.Error(c, "无效的服务器ID")
	}

	if err := h.proxyService.DisconnectServer(req.ServerID); err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, nil)
}

// ensureConnected 确保 MCP 服务器已连接，未连接时自动查询配置并连接
func (h *MCPProxyHandler) ensureConnected(c *fiber.Ctx, serverID int64) error {
	status := h.proxyService.GetServerStatus(serverID)
	if status.Connected {
		return nil
	}

	// 从数据库查询服务器配置
	mcpServerLogic := logic.NewMcpServerLogic(c.UserContext())
	serverInfo, err := mcpServerLogic.GetByID(serverID)
	if err != nil {
		return fmt.Errorf("MCP 服务器不存在: %v", err)
	}

	// 转换为连接配置
	config := toConnConfig(serverInfo)

	// 连接服务器
	if err := h.proxyService.ConnectServer(c.UserContext(), serverID, config); err != nil {
		return fmt.Errorf("连接 MCP 服务器失败: %v", err)
	}

	return nil
}

// toConnConfig 将 McpServerInfo 转换为 MCPServerConnConfig
func toConnConfig(info *logic.McpServerInfo) mcpproxy.MCPServerConnConfig {
	return mcpproxy.MCPServerConnConfig{
		Transport: info.Transport,
		Command:   info.Command,
		Args:      info.Args,
		URL:       info.URL,
		Env:       info.Env,
		Timeout:   int(info.Timeout),
	}
}
