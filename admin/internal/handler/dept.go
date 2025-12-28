package handler

import (
	"strconv"

	"yqhp/admin/internal/logic"
	"yqhp/admin/internal/types"
	"yqhp/common/response"

	"github.com/gofiber/fiber/v2"
)

// DeptHandler 部门处理器
type DeptHandler struct {
	deptLogic *logic.DeptLogic
}

// NewDeptHandler 创建部门处理器
func NewDeptHandler(deptLogic *logic.DeptLogic) *DeptHandler {
	return &DeptHandler{deptLogic: deptLogic}
}

// Tree 获取部门树
func (h *DeptHandler) Tree(c *fiber.Ctx) error {
	tree, err := h.deptLogic.GetDeptTree()
	if err != nil {
		return response.Error(c, "获取失败")
	}
	return response.Success(c, tree)
}

// All 获取所有部门
func (h *DeptHandler) All(c *fiber.Ctx) error {
	depts, err := h.deptLogic.GetAllDepts()
	if err != nil {
		return response.Error(c, "获取失败")
	}
	return response.Success(c, depts)
}

// Get 获取部门详情
func (h *DeptHandler) Get(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "参数错误")
	}

	dept, err := h.deptLogic.GetDept(uint(id))
	if err != nil {
		return response.Error(c, "获取失败")
	}

	return response.Success(c, dept)
}

// Create 创建部门
func (h *DeptHandler) Create(c *fiber.Ctx) error {
	var req types.CreateDeptRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}

	if req.Name == "" {
		return response.Error(c, "部门名称不能为空")
	}

	dept, err := h.deptLogic.CreateDept(&req)
	if err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, dept)
}

// Update 更新部门
func (h *DeptHandler) Update(c *fiber.Ctx) error {
	var req types.UpdateDeptRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}

	if req.ID == 0 {
		return response.Error(c, "部门ID不能为空")
	}

	if err := h.deptLogic.UpdateDept(&req); err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, nil)
}

// Delete 删除部门
func (h *DeptHandler) Delete(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "参数错误")
	}

	if err := h.deptLogic.DeleteDept(uint(id)); err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, nil)
}
