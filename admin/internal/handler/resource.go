package handler

import (
	"strconv"

	"yqhp/admin/internal/middleware"
	"yqhp/admin/internal/service"
	"yqhp/common/response"

	"github.com/gofiber/fiber/v2"
)

// ResourceHandler 资源处理器
type ResourceHandler struct {
	resourceService *service.ResourceService
}

// NewResourceHandler 创建资源处理器
func NewResourceHandler(resourceService *service.ResourceService) *ResourceHandler {
	return &ResourceHandler{resourceService: resourceService}
}

// Tree 获取资源树
func (h *ResourceHandler) Tree(c *fiber.Ctx) error {
	tree, err := h.resourceService.GetResourceTree()
	if err != nil {
		return response.Error(c, "获取失败")
	}
	return response.Success(c, tree)
}

// All 获取所有资源
func (h *ResourceHandler) All(c *fiber.Ctx) error {
	resources, err := h.resourceService.GetAllResources()
	if err != nil {
		return response.Error(c, "获取失败")
	}
	return response.Success(c, resources)
}

// GetUserMenus 获取当前用户菜单
func (h *ResourceHandler) GetUserMenus(c *fiber.Ctx) error {
	userID := middleware.GetCurrentUserID(c)
	if userID == 0 {
		return response.Unauthorized(c, "请先登录")
	}

	menus, err := h.resourceService.GetUserMenus(userID)
	if err != nil {
		return response.Error(c, "获取失败")
	}

	return response.Success(c, menus)
}

// GetUserPermissionCodes 获取当前用户权限码
func (h *ResourceHandler) GetUserPermissionCodes(c *fiber.Ctx) error {
	userID := middleware.GetCurrentUserID(c)
	if userID == 0 {
		return response.Unauthorized(c, "请先登录")
	}

	codes, err := h.resourceService.GetUserPermissionCodes(userID)
	if err != nil {
		return response.Error(c, "获取失败")
	}

	return response.Success(c, codes)
}

// Get 获取资源详情
func (h *ResourceHandler) Get(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "参数错误")
	}

	resource, err := h.resourceService.GetResource(uint(id))
	if err != nil {
		return response.Error(c, "获取失败")
	}

	return response.Success(c, resource)
}

// Create 创建资源
func (h *ResourceHandler) Create(c *fiber.Ctx) error {
	var req service.CreateResourceRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}

	if req.Name == "" {
		return response.Error(c, "资源名称不能为空")
	}

	resource, err := h.resourceService.CreateResource(&req)
	if err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, resource)
}

// Update 更新资源
func (h *ResourceHandler) Update(c *fiber.Ctx) error {
	var req service.UpdateResourceRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}

	if req.ID == 0 {
		return response.Error(c, "资源ID不能为空")
	}

	if err := h.resourceService.UpdateResource(&req); err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, nil)
}

// Delete 删除资源
func (h *ResourceHandler) Delete(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "参数错误")
	}

	if err := h.resourceService.DeleteResource(uint(id)); err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, nil)
}

