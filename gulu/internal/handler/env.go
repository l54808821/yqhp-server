package handler

import (
	"strconv"

	"yqhp/common/response"
	"yqhp/gulu/internal/logic"
	"yqhp/gulu/internal/middleware"

	"github.com/gofiber/fiber/v2"
)

// EnvHandler 环境处理器
type EnvHandler struct{}

// NewEnvHandler 创建环境处理器
func NewEnvHandler() *EnvHandler {
	return &EnvHandler{}
}

// Create 创建环境
// POST /api/envs
func (h *EnvHandler) Create(c *fiber.Ctx) error {
	var req logic.CreateEnvReq
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}

	if req.ProjectID <= 0 {
		return response.Error(c, "项目ID不能为空")
	}
	if req.Name == "" {
		return response.Error(c, "环境名称不能为空")
	}
	if req.Code == "" {
		return response.Error(c, "环境代码不能为空")
	}

	userID := middleware.GetCurrentUserID(c)
	envLogic := logic.NewEnvLogic(c.UserContext())

	env, err := envLogic.Create(&req, userID)
	if err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, env)
}

// Update 更新环境
// PUT /api/envs/:id
func (h *EnvHandler) Update(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "无效的环境ID")
	}

	var req logic.UpdateEnvReq
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}

	userID := middleware.GetCurrentUserID(c)
	envLogic := logic.NewEnvLogic(c.UserContext())

	if err := envLogic.Update(id, &req, userID); err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, nil)
}

// Delete 删除环境
// DELETE /api/envs/:id
func (h *EnvHandler) Delete(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "无效的环境ID")
	}

	envLogic := logic.NewEnvLogic(c.UserContext())

	if err := envLogic.Delete(id); err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, nil)
}

// GetByID 获取环境详情
// GET /api/envs/:id
func (h *EnvHandler) GetByID(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "无效的环境ID")
	}

	envLogic := logic.NewEnvLogic(c.UserContext())

	env, err := envLogic.GetByID(id)
	if err != nil {
		return response.NotFound(c, "环境不存在")
	}

	return response.Success(c, env)
}

// List 获取环境列表
// GET /api/envs
func (h *EnvHandler) List(c *fiber.Ctx) error {
	var req logic.EnvListReq
	if err := c.QueryParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}

	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 10
	}

	envLogic := logic.NewEnvLogic(c.UserContext())

	list, total, err := envLogic.List(&req)
	if err != nil {
		return response.Error(c, err.Error())
	}

	return response.Page(c, list, total, req.Page, req.PageSize)
}

// Copy 复制环境
// POST /api/envs/:id/copy
func (h *EnvHandler) Copy(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "无效的环境ID")
	}

	var req struct {
		Name string `json:"name"`
		Code string `json:"code"`
	}
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}

	if req.Name == "" {
		return response.Error(c, "新环境名称不能为空")
	}
	if req.Code == "" {
		return response.Error(c, "新环境代码不能为空")
	}

	userID := middleware.GetCurrentUserID(c)
	envLogic := logic.NewEnvLogic(c.UserContext())

	newEnv, err := envLogic.CopyEnv(id, req.Name, req.Code, userID)
	if err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, newEnv)
}

// GetByProjectID 获取项目下所有环境
// GET /api/envs/project/:projectId
func (h *EnvHandler) GetByProjectID(c *fiber.Ctx) error {
	projectID, err := strconv.ParseInt(c.Params("projectId"), 10, 64)
	if err != nil {
		return response.Error(c, "无效的项目ID")
	}

	envLogic := logic.NewEnvLogic(c.UserContext())

	list, err := envLogic.GetEnvsByProjectID(projectID)
	if err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, list)
}
