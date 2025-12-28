package handler

import (
	"strconv"

	"yqhp/admin/internal/logic"
	"yqhp/admin/internal/types"
	"yqhp/common/response"

	"github.com/gofiber/fiber/v2"
)

// AppList 获取应用列表
func AppList(c *fiber.Ctx) error {
	var req types.ListApplicationsRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}

	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 10
	}

	apps, total, err := logic.NewApplicationLogic(c).ListApplications(&req)
	if err != nil {
		return response.Error(c, "获取失败")
	}

	return response.Page(c, apps, total, req.Page, req.PageSize)
}

// AppAll 获取所有应用
func AppAll(c *fiber.Ctx) error {
	apps, err := logic.NewApplicationLogic(c).GetAllApplications()
	if err != nil {
		return response.Error(c, "获取失败")
	}
	return response.Success(c, apps)
}

// AppGet 获取应用详情
func AppGet(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "参数错误")
	}

	app, err := logic.NewApplicationLogic(c).GetApplication(id)
	if err != nil {
		return response.Error(c, "获取失败")
	}

	return response.Success(c, app)
}

// AppCreate 创建应用
func AppCreate(c *fiber.Ctx) error {
	var req types.CreateApplicationRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}

	if req.Name == "" || req.Code == "" {
		return response.Error(c, "应用名称和编码不能为空")
	}

	app, err := logic.NewApplicationLogic(c).CreateApplication(&req)
	if err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, app)
}

// AppUpdate 更新应用
func AppUpdate(c *fiber.Ctx) error {
	var req types.UpdateApplicationRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}

	if req.ID == 0 {
		return response.Error(c, "应用ID不能为空")
	}

	if err := logic.NewApplicationLogic(c).UpdateApplication(&req); err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, nil)
}

// AppDelete 删除应用
func AppDelete(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "参数错误")
	}

	if err := logic.NewApplicationLogic(c).DeleteApplication(id); err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, nil)
}
