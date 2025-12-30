package handler

import (
	"strconv"

	"yqhp/admin/internal/logic"
	"yqhp/admin/internal/types"
	"yqhp/common/response"

	"github.com/gofiber/fiber/v2"
)

// UserAppList 获取用户-应用关联列表
func UserAppList(c *fiber.Ctx) error {
	var req types.ListUserAppsRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}

	list, total, err := logic.NewUserAppLogic(c).ListUserApps(&req)
	if err != nil {
		return response.Error(c, "获取失败")
	}

	return response.Success(c, fiber.Map{
		"list":     list,
		"total":    total,
		"page":     req.Page,
		"pageSize": req.PageSize,
	})
}

// UserAppGetByUser 获取用户的应用关联
func UserAppGetByUser(c *fiber.Ctx) error {
	userID, err := strconv.ParseInt(c.Params("userId"), 10, 64)
	if err != nil {
		return response.Error(c, "参数错误")
	}

	list, err := logic.NewUserAppLogic(c).GetUserApps(userID)
	if err != nil {
		return response.Error(c, "获取失败")
	}

	return response.Success(c, list)
}

// UserAppGetByApp 获取应用的用户关联
func UserAppGetByApp(c *fiber.Ctx) error {
	appID, err := strconv.ParseInt(c.Params("appId"), 10, 64)
	if err != nil {
		return response.Error(c, "参数错误")
	}

	list, err := logic.NewUserAppLogic(c).GetAppUsers(appID)
	if err != nil {
		return response.Error(c, "获取失败")
	}

	return response.Success(c, list)
}
