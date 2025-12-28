package handler

import (
	"strconv"

	"yqhp/admin/internal/logic"
	"yqhp/admin/internal/types"
	"yqhp/common/response"

	"github.com/gofiber/fiber/v2"
)

// DeptTree 获取部门树
func DeptTree(c *fiber.Ctx) error {
	tree, err := logic.NewDeptLogic(c).GetDeptTree()
	if err != nil {
		return response.Error(c, "获取失败")
	}
	return response.Success(c, tree)
}

// DeptAll 获取所有部门
func DeptAll(c *fiber.Ctx) error {
	depts, err := logic.NewDeptLogic(c).GetAllDepts()
	if err != nil {
		return response.Error(c, "获取失败")
	}
	return response.Success(c, depts)
}

// DeptGet 获取部门详情
func DeptGet(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "参数错误")
	}

	dept, err := logic.NewDeptLogic(c).GetDept(id)
	if err != nil {
		return response.Error(c, "获取失败")
	}

	return response.Success(c, dept)
}

// DeptCreate 创建部门
func DeptCreate(c *fiber.Ctx) error {
	var req types.CreateDeptRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}

	if req.Name == "" {
		return response.Error(c, "部门名称不能为空")
	}

	dept, err := logic.NewDeptLogic(c).CreateDept(&req)
	if err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, dept)
}

// DeptUpdate 更新部门
func DeptUpdate(c *fiber.Ctx) error {
	var req types.UpdateDeptRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}

	if req.ID == 0 {
		return response.Error(c, "部门ID不能为空")
	}

	if err := logic.NewDeptLogic(c).UpdateDept(&req); err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, nil)
}

// DeptDelete 删除部门
func DeptDelete(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "参数错误")
	}

	if err := logic.NewDeptLogic(c).DeleteDept(id); err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, nil)
}
