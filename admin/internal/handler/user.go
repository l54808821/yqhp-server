package handler

import (
	"strconv"

	"yqhp/admin/internal/logic"
	"yqhp/admin/internal/types"
	"yqhp/common/response"

	"github.com/gofiber/fiber/v2"
)

// UserHandler 用户处理器
type UserHandler struct {
	userLogic *logic.UserLogic
}

// NewUserHandler 创建用户处理器
func NewUserHandler(userLogic *logic.UserLogic) *UserHandler {
	return &UserHandler{userLogic: userLogic}
}

// List 获取用户列表
func (h *UserHandler) List(c *fiber.Ctx) error {
	var req types.ListUsersRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}

	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 10
	}

	users, total, err := h.userLogic.ListUsers(&req)
	if err != nil {
		return response.Error(c, "获取失败")
	}

	return response.Page(c, users, total, req.Page, req.PageSize)
}

// Get 获取用户详情
func (h *UserHandler) Get(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "参数错误")
	}

	user, err := h.userLogic.GetUserInfo(id)
	if err != nil {
		return response.Error(c, "获取失败")
	}

	return response.Success(c, user)
}

// Create 创建用户
func (h *UserHandler) Create(c *fiber.Ctx) error {
	var req types.CreateUserRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}

	if req.Username == "" || req.Password == "" {
		return response.Error(c, "用户名和密码不能为空")
	}

	user, err := h.userLogic.CreateUser(&req)
	if err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, user)
}

// Update 更新用户
func (h *UserHandler) Update(c *fiber.Ctx) error {
	var req types.UpdateUserRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}

	if req.ID == 0 {
		return response.Error(c, "用户ID不能为空")
	}

	if err := h.userLogic.UpdateUser(&req); err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, nil)
}

// Delete 删除用户
func (h *UserHandler) Delete(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "参数错误")
	}

	if err := h.userLogic.DeleteUser(id); err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, nil)
}

// ResetPassword 重置密码
func (h *UserHandler) ResetPassword(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "参数错误")
	}

	var req struct {
		Password string `json:"password"`
	}
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}

	if req.Password == "" {
		req.Password = "123456" // 默认密码
	}

	if err := h.userLogic.ResetPassword(id, req.Password); err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, nil)
}
