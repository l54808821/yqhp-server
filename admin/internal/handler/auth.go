package handler

import (
	"yqhp/admin/internal/logic"
	"yqhp/admin/internal/middleware"
	"yqhp/admin/internal/types"
	"yqhp/common/response"

	"github.com/gofiber/fiber/v2"
)

// AuthLogin 登录
func AuthLogin(c *fiber.Ctx) error {
	var req types.LoginRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}

	if req.Username == "" || req.Password == "" {
		return response.Error(c, "用户名和密码不能为空")
	}

	result, err := logic.NewUserLogic(c).Login(&req, c.IP())
	if err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, result)
}

// AuthRegister 注册
func AuthRegister(c *fiber.Ctx) error {
	var req types.RegisterRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}

	if req.Username == "" || req.Password == "" {
		return response.Error(c, "用户名和密码不能为空")
	}

	result, err := logic.NewUserLogic(c).Register(&req, c.IP())
	if err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, result)
}

// AuthLogout 登出
func AuthLogout(c *fiber.Ctx) error {
	token := getTokenFromRequest(c)
	if token == "" {
		return response.Success(c, nil)
	}
	_ = logic.NewUserLogic(c).Logout(token)
	return response.Success(c, nil)
}

// getTokenFromRequest 从请求中获取Token
func getTokenFromRequest(c *fiber.Ctx) string {
	if token := c.Get("satoken"); token != "" {
		return token
	}
	if authHeader := c.Get("Authorization"); authHeader != "" {
		if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
			return authHeader[7:]
		}
		return authHeader
	}
	if token := c.Query("satoken"); token != "" {
		return token
	}
	return c.Cookies("satoken")
}

// AuthGetUserInfo 获取当前用户信息
func AuthGetUserInfo(c *fiber.Ctx) error {
	userID := middleware.GetCurrentUserID(c)
	if userID == 0 {
		return response.Unauthorized(c, "请先登录")
	}

	user, err := logic.NewUserLogic(c).GetUserInfo(int64(userID))
	if err != nil {
		return response.Error(c, "获取用户信息失败")
	}

	return response.Success(c, user)
}

// AuthChangePassword 修改密码
func AuthChangePassword(c *fiber.Ctx) error {
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

	if err := logic.NewUserLogic(c).ChangePassword(int64(userID), req.OldPassword, req.NewPassword); err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, nil)
}

// AuthGetOAuthProviders 获取OAuth提供商列表
func AuthGetOAuthProviders(c *fiber.Ctx) error {
	providers, err := logic.NewOAuthLogic(c).ListAllProviders()
	if err != nil {
		return response.Error(c, "获取失败")
	}
	return response.Success(c, providers)
}

// AuthGetOAuthURL 获取OAuth授权URL
func AuthGetOAuthURL(c *fiber.Ctx) error {
	providerCode := c.Params("provider")
	state := c.Query("state", "")

	url, err := logic.NewOAuthLogic(c).GetAuthURL(providerCode, state)
	if err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, fiber.Map{"url": url})
}

// AuthOAuthCallback OAuth回调
func AuthOAuthCallback(c *fiber.Ctx) error {
	providerCode := c.Params("provider")
	code := c.Query("code")

	if code == "" {
		return response.Error(c, "授权码不能为空")
	}

	result, err := logic.NewOAuthLogic(c).HandleCallback(providerCode, code, c.IP())
	if err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, result)
}

// AuthGetUserBindings 获取用户绑定的第三方账号
func AuthGetUserBindings(c *fiber.Ctx) error {
	userID := middleware.GetCurrentUserID(c)
	if userID == 0 {
		return response.Unauthorized(c, "请先登录")
	}

	bindings, err := logic.NewOAuthLogic(c).GetUserBindings(int64(userID))
	if err != nil {
		return response.Error(c, "获取失败")
	}

	return response.Success(c, bindings)
}

// AuthBindOAuth 绑定第三方账号
func AuthBindOAuth(c *fiber.Ctx) error {
	userID := middleware.GetCurrentUserID(c)
	if userID == 0 {
		return response.Unauthorized(c, "请先登录")
	}

	providerCode := c.Params("provider")
	code := c.Query("code")

	if err := logic.NewOAuthLogic(c).BindOAuth(int64(userID), providerCode, code); err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, nil)
}

// AuthUnbindOAuth 解绑第三方账号
func AuthUnbindOAuth(c *fiber.Ctx) error {
	userID := middleware.GetCurrentUserID(c)
	if userID == 0 {
		return response.Unauthorized(c, "请先登录")
	}

	providerCode := c.Params("provider")

	if err := logic.NewOAuthLogic(c).UnbindOAuth(int64(userID), providerCode); err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, nil)
}
