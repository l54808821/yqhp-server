package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"yqhp/admin/internal/auth"
	"yqhp/admin/internal/config"
	"yqhp/admin/internal/model"
	"yqhp/admin/internal/router"
	"yqhp/common/database"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

func main() {
	// 加载配置
	cfg, err := config.LoadConfig("config/config.yml")
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}

	// 初始化数据库
	if err := database.Init(&cfg.Database); err != nil {
		log.Fatalf("初始化数据库失败: %v", err)
	}
	defer database.Close()

	// 自动迁移数据库表
	db := database.GetDB()
	if err := db.AutoMigrate(
		&model.Application{},
		&model.User{},
		&model.Role{},
		&model.Resource{},
		&model.Dept{},
		&model.DictType{},
		&model.DictData{},
		&model.SysConfig{},
		&model.OAuthProvider{},
		&model.OAuthUser{},
		&model.UserToken{},
		&model.LoginLog{},
		&model.OperationLog{},
		&model.UserRole{},
		&model.RoleResource{},
	); err != nil {
		log.Fatalf("数据库迁移失败: %v", err)
	}

	// 初始化默认数据
	initDefaultData(db)

	// 初始化SaToken
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
	router.Setup(app, db)

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

// initDefaultData 初始化默认数据
func initDefaultData(db *gorm.DB) {
	// 检查是否已有管理员用户
	var count int64
	db.Model(&model.User{}).Count(&count)
	if count > 0 {
		return
	}

	log.Println("初始化默认数据...")

	// 创建默认应用（后台管理系统）
	adminApp := &model.Application{
		Name:        "后台管理系统",
		Code:        model.AppCodeAdmin,
		Description: "系统管理后台",
		Icon:        "ant-design:setting-outlined",
		Sort:        1,
		Status:      1,
	}
	db.Create(adminApp)

	// 创建默认部门
	dept := &model.Dept{
		Name:   "总公司",
		Code:   "HQ",
		Status: 1,
	}
	db.Create(dept)

	// 创建默认角色
	adminRole := &model.Role{
		AppID:  adminApp.ID,
		Name:   "超级管理员",
		Code:   "admin",
		Status: 1,
		Remark: "拥有所有权限",
	}
	db.Create(adminRole)

	// 创建默认用户
	adminUser := &model.User{
		Username: "admin",
		Password: "e10adc3949ba59abbe56e057f20f883e", // 123456 的 MD5
		Nickname: "管理员",
		Status:   1,
		DeptID:   dept.ID,
	}
	db.Create(adminUser)

	// 关联用户和角色
	db.Create(&model.UserRole{
		UserID: adminUser.ID,
		RoleID: adminRole.ID,
	})

	// 创建默认菜单
	createDefaultMenus(db, adminApp.ID, adminRole.ID)

	// 创建默认第三方登录配置
	createDefaultOAuthProviders(db)

	log.Println("默认数据初始化完成")
}

