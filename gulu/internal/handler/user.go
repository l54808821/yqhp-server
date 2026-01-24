package handler

import (
	"yqhp/common/response"
	"yqhp/gulu/internal/logic"
	"yqhp/gulu/internal/middleware"

	"github.com/gofiber/fiber/v2"
)

// UserGetInfo 获取用户信息
// GET /api/user/info
func UserGetInfo(c *fiber.Ctx) error {
	token := middleware.GetCurrentToken(c)
	if token == "" {
		return response.Unauthorized(c, "请先登录")
	}

	info, err := logic.NewUserLogic().GetUserInfo(token)
	if err != nil {
		return response.ServerError(c, "获取用户信息失败")
	}

	return response.Success(c, info)
}

// UserGetMenus 获取用户菜单
// GET /api/user/menus
func UserGetMenus(c *fiber.Ctx) error {
	token := middleware.GetCurrentToken(c)
	if token == "" {
		return response.Unauthorized(c, "请先登录")
	}

	menus, err := logic.NewResourceLogic().GetUserMenus(token)
	if err != nil {
		return response.ServerError(c, "获取用户菜单失败")
	}

	return response.Success(c, menus)
}

// UserGetCodes 获取用户权限码
// GET /api/user/codes
func UserGetCodes(c *fiber.Ctx) error {
	token := middleware.GetCurrentToken(c)
	if token == "" {
		return response.Unauthorized(c, "请先登录")
	}

	codes, err := logic.NewResourceLogic().GetUserCodes(token)
	if err != nil {
		return response.ServerError(c, "获取用户权限码失败")
	}

	return response.Success(c, codes)
}
