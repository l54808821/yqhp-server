package svc

import (
	"yqhp/admin/internal/config"
	"yqhp/admin/internal/query"

	"gorm.io/gorm"
)

// ServiceContext 全局服务上下文
type ServiceContext struct {
	Config *config.Config
	DB     *gorm.DB
	Query  *query.Query
}

var Ctx *ServiceContext

// Init 初始化服务上下文
func Init(cfg *config.Config, db *gorm.DB) {
	query.SetDefault(db)
	Ctx = &ServiceContext{
		Config: cfg,
		DB:     db,
		Query:  query.Q,
	}
}
