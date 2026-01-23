package router

import (
	"time"

	"yqhp/gulu/internal/client"
	"yqhp/gulu/internal/executor"
	"yqhp/gulu/internal/handler"
	"yqhp/gulu/internal/middleware"
	"yqhp/gulu/internal/scheduler"

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
		AllowHeaders:     "Origin,Content-Type,Accept,Authorization,satoken,X-Project-ID",
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
	teamHandler := handler.NewTeamHandler()
	projectHandler := handler.NewProjectHandler()
	categoryHandler := handler.NewCategoryWorkflowHandler()
	envHandler := handler.NewEnvHandler()
	domainHandler := handler.NewDomainHandler()
	varHandler := handler.NewVarHandler()
	dbConfigHandler := handler.NewDatabaseConfigHandler()
	mqConfigHandler := handler.NewMQConfigHandler()
	executorHandler := handler.NewExecutorHandler()
	workflowHandler := handler.NewWorkflowHandler()
	executionHandler := handler.NewExecutionHandler()

	// 创建执行相关组件
	engineClient := client.NewWorkflowEngineClient()
	sched := scheduler.NewScheduler(engineClient)
	sessionManager := executor.NewSessionManager()
	streamExecutor := executor.NewStreamExecutor(sessionManager, 30*time.Minute)
	executionStreamHandler := handler.NewStreamExecutionHandler(sched, streamExecutor, sessionManager)
	debugStepHandler := handler.NewDebugStepHandler(sessionManager)

	// API 路由组 (需要认证)
	api := app.Group("/api", middleware.AuthMiddleware(), middleware.ProjectMiddleware())

	// 用户相关路由
	user := api.Group("/user")
	user.Get("/info", userHandler.GetUserInfo)
	user.Get("/menus", userHandler.GetUserMenus)
	user.Get("/codes", userHandler.GetUserCodes)

	// 团队管理路由
	teams := api.Group("/teams")
	teams.Post("", teamHandler.Create)
	teams.Get("", teamHandler.List)
	teams.Get("/my", teamHandler.GetUserTeams)
	teams.Get("/:id", teamHandler.GetByID)
	teams.Put("/:id", teamHandler.Update)
	teams.Delete("/:id", teamHandler.Delete)
	teams.Post("/:id/members", teamHandler.AddMember)
	teams.Get("/:id/members", teamHandler.GetMembers)
	teams.Delete("/:id/members/:userId", teamHandler.RemoveMember)
	teams.Put("/:id/members/:userId/role", teamHandler.UpdateMemberRole)

	// 项目管理路由
	projects := api.Group("/projects")
	projects.Post("", projectHandler.Create)
	projects.Get("", projectHandler.List)
	projects.Get("/all", projectHandler.GetAll)
	projects.Get("/:id", projectHandler.GetByID)
	projects.Put("/:id", projectHandler.Update)
	projects.Delete("/:id", projectHandler.Delete)
	projects.Put("/:id/status", projectHandler.UpdateStatus)
	projects.Post("/:id/members", projectHandler.AddMember)
	projects.Get("/:id/members", projectHandler.GetMembers)
	projects.Delete("/:id/members/:userId", projectHandler.RemoveMember)
	projects.Post("/:id/permissions", projectHandler.GrantPermission)
	projects.Get("/:id/permissions", projectHandler.GetPermissions)
	projects.Get("/:id/permissions/user/:userId", projectHandler.GetUserPermissions)
	projects.Delete("/:id/permissions/:userId/:code", projectHandler.RevokePermission)

	// 团队下的项目路由
	teams.Get("/:id/projects", projectHandler.GetByTeamID)
	teams.Post("/:id/projects", projectHandler.CreateInTeam)

	// 工作流分类路由
	api.Post("/projects/:projectId/categories", categoryHandler.Create)
	api.Get("/projects/:projectId/categories", categoryHandler.GetTree)
	api.Get("/projects/:projectId/categories/search", categoryHandler.Search)
	categories := api.Group("/categories")
	categories.Put("/sort", categoryHandler.UpdateSort)
	categories.Get("/:id", categoryHandler.GetByID)
	categories.Put("/:id", categoryHandler.Update)
	categories.Delete("/:id", categoryHandler.Delete)
	categories.Put("/:id/move", categoryHandler.Move)

	// 环境管理路由
	envs := api.Group("/envs")
	envs.Post("", envHandler.Create)
	envs.Get("", envHandler.List)
	envs.Get("/project/:projectId", envHandler.GetByProjectID)
	envs.Get("/:id", envHandler.GetByID)
	envs.Put("/:id", envHandler.Update)
	envs.Delete("/:id", envHandler.Delete)
	envs.Post("/:id/copy", envHandler.Copy)

	// 域名管理路由
	domains := api.Group("/domains")
	domains.Post("", domainHandler.Create)
	domains.Get("", domainHandler.List)
	domains.Get("/env/:envId", domainHandler.GetByEnvID)
	domains.Get("/:id", domainHandler.GetByID)
	domains.Put("/:id", domainHandler.Update)
	domains.Delete("/:id", domainHandler.Delete)

	// 变量管理路由
	vars := api.Group("/vars")
	vars.Post("", varHandler.Create)
	vars.Get("", varHandler.List)
	vars.Get("/env/:envId", varHandler.GetByEnvID)
	vars.Get("/export", varHandler.Export)
	vars.Post("/import", varHandler.Import)
	vars.Get("/:id", varHandler.GetByID)
	vars.Put("/:id", varHandler.Update)
	vars.Delete("/:id", varHandler.Delete)

	// 数据库配置路由
	dbConfigs := api.Group("/database-configs")
	dbConfigs.Post("", dbConfigHandler.Create)
	dbConfigs.Get("", dbConfigHandler.List)
	dbConfigs.Get("/env/:envId", dbConfigHandler.GetByEnvID)
	dbConfigs.Get("/:id", dbConfigHandler.GetByID)
	dbConfigs.Put("/:id", dbConfigHandler.Update)
	dbConfigs.Delete("/:id", dbConfigHandler.Delete)

	// MQ配置路由
	mqConfigs := api.Group("/mq-configs")
	mqConfigs.Post("", mqConfigHandler.Create)
	mqConfigs.Get("", mqConfigHandler.List)
	mqConfigs.Get("/env/:envId", mqConfigHandler.GetByEnvID)
	mqConfigs.Get("/:id", mqConfigHandler.GetByID)
	mqConfigs.Put("/:id", mqConfigHandler.Update)
	mqConfigs.Delete("/:id", mqConfigHandler.Delete)

	// 执行机管理路由
	executors := api.Group("/executors")
	executors.Post("", executorHandler.Create)
	executors.Get("", executorHandler.List)
	executors.Post("/sync", executorHandler.Sync)
	executors.Get("/by-labels", executorHandler.ListByLabels)
	executors.Get("/:id", executorHandler.GetByID)
	executors.Put("/:id", executorHandler.Update)
	executors.Delete("/:id", executorHandler.Delete)
	executors.Put("/:id/status", executorHandler.UpdateStatus)

	// 工作流管理路由
	workflows := api.Group("/workflows")
	workflows.Post("", workflowHandler.Create)
	workflows.Get("", workflowHandler.List)
	workflows.Post("/import", workflowHandler.ImportYAML)
	workflows.Post("/validate", workflowHandler.ValidateDefinition)
	workflows.Get("/project/:projectId", workflowHandler.GetByProjectID)
	workflows.Get("/:id", workflowHandler.GetByID)
	workflows.Put("/:id", workflowHandler.Update)
	workflows.Delete("/:id", workflowHandler.Delete)
	workflows.Post("/:id/copy", workflowHandler.Copy)
	workflows.Get("/:id/yaml", workflowHandler.ExportYAML)
	workflows.Post("/:id/validate", workflowHandler.Validate)
	workflows.Put("/:id/status", workflowHandler.UpdateStatus)

	// 执行管理路由
	executions := api.Group("/executions")
	executions.Post("", executionHandler.Execute)
	executions.Get("", executionHandler.List)
	executions.Post("/webhook", executionHandler.Webhook)
	executions.Get("/by-execution-id/:executionId", executionHandler.GetByExecutionID)
	executions.Get("/:id", executionHandler.GetByID)
	executions.Get("/:id/logs", executionHandler.GetLogs)
	executions.Get("/:id/status", executionHandler.GetStatus)
	executions.Delete("/:id", executionHandler.Stop)
	executions.Post("/:id/pause", executionHandler.Pause)
	executions.Post("/:id/resume", executionHandler.Resume)

	// 流式执行路由（同时支持 GET 和 POST）
	workflows.Get("/:id/run/stream", executionStreamHandler.RunStream)
	workflows.Post("/:id/run/stream", executionStreamHandler.RunStream) // POST 方式，支持大数据量
	workflows.Post("/:id/run", executionStreamHandler.RunBlocking)

	// 单步调试路由
	debug := api.Group("/debug")
	debug.Post("/step", debugStepHandler.DebugStep)

	// 执行会话管理路由
	streamExecutions := api.Group("/executions")
	streamExecutions.Get("/:sessionId/status", executionStreamHandler.GetExecutionStatus)
	streamExecutions.Delete("/:sessionId/stop", executionStreamHandler.StopExecution)
	streamExecutions.Post("/:sessionId/interaction", executionStreamHandler.SubmitInteraction)
}
