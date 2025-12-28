package middleware

import (
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/recover"
)

// Recover 异常恢复中间件
func Recover() fiber.Handler {
	return recover.New(recover.Config{
		EnableStackTrace: true,
	})
}

// RecoverWithConfig 自定义异常恢复配置
func RecoverWithConfig(config recover.Config) fiber.Handler {
	return recover.New(config)
}

