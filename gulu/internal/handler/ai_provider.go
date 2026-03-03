package handler

import (
	"strconv"

	"yqhp/common/response"
	"yqhp/gulu/internal/logic"

	"github.com/gofiber/fiber/v2"
)

// AiProviderCreate 创建AI供应商
func AiProviderCreate(c *fiber.Ctx) error {
	var req logic.CreateAiProviderReq
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}
	if req.Name == "" {
		return response.Error(c, "供应商名称不能为空")
	}
	if req.ProviderType == "" {
		return response.Error(c, "供应商类型不能为空")
	}
	if req.APIBaseURL == "" {
		return response.Error(c, "API Base URL 不能为空")
	}

	l := logic.NewAiProviderLogic(c.UserContext())
	result, err := l.Create(&req)
	if err != nil {
		return response.Error(c, err.Error())
	}
	return response.Success(c, result)
}

// AiProviderUpdate 更新AI供应商
func AiProviderUpdate(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "无效的供应商ID")
	}
	var req logic.UpdateAiProviderReq
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}

	l := logic.NewAiProviderLogic(c.UserContext())
	if err := l.Update(id, &req); err != nil {
		return response.Error(c, err.Error())
	}
	return response.Success(c, nil)
}

// AiProviderDelete 删除AI供应商
func AiProviderDelete(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "无效的供应商ID")
	}

	l := logic.NewAiProviderLogic(c.UserContext())
	if err := l.Delete(id); err != nil {
		return response.Error(c, err.Error())
	}
	return response.Success(c, nil)
}

// AiProviderGetByID 获取AI供应商详情
func AiProviderGetByID(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "无效的供应商ID")
	}

	l := logic.NewAiProviderLogic(c.UserContext())
	result, err := l.GetByID(id)
	if err != nil {
		return response.NotFound(c, "AI供应商不存在")
	}
	return response.Success(c, result)
}

// AiProviderList 获取AI供应商列表
func AiProviderList(c *fiber.Ctx) error {
	var req logic.AiProviderListReq
	if err := c.QueryParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 50
	}

	l := logic.NewAiProviderLogic(c.UserContext())
	list, total, err := l.List(&req)
	if err != nil {
		return response.Error(c, err.Error())
	}
	return response.Page(c, list, total, req.Page, req.PageSize)
}

// AiProviderUpdateStatus 更新AI供应商状态
func AiProviderUpdateStatus(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "无效的供应商ID")
	}
	var req struct {
		Status int32 `json:"status"`
	}
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}

	l := logic.NewAiProviderLogic(c.UserContext())
	if err := l.UpdateStatus(id, req.Status); err != nil {
		return response.Error(c, err.Error())
	}
	return response.Success(c, nil)
}
