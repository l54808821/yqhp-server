package middleware

import (
	"strconv"
	"strings"

	"yqhp/admin/internal/auth"
	"yqhp/admin/internal/ctxutil"
	"yqhp/common/response"

	"github.com/gofiber/fiber/v2"
)

// AuthMiddleware 认证中间件
func AuthMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		// 获取Token
		token := getToken(c)
		if token == "" {
			return response.Unauthorized(c, "请先登录")
		}

		// 检查登录状态
		if !auth.IsLogin(token) {
			return response.Unauthorized(c, "登录已过期，请重新登录")
		}

		// 获取登录ID
		loginId, err := auth.GetLoginId(token)
		if err != nil {
			return response.Unauthorized(c, "获取用户信息失败")
		}

		// 解析用户ID
		userID, err := parseUserID(loginId)
		if err != nil {
			return response.Unauthorized(c, "用户信息无效")
		}

		// 将用户ID存入上下文
		c.Locals("userId", loginId)
		c.Locals("token", token)

		// 将用户ID存入context（供Logic层使用）
		ctx := ctxutil.WithUserID(c.Context(), userID)
		c.SetUserContext(ctx)

		return c.Next()
	}
}

// PermissionMiddleware 权限验证中间件
func PermissionMiddleware(permissionService *auth.PermissionService, permissions ...string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		userIdAny := c.Locals("userId")
		if userIdAny == nil {
			return response.Unauthorized(c, "请先登录")
		}

		userID, err := parseUserID(userIdAny)
		if err != nil {
			return response.Unauthorized(c, "用户信息无效")
		}

		// 检查权限
		hasPermission, err := permissionService.HasAnyPermission(userID, permissions...)
		if err != nil {
			return response.ServerError(c, "权限验证失败")
		}

		if !hasPermission {
			return response.Forbidden(c, "没有操作权限")
		}

		return c.Next()
	}
}

// RoleMiddleware 角色验证中间件
func RoleMiddleware(permissionService *auth.PermissionService, roles ...string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		userIdAny := c.Locals("userId")
		if userIdAny == nil {
			return response.Unauthorized(c, "请先登录")
		}

		userID, err := parseUserID(userIdAny)
		if err != nil {
			return response.Unauthorized(c, "用户信息无效")
		}

		// 检查角色
		hasRole, err := permissionService.HasAnyRole(userID, roles...)
		if err != nil {
			return response.ServerError(c, "角色验证失败")
		}

		if !hasRole {
			return response.Forbidden(c, "没有操作权限")
		}

		return c.Next()
	}
}

// getToken 从请求中获取Token
func getToken(c *fiber.Ctx) string {
	// 从Header获取
	token := c.Get("satoken")
	if token != "" {
		return token
	}

	// 从Authorization获取
	authHeader := c.Get("Authorization")
	if authHeader != "" {
		if strings.HasPrefix(authHeader, "Bearer ") {
			return strings.TrimPrefix(authHeader, "Bearer ")
		}
		return authHeader
	}

	// 从Query获取
	token = c.Query("satoken")
	if token != "" {
		return token
	}

	// 从Cookie获取
	token = c.Cookies("satoken")
	return token
}

// parseUserID 解析用户ID
func parseUserID(userIdAny any) (int64, error) {
	switch v := userIdAny.(type) {
	case uint:
		return int64(v), nil
	case int:
		return int64(v), nil
	case int64:
		return v, nil
	case float64:
		return int64(v), nil
	case string:
		id, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return 0, err
		}
		return id, nil
	default:
		return 0, nil
	}
}

// GetCurrentUserID 获取当前用户ID
func GetCurrentUserID(c *fiber.Ctx) int64 {
	userIdAny := c.Locals("userId")
	if userIdAny == nil {
		return 0
	}
	userID, _ := parseUserID(userIdAny)
	return userID
}
