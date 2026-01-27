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
	}
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}

	if req.Name == "" {
		return response.Error(c, "新环境名称不能为空")
	}

	userID := middleware.GetCurrentUserID(c)
	envLogic := logic.NewEnvLogic(c.UserContext())

	newEnv, err := envLogic.CopyEnv(id, req.Name, userID)
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

// ============================================
// 域名配置接口（存储在 t_env.domains_json）
// ============================================

// EnvGetDomains 获取环境的域名配置
// GET /api/envs/:id/domains
func EnvGetDomains(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "无效的环境ID")
	}

	envLogic := logic.NewEnvLogic(c.UserContext())

	domains, version, err := envLogic.GetDomains(id)
	if err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, fiber.Map{
		"version": version,
		"domains": domains,
	})
}

// EnvUpdateDomains 更新环境的域名配置
// PUT /api/envs/:id/domains
func EnvUpdateDomains(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "无效的环境ID")
	}

	var req logic.UpdateDomainsReq
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}

	userID := middleware.GetCurrentUserID(c)
	envLogic := logic.NewEnvLogic(c.UserContext())

	resp, err := envLogic.UpdateDomains(id, &req, userID)
	if err != nil {
		if err == logic.ErrVersionConflict {
			return response.ErrorWithCode(c, 409, err.Error())
		}
		return response.Error(c, err.Error())
	}

	return response.Success(c, resp)
}

// ============================================
// 变量配置接口（存储在 t_env.vars_json）
// ============================================

// EnvGetVars 获取环境的变量配置
// GET /api/envs/:id/vars
func EnvGetVars(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "无效的环境ID")
	}

	envLogic := logic.NewEnvLogic(c.UserContext())

	vars, version, err := envLogic.GetVars(id)
	if err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, fiber.Map{
		"version": version,
		"vars":    vars,
	})
}

// EnvUpdateVars 更新环境的变量配置
// PUT /api/envs/:id/vars
func EnvUpdateVars(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "无效的环境ID")
	}

	var req logic.UpdateVarsReq
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}

	userID := middleware.GetCurrentUserID(c)
	envLogic := logic.NewEnvLogic(c.UserContext())

	resp, err := envLogic.UpdateVars(id, &req, userID)
	if err != nil {
		if err == logic.ErrVersionConflict {
			return response.ErrorWithCode(c, 409, err.Error())
		}
		return response.Error(c, err.Error())
	}

	return response.Success(c, resp)
}

// EnvExportVars 导出环境变量
// GET /api/envs/:id/vars/export
func EnvExportVars(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "无效的环境ID")
	}

	envLogic := logic.NewEnvLogic(c.UserContext())

	items, err := envLogic.ExportVars(id)
	if err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, items)
}

// EnvImportVars 导入环境变量
// POST /api/envs/:id/vars/import
func EnvImportVars(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "无效的环境ID")
	}

	var req struct {
		Items []logic.VarExportItem `json:"items"`
	}
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}

	if len(req.Items) == 0 {
		return response.Error(c, "导入数据不能为空")
	}

	userID := middleware.GetCurrentUserID(c)
	envLogic := logic.NewEnvLogic(c.UserContext())

	if err := envLogic.ImportVars(id, req.Items, userID); err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, nil)
}
