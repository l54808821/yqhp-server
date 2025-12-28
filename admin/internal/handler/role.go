package handler

import (
	"strconv"
	"yqhp/admin/internal/logic"
	"yqhp/admin/internal/types"
	"yqhp/common/response"

	"github.com/gofiber/fiber/v2"
)

// RoleHandler 角色处理器
type RoleHandler struct {
	roleLogic *logic.RoleLogic
}

// NewRoleHandler 创建角色处理器
func NewRoleHandler(roleLogic *logic.RoleLogic) *RoleHandler {
	return &RoleHandler{roleLogic: roleLogic}
}

// List 获取角色列表
func (h *RoleHandler) List(c *fiber.Ctx) error {
	var req types.ListRolesRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}

	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 10
	}

	roles, total, err := h.roleLogic.ListRoles(&req)
	if err != nil {
		return response.Error(c, "获取失败")
	}

	return response.Page(c, roles, total, req.Page, req.PageSize)
}

// All 获取所有角色
func (h *RoleHandler) All(c *fiber.Ctx) error {
	roles, err := h.roleLogic.GetAllRoles()
	if err != nil {
		return response.Error(c, "获取失败")
	}
	return response.Success(c, roles)
}

// Get 获取角色详情
func (h *RoleHandler) Get(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "参数错误")
	}

	role, err := h.roleLogic.GetRole(id)
	if err != nil {
		return response.Error(c, "获取失败")
	}

	return response.Success(c, role)
}

// Create 创建角色
func (h *RoleHandler) Create(c *fiber.Ctx) error {
	var req types.CreateRoleRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}

	if req.Name == "" || req.Code == "" {
		return response.Error(c, "角色名称和编码不能为空")
	}

	role, err := h.roleLogic.CreateRole(&req)
	if err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, role)
}

// Update 更新角色
func (h *RoleHandler) Update(c *fiber.Ctx) error {
	var req types.UpdateRoleRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}

	if req.ID == 0 {
		return response.Error(c, "角色ID不能为空")
	}

	if err := h.roleLogic.UpdateRole(&req); err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, nil)
}

// Delete 删除角色
func (h *RoleHandler) Delete(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "参数错误")
	}

	if err := h.roleLogic.DeleteRole(id); err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, nil)
}

// GetResourceIDs 获取角色的资源ID列表
func (h *RoleHandler) GetResourceIDs(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "参数错误")
	}

	resourceIDs, err := h.roleLogic.GetRoleResourceIDs(id)
	if err != nil {
		return response.Error(c, "获取失败")
	}

	return response.Success(c, resourceIDs)
}
