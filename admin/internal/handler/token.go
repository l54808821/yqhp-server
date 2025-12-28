package handler

import (
	"strconv"

	"yqhp/admin/internal/logic"
	"yqhp/admin/internal/types"
	"yqhp/common/response"

	"github.com/gofiber/fiber/v2"
)

// TokenList 获取令牌列表
func TokenList(c *fiber.Ctx) error {
	var req types.ListTokensRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}

	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 10
	}

	tokens, total, err := logic.NewTokenLogic(c).ListTokens(&req)
	if err != nil {
		return response.Error(c, "获取失败")
	}

	return response.Page(c, tokens, total, req.Page, req.PageSize)
}

// TokenKickOut 踢人下线（根据Token记录ID）
func TokenKickOut(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "参数错误")
	}

	if err := logic.NewTokenLogic(c).KickOutByTokenID(id); err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, nil)
}

// TokenKickOutByUserID 根据用户ID踢人下线
func TokenKickOutByUserID(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "参数错误")
	}

	if err := logic.NewTokenLogic(c).KickOutByUserID(id); err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, nil)
}

// TokenKickOutByToken 根据Token踢人下线
func TokenKickOutByToken(c *fiber.Ctx) error {
	var req struct {
		Token string `json:"token"`
	}
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}

	if err := logic.NewTokenLogic(c).KickOutByToken(req.Token); err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, nil)
}

// TokenDisableUser 禁用用户
func TokenDisableUser(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "参数错误")
	}

	var req struct {
		DisableTime int64 `json:"disableTime"`
	}
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}

	if err := logic.NewTokenLogic(c).DisableUser(id, req.DisableTime); err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, nil)
}

// TokenEnableUser 解禁用户
func TokenEnableUser(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "参数错误")
	}

	if err := logic.NewTokenLogic(c).EnableUser(id); err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, nil)
}

// TokenGetLoginLogs 获取登录日志
func TokenGetLoginLogs(c *fiber.Ctx) error {
	var req types.ListLoginLogsRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}

	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 10
	}

	logs, total, err := logic.NewTokenLogic(c).ListLoginLogs(&req)
	if err != nil {
		return response.Error(c, "获取失败")
	}

	return response.Page(c, logs, total, req.Page, req.PageSize)
}

// TokenGetOperationLogs 获取操作日志
func TokenGetOperationLogs(c *fiber.Ctx) error {
	var req types.ListOperationLogsRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}

	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 10
	}

	logs, total, err := logic.NewTokenLogic(c).ListOperationLogs(&req)
	if err != nil {
		return response.Error(c, "获取失败")
	}

	return response.Page(c, logs, total, req.Page, req.PageSize)
}

// TokenClearLoginLogs 清空登录日志
func TokenClearLoginLogs(c *fiber.Ctx) error {
	if err := logic.NewTokenLogic(c).ClearLoginLogs(); err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, nil)
}

// TokenClearOperationLogs 清空操作日志
func TokenClearOperationLogs(c *fiber.Ctx) error {
	if err := logic.NewTokenLogic(c).ClearOperationLogs(); err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, nil)
}
