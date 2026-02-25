package router

import (
	"time"

	"yqhp/gulu/internal/client"
	"yqhp/gulu/internal/executor"
	"yqhp/gulu/internal/handler"
	"yqhp/gulu/internal/mcpproxy"
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
		AllowMethods:     "GET,POST,PUT,PATCH,DELETE,OPTIONS",
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

	// 知识库图片访问（无需认证，支持前端直接渲染）
	app.Get("/api/knowledge-bases/:id/images/:filename", handler.KnowledgeImageServe)

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
	// 环境配置值接口
	envs.Get("/:envId/configs", handler.ConfigList)
	envs.Put("/:envId/configs/:code", handler.ConfigUpdate)
	envs.Put("/:envId/configs", handler.ConfigBatchUpdate)

	// 数据库 Schema 查询路由
	database := api.Group("/database")
	database.Get("/:configCode/tables", handler.DatabaseGetTables)
	database.Get("/:configCode/columns", handler.DatabaseGetColumns)

	// 配置定义管理路由（项目级别）
	configDefs := api.Group("/projects/:projectId/config-definitions")
	configDefs.Get("", handler.ConfigDefinitionList)
	configDefs.Post("", handler.ConfigDefinitionCreate)
	configDefs.Put("/sort", handler.ConfigDefinitionSort)
	configDefs.Put("/:code", handler.ConfigDefinitionUpdate)
	configDefs.Delete("/:code", handler.ConfigDefinitionDelete)

	// 执行机管理路由
	executors := api.Group("/executors")
	executors.Post("", handler.ExecutorCreate)
	executors.Get("", handler.ExecutorList)
	executors.Post("/register", handler.ExecutorRegister)
	executors.Post("/sync", handler.ExecutorSync)
	executors.Get("/available", handler.ExecutorListAvailable)
	executors.Get("/by-labels", handler.ExecutorListByLabels)
	executors.Get("/:id", handler.ExecutorGetByID)
	executors.Put("/:id", handler.ExecutorUpdate)
	executors.Delete("/:id", handler.ExecutorDelete)
	executors.Put("/:id/status", handler.ExecutorUpdateStatus)

	// AI 模型管理路由
	aiModels := api.Group("/ai-models")
	aiModels.Post("", handler.AiModelCreate)
	aiModels.Get("", handler.AiModelList)
	aiModels.Get("/providers", handler.AiModelGetProviders)
	aiModels.Get("/:id", handler.AiModelGetByID)
	aiModels.Put("/:id", handler.AiModelUpdate)
	aiModels.Delete("/:id", handler.AiModelDelete)
	aiModels.Put("/:id/status", handler.AiModelUpdateStatus)
	aiModels.Post("/:id/chat", handler.AiChatStream)

	// Skill 管理路由
	skills := api.Group("/skills")
	skills.Post("", handler.SkillCreate)
	skills.Get("", handler.SkillList)
	skills.Get("/categories", handler.SkillGetCategories)
	skills.Post("/import", handler.SkillImport)
	skills.Get("/:id", handler.SkillGetByID)
	skills.Put("/:id", handler.SkillUpdate)
	skills.Delete("/:id", handler.SkillDelete)
	skills.Put("/:id/status", handler.SkillUpdateStatus)
	skills.Get("/:id/export", handler.SkillExport)
	skills.Get("/:id/resources", handler.SkillResourceList)
	skills.Post("/:id/resources", handler.SkillResourceCreate)
	skills.Delete("/:id/resources/:resourceId", handler.SkillResourceDelete)

	// MCP 服务器管理路由
	mcpServers := api.Group("/mcp-servers")
	mcpServers.Post("", handler.McpServerCreate)
	mcpServers.Get("", handler.McpServerList)
	mcpServers.Get("/:id", handler.McpServerGetByID)
	mcpServers.Put("/:id", handler.McpServerUpdate)
	mcpServers.Delete("/:id", handler.McpServerDelete)
	mcpServers.Put("/:id/status", handler.McpServerUpdateStatus)

	// 知识库管理路由
	kb := api.Group("/knowledge-bases")
	kb.Post("", handler.KnowledgeBaseCreate)
	kb.Get("", handler.KnowledgeBaseList)
	kb.Get("/:id", handler.KnowledgeBaseGetByID)
	kb.Put("/:id", handler.KnowledgeBaseUpdate)
	kb.Delete("/:id", handler.KnowledgeBaseDelete)
	kb.Put("/:id/status", handler.KnowledgeBaseUpdateStatus)
	// 文档管理
	kb.Post("/:id/upload-file", handler.KnowledgeFileUpload)
	kb.Delete("/:id/upload-file", handler.KnowledgeFileDelete)
	kb.Post("/:id/documents/create-and-process", handler.KnowledgeDocumentCreateAndProcess)
	kb.Post("/:id/documents", handler.KnowledgeDocumentUpload)
	kb.Get("/:id/documents", handler.KnowledgeDocumentList)
	kb.Delete("/:id/documents/:docId", handler.KnowledgeDocumentDelete)
	kb.Post("/:id/documents/:docId/reprocess", handler.KnowledgeDocumentReprocess)
	kb.Post("/:id/documents/preview-chunks", handler.KnowledgeDocumentPreviewChunks)
	kb.Put("/:id/documents/:docId/process", handler.KnowledgeDocumentProcess)
	// 批量操作
	kb.Post("/:id/documents/batch-delete", handler.KnowledgeDocumentBatchDelete)
	kb.Post("/:id/documents/batch-reprocess", handler.KnowledgeDocumentBatchReprocess)
	kb.Get("/:id/indexing-status", handler.KnowledgeIndexingStatus)
	// 分块管理
	kb.Get("/:id/documents/:docId/segments", handler.KnowledgeDocumentSegments)
	kb.Patch("/:id/segments/:segId", handler.KnowledgeSegmentUpdate)
	// 检索与查询历史
	kb.Post("/:id/search", handler.KnowledgeBaseSearch)
	kb.Get("/:id/queries", handler.KnowledgeQueryHistory)
	// 诊断接口（排查向量数据问题）
	kb.Get("/:id/diagnose", handler.KnowledgeBaseDiagnose)
	// 图知识库（Phase 3）
	kb.Post("/:id/graph/search", handler.KnowledgeGraphSearch)
	kb.Get("/:id/graph/entities", handler.KnowledgeGraphEntities)
	kb.Get("/:id/graph/relations", handler.KnowledgeGraphRelations)

	// MCP 代理服务路由
	mcpProxyService := mcpproxy.NewMCPProxyService()
	mcpProxyHandler := handler.NewMCPProxyHandler(mcpProxyService)
	mcpProxy := api.Group("/mcp-proxy")
	mcpProxy.Post("/tools", mcpProxyHandler.GetTools)
	mcpProxy.Post("/call-tool", mcpProxyHandler.CallTool)
	mcpProxy.Get("/status/:serverId", mcpProxyHandler.GetStatus)
	mcpProxy.Post("/connect", mcpProxyHandler.Connect)
	mcpProxy.Post("/disconnect", mcpProxyHandler.Disconnect)

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

	// 执行记录管理路由（历史记录查询等）
	executionRecords := api.Group("/execution-records")
	executionRecords.Post("", handler.ExecutionExecute)
	executionRecords.Get("", handler.ExecutionList)
	executionRecords.Post("/webhook", handler.ExecutionWebhook)
	executionRecords.Get("/by-execution-id/:executionId", handler.ExecutionGetByExecutionID)
	executionRecords.Get("/:id", handler.ExecutionGetByID)
	executionRecords.Get("/:id/status", handler.ExecutionGetStatus)
	executionRecords.Get("/:id/metrics", handler.ExecutionGetMetrics)
	executionRecords.Delete("/:id", handler.ExecutionStop)
	executionRecords.Post("/:id/pause", handler.ExecutionPause)
	executionRecords.Post("/:id/resume", handler.ExecutionResume)

	// Performance testing routes (k6-style API)
	executionRecords.Get("/:id/realtime", handler.ExecutionGetRealtimeMetrics)
	executionRecords.Get("/:id/report", handler.ExecutionGetReport)
	executionRecords.Get("/:id/timeseries", handler.ExecutionGetTimeSeries)
	executionRecords.Post("/:id/scale", handler.ExecutionScaleVUs)
	executionRecords.Get("/:id/metrics/stream", handler.ExecutionMetricsStream)
	executionRecords.Get("/:id/sample-logs", handler.ExecutionGetSampleLogs)

	// 统一执行接口（RESTful 风格）
	// POST   /api/executions              - 创建执行（支持 SSE 和阻塞模式）
	// GET    /api/executions/:sessionId   - 获取执行状态
	// DELETE /api/executions/:sessionId   - 停止执行
	// POST   /api/executions/:sessionId/interact - 提交交互响应
	executions := api.Group("/executions")
	executions.Post("", executionHandler.Execute)
	executions.Get("/:sessionId", executionHandler.GetExecutionStatus)
	executions.Delete("/:sessionId", executionHandler.StopExecution)
	executions.Post("/:sessionId/interact", executionHandler.SubmitInteraction)
}
