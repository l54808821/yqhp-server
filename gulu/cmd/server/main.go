package main

import (
	"context"
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
	"yqhp/gulu/internal/logic"
	"yqhp/gulu/internal/router"
	"yqhp/gulu/internal/svc"
	"yqhp/gulu/internal/workflow"
	"yqhp/workflow-engine/pkg/types"

	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
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

	// 初始化工作流引擎
	if err := workflow.Init(&cfg.WorkflowEngine); err != nil {
		log.Fatalf("初始化工作流引擎失败: %v", err)
	}
	if cfg.WorkflowEngine.Embedded {
		logger.Info("内置工作流引擎已启动")
		// 启动 Slave 事件监听，自动同步到数据库
		go watchAndSyncSlaves()
	} else {
		logger.Info("使用外部工作流引擎: " + cfg.WorkflowEngine.ExternalURL)
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

	// 停止工作流引擎
	if engine := workflow.GetEngine(); engine != nil {
		if err := engine.Stop(); err != nil {
			log.Printf("停止工作流引擎失败: %v", err)
		}
	}

	if err := app.Shutdown(); err != nil {
		log.Printf("服务器关闭失败: %v", err)
	}
	log.Println("服务器已关闭")
}

// watchAndSyncSlaves 监听 Slave 注册事件，自动同步到数据库
func watchAndSyncSlaves() {
	engine := workflow.GetEngine()
	if engine == nil {
		return
	}

	ctx := context.Background()
	events, err := engine.WatchSlaves(ctx)
	if err != nil {
		logger.Warn("监听 Slave 事件失败: " + err.Error())
		return
	}

	logger.Info("开始监听 Slave 注册事件")
	for event := range events {
		if event.Type == types.SlaveEventRegistered && event.Slave != nil {
			logger.Info("检测到新 Slave 注册，自动同步", zap.String("slave_id", event.SlaveID))
			executorLogic := logic.NewExecutorLogic(ctx)
			_, err := executorLogic.Register(&logic.RegisterExecutorReq{
				SlaveID:      event.SlaveID,
				Name:         event.SlaveID,
				Address:      event.Slave.Address,
				Type:         string(event.Slave.Type),
				Capabilities: event.Slave.Capabilities,
				Labels:       event.Slave.Labels,
			})
			if err != nil {
				logger.Warn("自动同步 Slave 失败", zap.String("slave_id", event.SlaveID), zap.Error(err))
			} else {
				logger.Info("自动同步 Slave 成功", zap.String("slave_id", event.SlaveID))
			}
		}
	}
}
