package middleware

import (
	"yqhp/common/logger"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/requestid"
)

// RequestID 请求ID中间件
func RequestID() fiber.Handler {
	return requestid.New()
}

// Logger 日志中间件
func Logger() fiber.Handler {
	return logger.Middleware()
}
