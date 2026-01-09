package middleware

import (
	"strconv"

	"github.com/gofiber/fiber/v2"
)

const (
	// ProjectIDHeader 项目ID请求头
	ProjectIDHeader = "X-Project-ID"
	// ProjectIDContextKey 项目ID上下文键
	ProjectIDContextKey = "projectID"
)

// ProjectMiddleware 项目上下文中间件
// 从请求头获取当前项目ID并注入到上下文
func ProjectMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		projectIDStr := c.Get(ProjectIDHeader)
		if projectIDStr != "" {
			projectID, err := strconv.ParseInt(projectIDStr, 10, 64)
			if err == nil && projectID > 0 {
				c.Locals(ProjectIDContextKey, projectID)
			}
		}
		return c.Next()
	}
}

// GetCurrentProjectID 从上下文获取当前项目ID
func GetCurrentProjectID(c *fiber.Ctx) int64 {
	if projectID, ok := c.Locals(ProjectIDContextKey).(int64); ok {
		return projectID
	}
	return 0
}

// RequireProjectMiddleware 要求项目ID的中间件
// 如果没有项目ID则返回错误
func RequireProjectMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		projectID := GetCurrentProjectID(c)
		if projectID <= 0 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"code":    400,
				"message": "请选择项目",
			})
		}
		return c.Next()
	}
}
