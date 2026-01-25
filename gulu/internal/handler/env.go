package handler

import (
	"strconv"

	"yqhp/common/response"
	"yqhp/gulu/internal/logic"
	"yqhp/gulu/internal/middleware"

	"github.com/gofiber/fiber/v2"
)

// EnvCreate 创建环境
// POST /api/envs
func EnvCreate(c *fiber.Ctx) error {
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

// EnvUpdate 更新环境
// PUT /api/envs/:id
func EnvUpdate(c *fiber.Ctx) error {
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

// EnvDelete 删除环境
// DELETE /api/envs/:id
func EnvDelete(c *fiber.Ctx) error {
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

// EnvGetByID 获取环境详情
// GET /api/envs/:id
func EnvGetByID(c *fiber.Ctx) error {
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

// EnvList 获取环境列表
// GET /api/envs
func EnvList(c *fiber.Ctx) error {
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

// EnvCopy 复制环境
// POST /api/envs/:id/copy
func EnvCopy(c *fiber.Ctx) error {
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

// EnvGetByProjectID 获取项目下所有环境
// GET /api/envs/project/:projectId
func EnvGetByProjectID(c *fiber.Ctx) error {
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

// EnvUpdateSort 更新环境排序
// PUT /api/envs/sort
func EnvUpdateSort(c *fiber.Ctx) error {
	var req logic.EnvUpdateSortReq
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}

	if req.ID <= 0 {
		return response.Error(c, "环境ID不能为空")
	}
	if req.TargetID <= 0 {
		return response.Error(c, "目标环境ID不能为空")
	}
	if req.Position != "before" && req.Position != "after" {
		return response.Error(c, "position 必须是 before 或 after")
	}

	envLogic := logic.NewEnvLogic(c.UserContext())

	if err := envLogic.UpdateSort(&req); err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, nil)
}
