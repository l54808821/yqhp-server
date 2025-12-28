package handler

import (
	"strconv"

	"yqhp/admin/internal/service"
	"yqhp/common/response"

	"github.com/gofiber/fiber/v2"
)

// OAuthProviderHandler OAuth提供商处理器
type OAuthProviderHandler struct {
	oauthService *service.OAuthService
}

// NewOAuthProviderHandler 创建OAuth提供商处理器
func NewOAuthProviderHandler(oauthService *service.OAuthService) *OAuthProviderHandler {
	return &OAuthProviderHandler{oauthService: oauthService}
}

// List 获取OAuth提供商列表
func (h *OAuthProviderHandler) List(c *fiber.Ctx) error {
	providers, err := h.oauthService.ListProviders()
	if err != nil {
		return response.Error(c, "获取失败")
	}
	return response.Success(c, providers)
}

// Get 获取OAuth提供商详情
func (h *OAuthProviderHandler) Get(c *fiber.Ctx) error {
	code := c.Params("code")
	if code == "" {
		return response.Error(c, "参数错误")
	}

	provider, err := h.oauthService.GetProvider(code)
	if err != nil {
		return response.Error(c, "获取失败")
	}

	return response.Success(c, provider)
}

// Create 创建OAuth提供商
func (h *OAuthProviderHandler) Create(c *fiber.Ctx) error {
	var req service.CreateProviderRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}

	if req.Name == "" || req.Code == "" {
		return response.Error(c, "名称和编码不能为空")
	}

	provider, err := h.oauthService.CreateProvider(&req)
	if err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, provider)
}

// Update 更新OAuth提供商
func (h *OAuthProviderHandler) Update(c *fiber.Ctx) error {
	var req service.UpdateProviderRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}

	if req.ID == 0 {
		return response.Error(c, "ID不能为空")
	}

	if err := h.oauthService.UpdateProvider(&req); err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, nil)
}

// Delete 删除OAuth提供商
func (h *OAuthProviderHandler) Delete(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "参数错误")
	}

	if err := h.oauthService.DeleteProvider(uint(id)); err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, nil)
}

