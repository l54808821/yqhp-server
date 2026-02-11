package handler

import (
	"strconv"

	"yqhp/common/response"
	"yqhp/gulu/internal/logic"

	"github.com/gofiber/fiber/v2"
)

// McpServerCreate 创建MCP服务器配置
// POST /api/mcp-servers
func McpServerCreate(c *fiber.Ctx) error {
	var req logic.CreateMcpServerReq
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}

	if req.Name == "" {
		return response.Error(c, "服务器名称不能为空")
	}
	if req.Transport == "" {
		return response.Error(c, "传输方式不能为空")
	}

	mcpServerLogic := logic.NewMcpServerLogic(c.UserContext())

	result, err := mcpServerLogic.Create(&req)
	if err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, result)
}

// McpServerUpdate 更新MCP服务器配置
// PUT /api/mcp-servers/:id
func McpServerUpdate(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "无效的服务器ID")
	}

	var req logic.UpdateMcpServerReq
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}

	mcpServerLogic := logic.NewMcpServerLogic(c.UserContext())

	if err := mcpServerLogic.Update(id, &req); err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, nil)
}

// McpServerDelete 删除MCP服务器配置（软删除）
// DELETE /api/mcp-servers/:id
func McpServerDelete(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "无效的服务器ID")
	}

	mcpServerLogic := logic.NewMcpServerLogic(c.UserContext())

	if err := mcpServerLogic.Delete(id); err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, nil)
}

// McpServerGetByID 获取MCP服务器详情
// GET /api/mcp-servers/:id
func McpServerGetByID(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "无效的服务器ID")
	}

	mcpServerLogic := logic.NewMcpServerLogic(c.UserContext())

	result, err := mcpServerLogic.GetByID(id)
	if err != nil {
		return response.NotFound(c, "MCP服务器不存在")
	}

	return response.Success(c, result)
}

// McpServerList 获取MCP服务器列表
// GET /api/mcp-servers
func McpServerList(c *fiber.Ctx) error {
	var req logic.McpServerListReq
	if err := c.QueryParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}

	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 20
	}

	mcpServerLogic := logic.NewMcpServerLogic(c.UserContext())

	list, total, err := mcpServerLogic.List(&req)
	if err != nil {
		return response.Error(c, err.Error())
	}

	return response.Page(c, list, total, req.Page, req.PageSize)
}

// McpServerUpdateStatus 更新MCP服务器状态
// PUT /api/mcp-servers/:id/status
func McpServerUpdateStatus(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "无效的服务器ID")
	}

	var req struct {
		Status int32 `json:"status"`
	}
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}

	mcpServerLogic := logic.NewMcpServerLogic(c.UserContext())

	if err := mcpServerLogic.UpdateStatus(id, req.Status); err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, nil)
}
