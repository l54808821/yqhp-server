package handler

import (
	"strconv"

	"yqhp/common/response"
	"yqhp/gulu/internal/logic"

	"github.com/gofiber/fiber/v2"
)

// VarCreate 创建变量
// POST /api/vars
func VarCreate(c *fiber.Ctx) error {
	var req logic.CreateVarReq
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}

	if req.ProjectID <= 0 {
		return response.Error(c, "项目ID不能为空")
	}
	if req.EnvID <= 0 {
		return response.Error(c, "环境ID不能为空")
	}
	if req.Name == "" {
		return response.Error(c, "变量名称不能为空")
	}
	if req.Key == "" {
		return response.Error(c, "变量Key不能为空")
	}

	varLogic := logic.NewVarLogic(c.UserContext())

	variable, err := varLogic.Create(&req)
	if err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, variable)
}

// VarUpdate 更新变量
// PUT /api/vars/:id
func VarUpdate(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "无效的变量ID")
	}

	var req logic.UpdateVarReq
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}

	varLogic := logic.NewVarLogic(c.UserContext())

	if err := varLogic.Update(id, &req); err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, nil)
}

// VarDelete 删除变量
// DELETE /api/vars/:id
func VarDelete(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "无效的变量ID")
	}

	varLogic := logic.NewVarLogic(c.UserContext())

	if err := varLogic.Delete(id); err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, nil)
}

// VarGetByID 获取变量详情
// GET /api/vars/:id
func VarGetByID(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "无效的变量ID")
	}

	varLogic := logic.NewVarLogic(c.UserContext())

	variable, err := varLogic.GetByID(id)
	if err != nil {
		return response.NotFound(c, "变量不存在")
	}

	return response.Success(c, variable)
}

// VarList 获取变量列表
// GET /api/vars
func VarList(c *fiber.Ctx) error {
	var req logic.VarListReq
	if err := c.QueryParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}

	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 10
	}

	varLogic := logic.NewVarLogic(c.UserContext())

	list, total, err := varLogic.List(&req)
	if err != nil {
		return response.Error(c, err.Error())
	}

	return response.Page(c, list, total, req.Page, req.PageSize)
}

// VarExport 导出环境变量
// GET /api/vars/export
func VarExport(c *fiber.Ctx) error {
	envID, err := strconv.ParseInt(c.Query("env_id"), 10, 64)
	if err != nil || envID <= 0 {
		return response.Error(c, "环境ID不能为空")
	}

	varLogic := logic.NewVarLogic(c.UserContext())

	items, err := varLogic.Export(envID)
	if err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, items)
}

// VarImport 导入环境变量
// POST /api/vars/import
func VarImport(c *fiber.Ctx) error {
	var req struct {
		ProjectID int64                 `json:"project_id"`
		EnvID     int64                 `json:"env_id"`
		Items     []logic.VarExportItem `json:"items"`
	}
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}

	if req.ProjectID <= 0 {
		return response.Error(c, "项目ID不能为空")
	}
	if req.EnvID <= 0 {
		return response.Error(c, "环境ID不能为空")
	}
	if len(req.Items) == 0 {
		return response.Error(c, "导入数据不能为空")
	}

	varLogic := logic.NewVarLogic(c.UserContext())

	if err := varLogic.Import(req.ProjectID, req.EnvID, req.Items); err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, nil)
}

// VarGetByEnvID 获取环境下所有变量
// GET /api/vars/env/:envId
func VarGetByEnvID(c *fiber.Ctx) error {
	envID, err := strconv.ParseInt(c.Params("envId"), 10, 64)
	if err != nil {
		return response.Error(c, "无效的环境ID")
	}

	varLogic := logic.NewVarLogic(c.UserContext())

	list, err := varLogic.GetVarsByEnvID(envID)
	if err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, list)
}
