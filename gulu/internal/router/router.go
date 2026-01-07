package router

import (
	"yqhp/gulu/internal/handler"
	"yqhp/gulu/internal/middleware"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	fiberLogger "github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
)

// Setup 设置路由
func Setup(app *fiber.App) {
	// 全局中间件
	app.Use(recover.New())
	app.Use(fiberLogger.New())
	app.Use(cors.New(cors.Config{
		AllowOrigins:     "http://localhost:5777,http://127.0.0.1:5777",
		AllowMethods:     "GET,POST,PUT,DELETE,OPTIONS",
		AllowHeaders:     "Origin,Content-Type,Accept,Authorization,satoken",
		AllowCredentials: true,
	}))

	// 健康检查
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status": "ok",
			"app":    "gulu",
		})
	})

	// 创建 Handler
	userHandler := handler.NewUserHandler()

	// API 路由组 (需要认证)
	api := app.Group("/api", middleware.AuthMiddleware())

	// 用户相关路由
	user := api.Group("/user")
	user.Get("/info", userHandler.GetUserInfo)
	user.Get("/menus", userHandler.GetUserMenus)
	user.Get("/codes", userHandler.GetUserCodes)
}
