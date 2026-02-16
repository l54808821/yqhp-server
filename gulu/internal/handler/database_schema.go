package handler

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"

	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"yqhp/common/response"
	"yqhp/gulu/internal/workflow"
)

// TableInfo 表信息
type TableInfo struct {
	Name    string `json:"name"`
	Comment string `json:"comment,omitempty"`
}

// ColumnInfo 字段信息
type ColumnInfo struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Comment  string `json:"comment,omitempty"`
	Nullable bool   `json:"nullable"`
}

// DatabaseGetTables 获取数据库的表列表
// GET /api/database/:configCode/tables?envId=xx
func DatabaseGetTables(c *fiber.Ctx) error {
	configCode := c.Params("configCode")
	if configCode == "" {
		return response.Error(c, "configCode 不能为空")
	}

	envID, err := strconv.ParseInt(c.Query("envId"), 10, 64)
	if err != nil || envID <= 0 {
		return response.Error(c, "envId 无效")
	}

	dbConfig, err := getDatabaseConfig(c.Context(), envID, configCode)
	if err != nil {
		return response.Error(c, err.Error())
	}

	tables, err := queryTables(dbConfig)
	if err != nil {
		return response.Error(c, "查询表列表失败: "+err.Error())
	}

	return response.Success(c, tables)
}

// DatabaseGetColumns 获取指定表的字段列表
// GET /api/database/:configCode/columns?envId=xx&table=xx
func DatabaseGetColumns(c *fiber.Ctx) error {
	configCode := c.Params("configCode")
	if configCode == "" {
		return response.Error(c, "configCode 不能为空")
	}

	envID, err := strconv.ParseInt(c.Query("envId"), 10, 64)
	if err != nil || envID <= 0 {
		return response.Error(c, "envId 无效")
	}

	tableName := c.Query("table")
	if tableName == "" {
		return response.Error(c, "table 不能为空")
	}

	dbConfig, err := getDatabaseConfig(c.Context(), envID, configCode)
	if err != nil {
		return response.Error(c, err.Error())
	}

	columns, err := queryColumns(dbConfig, tableName)
	if err != nil {
		return response.Error(c, "查询字段列表失败: "+err.Error())
	}

	return response.Success(c, columns)
}

// getDatabaseConfig 根据 envId 和 configCode 获取数据库配置
func getDatabaseConfig(ctx context.Context, envID int64, configCode string) (*workflow.DatabaseConfig, error) {
	merger := workflow.NewConfigMerger(ctx, envID)
	mergedConfig, err := merger.Merge()
	if err != nil {
		return nil, fmt.Errorf("获取配置失败: %w", err)
	}

	dbConfig, ok := mergedConfig.Databases[configCode]
	if !ok || dbConfig == nil {
		return nil, fmt.Errorf("数据库配置 %s 不存在", configCode)
	}

	return dbConfig, nil
}

// openSchemaDB 打开用于 Schema 查询的 gorm 数据库连接
func openSchemaDB(dbConfig *workflow.DatabaseConfig) (*gorm.DB, string, error) {
	dsn := buildDSN(dbConfig)
	if dsn == "" {
		return nil, "", fmt.Errorf("无法构建 DSN")
	}

	dbType := strings.ToLower(dbConfig.Type)

	var dialector gorm.Dialector
	switch dbType {
	case "mysql":
		dialector = mysql.Open(dsn)
	case "postgres", "postgresql":
		dialector = postgres.Open(dsn)
	default:
		return nil, "", fmt.Errorf("不支持的数据库类型: %s（仅支持 MySQL 和 PostgreSQL 的 Schema 查询）", dbConfig.Type)
	}

	db, err := gorm.Open(dialector, &gorm.Config{})
	if err != nil {
		return nil, "", fmt.Errorf("连接数据库失败: %w", err)
	}

	// 设置连接池参数：用完即关
	sqlDB, err := db.DB()
	if err != nil {
		return nil, "", fmt.Errorf("获取底层连接失败: %w", err)
	}
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(0)
	sqlDB.SetConnMaxLifetime(10 * time.Second)

	return db, dbType, nil
}

// closeSchemaDB 关闭 Schema 查询数据库连接
func closeSchemaDB(db *gorm.DB) {
	if sqlDB, err := db.DB(); err == nil {
		sqlDB.Close()
	}
}

