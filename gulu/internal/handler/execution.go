package handler

import (
	"strconv"

	"yqhp/common/response"
	"yqhp/gulu/internal/logic"
	"yqhp/gulu/internal/middleware"

	"github.com/gofiber/fiber/v2"
)

// ExecutionHandler 执行处理器
type ExecutionHandler struct{}

// NewExecutionHandler 创建执行处理器
func NewExecutionHandler() *ExecutionHandler {
	return &ExecutionHandler{}
}

// Execute 执行工作流
// POST /api/executions
func (h *ExecutionHandler) Execute(c *fiber.Ctx) error {
	var req logic.ExecuteWorkflowReq
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}

	if req.WorkflowID <= 0 {
		return response.Error(c, "工作流ID不能为空")
	}
	if req.EnvID <= 0 {
		return response.Error(c, "环境ID不能为空")
	}

	userID := middleware.GetCurrentUserID(c)
	executionLogic := logic.NewExecutionLogic(c.UserContext())

	execution, err := executionLogic.Execute(&req, userID)
	if err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, execution)
}

// GetByID 获取执行详情
// GET /api/executions/:id
func (h *ExecutionHandler) GetByID(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "无效的执行ID")
	}

	executionLogic := logic.NewExecutionLogic(c.UserContext())

	execution, err := executionLogic.GetByID(id)
	if err != nil {
		return response.NotFound(c, "执行记录不存在")
	}

	return response.Success(c, execution)
}

// List 获取执行记录列表
// GET /api/executions
func (h *ExecutionHandler) List(c *fiber.Ctx) error {
	var req logic.ExecutionListReq
	if err := c.QueryParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}

	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 10
	}

	executionLogic := logic.NewExecutionLogic(c.UserContext())

	list, total, err := executionLogic.List(&req)
	if err != nil {
		return response.Error(c, err.Error())
	}

	return response.Page(c, list, total, req.Page, req.PageSize)
}

// GetLogs 获取执行日志
// GET /api/executions/:id/logs
func (h *ExecutionHandler) GetLogs(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "无效的执行ID")
	}

	executionLogic := logic.NewExecutionLogic(c.UserContext())

	logs, err := executionLogic.GetLogs(id)
	if err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, fiber.Map{
		"logs": logs,
	})
}

// Stop 停止执行
// DELETE /api/executions/:id
func (h *ExecutionHandler) Stop(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "无效的执行ID")
	}

	executionLogic := logic.NewExecutionLogic(c.UserContext())

	if err := executionLogic.Stop(id); err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, nil)
}

// Pause 暂停执行
// POST /api/executions/:id/pause
func (h *ExecutionHandler) Pause(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "无效的执行ID")
	}

	executionLogic := logic.NewExecutionLogic(c.UserContext())

	if err := executionLogic.Pause(id); err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, nil)
}

// Resume 恢复执行
// POST /api/executions/:id/resume
func (h *ExecutionHandler) Resume(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "无效的执行ID")
	}

	executionLogic := logic.NewExecutionLogic(c.UserContext())

	if err := executionLogic.Resume(id); err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, nil)
}

// Webhook 执行状态回调（供 workflow-engine 调用）
// POST /api/executions/webhook
func (h *ExecutionHandler) Webhook(c *fiber.Ctx) error {
	var req struct {
		ExecutionID string `json:"execution_id"`
		Status      string `json:"status"`
		Result      string `json:"result"`
		Logs        string `json:"logs"`
	}
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}

	if req.ExecutionID == "" {
		return response.Error(c, "执行ID不能为空")
	}

	executionLogic := logic.NewExecutionLogic(c.UserContext())

	if err := executionLogic.UpdateStatus(req.ExecutionID, req.Status, req.Result, req.Logs); err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, nil)
}
