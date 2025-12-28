package handler

import (
	"strconv"

	"yqhp/admin/internal/logic"
	"yqhp/admin/internal/types"
	"yqhp/common/response"

	"github.com/gofiber/fiber/v2"
)

// OAuthProviderList 获取OAuth提供商列表
func OAuthProviderList(c *fiber.Ctx) error {
	var req types.ListProvidersRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}

	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 10
	}

	providers, total, err := logic.NewOAuthLogic(c).ListProviders(&req)
	if err != nil {
		return response.Error(c, "获取失败")
	}

	return response.Page(c, providers, total, req.Page, req.PageSize)
}

// OAuthProviderGet 获取OAuth提供商详情
func OAuthProviderGet(c *fiber.Ctx) error {
	code := c.Params("code")
	if code == "" {
		return response.Error(c, "参数错误")
	}

	provider, err := logic.NewOAuthLogic(c).GetProvider(code)
	if err != nil {
		return response.Error(c, "获取失败")
	}

	return response.Success(c, types.ToOAuthProviderInfo(provider))
}

// OAuthProviderCreate 创建OAuth提供商
func OAuthProviderCreate(c *fiber.Ctx) error {
	var req types.CreateProviderRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}

	if req.Name == "" || req.Code == "" {
		return response.Error(c, "名称和编码不能为空")
	}

	provider, err := logic.NewOAuthLogic(c).CreateProvider(&req)
	if err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, provider)
}

// OAuthProviderUpdate 更新OAuth提供商
func OAuthProviderUpdate(c *fiber.Ctx) error {
	var req types.UpdateProviderRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}

	if req.ID == 0 {
		return response.Error(c, "ID不能为空")
	}

	if err := logic.NewOAuthLogic(c).UpdateProvider(&req); err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, nil)
}

// OAuthProviderDelete 删除OAuth提供商
func OAuthProviderDelete(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "参数错误")
	}

	if err := logic.NewOAuthLogic(c).DeleteProvider(id); err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, nil)
}