// queryTables 查询表列表
func queryTables(dbConfig *workflow.DatabaseConfig) ([]TableInfo, error) {
	db, dbType, err := openSchemaDB(dbConfig)
	if err != nil {
		return nil, err
	}
	defer closeSchemaDB(db)

	var tables []TableInfo

	switch dbType {
	case "mysql":
		err = db.Raw(
			`SELECT TABLE_NAME as name, IFNULL(TABLE_COMMENT, '') as comment 
			 FROM information_schema.tables 
			 WHERE table_schema = ? AND table_type = 'BASE TABLE'
			 ORDER BY TABLE_NAME`,
			dbConfig.Database,
		).Scan(&tables).Error

	case "postgres", "postgresql":
		err = db.Raw(
			`SELECT tablename as name, 
			        COALESCE(obj_description((schemaname || '.' || tablename)::regclass), '') as comment 
			 FROM pg_tables 
			 WHERE schemaname = 'public' 
			 ORDER BY tablename`,
		).Scan(&tables).Error

	default:
		return []TableInfo{}, nil
	}

	if err != nil {
		return nil, fmt.Errorf("查询失败: %w", err)
	}

	if tables == nil {
		tables = []TableInfo{}
	}

	return tables, nil
}

// queryColumns 查询指定表的字段列表
func queryColumns(dbConfig *workflow.DatabaseConfig, tableName string) ([]ColumnInfo, error) {
	db, dbType, err := openSchemaDB(dbConfig)
	if err != nil {
		return nil, err
	}
	defer closeSchemaDB(db)

	switch dbType {
	case "mysql":
		return queryMySQLColumns(db, dbConfig.Database, tableName)
	case "postgres", "postgresql":
		return queryPostgresColumns(db, tableName)
	default:
		return []ColumnInfo{}, nil
	}
}

// mysqlColumnRow MySQL 字段查询结果行
type mysqlColumnRow struct {
	Name     string `gorm:"column:COLUMN_NAME"`
	Type     string `gorm:"column:COLUMN_TYPE"`
	Comment  string `gorm:"column:COLUMN_COMMENT"`
	Nullable string `gorm:"column:IS_NULLABLE"`
}

func queryMySQLColumns(db *gorm.DB, database, tableName string) ([]ColumnInfo, error) {
	var rows []mysqlColumnRow
	err := db.Raw(
		`SELECT COLUMN_NAME, COLUMN_TYPE, IFNULL(COLUMN_COMMENT, '') as COLUMN_COMMENT, IS_NULLABLE
		 FROM information_schema.columns
		 WHERE table_schema = ? AND table_name = ?
		 ORDER BY ORDINAL_POSITION`,
		database, tableName,
	).Scan(&rows).Error

	if err != nil {
		return nil, fmt.Errorf("查询失败: %w", err)
	}

	columns := make([]ColumnInfo, 0, len(rows))
	for _, r := range rows {
		columns = append(columns, ColumnInfo{
			Name:     r.Name,
			Type:     r.Type,
			Comment:  r.Comment,
			Nullable: strings.ToUpper(r.Nullable) == "YES",
		})
	}

	return columns, nil
}

// pgColumnRow PostgreSQL 字段查询结果行
type pgColumnRow struct {
	Name     string `gorm:"column:column_name"`
	Type     string `gorm:"column:data_type"`
	Comment  string `gorm:"column:comment"`
	Nullable string `gorm:"column:is_nullable"`
}

func queryPostgresColumns(db *gorm.DB, tableName string) ([]ColumnInfo, error) {
	var rows []pgColumnRow
	err := db.Raw(
		`SELECT c.column_name, c.data_type,
		        COALESCE(pgd.description, '') as comment,
		        c.is_nullable
		 FROM information_schema.columns c
		 LEFT JOIN pg_catalog.pg_statio_all_tables st 
		   ON st.relname = c.table_name AND st.schemaname = c.table_schema
		 LEFT JOIN pg_catalog.pg_description pgd 
		   ON pgd.objoid = st.relid AND pgd.objsubid = c.ordinal_position
		 WHERE c.table_schema = 'public' AND c.table_name = ?
		 ORDER BY c.ordinal_position`,
		tableName,
	).Scan(&rows).Error

	if err != nil {
		return nil, fmt.Errorf("查询失败: %w", err)
	}

	columns := make([]ColumnInfo, 0, len(rows))
	for _, r := range rows {
		columns = append(columns, ColumnInfo{
			Name:     r.Name,
			Type:     r.Type,
			Comment:  r.Comment,
			Nullable: strings.ToUpper(r.Nullable) == "YES",
		})
	}

	return columns, nil
}
