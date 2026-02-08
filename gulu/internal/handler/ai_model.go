package handler

import (
	"strconv"

	"yqhp/common/response"
	"yqhp/gulu/internal/logic"

	"github.com/gofiber/fiber/v2"
)

// AiModelCreate 创建AI模型
// POST /api/ai-models
func AiModelCreate(c *fiber.Ctx) error {
	var req logic.CreateAiModelReq
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}

	if req.Name == "" {
		return response.Error(c, "模型名称不能为空")
	}
	if req.Provider == "" {
		return response.Error(c, "厂商不能为空")
	}
	if req.ModelID == "" {
		return response.Error(c, "模型标识符不能为空")
	}
	if req.APIBaseURL == "" {
		return response.Error(c, "API Base URL 不能为空")
	}
	if req.APIKey == "" {
		return response.Error(c, "API Key 不能为空")
	}

	aiModelLogic := logic.NewAiModelLogic(c.UserContext())

	result, err := aiModelLogic.Create(&req)
	if err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, result)
}

// AiModelUpdate 更新AI模型
// PUT /api/ai-models/:id
func AiModelUpdate(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "无效的模型ID")
	}

	var req logic.UpdateAiModelReq
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}

	aiModelLogic := logic.NewAiModelLogic(c.UserContext())

	if err := aiModelLogic.Update(id, &req); err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, nil)
}

// AiModelDelete 删除AI模型
// DELETE /api/ai-models/:id
func AiModelDelete(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "无效的模型ID")
	}

	aiModelLogic := logic.NewAiModelLogic(c.UserContext())

	if err := aiModelLogic.Delete(id); err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, nil)
}

// AiModelGetByID 获取AI模型详情
// GET /api/ai-models/:id
func AiModelGetByID(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "无效的模型ID")
	}

	aiModelLogic := logic.NewAiModelLogic(c.UserContext())

	result, err := aiModelLogic.GetByID(id)
	if err != nil {
		return response.NotFound(c, "AI模型不存在")
	}

	return response.Success(c, result)
}

// AiModelList 获取AI模型列表
// GET /api/ai-models
func AiModelList(c *fiber.Ctx) error {
	var req logic.AiModelListReq
	if err := c.QueryParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}

	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 20
	}

	aiModelLogic := logic.NewAiModelLogic(c.UserContext())

	list, total, err := aiModelLogic.List(&req)
	if err != nil {
		return response.Error(c, err.Error())
	}

	return response.Page(c, list, total, req.Page, req.PageSize)
}

// AiModelUpdateStatus 更新AI模型状态
// PUT /api/ai-models/:id/status
func AiModelUpdateStatus(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "无效的模型ID")
	}

	var req struct {
		Status int32 `json:"status"`
	}
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}

	aiModelLogic := logic.NewAiModelLogic(c.UserContext())

	if err := aiModelLogic.UpdateStatus(id, req.Status); err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, nil)
}

// AiModelGetProviders 获取厂商列表
// GET /api/ai-models/providers
func AiModelGetProviders(c *fiber.Ctx) error {
	aiModelLogic := logic.NewAiModelLogic(c.UserContext())

	providers, err := aiModelLogic.GetProviders()
	if err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, providers)
}
