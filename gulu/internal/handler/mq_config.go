package handler

import (
	"strconv"

	"yqhp/common/response"
	"yqhp/gulu/internal/logic"

	"github.com/gofiber/fiber/v2"
)

// MQConfigHandler MQ配置处理器
type MQConfigHandler struct{}

// NewMQConfigHandler 创建MQ配置处理器
func NewMQConfigHandler() *MQConfigHandler {
	return &MQConfigHandler{}
}

// Create 创建MQ配置
// POST /api/mq-configs
func (h *MQConfigHandler) Create(c *fiber.Ctx) error {
	var req logic.CreateMQConfigReq
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
		return response.Error(c, "MQ类型不能为空")
	}
	if req.Host == "" {
		return response.Error(c, "主机地址不能为空")
	}
	if req.Port <= 0 {
		return response.Error(c, "端口不能为空")
	}

	mqConfigLogic := logic.NewMQConfigLogic(c.UserContext())

	config, err := mqConfigLogic.Create(&req)
	if err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, config)
}

// Update 更新MQ配置
// PUT /api/mq-configs/:id
func (h *MQConfigHandler) Update(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "无效的配置ID")
	}

	var req logic.UpdateMQConfigReq
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}

	mqConfigLogic := logic.NewMQConfigLogic(c.UserContext())

	if err := mqConfigLogic.Update(id, &req); err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, nil)
}

// Delete 删除MQ配置
// DELETE /api/mq-configs/:id
func (h *MQConfigHandler) Delete(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "无效的配置ID")
	}

	mqConfigLogic := logic.NewMQConfigLogic(c.UserContext())

	if err := mqConfigLogic.Delete(id); err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, nil)
}

// GetByID 获取MQ配置详情
// GET /api/mq-configs/:id
func (h *MQConfigHandler) GetByID(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "无效的配置ID")
	}

	mqConfigLogic := logic.NewMQConfigLogic(c.UserContext())

	config, err := mqConfigLogic.GetByID(id)
	if err != nil {
		return response.NotFound(c, "配置不存在")
	}

	return response.Success(c, config)
}

// List 获取MQ配置列表
// GET /api/mq-configs
func (h *MQConfigHandler) List(c *fiber.Ctx) error {
	var req logic.MQConfigListReq
	if err := c.QueryParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}

	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 10
	}

	mqConfigLogic := logic.NewMQConfigLogic(c.UserContext())

	list, total, err := mqConfigLogic.List(&req)
	if err != nil {
		return response.Error(c, err.Error())
	}

	return response.Page(c, list, total, req.Page, req.PageSize)
}

// GetByEnvID 获取环境下所有MQ配置
// GET /api/mq-configs/env/:envId
func (h *MQConfigHandler) GetByEnvID(c *fiber.Ctx) error {
	envID, err := strconv.ParseInt(c.Params("envId"), 10, 64)
	if err != nil {
		return response.Error(c, "无效的环境ID")
	}

	mqConfigLogic := logic.NewMQConfigLogic(c.UserContext())

	list, err := mqConfigLogic.GetConfigsByEnvID(envID)
	if err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, list)
}
