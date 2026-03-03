package handler

import (
	"strconv"

	"yqhp/common/response"
	"yqhp/gulu/internal/logic"

	"github.com/gofiber/fiber/v2"
)

// AiModelCreate 创建AI模型
func AiModelCreate(c *fiber.Ctx) error {
	var req logic.CreateAiModelReq
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}

	if req.Name == "" {
		return response.Error(c, "模型名称不能为空")
	}
	if req.ModelID == "" {
		return response.Error(c, "模型标识符不能为空")
	}
	// 新模式：通过 provider_id 关联供应商，不再要求直接传 api_key/api_base_url
	// 旧模式：兼容直接传 provider + api_base_url + api_key
	if req.ProviderID == 0 {
		if req.Provider == "" {
			return response.Error(c, "请选择供应商或填写厂商名称")
		}
		if req.APIBaseURL == "" {
			return response.Error(c, "API Base URL 不能为空")
		}
		if req.APIKey == "" {
			return response.Error(c, "API Key 不能为空")
		}
	}

	aiModelLogic := logic.NewAiModelLogic(c.UserContext())
	result, err := aiModelLogic.Create(&req)
	if err != nil {
		return response.Error(c, err.Error())
	}
	return response.Success(c, result)
}

// AiModelBatchCreate 批量创建模型（在供应商下）
func AiModelBatchCreate(c *fiber.Ctx) error {
	providerID, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "无效的供应商ID")
	}

	var req logic.BatchCreateModelReq
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}
	req.ProviderID = providerID

	if len(req.Models) == 0 {
		return response.Error(c, "模型列表不能为空")
	}
	for _, m := range req.Models {
		if m.Name == "" || m.ModelID == "" {
			return response.Error(c, "每个模型的名称和标识符不能为空")
		}
	}

	aiModelLogic := logic.NewAiModelLogic(c.UserContext())
	result, err := aiModelLogic.BatchCreate(&req)
	if err != nil {
		return response.Error(c, err.Error())
	}
	return response.Success(c, result)
}

// AiModelUpdate 更新AI模型
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
func AiModelGetProviders(c *fiber.Ctx) error {
	aiModelLogic := logic.NewAiModelLogic(c.UserContext())
	providers, err := aiModelLogic.GetProviders()
	if err != nil {
		return response.Error(c, err.Error())
	}
	return response.Success(c, providers)
}
