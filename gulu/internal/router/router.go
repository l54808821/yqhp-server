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

	// 创建执行相关组件（需要依赖注入的 handler）
	engineClient := client.NewWorkflowEngineClient()
	sched := scheduler.NewScheduler(engineClient)
	sessionManager := executor.NewSessionManager()
	streamExecutor := executor.NewStreamExecutor(sessionManager, 30*time.Minute)
	executionHandler := handler.NewStreamExecutionHandler(sched, streamExecutor, sessionManager)

	// API 路由组 (需要认证)
	api := app.Group("/api", middleware.AuthMiddleware(), middleware.ProjectMiddleware())

	// 用户相关路由
	user := api.Group("/user")
	user.Get("/info", handler.UserGetInfo)
	user.Get("/menus", handler.UserGetMenus)
	user.Get("/codes", handler.UserGetCodes)

	// 团队管理路由
	teams := api.Group("/teams")
	teams.Post("", handler.TeamCreate)
	teams.Get("", handler.TeamList)
	teams.Get("/my", handler.TeamGetUserTeams)
	teams.Get("/:id", handler.TeamGetByID)
	teams.Put("/:id", handler.TeamUpdate)
	teams.Delete("/:id", handler.TeamDelete)
	teams.Post("/:id/members", handler.TeamAddMember)
	teams.Get("/:id/members", handler.TeamGetMembers)
	teams.Delete("/:id/members/:userId", handler.TeamRemoveMember)
	teams.Put("/:id/members/:userId/role", handler.TeamUpdateMemberRole)

	// 项目管理路由
	projects := api.Group("/projects")
	projects.Post("", handler.ProjectCreate)
	projects.Get("", handler.ProjectList)
	projects.Get("/all", handler.ProjectGetAll)
	projects.Get("/:id", handler.ProjectGetByID)
	projects.Put("/:id", handler.ProjectUpdate)
	projects.Delete("/:id", handler.ProjectDelete)
	projects.Put("/:id/status", handler.ProjectUpdateStatus)
	projects.Post("/:id/members", handler.ProjectAddMember)
	projects.Get("/:id/members", handler.ProjectGetMembers)
	projects.Delete("/:id/members/:userId", handler.ProjectRemoveMember)
	projects.Post("/:id/permissions", handler.ProjectGrantPermission)
	projects.Get("/:id/permissions", handler.ProjectGetPermissions)
	projects.Get("/:id/permissions/user/:userId", handler.ProjectGetUserPermissions)
	projects.Delete("/:id/permissions/:userId/:code", handler.ProjectRevokePermission)

	// 团队下的项目路由
	teams.Get("/:id/projects", handler.ProjectGetByTeamID)
	teams.Post("/:id/projects", handler.ProjectCreateInTeam)

	// 工作流分类路由
	api.Post("/projects/:projectId/categories", handler.CategoryCreate)
	api.Get("/projects/:projectId/categories", handler.CategoryGetTree)
	api.Get("/projects/:projectId/categories/search", handler.CategorySearch)
	categories := api.Group("/categories")
	categories.Put("/sort", handler.CategoryUpdateSort)
	categories.Get("/:id", handler.CategoryGetByID)
	categories.Put("/:id", handler.CategoryUpdate)
	categories.Delete("/:id", handler.CategoryDelete)
	categories.Put("/:id/move", handler.CategoryMove)

	// 环境管理路由
	envs := api.Group("/envs")
	envs.Post("", handler.EnvCreate)
	envs.Get("", handler.EnvList)
	envs.Put("/sort", handler.EnvUpdateSort)
	envs.Get("/project/:projectId", handler.EnvGetByProjectID)
	envs.Get("/:id", handler.EnvGetByID)
	envs.Put("/:id", handler.EnvUpdate)
	envs.Delete("/:id", handler.EnvDelete)
	envs.Post("/:id/copy", handler.EnvCopy)

	// 域名管理路由
	domains := api.Group("/domains")
	domains.Post("", handler.DomainCreate)
	domains.Get("", handler.DomainList)
	domains.Get("/env/:envId", handler.DomainGetByEnvID)
	domains.Get("/:id", handler.DomainGetByID)
	domains.Put("/:id", handler.DomainUpdate)
	domains.Delete("/:id", handler.DomainDelete)

	// 变量管理路由
	vars := api.Group("/vars")
	vars.Post("", handler.VarCreate)
	vars.Get("", handler.VarList)
	vars.Get("/env/:envId", handler.VarGetByEnvID)
	vars.Get("/export", handler.VarExport)
	vars.Post("/import", handler.VarImport)
	vars.Get("/:id", handler.VarGetByID)
	vars.Put("/:id", handler.VarUpdate)
	vars.Delete("/:id", handler.VarDelete)

	// 数据库配置路由
	dbConfigs := api.Group("/database-configs")
	dbConfigs.Post("", handler.DatabaseConfigCreate)
	dbConfigs.Get("", handler.DatabaseConfigList)
	dbConfigs.Get("/env/:envId", handler.DatabaseConfigGetByEnvID)
	dbConfigs.Get("/:id", handler.DatabaseConfigGetByID)
	dbConfigs.Put("/:id", handler.DatabaseConfigUpdate)
	dbConfigs.Delete("/:id", handler.DatabaseConfigDelete)

	// MQ配置路由
	mqConfigs := api.Group("/mq-configs")
	mqConfigs.Post("", handler.MQConfigCreate)
	mqConfigs.Get("", handler.MQConfigList)
	mqConfigs.Get("/env/:envId", handler.MQConfigGetByEnvID)
	mqConfigs.Get("/:id", handler.MQConfigGetByID)
	mqConfigs.Put("/:id", handler.MQConfigUpdate)
	mqConfigs.Delete("/:id", handler.MQConfigDelete)

	// 执行机管理路由
	executors := api.Group("/executors")
	executors.Post("", handler.ExecutorCreate)
	executors.Get("", handler.ExecutorList)
	executors.Post("/sync", handler.ExecutorSync)
	executors.Get("/by-labels", handler.ExecutorListByLabels)
	executors.Get("/:id", handler.ExecutorGetByID)
	executors.Put("/:id", handler.ExecutorUpdate)
	executors.Delete("/:id", handler.ExecutorDelete)
	executors.Put("/:id/status", handler.ExecutorUpdateStatus)

	// 工作流管理路由
	workflows := api.Group("/workflows")
	workflows.Post("", handler.WorkflowCreate)
	workflows.Get("", handler.WorkflowList)
	workflows.Post("/import", handler.WorkflowImportYAML)
	workflows.Post("/validate", handler.WorkflowValidateDefinition)
	workflows.Get("/project/:projectId", handler.WorkflowGetByProjectID)
	workflows.Get("/:id", handler.WorkflowGetByID)
	workflows.Put("/:id", handler.WorkflowUpdate)
	workflows.Delete("/:id", handler.WorkflowDelete)
	workflows.Post("/:id/copy", handler.WorkflowCopy)
	workflows.Get("/:id/yaml", handler.WorkflowExportYAML)
	workflows.Post("/:id/validate", handler.WorkflowValidate)
	workflows.Put("/:id/status", handler.WorkflowUpdateStatus)

	// 执行管理路由
	executions := api.Group("/executions")
	executions.Post("", handler.ExecutionExecute)
	executions.Get("", handler.ExecutionList)
	executions.Post("/webhook", handler.ExecutionWebhook)
	executions.Get("/by-execution-id/:executionId", handler.ExecutionGetByExecutionID)
	executions.Get("/:id", handler.ExecutionGetByID)
	executions.Get("/:id/logs", handler.ExecutionGetLogs)
	executions.Get("/:id/status", handler.ExecutionGetStatus)
	executions.Delete("/:id", handler.ExecutionStop)
	executions.Post("/:id/pause", handler.ExecutionPause)
	executions.Post("/:id/resume", handler.ExecutionResume)

	// 统一执行接口（同时支持单步和流程执行，支持 SSE 和阻塞）
	api.Post("/execute", executionHandler.Execute)

	// 执行会话管理路由 - 需要依赖注入的 handler
	streamExecutions := api.Group("/executions")
	streamExecutions.Get("/:sessionId/status", executionHandler.GetExecutionStatus)
	streamExecutions.Delete("/:sessionId/stop", executionHandler.StopExecution)
	streamExecutions.Post("/:sessionId/interaction", executionHandler.SubmitInteraction)
}
