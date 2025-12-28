package main

import (
	"fmt"

	"gorm.io/driver/mysql"
	"gorm.io/gen"
	"gorm.io/gorm"
)

// 用于从数据库生成模型和查询代码
// 使用方法: go run cmd/gen/main.go

func main() {
	// 数据库连接配置
	dsn := "root:root@tcp(127.0.0.1:3306)/yqhp_admin?charset=utf8mb4&parseTime=True&loc=Local"

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		panic(fmt.Errorf("连接数据库失败: %w", err))
	}

	// 创建gen配置
	g := gen.NewGenerator(gen.Config{
		OutPath:           "./internal/query",   // 输出路径
		ModelPkgPath:      "./internal/model",   // 模型包路径
		Mode:              gen.WithDefaultQuery | gen.WithQueryInterface,
		FieldNullable:     true,
		FieldCoverable:    false,
		FieldSignable:     false,
		FieldWithIndexTag: true,
		FieldWithTypeTag:  true,
	})

	// 使用数据库连接
	g.UseDB(db)

	// 生成所有表的模型
	// 可以指定表名生成特定表
	// g.GenerateModel("sys_user")
	// g.GenerateModel("sys_role")
	// g.GenerateModel("sys_resource")
	// g.GenerateModel("sys_dept")
	// g.GenerateModel("sys_dict_type")
	// g.GenerateModel("sys_dict_data")
	// g.GenerateModel("sys_config")
	// g.GenerateModel("sys_oauth_provider")
	// g.GenerateModel("sys_oauth_user")
	// g.GenerateModel("sys_user_token")
	// g.GenerateModel("sys_login_log")
	// g.GenerateModel("sys_operation_log")

	// 生成所有表
	g.GenerateAllTable()

	// 执行生成
	g.Execute()

	fmt.Println("代码生成完成!")
}

