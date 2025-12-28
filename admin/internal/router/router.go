package router

import (
	"yqhp/admin/internal/auth"
	"yqhp/admin/internal/handler"
	"yqhp/admin/internal/logic"
	"yqhp/admin/internal/middleware"
	commonMiddleware "yqhp/common/middleware"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

// Setup 设置路由
func Setup(app *fiber.App, db *gorm.DB) {
	// 初始化逻辑层
	userLogic := logic.NewUserLogic(db)
	roleLogic := logic.NewRoleLogic(db)
	resourceLogic := logic.NewResourceLogic(db)
	deptLogic := logic.NewDeptLogic(db)
	dictLogic := logic.NewDictLogic(db)
	configLogic := logic.NewConfigLogic(db)
	oauthLogic := logic.NewOAuthLogic(db)
	tokenLogic := logic.NewTokenLogic(db)
	appLogic := logic.NewApplicationLogic(db)
	permissionService := auth.NewPermissionService(db)

	// 初始化处理器
	authHandler := handler.NewAuthHandler(userLogic, oauthLogic)
	userHandler := handler.NewUserHandler(userLogic)
	roleHandler := handler.NewRoleHandler(roleLogic)
	resourceHandler := handler.NewResourceHandler(resourceLogic)
	deptHandler := handler.NewDeptHandler(deptLogic)
	dictHandler := handler.NewDictHandler(dictLogic)
	configHandler := handler.NewConfigHandler(configLogic)
	tokenHandler := handler.NewTokenHandler(tokenLogic)
	oauthProviderHandler := handler.NewOAuthProviderHandler(oauthLogic)
	appHandler := handler.NewApplicationHandler(appLogic)

	// 全局中间件
	app.Use(commonMiddleware.CORS())
	app.Use(commonMiddleware.Logger())
	app.Use(commonMiddleware.Recover())

	// API路由组
	api := app.Group("/api")

	// 公开路由(无需认证)
	setupPublicRoutes(api, authHandler)

	// 需要认证的路由
	authApi := api.Group("", middleware.AuthMiddleware())
	setupAuthRoutes(authApi, authHandler, userHandler, roleHandler, resourceHandler,
		deptHandler, dictHandler, configHandler, tokenHandler, oauthProviderHandler, appHandler, permissionService)
}

// setupPublicRoutes 设置公开路由
func setupPublicRoutes(api fiber.Router, authHandler *handler.AuthHandler) {
	// 认证相关
	auth := api.Group("/auth")
	auth.Post("/login", authHandler.Login)
	auth.Post("/register", authHandler.Register) // 用户注册
	auth.Post("/logout", authHandler.Logout)     // 登出不需要认证，因为token可能已过期
	auth.Get("/oauth/providers", authHandler.GetOAuthProviders)
	auth.Get("/oauth/:provider/url", authHandler.GetOAuthURL)
	auth.Get("/oauth/:provider/callback", authHandler.OAuthCallback)
}

// setupAuthRoutes 设置需要认证的路由
func setupAuthRoutes(
	api fiber.Router,
	authHandler *handler.AuthHandler,
	userHandler *handler.UserHandler,
	roleHandler *handler.RoleHandler,
	resourceHandler *handler.ResourceHandler,
	deptHandler *handler.DeptHandler,
	dictHandler *handler.DictHandler,
	configHandler *handler.ConfigHandler,
	tokenHandler *handler.TokenHandler,
	oauthProviderHandler *handler.OAuthProviderHandler,
	appHandler *handler.ApplicationHandler,
	permissionService *auth.PermissionService,
) {
	// 认证相关
	authGroup := api.Group("/auth")
	// 注意：/logout 已移到公开路由中
	authGroup.Get("/user-info", authHandler.GetUserInfo)
	authGroup.Post("/change-password", authHandler.ChangePassword)
	authGroup.Get("/bindings", authHandler.GetUserBindings)
	authGroup.Post("/bind/:provider", authHandler.BindOAuth)
	authGroup.Delete("/unbind/:provider", authHandler.UnbindOAuth)

	// 用户菜单和权限码
	api.Get("/menus", resourceHandler.GetUserMenus)
	api.Get("/permissions", resourceHandler.GetUserPermissionCodes)

	// 系统管理
	system := api.Group("/system")

	// 用户管理
	users := system.Group("/users")
	users.Post("/list", userHandler.List) // 改为POST
	users.Get("/:id", userHandler.Get)
	users.Post("", middleware.PermissionMiddleware(permissionService, "system:user:add"), userHandler.Create)
	users.Put("", middleware.PermissionMiddleware(permissionService, "system:user:edit"), userHandler.Update)
	users.Delete("/:id", middleware.PermissionMiddleware(permissionService, "system:user:delete"), userHandler.Delete)
	users.Post("/:id/reset-password", middleware.PermissionMiddleware(permissionService, "system:user:resetPwd"), userHandler.ResetPassword)

	// 角色管理
	roles := system.Group("/roles")
	roles.Post("/list", roleHandler.List) // 改为POST
	roles.Get("/all", roleHandler.All)
	roles.Get("/:id", roleHandler.Get)
	roles.Get("/:id/resources", roleHandler.GetResourceIDs)
	roles.Post("", middleware.PermissionMiddleware(permissionService, "system:role:add"), roleHandler.Create)
	roles.Put("", middleware.PermissionMiddleware(permissionService, "system:role:edit"), roleHandler.Update)
	roles.Delete("/:id", middleware.PermissionMiddleware(permissionService, "system:role:delete"), roleHandler.Delete)

	// 资源/菜单管理
	resources := system.Group("/resources")
	resources.Get("/tree", resourceHandler.Tree)
	resources.Get("/all", resourceHandler.All)
	resources.Get("/:id", resourceHandler.Get)
	resources.Post("", middleware.PermissionMiddleware(permissionService, "system:resource:add"), resourceHandler.Create)
	resources.Put("", middleware.PermissionMiddleware(permissionService, "system:resource:edit"), resourceHandler.Update)
	resources.Delete("/:id", middleware.PermissionMiddleware(permissionService, "system:resource:delete"), resourceHandler.Delete)

	// 部门管理
	depts := system.Group("/depts")
	depts.Get("/tree", deptHandler.Tree)
	depts.Get("/all", deptHandler.All)
	depts.Get("/:id", deptHandler.Get)
	depts.Post("", middleware.PermissionMiddleware(permissionService, "system:dept:add"), deptHandler.Create)
	depts.Put("", middleware.PermissionMiddleware(permissionService, "system:dept:edit"), deptHandler.Update)
	depts.Delete("/:id", middleware.PermissionMiddleware(permissionService, "system:dept:delete"), deptHandler.Delete)

	// 字典管理
	dict := system.Group("/dict")
	// 字典类型
	dictTypes := dict.Group("/types")
	dictTypes.Post("/list", dictHandler.ListTypes) // 改为POST
	dictTypes.Get("/:id", dictHandler.GetType)
	dictTypes.Post("", middleware.PermissionMiddleware(permissionService, "system:dict:add"), dictHandler.CreateType)
	dictTypes.Put("", middleware.PermissionMiddleware(permissionService, "system:dict:edit"), dictHandler.UpdateType)
	dictTypes.Delete("/:id", middleware.PermissionMiddleware(permissionService, "system:dict:delete"), dictHandler.DeleteType)
	// 字典数据
	dictData := dict.Group("/data")
	dictData.Post("/list", dictHandler.ListData) // 改为POST
	dictData.Get("/type/:typeCode", dictHandler.GetDataByTypeCode)
	dictData.Post("", middleware.PermissionMiddleware(permissionService, "system:dict:add"), dictHandler.CreateData)
	dictData.Put("", middleware.PermissionMiddleware(permissionService, "system:dict:edit"), dictHandler.UpdateData)
	dictData.Delete("/:id", middleware.PermissionMiddleware(permissionService, "system:dict:delete"), dictHandler.DeleteData)

	// 配置管理
	configs := system.Group("/configs")
	configs.Post("/list", configHandler.List) // 改为POST
	configs.Get("/:id", configHandler.Get)
	configs.Get("/key/:key", configHandler.GetByKey)
	configs.Post("", middleware.PermissionMiddleware(permissionService, "system:config:add"), configHandler.Create)
	configs.Put("", middleware.PermissionMiddleware(permissionService, "system:config:edit"), configHandler.Update)
	configs.Delete("/:id", middleware.PermissionMiddleware(permissionService, "system:config:delete"), configHandler.Delete)
	configs.Post("/refresh", middleware.PermissionMiddleware(permissionService, "system:config:edit"), configHandler.Refresh)

	// 第三方登录管理
	oauthProviders := system.Group("/oauth-providers")
	oauthProviders.Post("/list", oauthProviderHandler.List) // 改为POST
	oauthProviders.Get("/:code", oauthProviderHandler.Get)
	oauthProviders.Post("", middleware.PermissionMiddleware(permissionService, "system:oauth:add"), oauthProviderHandler.Create)
	oauthProviders.Put("", middleware.PermissionMiddleware(permissionService, "system:oauth:edit"), oauthProviderHandler.Update)
	oauthProviders.Delete("/:id", middleware.PermissionMiddleware(permissionService, "system:oauth:delete"), oauthProviderHandler.Delete)

	// 令牌管理
	tokens := system.Group("/tokens")
	tokens.Post("/list", tokenHandler.List) // 改为POST
	tokens.Post("/kickout/:id", middleware.PermissionMiddleware(permissionService, "system:token:kickout"), tokenHandler.KickOut)
	tokens.Post("/kickout-user/:id", middleware.PermissionMiddleware(permissionService, "system:token:kickout"), tokenHandler.KickOutByUserID)
	tokens.Post("/kickout-by-token", middleware.PermissionMiddleware(permissionService, "system:token:kickout"), tokenHandler.KickOutByToken)
	tokens.Post("/disable/:id", middleware.PermissionMiddleware(permissionService, "system:token:disable"), tokenHandler.DisableUser)
	tokens.Post("/enable/:id", middleware.PermissionMiddleware(permissionService, "system:token:enable"), tokenHandler.EnableUser)

	// 应用管理
	apps := system.Group("/applications")
	apps.Post("/list", appHandler.List)
	apps.Get("/all", appHandler.All)
	apps.Get("/:id", appHandler.Get)
	apps.Post("", middleware.PermissionMiddleware(permissionService, "system:app:add"), appHandler.Create)
	apps.Put("", middleware.PermissionMiddleware(permissionService, "system:app:edit"), appHandler.Update)
	apps.Delete("/:id", middleware.PermissionMiddleware(permissionService, "system:app:delete"), appHandler.Delete)

	// 日志管理
	logs := system.Group("/logs")
	logs.Post("/login", tokenHandler.GetLoginLogs)         // 改为POST
	logs.Post("/operation", tokenHandler.GetOperationLogs) // 改为POST
	logs.Delete("/login", middleware.PermissionMiddleware(permissionService, "system:log:delete"), tokenHandler.ClearLoginLogs)
	logs.Delete("/operation", middleware.PermissionMiddleware(permissionService, "system:log:delete"), tokenHandler.ClearOperationLogs)
}
