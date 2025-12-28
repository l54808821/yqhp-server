package logic

import (
	"context"
	"errors"

	"yqhp/admin/internal/model"
	"yqhp/admin/internal/query"
	"yqhp/admin/internal/svc"
	"yqhp/admin/internal/types"

	"github.com/gofiber/fiber/v2"
)

// ConfigLogic 系统配置逻辑
type ConfigLogic struct {
	ctx context.Context
}

// NewConfigLogic 创建配置逻辑
func NewConfigLogic(c *fiber.Ctx) *ConfigLogic {
	return &ConfigLogic{ctx: c.Context()}
}

func (l *ConfigLogic) db() *query.Query {
	return svc.Ctx.Query
}

// CreateConfig 创建配置
func (l *ConfigLogic) CreateConfig(req *types.CreateConfigRequest) (*model.SysConfig, error) {
	cfg := l.db().SysConfig
	count, _ := cfg.WithContext(l.ctx).Where(cfg.Key.Eq(req.Key), cfg.IsDelete.Is(false)).Count()
	if count > 0 {
		return nil, errors.New("配置键已存在")
	}

	config := &model.SysConfig{
		Name:     req.Name,
		Key:      req.Key,
		Value:    model.StringPtr(req.Value),
		Type:     model.StringPtr(req.Type),
		IsBuilt:  model.BoolPtr(req.IsBuilt),
		Remark:   model.StringPtr(req.Remark),
		IsDelete: model.BoolPtr(false),
	}

	if err := cfg.WithContext(l.ctx).Create(config); err != nil {
		return nil, err
	}

	return config, nil
}

// UpdateConfig 更新配置
func (l *ConfigLogic) UpdateConfig(req *types.UpdateConfigRequest) error {
	cfg := l.db().SysConfig
	_, err := cfg.WithContext(l.ctx).Where(cfg.ID.Eq(int64(req.ID))).Updates(map[string]any{
		"name":   req.Name,
		"value":  req.Value,
		"type":   req.Type,
		"remark": req.Remark,
	})
	return err
}

// DeleteConfig 删除配置
func (l *ConfigLogic) DeleteConfig(id int64) error {
	cfg := l.db().SysConfig
	config, err := cfg.WithContext(l.ctx).Where(cfg.ID.Eq(id), cfg.IsDelete.Is(false)).First()
	if err != nil {
		return err
	}

	if model.GetBool(config.IsBuilt) {
		return errors.New("内置配置不允许删除")
	}

	_, err = cfg.WithContext(l.ctx).Where(cfg.ID.Eq(id)).Update(cfg.IsDelete, true)
	return err
}

// GetConfig 获取配置详情
func (l *ConfigLogic) GetConfig(id int64) (*model.SysConfig, error) {
	cfg := l.db().SysConfig
	return cfg.WithContext(l.ctx).Where(cfg.ID.Eq(id), cfg.IsDelete.Is(false)).First()
}

// GetConfigByKey 根据Key获取配置
func (l *ConfigLogic) GetConfigByKey(key string) (*model.SysConfig, error) {
	cfg := l.db().SysConfig
	return cfg.WithContext(l.ctx).Where(cfg.Key.Eq(key), cfg.IsDelete.Is(false)).First()
}

// ListConfigs 获取配置列表
func (l *ConfigLogic) ListConfigs(req *types.ListConfigsRequest) ([]*model.SysConfig, int64, error) {
	cfg := l.db().SysConfig
	q := cfg.WithContext(l.ctx).Where(cfg.IsDelete.Is(false))

	if req.Name != "" {
		q = q.Where(cfg.Name.Like("%" + req.Name + "%"))
	}
	if req.Key != "" {
		q = q.Where(cfg.Key.Like("%" + req.Key + "%"))
	}

	total, _ := q.Count()

	if req.Page > 0 && req.PageSize > 0 {
		q = q.Offset((req.Page - 1) * req.PageSize).Limit(req.PageSize)
	}

	configs, err := q.Find()
	return configs, total, err
}

// RefreshConfig 刷新配置缓存
func (l *ConfigLogic) RefreshConfig() error {
	return nil
}
