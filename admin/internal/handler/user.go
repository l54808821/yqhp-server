package handler

import (
	"strconv"

	"yqhp/admin/internal/logic"
	"yqhp/admin/internal/types"
	"yqhp/common/response"

	"github.com/gofiber/fiber/v2"
)

// UserList 获取用户列表
func UserList(c *fiber.Ctx) error {
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

	users, total, err := logic.NewUserLogic(c).ListUsers(&req)
	if err != nil {
		return response.Error(c, "获取失败")
	}

	return response.Page(c, users, total, req.Page, req.PageSize)
}

// UserGet 获取用户详情
func UserGet(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "参数错误")
	}

	user, err := logic.NewUserLogic(c).GetUserInfo(id)
	if err != nil {
		return response.Error(c, "获取失败")
	}

	return response.Success(c, user)
}

// UserCreate 创建用户
func UserCreate(c *fiber.Ctx) error {
	var req types.CreateUserRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}

	if req.Username == "" || req.Password == "" {
		return response.Error(c, "用户名和密码不能为空")
	}

	user, err := logic.NewUserLogic(c).CreateUser(&req)
	if err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, user)
}

// UserUpdate 更新用户
func UserUpdate(c *fiber.Ctx) error {
	var req types.UpdateUserRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}

	if req.ID == 0 {
		return response.Error(c, "用户ID不能为空")
	}

	if err := logic.NewUserLogic(c).UpdateUser(&req); err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, nil)
}

// UserDelete 删除用户
func UserDelete(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "参数错误")
	}

	if err := logic.NewUserLogic(c).DeleteUser(id); err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, nil)
}

// UserResetPassword 重置密码
func UserResetPassword(c *fiber.Ctx) error {
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
		req.Password = "123456"
	}

	if err := logic.NewUserLogic(c).ResetPassword(id, req.Password); err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, nil)
}

// UserBatchGet 批量获取用户基本信息
func UserBatchGet(c *fiber.Ctx) error {
	var req struct {
		IDs []int64 `json:"ids"`
	}
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}

	if len(req.IDs) == 0 {
		return response.Success(c, []types.UserBasicInfo{})
	}

	users, err := logic.NewUserLogic(c).BatchGetUsers(req.IDs)
	if err != nil {
		return response.Error(c, "获取失败")
	}

	return response.Success(c, users)
}