// createDefaultMenus 创建默认菜单
func createDefaultMenus(db *gorm.DB, appID uint, adminRoleID uint) {
	// 系统管理目录
	systemMenu := &model.Resource{
		AppID:  appID,
		Name:   "系统管理",
		Code:   "system",
		Type:   1,
		Path:   "/system",
		Icon:   "ant-design:setting-outlined",
		Sort:   1,
		Status: 1,
	}
	db.Create(systemMenu)

	// 应用管理
	appMenu := &model.Resource{
		AppID:     appID,
		ParentID:  systemMenu.ID,
		Name:      "应用管理",
		Code:      "system:app",
		Type:      2,
		Path:      "/system/app",
		Component: "system/app/index",
		Icon:      "ant-design:appstore-outlined",
		Sort:      0,
		Status:    1,
	}
	db.Create(appMenu)

	// 应用管理按钮
	appButtons := []model.Resource{
		{AppID: appID, ParentID: appMenu.ID, Name: "新增应用", Code: "system:app:add", Type: 3, Sort: 1, Status: 1},
		{AppID: appID, ParentID: appMenu.ID, Name: "编辑应用", Code: "system:app:edit", Type: 3, Sort: 2, Status: 1},
		{AppID: appID, ParentID: appMenu.ID, Name: "删除应用", Code: "system:app:delete", Type: 3, Sort: 3, Status: 1},
	}
	for _, btn := range appButtons {
		db.Create(&btn)
	}

	// 用户管理
	userMenu := &model.Resource{
		AppID:     appID,
		ParentID:  systemMenu.ID,
		Name:      "用户管理",
		Code:      "system:user",
		Type:      2,
		Path:      "/system/user",
		Component: "system/user/index",
		Icon:      "ant-design:user-outlined",
		Sort:      1,
		Status:    1,
	}
	db.Create(userMenu)

	// 用户管理按钮
	userButtons := []model.Resource{
		{AppID: appID, ParentID: userMenu.ID, Name: "新增用户", Code: "system:user:add", Type: 3, Sort: 1, Status: 1},
		{AppID: appID, ParentID: userMenu.ID, Name: "编辑用户", Code: "system:user:edit", Type: 3, Sort: 2, Status: 1},
		{AppID: appID, ParentID: userMenu.ID, Name: "删除用户", Code: "system:user:delete", Type: 3, Sort: 3, Status: 1},
		{AppID: appID, ParentID: userMenu.ID, Name: "重置密码", Code: "system:user:resetPwd", Type: 3, Sort: 4, Status: 1},
	}
	for _, btn := range userButtons {
		db.Create(&btn)
	}

	// 角色管理
	roleMenu := &model.Resource{
		AppID:     appID,
		ParentID:  systemMenu.ID,
		Name:      "角色管理",
		Code:      "system:role",
		Type:      2,
		Path:      "/system/role",
		Component: "system/role/index",
		Icon:      "ant-design:team-outlined",
		Sort:      2,
		Status:    1,
	}
	db.Create(roleMenu)

	// 角色管理按钮
	roleButtons := []model.Resource{
		{AppID: appID, ParentID: roleMenu.ID, Name: "新增角色", Code: "system:role:add", Type: 3, Sort: 1, Status: 1},
		{AppID: appID, ParentID: roleMenu.ID, Name: "编辑角色", Code: "system:role:edit", Type: 3, Sort: 2, Status: 1},
		{AppID: appID, ParentID: roleMenu.ID, Name: "删除角色", Code: "system:role:delete", Type: 3, Sort: 3, Status: 1},
	}
	for _, btn := range roleButtons {
		db.Create(&btn)
	}

	// 菜单管理
	menuMenu := &model.Resource{
		AppID:     appID,
		ParentID:  systemMenu.ID,
		Name:      "菜单管理",
		Code:      "system:resource",
		Type:      2,
		Path:      "/system/menu",
		Component: "system/menu/index",
		Icon:      "ant-design:menu-outlined",
		Sort:      3,
		Status:    1,
	}
	db.Create(menuMenu)

	// 菜单管理按钮
	menuButtons := []model.Resource{
		{AppID: appID, ParentID: menuMenu.ID, Name: "新增菜单", Code: "system:resource:add", Type: 3, Sort: 1, Status: 1},
		{AppID: appID, ParentID: menuMenu.ID, Name: "编辑菜单", Code: "system:resource:edit", Type: 3, Sort: 2, Status: 1},
		{AppID: appID, ParentID: menuMenu.ID, Name: "删除菜单", Code: "system:resource:delete", Type: 3, Sort: 3, Status: 1},
	}
	for _, btn := range menuButtons {
		db.Create(&btn)
	}

	// 部门管理
	deptMenu := &model.Resource{
		AppID:     appID,
		ParentID:  systemMenu.ID,
		Name:      "部门管理",
		Code:      "system:dept",
		Type:      2,
		Path:      "/system/dept",
		Component: "system/dept/index",
		Icon:      "ant-design:apartment-outlined",
		Sort:      4,
		Status:    1,
	}
	db.Create(deptMenu)

	// 部门管理按钮
	deptButtons := []model.Resource{
		{AppID: appID, ParentID: deptMenu.ID, Name: "新增部门", Code: "system:dept:add", Type: 3, Sort: 1, Status: 1},
		{AppID: appID, ParentID: deptMenu.ID, Name: "编辑部门", Code: "system:dept:edit", Type: 3, Sort: 2, Status: 1},
		{AppID: appID, ParentID: deptMenu.ID, Name: "删除部门", Code: "system:dept:delete", Type: 3, Sort: 3, Status: 1},
	}
	for _, btn := range deptButtons {
		db.Create(&btn)
	}

	// 字典管理
	dictMenu := &model.Resource{
		AppID:     appID,
		ParentID:  systemMenu.ID,
		Name:      "字典管理",
		Code:      "system:dict",
		Type:      2,
		Path:      "/system/dict",
		Component: "system/dict/index",
		Icon:      "ant-design:book-outlined",
		Sort:      5,
		Status:    1,
	}
	db.Create(dictMenu)

	// 字典管理按钮
	dictButtons := []model.Resource{
		{AppID: appID, ParentID: dictMenu.ID, Name: "新增字典", Code: "system:dict:add", Type: 3, Sort: 1, Status: 1},
		{AppID: appID, ParentID: dictMenu.ID, Name: "编辑字典", Code: "system:dict:edit", Type: 3, Sort: 2, Status: 1},
		{AppID: appID, ParentID: dictMenu.ID, Name: "删除字典", Code: "system:dict:delete", Type: 3, Sort: 3, Status: 1},
	}
	for _, btn := range dictButtons {
		db.Create(&btn)
	}

	// 参数配置
	configMenu := &model.Resource{
		AppID:     appID,
		ParentID:  systemMenu.ID,
		Name:      "参数配置",
		Code:      "system:config",
		Type:      2,
		Path:      "/system/config",
		Component: "system/config/index",
		Icon:      "ant-design:tool-outlined",
		Sort:      6,
		Status:    1,
	}
	db.Create(configMenu)

	// 参数配置按钮
	configButtons := []model.Resource{
		{AppID: appID, ParentID: configMenu.ID, Name: "新增配置", Code: "system:config:add", Type: 3, Sort: 1, Status: 1},
		{AppID: appID, ParentID: configMenu.ID, Name: "编辑配置", Code: "system:config:edit", Type: 3, Sort: 2, Status: 1},
		{AppID: appID, ParentID: configMenu.ID, Name: "删除配置", Code: "system:config:delete", Type: 3, Sort: 3, Status: 1},
	}
	for _, btn := range configButtons {
		db.Create(&btn)
	}

	// 第三方登录
	oauthMenu := &model.Resource{
		AppID:     appID,
		ParentID:  systemMenu.ID,
		Name:      "第三方登录",
		Code:      "system:oauth",
		Type:      2,
		Path:      "/system/oauth",
		Component: "system/oauth/index",
		Icon:      "ant-design:api-outlined",
		Sort:      7,
		Status:    1,
	}
	db.Create(oauthMenu)

	// 第三方登录按钮
	oauthButtons := []model.Resource{
		{AppID: appID, ParentID: oauthMenu.ID, Name: "新增配置", Code: "system:oauth:add", Type: 3, Sort: 1, Status: 1},
		{AppID: appID, ParentID: oauthMenu.ID, Name: "编辑配置", Code: "system:oauth:edit", Type: 3, Sort: 2, Status: 1},
		{AppID: appID, ParentID: oauthMenu.ID, Name: "删除配置", Code: "system:oauth:delete", Type: 3, Sort: 3, Status: 1},
	}
	for _, btn := range oauthButtons {
		db.Create(&btn)
	}

	// 令牌管理
	tokenMenu := &model.Resource{
		AppID:     appID,
		ParentID:  systemMenu.ID,
		Name:      "令牌管理",
		Code:      "system:token",
		Type:      2,
		Path:      "/system/token",
		Component: "system/token/index",
		Icon:      "ant-design:key-outlined",
		Sort:      8,
		Status:    1,
	}
	db.Create(tokenMenu)

	// 令牌管理按钮
	tokenButtons := []model.Resource{
		{AppID: appID, ParentID: tokenMenu.ID, Name: "踢人下线", Code: "system:token:kickout", Type: 3, Sort: 1, Status: 1},
		{AppID: appID, ParentID: tokenMenu.ID, Name: "禁用用户", Code: "system:token:disable", Type: 3, Sort: 2, Status: 1},
		{AppID: appID, ParentID: tokenMenu.ID, Name: "解禁用户", Code: "system:token:enable", Type: 3, Sort: 3, Status: 1},
	}
	for _, btn := range tokenButtons {
		db.Create(&btn)
	}

	// 日志管理
	logMenu := &model.Resource{
		AppID:     appID,
		ParentID:  systemMenu.ID,
		Name:      "日志管理",
		Code:      "system:log",
		Type:      2,
		Path:      "/system/log",
		Component: "system/log/index",
		Icon:      "ant-design:file-text-outlined",
		Sort:      9,
		Status:    1,
	}
	db.Create(logMenu)

	// 日志管理按钮
	logButtons := []model.Resource{
		{AppID: appID, ParentID: logMenu.ID, Name: "清空日志", Code: "system:log:delete", Type: 3, Sort: 1, Status: 1},
	}
	for _, btn := range logButtons {
		db.Create(&btn)
	}

	// 为管理员角色分配所有菜单权限
	var resources []model.Resource
	db.Find(&resources)
	for _, resource := range resources {
		db.Create(&model.RoleResource{
			RoleID:     adminRoleID,
			ResourceID: resource.ID,
		})
	}
}

