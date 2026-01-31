package handler

import (
	"strconv"

	"github.com/gofiber/fiber/v2"

	"yqhp/common/response"
	"yqhp/gulu/internal/logic"
)

// ConfigList 获取环境的配置列表
func ConfigList(c *fiber.Ctx) error {
	envID, err := strconv.ParseInt(c.Params("envId"), 10, 64)
	if err != nil {
		return response.Error(c, "环境ID无效")
	}

	configType := c.Query("type") // 可选参数，过滤配置类型

	configs, err := logic.GetConfigsByEnv(c.Context(), envID, configType)
	if err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, configs)
}

// ConfigUpdate 更新单个配置值
func ConfigUpdate(c *fiber.Ctx) error {
	envID, err := strconv.ParseInt(c.Params("envId"), 10, 64)
	if err != nil {
		return response.Error(c, "环境ID无效")
	}

	code := c.Params("code")
	if code == "" {
		return response.Error(c, "配置code无效")
	}

	var req logic.UpdateConfigValueReq
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "请求参数无效")
	}

	if err := logic.UpdateConfigValue(c.Context(), envID, code, &req); err != nil {
		if err == logic.ErrConfigDefinitionNotFound {
			return response.NotFound(c, err.Error())
		}
		return response.Error(c, err.Error())
	}

	return response.Success(c, nil)
}

// ConfigBatchUpdate 批量更新配置值
func ConfigBatchUpdate(c *fiber.Ctx) error {
	envID, err := strconv.ParseInt(c.Params("envId"), 10, 64)
	if err != nil {
		return response.Error(c, "环境ID无效")
	}

	var req logic.BatchUpdateConfigValuesReq
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "请求参数无效")
	}

	if err := logic.BatchUpdateConfigValues(c.Context(), envID, &req); err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, nil)
}
