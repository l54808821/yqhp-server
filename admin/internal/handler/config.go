package handler

import (
	"strconv"

	"yqhp/admin/internal/service"
	"yqhp/common/response"

	"github.com/gofiber/fiber/v2"
)

// ConfigHandler 配置处理器
type ConfigHandler struct {
	configService *service.ConfigService
}

// NewConfigHandler 创建配置处理器
func NewConfigHandler(configService *service.ConfigService) *ConfigHandler {
	return &ConfigHandler{configService: configService}
}

// List 获取配置列表
func (h *ConfigHandler) List(c *fiber.Ctx) error {
	var req service.ListConfigsRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}

	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 10
	}

	configs, total, err := h.configService.ListConfigs(&req)
	if err != nil {
		return response.Error(c, "获取失败")
	}

	return response.Page(c, configs, total, req.Page, req.PageSize)
}

// Get 获取配置详情
func (h *ConfigHandler) Get(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "参数错误")
	}

	config, err := h.configService.GetConfig(uint(id))
	if err != nil {
		return response.Error(c, "获取失败")
	}

	return response.Success(c, config)
}

// GetByKey 根据Key获取配置
func (h *ConfigHandler) GetByKey(c *fiber.Ctx) error {
	key := c.Params("key")
	if key == "" {
		return response.Error(c, "Key不能为空")
	}

	config, err := h.configService.GetConfigByKey(key)
	if err != nil {
		return response.Error(c, "获取失败")
	}

	return response.Success(c, config)
}

// Create 创建配置
func (h *ConfigHandler) Create(c *fiber.Ctx) error {
	var req service.CreateConfigRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}

	if req.Name == "" || req.Key == "" {
		return response.Error(c, "名称和Key不能为空")
	}

	config, err := h.configService.CreateConfig(&req)
	if err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, config)
}

// Update 更新配置
func (h *ConfigHandler) Update(c *fiber.Ctx) error {
	var req service.UpdateConfigRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}

	if req.ID == 0 {
		return response.Error(c, "ID不能为空")
	}

	if err := h.configService.UpdateConfig(&req); err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, nil)
}

// Delete 删除配置
func (h *ConfigHandler) Delete(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "参数错误")
	}

	if err := h.configService.DeleteConfig(uint(id)); err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, nil)
}

// Refresh 刷新配置缓存
func (h *ConfigHandler) Refresh(c *fiber.Ctx) error {
	if err := h.configService.RefreshConfig(); err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, nil)
}

