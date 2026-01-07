package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"yqhp/common/database"
	"yqhp/common/logger"
	commonRedis "yqhp/common/redis"
	"yqhp/gulu/internal/auth"
	"yqhp/gulu/internal/config"
	"yqhp/gulu/internal/router"
	"yqhp/gulu/internal/svc"

	"github.com/gofiber/fiber/v2"
)

func main() {
	// 加载配置
	cfg, err := config.LoadConfig("config/config.yml")
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}

	// 初始化日志
	logger.Init(&logger.Config{
		Level:      cfg.Log.Level,
		Format:     cfg.Log.Format,
		Output:     cfg.Log.Output,
		FilePath:   cfg.Log.FilePath,
		MaxSize:    cfg.Log.MaxSize,
		MaxBackups: cfg.Log.MaxBackups,
		MaxAge:     cfg.Log.MaxAge,
	})
	defer logger.Sync()
	logger.Info("日志初始化完成")

	// 初始化数据库
	if err := database.Init(&cfg.Database); err != nil {
		log.Fatalf("初始化数据库失败: %v", err)
	}
	defer database.Close()
	db := database.GetDB()

	// 初始化Redis (与 Admin 服务共享，用于 SSO)
	if err := commonRedis.Init(&cfg.Redis); err != nil {
		log.Fatalf("初始化Redis失败: %v", err)
	}
	defer commonRedis.Close()
	rdb := commonRedis.GetClient()

	// 初始化服务上下文
	svc.Init(cfg, db, rdb)

	// 初始化SaToken (共享 Redis 存储，用于 SSO Token 验证)
	if err := auth.InitSaToken(cfg); err != nil {
		log.Fatalf("初始化SaToken失败: %v", err)
	}

	// 创建Fiber应用
	app := fiber.New(fiber.Config{
		AppName:      cfg.App.Name,
		ReadTimeout:  0,
		WriteTimeout: 0,
	})

	// 设置路由
	router.Setup(app)

	// 启动服务器
	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	go func() {
		log.Printf("服务器启动在 http://%s", addr)
		if err := app.Listen(addr); err != nil {
			log.Fatalf("服务器启动失败: %v", err)
		}
	}()

	// 优雅关闭
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("正在关闭服务器...")
	if err := app.Shutdown(); err != nil {
		log.Printf("服务器关闭失败: %v", err)
	}
	log.Println("服务器已关闭")
}
