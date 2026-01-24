package handler

import (
	"strconv"

	"yqhp/common/response"
	"yqhp/gulu/internal/logic"

	"github.com/gofiber/fiber/v2"
)

// DatabaseConfigCreate 创建数据库配置
// POST /api/database-configs
func DatabaseConfigCreate(c *fiber.Ctx) error {
	var req logic.CreateDatabaseConfigReq
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
		return response.Error(c, "配置名称不能为空")
	}
	if req.Code == "" {
		return response.Error(c, "配置代码不能为空")
	}
	if req.Type == "" {
		return response.Error(c, "数据库类型不能为空")
	}
	if req.Host == "" {
		return response.Error(c, "主机地址不能为空")
	}
	if req.Port <= 0 {
		return response.Error(c, "端口不能为空")
	}

	dbConfigLogic := logic.NewDatabaseConfigLogic(c.UserContext())

	config, err := dbConfigLogic.Create(&req)
	if err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, config)
}

// DatabaseConfigUpdate 更新数据库配置
// PUT /api/database-configs/:id
func DatabaseConfigUpdate(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "无效的配置ID")
	}

	var req logic.UpdateDatabaseConfigReq
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}

	dbConfigLogic := logic.NewDatabaseConfigLogic(c.UserContext())

	if err := dbConfigLogic.Update(id, &req); err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, nil)
}

// DatabaseConfigDelete 删除数据库配置
// DELETE /api/database-configs/:id
func DatabaseConfigDelete(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "无效的配置ID")
	}

	dbConfigLogic := logic.NewDatabaseConfigLogic(c.UserContext())

	if err := dbConfigLogic.Delete(id); err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, nil)
}

// DatabaseConfigGetByID 获取数据库配置详情
// GET /api/database-configs/:id
func DatabaseConfigGetByID(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "无效的配置ID")
	}

	dbConfigLogic := logic.NewDatabaseConfigLogic(c.UserContext())

	config, err := dbConfigLogic.GetByID(id)
	if err != nil {
		return response.NotFound(c, "配置不存在")
	}

	return response.Success(c, config)
}

// DatabaseConfigList 获取数据库配置列表
// GET /api/database-configs
func DatabaseConfigList(c *fiber.Ctx) error {
	var req logic.DatabaseConfigListReq
	if err := c.QueryParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}

	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 10
	}

	dbConfigLogic := logic.NewDatabaseConfigLogic(c.UserContext())

	list, total, err := dbConfigLogic.List(&req)
	if err != nil {
		return response.Error(c, err.Error())
	}

	return response.Page(c, list, total, req.Page, req.PageSize)
}

// DatabaseConfigGetByEnvID 获取环境下所有数据库配置
// GET /api/database-configs/env/:envId
func DatabaseConfigGetByEnvID(c *fiber.Ctx) error {
	envID, err := strconv.ParseInt(c.Params("envId"), 10, 64)
	if err != nil {
		return response.Error(c, "无效的环境ID")
	}

	dbConfigLogic := logic.NewDatabaseConfigLogic(c.UserContext())

	list, err := dbConfigLogic.GetConfigsByEnvID(envID)
	if err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, list)
}
