package middleware

import (
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
)

// CORS 跨域中间件
func CORS() fiber.Handler {
	return cors.New(cors.Config{
		AllowOrigins:     "*",
		AllowMethods:     "GET,POST,PUT,DELETE,OPTIONS,PATCH",
		AllowHeaders:     "Origin,Content-Type,Accept,Authorization,X-Requested-With,satoken",
		ExposeHeaders:    "Content-Length,Content-Type",
		AllowCredentials: false,
		MaxAge:           86400,
	})
}

// CORSWithConfig 自定义跨域配置
func CORSWithConfig(config cors.Config) fiber.Handler {
	return cors.New(config)
}

