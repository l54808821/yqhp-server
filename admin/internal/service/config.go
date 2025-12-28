package service

import (
	"errors"

	"yqhp/admin/internal/model"

	"gorm.io/gorm"
)

// ConfigService 系统配置服务
type ConfigService struct {
	db *gorm.DB
}

// NewConfigService 创建配置服务
func NewConfigService(db *gorm.DB) *ConfigService {
	return &ConfigService{db: db}
}

// CreateConfigRequest 创建配置请求
type CreateConfigRequest struct {
	Name    string `json:"name" validate:"required"`
	Key     string `json:"key" validate:"required"`
	Value   string `json:"value"`
	Type    string `json:"type"`
	IsBuilt bool   `json:"isBuilt"`
	Remark  string `json:"remark"`
}

// CreateConfig 创建配置
func (s *ConfigService) CreateConfig(req *CreateConfigRequest) (*model.SysConfig, error) {
	// 检查Key是否存在
	var count int64
	s.db.Model(&model.SysConfig{}).Where("`key` = ?", req.Key).Count(&count)
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

	if err := s.db.Create(config).Error; err != nil {
		return nil, err
	}

	return config, nil
}

// UpdateConfigRequest 更新配置请求
type UpdateConfigRequest struct {
	ID     uint   `json:"id" validate:"required"`
	Name   string `json:"name"`
	Value  string `json:"value"`
	Type   string `json:"type"`
	Remark string `json:"remark"`
}

// UpdateConfig 更新配置
func (s *ConfigService) UpdateConfig(req *UpdateConfigRequest) error {
	// 检查是否为内置配置
	var config model.SysConfig
	if err := s.db.First(&config, req.ID).Error; err != nil {
		return err
	}

	updates := map[string]any{
		"name":   req.Name,
		"value":  req.Value,
		"type":   req.Type,
		"remark": req.Remark,
	}

	return s.db.Model(&model.SysConfig{}).Where("id = ?", req.ID).Updates(updates).Error
}

// DeleteConfig 删除配置
func (s *ConfigService) DeleteConfig(id uint) error {
	// 检查是否为内置配置
	var config model.SysConfig
	if err := s.db.First(&config, id).Error; err != nil {
		return err
	}

	if config.IsBuilt {
		return errors.New("内置配置不允许删除")
	}

	return s.db.Delete(&config).Error
}

// GetConfig 获取配置详情
func (s *ConfigService) GetConfig(id uint) (*model.SysConfig, error) {
	var config model.SysConfig
	if err := s.db.First(&config, id).Error; err != nil {
		return nil, err
	}
	return &config, nil
}

// GetConfigByKey 根据Key获取配置
func (s *ConfigService) GetConfigByKey(key string) (*model.SysConfig, error) {
	var config model.SysConfig
	if err := s.db.Where("`key` = ?", key).First(&config).Error; err != nil {
		return nil, err
	}
	return &config, nil
}

// GetConfigValue 获取配置值
func (s *ConfigService) GetConfigValue(key string) (string, error) {
	config, err := s.GetConfigByKey(key)
	if err != nil {
		return "", err
	}
	return config.Value, nil
}

// ListConfigsRequest 配置列表请求
type ListConfigsRequest struct {
	Page     int    `json:"page"`
	PageSize int    `json:"pageSize"`
	Name     string `json:"name"`
	Key      string `json:"key"`
}

// ListConfigs 获取配置列表
func (s *ConfigService) ListConfigs(req *ListConfigsRequest) ([]model.SysConfig, int64, error) {
	var configs []model.SysConfig
	var total int64

	query := s.db.Model(&model.SysConfig{})

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
func (s *ConfigService) RefreshConfig() error {
	// 这里可以实现配置缓存刷新逻辑
	return nil
}

