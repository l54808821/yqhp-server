package handler

import (
	"strconv"

	"yqhp/admin/internal/logic"
	"yqhp/admin/internal/types"
	"yqhp/common/response"

	"github.com/gofiber/fiber/v2"
)

// TokenHandler 令牌处理器
type TokenHandler struct {
	tokenLogic *logic.TokenLogic
}

// NewTokenHandler 创建令牌处理器
func NewTokenHandler(tokenLogic *logic.TokenLogic) *TokenHandler {
	return &TokenHandler{tokenLogic: tokenLogic}
}

// List 获取令牌列表
func (h *TokenHandler) List(c *fiber.Ctx) error {
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

	tokens, total, err := h.tokenLogic.ListTokens(&req)
	if err != nil {
		return response.Error(c, "获取失败")
	}

	return response.Page(c, tokens, total, req.Page, req.PageSize)
}

// KickOut 踢人下线（根据Token记录ID）
func (h *TokenHandler) KickOut(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "参数错误")
	}

	if err := h.tokenLogic.KickOutByTokenID(uint(id)); err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, nil)
}

// KickOutByUserID 根据用户ID踢人下线（踢掉该用户的所有会话）
func (h *TokenHandler) KickOutByUserID(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "参数错误")
	}

	if err := h.tokenLogic.KickOut(uint(id)); err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, nil)
}

// KickOutByToken 根据Token踢人下线
func (h *TokenHandler) KickOutByToken(c *fiber.Ctx) error {
	var req struct {
		Token string `json:"token"`
	}
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}

	if err := h.tokenLogic.KickOutByToken(req.Token); err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, nil)
}

// DisableUser 禁用用户
func (h *TokenHandler) DisableUser(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "参数错误")
	}

	var req struct {
		DisableTime int64 `json:"disableTime"` // 禁用时长(秒)，-1表示永久
	}
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}

	if err := h.tokenLogic.DisableUser(uint(id), req.DisableTime); err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, nil)
}

// EnableUser 解禁用户
func (h *TokenHandler) EnableUser(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "参数错误")
	}

	if err := h.tokenLogic.EnableUser(uint(id)); err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, nil)
}

// GetLoginLogs 获取登录日志
func (h *TokenHandler) GetLoginLogs(c *fiber.Ctx) error {
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

	logs, total, err := h.tokenLogic.GetLoginLogs(&req)
	if err != nil {
		return response.Error(c, "获取失败")
	}

	return response.Page(c, logs, total, req.Page, req.PageSize)
}

// GetOperationLogs 获取操作日志
func (h *TokenHandler) GetOperationLogs(c *fiber.Ctx) error {
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

	logs, total, err := h.tokenLogic.GetOperationLogs(&req)
	if err != nil {
		return response.Error(c, "获取失败")
	}

	return response.Page(c, logs, total, req.Page, req.PageSize)
}

// ClearLoginLogs 清空登录日志
func (h *TokenHandler) ClearLoginLogs(c *fiber.Ctx) error {
	if err := h.tokenLogic.ClearLoginLogs(); err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, nil)
}

// ClearOperationLogs 清空操作日志
func (h *TokenHandler) ClearOperationLogs(c *fiber.Ctx) error {
	if err := h.tokenLogic.ClearOperationLogs(); err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, nil)
}
