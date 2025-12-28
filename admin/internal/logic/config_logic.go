package logic

import (
	"errors"

	"yqhp/admin/internal/model"
	"yqhp/admin/internal/types"

	"gorm.io/gorm"
)

// ConfigLogic 系统配置逻辑
type ConfigLogic struct {
	db *gorm.DB
}

// NewConfigLogic 创建配置逻辑
func NewConfigLogic(db *gorm.DB) *ConfigLogic {
	return &ConfigLogic{db: db}
}

// CreateConfig 创建配置
func (l *ConfigLogic) CreateConfig(req *types.CreateConfigRequest) (*model.SysConfig, error) {
	// 检查Key是否存在
	var count int64
	l.db.Model(&model.SysConfig{}).Where("`key` = ? AND is_delete = ?", req.Key, false).Count(&count)
	if count > 0 {
		return nil, errors.New("配置键已存在")
	}

	config := &model.SysConfig{
		Name:    req.Name,
		Key:     req.Key,
		Value:   req.Value,
		Type:    req.Type,
		IsBuilt: req.IsBuilt,
		Remark:  req.Remark,
	}

	if err := l.db.Create(config).Error; err != nil {
		return nil, err
	}

	return config, nil
}

// UpdateConfig 更新配置
func (l *ConfigLogic) UpdateConfig(req *types.UpdateConfigRequest) error {
	// 检查是否为内置配置
	var config model.SysConfig
	if err := l.db.Where("is_delete = ?", false).First(&config, req.ID).Error; err != nil {
		return err
	}

	updates := map[string]any{
		"name":   req.Name,
		"value":  req.Value,
		"type":   req.Type,
		"remark": req.Remark,
	}

	return l.db.Model(&model.SysConfig{}).Where("id = ?", req.ID).Updates(updates).Error
}

// DeleteConfig 删除配置（软删除）
func (l *ConfigLogic) DeleteConfig(id uint) error {
	// 检查是否为内置配置
	var config model.SysConfig
	if err := l.db.Where("is_delete = ?", false).First(&config, id).Error; err != nil {
		return err
	}

	if config.IsBuilt {
		return errors.New("内置配置不允许删除")
	}

	return l.db.Model(&config).Update("is_delete", true).Error
}

// GetConfig 获取配置详情
func (l *ConfigLogic) GetConfig(id uint) (*model.SysConfig, error) {
	var config model.SysConfig
	if err := l.db.Where("is_delete = ?", false).First(&config, id).Error; err != nil {
		return nil, err
	}
	return &config, nil
}

// GetConfigByKey 根据Key获取配置
func (l *ConfigLogic) GetConfigByKey(key string) (*model.SysConfig, error) {
	var config model.SysConfig
	if err := l.db.Where("`key` = ? AND is_delete = ?", key, false).First(&config).Error; err != nil {
		return nil, err
	}
	return &config, nil
}

// GetConfigValue 获取配置值
func (l *ConfigLogic) GetConfigValue(key string) (string, error) {
	config, err := l.GetConfigByKey(key)
	if err != nil {
		return "", err
	}
	return config.Value, nil
}

// ListConfigs 获取配置列表
func (l *ConfigLogic) ListConfigs(req *types.ListConfigsRequest) ([]model.SysConfig, int64, error) {
	var configs []model.SysConfig
	var total int64

	query := l.db.Model(&model.SysConfig{}).Where("is_delete = ?", false)

	if req.Name != "" {
		query = query.Where("name LIKE ?", "%"+req.Name+"%")
	}
	if req.Key != "" {
		query = query.Where("`key` LIKE ?", "%"+req.Key+"%")
	}

	query.Count(&total)

	if req.Page > 0 && req.PageSize > 0 {
		offset := (req.Page - 1) * req.PageSize
		query = query.Offset(offset).Limit(req.PageSize)
	}

	if err := query.Find(&configs).Error; err != nil {
		return nil, 0, err
	}

	return configs, total, nil
}

// RefreshConfig 刷新配置缓存(可扩展)
func (l *ConfigLogic) RefreshConfig() error {
	// 这里可以实现配置缓存刷新逻辑
	return nil
}
