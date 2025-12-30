package router

import (
	"yqhp/admin/internal/auth"
	"yqhp/admin/internal/handler"
	"yqhp/admin/internal/middleware"
	commonMiddleware "yqhp/common/middleware"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

// Setup 设置路由
func Setup(app *fiber.App, db *gorm.DB) {
	// 权限中间件简写
	ps := auth.NewPermissionService(db)
	perm := func(code string) fiber.Handler { return middleware.PermissionMiddleware(ps, code) }

	// 全局中间件
	app.Use(commonMiddleware.CORS(), commonMiddleware.RequestID(), commonMiddleware.Logger(), commonMiddleware.Recover())

	api := app.Group("/api")

	// ========== 公开路由 ==========
	pub := api.Group("/auth")
	pub.Post("/login", handler.AuthLogin)
	pub.Post("/register", handler.AuthRegister)
	pub.Post("/logout", handler.AuthLogout)
	pub.Get("/oauth/providers", handler.AuthGetOAuthProviders)
	pub.Get("/oauth/:provider/url", handler.AuthGetOAuthURL)
	pub.Get("/oauth/:provider/callback", handler.AuthOAuthCallback)

	// ========== 需要认证的路由 ==========
	authed := api.Group("", middleware.AuthMiddleware())

	// 认证相关
	ag := authed.Group("/auth")
	ag.Get("/user-info", handler.AuthGetUserInfo)
	ag.Post("/change-password", handler.AuthChangePassword)
	ag.Get("/bindings", handler.AuthGetUserBindings)
	ag.Post("/bind/:provider", handler.AuthBindOAuth)
	ag.Delete("/unbind/:provider", handler.AuthUnbindOAuth)

	// 用户菜单和权限码
	authed.Get("/menus", handler.ResourceGetUserMenus)
	authed.Get("/permissions", handler.ResourceGetUserPermissionCodes)

	// ========== 系统管理 ==========
	sys := authed.Group("/system")

	// 用户管理
	u := sys.Group("/users")
	u.Post("/list", handler.UserList)
	u.Post("/batch", handler.UserBatchGet)
	u.Get("/:id", handler.UserGet)
	u.Post("", perm("system:user:add"), handler.UserCreate)
	u.Put("", perm("system:user:edit"), handler.UserUpdate)
	u.Delete("/:id", perm("system:user:delete"), handler.UserDelete)
	u.Post("/:id/reset-password", perm("system:user:resetPwd"), handler.UserResetPassword)

	// 角色管理
	r := sys.Group("/roles")
	r.Post("/list", handler.RoleList)
	r.Get("/all", handler.RoleAll)
	r.Get("/:id", handler.RoleGet)
	r.Get("/:id/resources", handler.RoleGetResourceIDs)
	r.Post("", perm("system:role:add"), handler.RoleCreate)
	r.Put("", perm("system:role:edit"), handler.RoleUpdate)
	r.Delete("/:id", perm("system:role:delete"), handler.RoleDelete)

	// 资源/菜单管理
	res := sys.Group("/resources")
	res.Get("/tree", handler.ResourceTree)
	res.Get("/all", handler.ResourceAll)
	res.Get("/:id", handler.ResourceGet)
	res.Post("", perm("system:resource:add"), handler.ResourceCreate)
	res.Put("", perm("system:resource:edit"), handler.ResourceUpdate)
	res.Delete("/:id", perm("system:resource:delete"), handler.ResourceDelete)

	// 部门管理
	d := sys.Group("/depts")
	d.Get("/tree", handler.DeptTree)
	d.Get("/all", handler.DeptAll)
	d.Get("/:id", handler.DeptGet)
	d.Post("", perm("system:dept:add"), handler.DeptCreate)
	d.Put("", perm("system:dept:edit"), handler.DeptUpdate)
	d.Delete("/:id", perm("system:dept:delete"), handler.DeptDelete)

	// 字典类型
	dt := sys.Group("/dict/types")
	dt.Post("/list", handler.DictListTypes)
	dt.Get("/:id", handler.DictGetType)
	dt.Post("", perm("system:dict:add"), handler.DictCreateType)
	dt.Put("", perm("system:dict:edit"), handler.DictUpdateType)
	dt.Delete("/:id", perm("system:dict:delete"), handler.DictDeleteType)

	// 字典数据
	dd := sys.Group("/dict/data")
	dd.Post("/list", handler.DictListData)
	dd.Get("/type/:typeCode", handler.DictGetDataByTypeCode)
	dd.Post("", perm("system:dict:add"), handler.DictCreateData)
	dd.Put("", perm("system:dict:edit"), handler.DictUpdateData)
	dd.Delete("/:id", perm("system:dict:delete"), handler.DictDeleteData)

	// 配置管理
	cfg := sys.Group("/configs")
	cfg.Post("/list", handler.ConfigList)
	cfg.Get("/:id", handler.ConfigGet)
	cfg.Get("/key/:key", handler.ConfigGetByKey)
	cfg.Post("", perm("system:config:add"), handler.ConfigCreate)
	cfg.Put("", perm("system:config:edit"), handler.ConfigUpdate)
	cfg.Delete("/:id", perm("system:config:delete"), handler.ConfigDelete)
	cfg.Post("/refresh", perm("system:config:edit"), handler.ConfigRefresh)

	// 第三方登录管理
	oauth := sys.Group("/oauth-providers")
	oauth.Post("/list", handler.OAuthProviderList)
	oauth.Get("/:code", handler.OAuthProviderGet)
	oauth.Post("", perm("system:oauth:add"), handler.OAuthProviderCreate)
	oauth.Put("", perm("system:oauth:edit"), handler.OAuthProviderUpdate)
	oauth.Delete("/:id", perm("system:oauth:delete"), handler.OAuthProviderDelete)

	// 令牌管理
	tk := sys.Group("/tokens")
	tk.Post("/list", handler.TokenList)
	tk.Post("/kickout/:id", perm("system:token:kickout"), handler.TokenKickOut)
	tk.Post("/kickout-user/:id", perm("system:token:kickout"), handler.TokenKickOutByUserID)
	tk.Post("/kickout-by-token", perm("system:token:kickout"), handler.TokenKickOutByToken)
	tk.Post("/disable/:id", perm("system:token:disable"), handler.TokenDisableUser)
	tk.Post("/enable/:id", perm("system:token:enable"), handler.TokenEnableUser)

	// 应用管理
	apps := sys.Group("/applications")
	apps.Post("/list", handler.AppList)
	apps.Get("/all", handler.AppAll)
	apps.Get("/:id", handler.AppGet)
	apps.Post("", perm("system:app:add"), handler.AppCreate)
	apps.Put("", perm("system:app:edit"), handler.AppUpdate)
	apps.Delete("/:id", perm("system:app:delete"), handler.AppDelete)

	// 日志管理
	logs := sys.Group("/logs")
	logs.Post("/login", handler.TokenGetLoginLogs)
	logs.Post("/operation", handler.TokenGetOperationLogs)
	logs.Delete("/login", perm("system:log:delete"), handler.TokenClearLoginLogs)
	logs.Delete("/operation", perm("system:log:delete"), handler.TokenClearOperationLogs)

	// 用户-应用关联
	userApps := sys.Group("/user-apps")
	userApps.Post("/list", handler.UserAppList)
	userApps.Get("/user/:userId", handler.UserAppGetByUser)
	userApps.Get("/app/:appId", handler.UserAppGetByApp)

	// 表格视图配置
	tableViews := sys.Group("/table-views")
	tableViews.Get("/:tableKey", handler.TableViewGet)
	tableViews.Post("", handler.TableViewSave)
	tableViews.Put("/:tableKey/default/:id", handler.TableViewSetDefault)
	tableViews.Put("/:tableKey/sort", handler.TableViewUpdateSort)
	tableViews.Delete("/:id", handler.TableViewDelete)
}