// createDefaultOAuthProviders 创建默认第三方登录配置
func createDefaultOAuthProviders(db *gorm.DB) {
	providers := []model.OAuthProvider{
		{
			Name:         "GitHub",
			Code:         "github",
			ClientID:     "your_github_client_id",
			ClientSecret: "your_github_client_secret",
			RedirectURI:  "http://localhost:5666/auth/oauth/github/callback",
			AuthURL:      "https://github.com/login/oauth/authorize",
			TokenURL:     "https://github.com/login/oauth/access_token",
			UserInfoURL:  "https://api.github.com/user",
			Scope:        "user:email",
			Status:       1,
			Sort:         1,
			Icon:         "github",
			Remark:       "GitHub 第三方登录",
		},
		{
			Name:         "微信",
			Code:         "wechat",
			ClientID:     "your_wechat_appid",
			ClientSecret: "your_wechat_secret",
			RedirectURI:  "http://localhost:5555/api/auth/oauth/wechat/callback",
			AuthURL:      "https://open.weixin.qq.com/connect/qrconnect",
			TokenURL:     "https://api.weixin.qq.com/sns/oauth2/access_token",
			UserInfoURL:  "https://api.weixin.qq.com/sns/userinfo",
			Scope:        "snsapi_login",
			Status:       1,
			Sort:         2,
			Icon:         "wechat",
			Remark:       "微信扫码登录",
		},
		{
			Name:         "飞书",
			Code:         "feishu",
			ClientID:     "your_feishu_app_id",
			ClientSecret: "your_feishu_app_secret",
			RedirectURI:  "http://localhost:5555/api/auth/oauth/feishu/callback",
			AuthURL:      "https://open.feishu.cn/open-apis/authen/v1/authorize",
			TokenURL:     "https://open.feishu.cn/open-apis/authen/v1/oidc/access_token",
			UserInfoURL:  "https://open.feishu.cn/open-apis/authen/v1/user_info",
			Scope:        "",
			Status:       1,
			Sort:         3,
			Icon:         "feishu",
			Remark:       "飞书第三方登录",
		},
		{
			Name:         "钉钉",
			Code:         "dingtalk",
			ClientID:     "your_dingtalk_appkey",
			ClientSecret: "your_dingtalk_appsecret",
			RedirectURI:  "http://localhost:5555/api/auth/oauth/dingtalk/callback",
			AuthURL:      "https://login.dingtalk.com/oauth2/auth",
			TokenURL:     "https://api.dingtalk.com/v1.0/oauth2/userAccessToken",
			UserInfoURL:  "https://api.dingtalk.com/v1.0/contact/users/me",
			Scope:        "openid",
			Status:       1,
			Sort:         4,
			Icon:         "dingtalk",
			Remark:       "钉钉第三方登录",
		},
		{
			Name:         "QQ",
			Code:         "qq",
			ClientID:     "your_qq_appid",
			ClientSecret: "your_qq_appkey",
			RedirectURI:  "http://localhost:5555/api/auth/oauth/qq/callback",
			AuthURL:      "https://graph.qq.com/oauth2.0/authorize",
			TokenURL:     "https://graph.qq.com/oauth2.0/token",
			UserInfoURL:  "https://graph.qq.com/user/get_user_info",
			Scope:        "get_user_info",
			Status:       1,
			Sort:         5,
			Icon:         "qq",
			Remark:       "QQ第三方登录",
		},
		{
			Name:         "Gitee",
			Code:         "gitee",
			ClientID:     "your_gitee_client_id",
			ClientSecret: "your_gitee_client_secret",
			RedirectURI:  "http://localhost:5555/api/auth/oauth/gitee/callback",
			AuthURL:      "https://gitee.com/oauth/authorize",
			TokenURL:     "https://gitee.com/oauth/token",
			UserInfoURL:  "https://gitee.com/api/v5/user",
			Scope:        "user_info",
			Status:       1,
			Sort:         6,
			Icon:         "gitee",
			Remark:       "Gitee 第三方登录",
		},
	}

	for _, provider := range providers {
		db.Create(&provider)
	}
}
