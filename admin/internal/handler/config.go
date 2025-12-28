package handler

import (
	"strconv"

	"yqhp/admin/internal/logic"
	"yqhp/admin/internal/types"
	"yqhp/common/response"

	"github.com/gofiber/fiber/v2"
)

// ConfigList 获取配置列表
func ConfigList(c *fiber.Ctx) error {
	var req types.ListConfigsRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}

	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 10
	}

	configs, total, err := logic.NewConfigLogic(c).ListConfigs(&req)
	if err != nil {
		return response.Error(c, "获取失败")
	}

	return response.Page(c, configs, total, req.Page, req.PageSize)
}

// ConfigGet 获取配置详情
func ConfigGet(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "参数错误")
	}

	config, err := logic.NewConfigLogic(c).GetConfig(id)
	if err != nil {
		return response.Error(c, "获取失败")
	}

	return response.Success(c, config)
}

// ConfigGetByKey 根据Key获取配置
func ConfigGetByKey(c *fiber.Ctx) error {
	key := c.Params("key")
	if key == "" {
		return response.Error(c, "Key不能为空")
	}

	config, err := logic.NewConfigLogic(c).GetConfigByKey(key)
	if err != nil {
		return response.Error(c, "获取失败")
	}

	return response.Success(c, config)
}

// ConfigCreate 创建配置
func ConfigCreate(c *fiber.Ctx) error {
	var req types.CreateConfigRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}

	if req.Name == "" || req.Key == "" {
		return response.Error(c, "名称和Key不能为空")
	}

	config, err := logic.NewConfigLogic(c).CreateConfig(&req)
	if err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, config)
}

// ConfigUpdate 更新配置
func ConfigUpdate(c *fiber.Ctx) error {
	var req types.UpdateConfigRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}

	if req.ID == 0 {
		return response.Error(c, "ID不能为空")
	}

	if err := logic.NewConfigLogic(c).UpdateConfig(&req); err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, nil)
}

// ConfigDelete 删除配置
func ConfigDelete(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "参数错误")
	}

	if err := logic.NewConfigLogic(c).DeleteConfig(id); err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, nil)
}

// ConfigRefresh 刷新配置缓存
func ConfigRefresh(c *fiber.Ctx) error {
	if err := logic.NewConfigLogic(c).RefreshConfig(); err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, nil)
}
