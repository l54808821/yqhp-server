package main

import (
	"fmt"

	"yqhp/admin/internal/config"

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

	// 生成指定表的模型和查询代码
	// 使用 ApplyBasic 会同时生成 model 和 query
	g.ApplyBasic(
		g.GenerateModel("sys_application"),
		g.GenerateModel("sys_config"),
		g.GenerateModel("sys_dept"),
		g.GenerateModel("sys_dict_data"),
		g.GenerateModel("sys_dict_type"),
		g.GenerateModel("sys_login_log"),
		g.GenerateModel("sys_oauth_provider"),
		g.GenerateModel("sys_oauth_user"),
		g.GenerateModel("sys_operation_log"),
		g.GenerateModel("sys_resource"),
		g.GenerateModel("sys_role"),
		g.GenerateModel("sys_role_resource"),
		g.GenerateModel("sys_user"),
		g.GenerateModel("sys_user_role"),
		g.GenerateModel("sys_user_token"),
	)

	// 执行生成
	g.Execute()

	fmt.Println("代码生成完成!")
}
