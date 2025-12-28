package handler

import (
	"yqhp/admin/internal/logic"
	"yqhp/admin/internal/middleware"
	"yqhp/admin/internal/types"
	"yqhp/common/response"

	"github.com/gofiber/fiber/v2"
)

// AuthHandler 认证处理器
type AuthHandler struct {
	userLogic  *logic.UserLogic
	oauthLogic *logic.OAuthLogic
}

// NewAuthHandler 创建认证处理器
func NewAuthHandler(userLogic *logic.UserLogic, oauthLogic *logic.OAuthLogic) *AuthHandler {
	return &AuthHandler{
		userLogic:  userLogic,
		oauthLogic: oauthLogic,
	}
}

// Login 登录
func (h *AuthHandler) Login(c *fiber.Ctx) error {
	var req types.LoginRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}

	if req.Username == "" || req.Password == "" {
		return response.Error(c, "用户名和密码不能为空")
	}

	result, err := h.userLogic.Login(&req, c.IP())
	if err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, result)
}

// Register 注册
func (h *AuthHandler) Register(c *fiber.Ctx) error {
	var req types.RegisterRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}

	if req.Username == "" || req.Password == "" {
		return response.Error(c, "用户名和密码不能为空")
	}

	result, err := h.userLogic.Register(&req, c.IP())
	if err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, result)
}

// Logout 登出
func (h *AuthHandler) Logout(c *fiber.Ctx) error {
	// 从请求中获取token（公开路由，不通过中间件）
	token := getTokenFromRequest(c)
	if token == "" {
		// 没有token也返回成功，因为用户可能已经登出
		return response.Success(c, nil)
	}
	// 尝试登出，即使失败也返回成功
	_ = h.userLogic.Logout(token)
	return response.Success(c, nil)
}

// getTokenFromRequest 从请求中获取Token
func getTokenFromRequest(c *fiber.Ctx) string {
	// 从Header获取
	token := c.Get("satoken")
	if token != "" {
		return token
	}

	// 从Authorization获取
	authHeader := c.Get("Authorization")
	if authHeader != "" {
		if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
			return authHeader[7:]
		}
		return authHeader
	}

	// 从Query获取
	token = c.Query("satoken")
	if token != "" {
		return token
	}

	// 从Cookie获取
	return c.Cookies("satoken")
}

// GetUserInfo 获取当前用户信息
func (h *AuthHandler) GetUserInfo(c *fiber.Ctx) error {
	userID := middleware.GetCurrentUserID(c)
	if userID == 0 {
		return response.Unauthorized(c, "请先登录")
	}

	user, err := h.userLogic.GetUserInfo(int64(userID))
	if err != nil {
		return response.Error(c, "获取用户信息失败")
	}

	return response.Success(c, user)
}

// ChangePassword 修改密码
func (h *AuthHandler) ChangePassword(c *fiber.Ctx) error {
	userID := middleware.GetCurrentUserID(c)
	if userID == 0 {
		return response.Unauthorized(c, "请先登录")
	}

	var req struct {
		OldPassword string `json:"oldPassword"`
		NewPassword string `json:"newPassword"`
	}
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}

	if err := h.userLogic.ChangePassword(int64(userID), req.OldPassword, req.NewPassword); err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, nil)
}

// GetOAuthProviders 获取OAuth提供商列表
func (h *AuthHandler) GetOAuthProviders(c *fiber.Ctx) error {
	providers, err := h.oauthLogic.ListAllProviders()
	if err != nil {
		return response.Error(c, "获取失败")
	}
	return response.Success(c, providers)
}

// GetOAuthURL 获取OAuth授权URL
func (h *AuthHandler) GetOAuthURL(c *fiber.Ctx) error {
	providerCode := c.Params("provider")
	state := c.Query("state", "")

	url, err := h.oauthLogic.GetAuthURL(providerCode, state)
	if err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, fiber.Map{"url": url})
}

// OAuthCallback OAuth回调
func (h *AuthHandler) OAuthCallback(c *fiber.Ctx) error {
	providerCode := c.Params("provider")
	code := c.Query("code")

	if code == "" {
		return response.Error(c, "授权码不能为空")
	}

	result, err := h.oauthLogic.HandleCallback(providerCode, code, c.IP())
	if err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, result)
}

// GetUserBindings 获取用户绑定的第三方账号
func (h *AuthHandler) GetUserBindings(c *fiber.Ctx) error {
	userID := middleware.GetCurrentUserID(c)
	if userID == 0 {
		return response.Unauthorized(c, "请先登录")
	}

	bindings, err := h.oauthLogic.GetUserBindings(int64(userID))
	if err != nil {
		return response.Error(c, "获取失败")
	}

	return response.Success(c, bindings)
}

// BindOAuth 绑定第三方账号
func (h *AuthHandler) BindOAuth(c *fiber.Ctx) error {
	userID := middleware.GetCurrentUserID(c)
	if userID == 0 {
		return response.Unauthorized(c, "请先登录")
	}

	providerCode := c.Params("provider")
	code := c.Query("code")

	if err := h.oauthLogic.BindOAuth(int64(userID), providerCode, code); err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, nil)
}

// UnbindOAuth 解绑第三方账号
func (h *AuthHandler) UnbindOAuth(c *fiber.Ctx) error {
	userID := middleware.GetCurrentUserID(c)
	if userID == 0 {
		return response.Unauthorized(c, "请先登录")
	}

	providerCode := c.Params("provider")

	if err := h.oauthLogic.UnbindOAuth(int64(userID), providerCode); err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, nil)
}
