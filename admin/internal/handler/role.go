package handler

import (
	"strconv"

	"yqhp/admin/internal/logic"
	"yqhp/admin/internal/types"
	"yqhp/common/response"

	"github.com/gofiber/fiber/v2"
)

// RoleList 获取角色列表
func RoleList(c *fiber.Ctx) error {
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

	roles, total, err := logic.NewRoleLogic(c).ListRoles(&req)
	if err != nil {
		return response.Error(c, "获取失败")
	}

	return response.Page(c, roles, total, req.Page, req.PageSize)
}

// RoleAll 获取所有角色
func RoleAll(c *fiber.Ctx) error {
	roles, err := logic.NewRoleLogic(c).GetAllRoles()
	if err != nil {
		return response.Error(c, "获取失败")
	}
	return response.Success(c, roles)
}

// RoleGet 获取角色详情
func RoleGet(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "参数错误")
	}

	role, err := logic.NewRoleLogic(c).GetRole(id)
	if err != nil {
		return response.Error(c, "获取失败")
	}

	return response.Success(c, role)
}

// RoleCreate 创建角色
func RoleCreate(c *fiber.Ctx) error {
	var req types.CreateRoleRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}

	if req.Name == "" || req.Code == "" {
		return response.Error(c, "角色名称和编码不能为空")
	}

	role, err := logic.NewRoleLogic(c).CreateRole(&req)
	if err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, role)
}

// RoleUpdate 更新角色
func RoleUpdate(c *fiber.Ctx) error {
	var req types.UpdateRoleRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}

	if req.ID == 0 {
		return response.Error(c, "角色ID不能为空")
	}

	if err := logic.NewRoleLogic(c).UpdateRole(&req); err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, nil)
}

// RoleDelete 删除角色
func RoleDelete(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "参数错误")
	}

	if err := logic.NewRoleLogic(c).DeleteRole(id); err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, nil)
}

// RoleGetResourceIDs 获取角色的资源ID列表
func RoleGetResourceIDs(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "参数错误")
	}

	resourceIDs, err := logic.NewRoleLogic(c).GetRoleResourceIDs(id)
	if err != nil {
		return response.Error(c, "获取失败")
	}

	return response.Success(c, resourceIDs)
}
