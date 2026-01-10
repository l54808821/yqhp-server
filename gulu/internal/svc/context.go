package svc

import (
	"yqhp/gulu/internal/config"
	"yqhp/gulu/internal/query"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

// ServiceContext 全局服务上下文
type ServiceContext struct {
	Config *config.Config
	DB     *gorm.DB
	Redis  *redis.Client
}

var Ctx *ServiceContext

// Init 初始化服务上下文
func Init(cfg *config.Config, db *gorm.DB, rdb *redis.Client) {
	// 初始化 query 包
	query.SetDefault(db)

	Ctx = &ServiceContext{
		Config: cfg,
		DB:     db,
		Redis:  rdb,
	}
}
