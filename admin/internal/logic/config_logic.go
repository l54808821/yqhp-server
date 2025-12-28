package logic

import (
	"context"
	"errors"

	"yqhp/admin/internal/ctxutil"
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
	return &ConfigLogic{ctx: c.UserContext()}
}

func (l *ConfigLogic) db() *query.Query {
	return svc.Ctx.Query
}

// CreateConfig 创建配置
func (l *ConfigLogic) CreateConfig(req *types.CreateConfigRequest) (*types.ConfigInfo, error) {
	cfg := l.db().SysConfig
	count, _ := cfg.WithContext(l.ctx).Where(cfg.Key.Eq(req.Key), cfg.IsDelete.Is(false)).Count()
	if count > 0 {
		return nil, errors.New("配置键已存在")
	}

	userID := ctxutil.GetUserID(l.ctx)
	config := &model.SysConfig{
		Name:      req.Name,
		Key:       req.Key,
		Value:     model.StringPtr(req.Value),
		Type:      model.StringPtr(req.Type),
		IsBuilt:   model.BoolPtr(req.IsBuilt),
		Remark:    model.StringPtr(req.Remark),
		IsDelete:  model.BoolPtr(false),
		CreatedBy: model.Int64Ptr(userID),
		UpdatedBy: model.Int64Ptr(userID),
	}

	if err := cfg.WithContext(l.ctx).Create(config); err != nil {
		return nil, err
	}

	return types.ToConfigInfo(config), nil
}

// UpdateConfig 更新配置
func (l *ConfigLogic) UpdateConfig(req *types.UpdateConfigRequest) error {
	userID := ctxutil.GetUserID(l.ctx)
	cfg := l.db().SysConfig
	_, err := cfg.WithContext(l.ctx).Where(cfg.ID.Eq(int64(req.ID))).Updates(map[string]any{
		"name":       req.Name,
		"value":      req.Value,
		"type":       req.Type,
		"remark":     req.Remark,
		"updated_by": userID,
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
func (l *ConfigLogic) GetConfig(id int64) (*types.ConfigInfo, error) {
	cfg := l.db().SysConfig
	config, err := cfg.WithContext(l.ctx).Where(cfg.ID.Eq(id), cfg.IsDelete.Is(false)).First()
	if err != nil {
		return nil, err
	}
	return types.ToConfigInfo(config), nil
}

// GetConfigByKey 根据Key获取配置
func (l *ConfigLogic) GetConfigByKey(key string) (*types.ConfigInfo, error) {
	cfg := l.db().SysConfig
	config, err := cfg.WithContext(l.ctx).Where(cfg.Key.Eq(key), cfg.IsDelete.Is(false)).First()
	if err != nil {
		return nil, err
	}
	return types.ToConfigInfo(config), nil
}

// ListConfigs 获取配置列表
func (l *ConfigLogic) ListConfigs(req *types.ListConfigsRequest) ([]*types.ConfigInfo, int64, error) {
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
	if err != nil {
		return nil, 0, err
	}
	return types.ToConfigInfoList(configs), total, nil
}

// RefreshConfig 刷新配置缓存
func (l *ConfigLogic) RefreshConfig() error {
	return nil
}
