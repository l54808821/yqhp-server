package handler

import (
	"strconv"

	"yqhp/admin/internal/logic"
	"yqhp/admin/internal/middleware"
	"yqhp/admin/internal/types"
	"yqhp/common/response"

	"github.com/gofiber/fiber/v2"
)

// ResourceTree 获取资源树
func ResourceTree(c *fiber.Ctx) error {
	l := logic.NewResourceLogic(c)
	appIDStr := c.Query("appId")
	if appIDStr != "" {
		appID, err := strconv.ParseInt(appIDStr, 10, 64)
		if err != nil {
			return response.Error(c, "参数错误")
		}
		tree, err := l.GetResourceTreeByAppID(appID)
		if err != nil {
			return response.Error(c, "获取失败")
		}
		return response.Success(c, tree)
	}

	tree, err := l.GetResourceTree()
	if err != nil {
		return response.Error(c, "获取失败")
	}
	return response.Success(c, tree)
}

// ResourceAll 获取所有资源
func ResourceAll(c *fiber.Ctx) error {
	l := logic.NewResourceLogic(c)
	appIDStr := c.Query("appId")
	if appIDStr != "" {
		appID, err := strconv.ParseInt(appIDStr, 10, 64)
		if err != nil {
			return response.Error(c, "参数错误")
		}
		resources, err := l.GetAllResourcesByAppID(appID)
		if err != nil {
			return response.Error(c, "获取失败")
		}
		return response.Success(c, resources)
	}

	resources, err := l.GetAllResources()
	if err != nil {
		return response.Error(c, "获取失败")
	}
	return response.Success(c, resources)
}

// ResourceGetUserMenus 获取当前用户菜单
func ResourceGetUserMenus(c *fiber.Ctx) error {
	userID := middleware.GetCurrentUserID(c)
	if userID == 0 {
		return response.Unauthorized(c, "请先登录")
	}

	menus, err := logic.NewResourceLogic(c).GetUserMenus(userID)
	if err != nil {
		return response.Error(c, "获取失败")
	}

	return response.Success(c, menus)
}

// ResourceGetUserPermissionCodes 获取当前用户权限码
func ResourceGetUserPermissionCodes(c *fiber.Ctx) error {
	userID := middleware.GetCurrentUserID(c)
	if userID == 0 {
		return response.Unauthorized(c, "请先登录")
	}

	codes, err := logic.NewResourceLogic(c).GetUserPermissionCodes(userID)
	if err != nil {
		return response.Error(c, "获取失败")
	}

	return response.Success(c, codes)
}

// ResourceGet 获取资源详情
func ResourceGet(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "参数错误")
	}

	resource, err := logic.NewResourceLogic(c).GetResource(id)
	if err != nil {
		return response.Error(c, "获取失败")
	}

	return response.Success(c, resource)
}

// ResourceCreate 创建资源
func ResourceCreate(c *fiber.Ctx) error {
	var req types.CreateResourceRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}

	if req.Name == "" {
		return response.Error(c, "资源名称不能为空")
	}

	resource, err := logic.NewResourceLogic(c).CreateResource(&req)
	if err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, resource)
}

// ResourceUpdate 更新资源
func ResourceUpdate(c *fiber.Ctx) error {
	var req types.UpdateResourceRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}

	if req.ID == 0 {
		return response.Error(c, "资源ID不能为空")
	}

	if err := logic.NewResourceLogic(c).UpdateResource(&req); err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, nil)
}

// ResourceDelete 删除资源
func ResourceDelete(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "参数错误")
	}

	if err := logic.NewResourceLogic(c).DeleteResource(id); err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, nil)
}
