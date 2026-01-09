package handler

import (
	"strconv"

	"yqhp/common/response"
	"yqhp/gulu/internal/logic"

	"github.com/gofiber/fiber/v2"
)

// ExecutorHandler 执行机处理器
type ExecutorHandler struct{}

// NewExecutorHandler 创建执行机处理器
func NewExecutorHandler() *ExecutorHandler {
	return &ExecutorHandler{}
}

// Create 创建执行机
// POST /api/executors
func (h *ExecutorHandler) Create(c *fiber.Ctx) error {
	var req logic.CreateExecutorReq
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}

	if req.SlaveID == "" {
		return response.Error(c, "SlaveID 不能为空")
	}
	if req.Name == "" {
		return response.Error(c, "执行机名称不能为空")
	}
	if req.Type == "" {
		return response.Error(c, "执行机类型不能为空")
	}

	executorLogic := logic.NewExecutorLogic(c.UserContext())

	executor, err := executorLogic.Create(&req)
	if err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, executor)
}

// Update 更新执行机
// PUT /api/executors/:id
func (h *ExecutorHandler) Update(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "无效的执行机ID")
	}

	var req logic.UpdateExecutorReq
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}

	executorLogic := logic.NewExecutorLogic(c.UserContext())

	if err := executorLogic.Update(id, &req); err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, nil)
}

// Delete 删除执行机
// DELETE /api/executors/:id
func (h *ExecutorHandler) Delete(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "无效的执行机ID")
	}

	executorLogic := logic.NewExecutorLogic(c.UserContext())

	if err := executorLogic.Delete(id); err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, nil)
}

// GetByID 获取执行机详情
// GET /api/executors/:id
func (h *ExecutorHandler) GetByID(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "无效的执行机ID")
	}

	executorLogic := logic.NewExecutorLogic(c.UserContext())

	executor, err := executorLogic.GetByID(id)
	if err != nil {
		return response.NotFound(c, "执行机不存在")
	}

	return response.Success(c, executor)
}

// List 获取执行机列表
// GET /api/executors
func (h *ExecutorHandler) List(c *fiber.Ctx) error {
	var req logic.ExecutorListReq
	if err := c.QueryParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}

	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 10
	}

	executorLogic := logic.NewExecutorLogic(c.UserContext())

	list, total, err := executorLogic.List(&req)
	if err != nil {
		return response.Error(c, err.Error())
	}

	return response.Page(c, list, total, req.Page, req.PageSize)
}

// UpdateStatus 更新执行机状态
// PUT /api/executors/:id/status
func (h *ExecutorHandler) UpdateStatus(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "无效的执行机ID")
	}

	var req struct {
		Status int32 `json:"status"`
	}
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}

	executorLogic := logic.NewExecutorLogic(c.UserContext())

	if err := executorLogic.UpdateStatus(id, req.Status); err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, nil)
}

// Sync 同步 workflow-engine 的执行机列表
// POST /api/executors/sync
func (h *ExecutorHandler) Sync(c *fiber.Ctx) error {
	executorLogic := logic.NewExecutorLogic(c.UserContext())

	count, err := executorLogic.Sync()
	if err != nil {
		return response.Error(c, "同步失败: "+err.Error())
	}

	return response.Success(c, fiber.Map{
		"synced_count": count,
		"message":      "同步完成",
	})
}

// ListByLabels 根据标签筛选执行机
// GET /api/executors/by-labels
func (h *ExecutorHandler) ListByLabels(c *fiber.Ctx) error {
	// 从查询参数解析标签
	labels := make(map[string]string)
	c.Request().URI().QueryArgs().VisitAll(func(key, value []byte) {
		k := string(key)
		if k != "page" && k != "pageSize" && k != "name" && k != "type" && k != "status" {
			labels[k] = string(value)
		}
	})

	executorLogic := logic.NewExecutorLogic(c.UserContext())

	list, err := executorLogic.ListByLabels(labels)
	if err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, list)
}
