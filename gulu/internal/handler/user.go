package handler

import (
	"yqhp/common/response"
	"yqhp/gulu/internal/logic"
	"yqhp/gulu/internal/middleware"

	"github.com/gofiber/fiber/v2"
)

// UserHandler 用户处理器
type UserHandler struct {
	userLogic     *logic.UserLogic
	resourceLogic *logic.ResourceLogic
}

// NewUserHandler 创建用户处理器
func NewUserHandler() *UserHandler {
	return &UserHandler{
		userLogic:     logic.NewUserLogic(),
		resourceLogic: logic.NewResourceLogic(),
	}
}

// GetUserInfo 获取用户信息
// GET /api/user/info
func (h *UserHandler) GetUserInfo(c *fiber.Ctx) error {
	token := middleware.GetCurrentToken(c)
	if token == "" {
		return response.Unauthorized(c, "请先登录")
	}

	info, err := h.userLogic.GetUserInfo(token)
	if err != nil {
		return response.ServerError(c, "获取用户信息失败")
	}

	return response.Success(c, info)
}

// GetUserMenus 获取用户菜单
// GET /api/user/menus
func (h *UserHandler) GetUserMenus(c *fiber.Ctx) error {
	token := middleware.GetCurrentToken(c)
	if token == "" {
		return response.Unauthorized(c, "请先登录")
	}

	menus, err := h.resourceLogic.GetUserMenus(token)
	if err != nil {
		return response.ServerError(c, "获取用户菜单失败")
	}

	return response.Success(c, menus)
}

// GetUserCodes 获取用户权限码
// GET /api/user/codes
func (h *UserHandler) GetUserCodes(c *fiber.Ctx) error {
	token := middleware.GetCurrentToken(c)
	if token == "" {
		return response.Unauthorized(c, "请先登录")
	}

	codes, err := h.resourceLogic.GetUserCodes(token)
	if err != nil {
		return response.ServerError(c, "获取用户权限码失败")
	}

	return response.Success(c, codes)
}
