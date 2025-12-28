package handler

import (
	"strconv"
	"yqhp/admin/internal/logic"
	"yqhp/admin/internal/types"
	"yqhp/common/response"

	"github.com/gofiber/fiber/v2"
)

// ApplicationHandler 应用处理器
type ApplicationHandler struct {
	appLogic *logic.ApplicationLogic
}

// NewApplicationHandler 创建应用处理器
func NewApplicationHandler(appLogic *logic.ApplicationLogic) *ApplicationHandler {
	return &ApplicationHandler{appLogic: appLogic}
}

// List 获取应用列表
func (h *ApplicationHandler) List(c *fiber.Ctx) error {
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

	apps, total, err := h.appLogic.ListApplications(&req)
	if err != nil {
		return response.Error(c, "获取失败")
	}

	return response.Page(c, apps, total, req.Page, req.PageSize)
}

// All 获取所有应用
func (h *ApplicationHandler) All(c *fiber.Ctx) error {
	apps, err := h.appLogic.GetAllApplications()
	if err != nil {
		return response.Error(c, "获取失败")
	}
	return response.Success(c, apps)
}

// Get 获取应用详情
func (h *ApplicationHandler) Get(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "参数错误")
	}

	app, err := h.appLogic.GetApplication(id)
	if err != nil {
		return response.Error(c, "获取失败")
	}

	return response.Success(c, app)
}

// Create 创建应用
func (h *ApplicationHandler) Create(c *fiber.Ctx) error {
	var req types.CreateApplicationRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}

	if req.Name == "" || req.Code == "" {
		return response.Error(c, "应用名称和编码不能为空")
	}

	app, err := h.appLogic.CreateApplication(&req)
	if err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, app)
}

// Update 更新应用
func (h *ApplicationHandler) Update(c *fiber.Ctx) error {
	var req types.UpdateApplicationRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}

	if req.ID == 0 {
		return response.Error(c, "应用ID不能为空")
	}

	if err := h.appLogic.UpdateApplication(&req); err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, nil)
}

// Delete 删除应用
func (h *ApplicationHandler) Delete(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "参数错误")
	}

	if err := h.appLogic.DeleteApplication(id); err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, nil)
}
