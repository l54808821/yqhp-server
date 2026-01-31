package main

import (
	"fmt"

	"yqhp/gulu/internal/config"

	"gorm.io/driver/mysql"
	"gorm.io/gen"
	"gorm.io/gorm"
)

// 用于从数据库生成模型和查询代码
// 使用方法: go run cmd/gen/main.go

func main() {
	// 加载配置文件
	cfg, err := config.LoadConfig("config/config.yml")
	if err != nil {
		panic(fmt.Errorf("加载配置文件失败: %w", err))
	}

	// 构建数据库连接字符串
	dbCfg := cfg.Database
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=%s&parseTime=True&loc=Local",
		dbCfg.Username,
		dbCfg.Password,
		dbCfg.Host,
		dbCfg.Port,
		dbCfg.Database,
		dbCfg.Charset,
	)

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		panic(fmt.Errorf("连接数据库失败: %w", err))
	}

	// 创建gen配置
	g := gen.NewGenerator(gen.Config{
		OutPath:           "./internal/query",
		ModelPkgPath:      "./internal/model",
		Mode:              gen.WithDefaultQuery | gen.WithQueryInterface,
		FieldNullable:     true,
		FieldCoverable:    false,
		FieldSignable:     false,
		FieldWithIndexTag: true,
		FieldWithTypeTag:  true,
	})

	// 使用数据库连接
	g.UseDB(db)

	// 生成 Gulu 扩展功能相关表的模型和查询代码
	g.ApplyBasic(
		// 团队管理
		g.GenerateModel("t_team"),
		g.GenerateModel("t_team_member"),
		// 项目管理
		g.GenerateModel("t_project"),
		g.GenerateModel("t_project_member"),
		g.GenerateModel("t_project_permission"),
		// 工作流分类
		g.GenerateModel("t_category_workflow"),
		// 环境管理
		g.GenerateModel("t_env"),
		// 配置管理
		g.GenerateModel("t_config_definition"),
		g.GenerateModel("t_config"),
		// 执行机管理
		g.GenerateModel("t_executor"),
		// 工作流管理
		g.GenerateModel("t_workflow"),
		// 执行记录
		g.GenerateModel("t_execution"),
	)

	// 执行生成
	g.Execute()

	fmt.Println("Gulu 模型代码生成完成!")
}
