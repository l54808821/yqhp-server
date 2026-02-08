package logic

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"yqhp/gulu/internal/model"
	"yqhp/gulu/internal/query"
	"yqhp/gulu/internal/svc"
)

// AiModelLogic AI模型逻辑
type AiModelLogic struct {
	ctx context.Context
}

// NewAiModelLogic 创建AI模型逻辑
func NewAiModelLogic(ctx context.Context) *AiModelLogic {
	return &AiModelLogic{ctx: ctx}
}

// CreateAiModelReq 创建AI模型请求
type CreateAiModelReq struct {
	Name           string   `json:"name" validate:"required,max=100"`
	Provider       string   `json:"provider" validate:"required,max=100"`
	ModelID        string   `json:"model_id" validate:"required,max=200"`
	Version        string   `json:"version" validate:"max=50"`
	Description    string   `json:"description" validate:"max=1000"`
	APIBaseURL     string   `json:"api_base_url" validate:"required,max=500"`
	APIKey         string   `json:"api_key" validate:"required,max=500"`
	ContextLength  int32    `json:"context_length"`
	ParamSize      string   `json:"param_size" validate:"max=50"`
	CapabilityTags []string `json:"capability_tags"`
	CustomTags     []string `json:"custom_tags"`
	Sort           int32    `json:"sort"`
	Status         int32    `json:"status"`
}

// UpdateAiModelReq 更新AI模型请求
type UpdateAiModelReq struct {
	Name           string   `json:"name" validate:"max=100"`
	Provider       string   `json:"provider" validate:"max=100"`
	ModelID        string   `json:"model_id" validate:"max=200"`
	Version        string   `json:"version" validate:"max=50"`
	Description    string   `json:"description" validate:"max=1000"`
	APIBaseURL     string   `json:"api_base_url" validate:"max=500"`
	APIKey         string   `json:"api_key" validate:"max=500"`
	ContextLength  int32    `json:"context_length"`
	ParamSize      string   `json:"param_size" validate:"max=50"`
	CapabilityTags []string `json:"capability_tags"`
	CustomTags     []string `json:"custom_tags"`
	Sort           int32    `json:"sort"`
	Status         int32    `json:"status"`
}

// AiModelListReq AI模型列表请求
type AiModelListReq struct {
	Page     int    `query:"page" validate:"min=1"`
	PageSize int    `query:"pageSize" validate:"min=1,max=100"`
	Name     string `query:"name"`
	Provider string `query:"provider"`
	Status   *int32 `query:"status"`
}

// AiModelInfo AI模型返回信息（隐藏API Key）
type AiModelInfo struct {
	ID             int64      `json:"id"`
	CreatedAt      *time.Time `json:"created_at"`
	UpdatedAt      *time.Time `json:"updated_at"`
	CreatedBy      *int64     `json:"created_by"`
	Name           string     `json:"name"`
	Provider       string     `json:"provider"`
	ModelID        string     `json:"model_id"`
	Version        string     `json:"version"`
	Description    string     `json:"description"`
	APIBaseURL     string     `json:"api_base_url"`
	APIKeyMasked   string     `json:"api_key_masked"`
	ContextLength  int32      `json:"context_length"`
	ParamSize      string     `json:"param_size"`
	CapabilityTags []string   `json:"capability_tags"`
	CustomTags     []string   `json:"custom_tags"`
	Sort           int32      `json:"sort"`
	Status         int32      `json:"status"`
}

// Create 创建AI模型
func (l *AiModelLogic) Create(req *CreateAiModelReq) (*AiModelInfo, error) {
	now := time.Now()
	isDelete := false
	status := req.Status
	if status == 0 {
		status = 1 // 默认启用
	}

	// 序列化标签
	var capabilityTagsJSON *string
	if len(req.CapabilityTags) > 0 {
		b, err := json.Marshal(req.CapabilityTags)
		if err != nil {
			return nil, errors.New("能力标签序列化失败")
		}
		s := string(b)
		capabilityTagsJSON = &s
	}

	var customTagsJSON *string
	if len(req.CustomTags) > 0 {
		b, err := json.Marshal(req.CustomTags)
		if err != nil {
			return nil, errors.New("自定义标签序列化失败")
		}
		s := string(b)
		customTagsJSON = &s
	}

	aiModel := &model.TAiModel{
		CreatedAt:      &now,
		UpdatedAt:      &now,
		IsDelete:       &isDelete,
		Name:           req.Name,
		Provider:       req.Provider,
		ModelID:        req.ModelID,
		Version:        &req.Version,
		Description:    &req.Description,
		APIBaseURL:     req.APIBaseURL,
		APIKey:         req.APIKey,
		ContextLength:  &req.ContextLength,
		ParamSize:      &req.ParamSize,
		CapabilityTags: capabilityTagsJSON,
		CustomTags:     customTagsJSON,
		Sort:           &req.Sort,
		Status:         &status,
	}

	q := query.Use(svc.Ctx.DB)
	err := q.TAiModel.WithContext(l.ctx).Create(aiModel)
	if err != nil {
		return nil, err
	}

	return l.toAiModelInfo(aiModel), nil
}

// Update 更新AI模型
func (l *AiModelLogic) Update(id int64, req *UpdateAiModelReq) error {
	q := query.Use(svc.Ctx.DB)
	m := q.TAiModel

	// 检查是否存在
	_, err := m.WithContext(l.ctx).Where(m.ID.Eq(id), m.IsDelete.Is(false)).First()
	if err != nil {
		return errors.New("AI模型不存在")
	}

	now := time.Now()
	updates := map[string]interface{}{
		"updated_at": now,
	}

	if req.Name != "" {
		updates["name"] = req.Name
	}
	if req.Provider != "" {
		updates["provider"] = req.Provider
	}
	if req.ModelID != "" {
		updates["model_id"] = req.ModelID
	}
	updates["version"] = req.Version
	updates["description"] = req.Description
	if req.APIBaseURL != "" {
		updates["api_base_url"] = req.APIBaseURL
	}
	// 只在提供了新 API Key 时更新
	if req.APIKey != "" {
		updates["api_key"] = req.APIKey
	}
	updates["context_length"] = req.ContextLength
	updates["param_size"] = req.ParamSize
	updates["sort"] = req.Sort
	updates["status"] = req.Status

	// 序列化标签
	if req.CapabilityTags != nil {
		b, _ := json.Marshal(req.CapabilityTags)
		updates["capability_tags"] = string(b)
	}
	if req.CustomTags != nil {
		b, _ := json.Marshal(req.CustomTags)
		updates["custom_tags"] = string(b)
	}

	_, err = m.WithContext(l.ctx).Where(m.ID.Eq(id)).Updates(updates)
	return err
}

// Delete 删除AI模型（软删除）
func (l *AiModelLogic) Delete(id int64) error {
	q := query.Use(svc.Ctx.DB)
	m := q.TAiModel

	isDelete := true
	_, err := m.WithContext(l.ctx).Where(m.ID.Eq(id)).Update(m.IsDelete, isDelete)
	return err
}

// GetByID 根据ID获取AI模型
func (l *AiModelLogic) GetByID(id int64) (*AiModelInfo, error) {
	q := query.Use(svc.Ctx.DB)
	m := q.TAiModel

	aiModel, err := m.WithContext(l.ctx).Where(m.ID.Eq(id), m.IsDelete.Is(false)).First()
	if err != nil {
		return nil, err
	}

	return l.toAiModelInfo(aiModel), nil
}

// GetByIDWithKey 根据ID获取AI模型（含API Key，仅内部使用）
func (l *AiModelLogic) GetByIDWithKey(id int64) (*model.TAiModel, error) {
	q := query.Use(svc.Ctx.DB)
	m := q.TAiModel

	aiModel, err := m.WithContext(l.ctx).Where(m.ID.Eq(id), m.IsDelete.Is(false)).First()
	if err != nil {
		return nil, err
	}

	return aiModel, nil
}

// List 获取AI模型列表
func (l *AiModelLogic) List(req *AiModelListReq) ([]*AiModelInfo, int64, error) {
	q := query.Use(svc.Ctx.DB)
	m := q.TAiModel

	queryBuilder := m.WithContext(l.ctx).Where(m.IsDelete.Is(false))

	if req.Name != "" {
		queryBuilder = queryBuilder.Where(m.Name.Like("%" + req.Name + "%"))
	}
	if req.Provider != "" {
		// 支持逗号分隔的多厂商筛选
		providers := strings.Split(req.Provider, ",")
		if len(providers) == 1 {
			queryBuilder = queryBuilder.Where(m.Provider.Eq(providers[0]))
		} else {
			queryBuilder = queryBuilder.Where(m.Provider.In(providers...))
		}
	}
	if req.Status != nil {
		queryBuilder = queryBuilder.Where(m.Status.Eq(*req.Status))
	}

	// 获取总数
	total, err := queryBuilder.Count()
	if err != nil {
		return nil, 0, err
	}

	// 分页
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 10
	}

	offset := (req.Page - 1) * req.PageSize
	list, err := queryBuilder.Order(m.Sort.Desc(), m.ID.Desc()).Offset(offset).Limit(req.PageSize).Find()
	if err != nil {
		return nil, 0, err
	}

	result := make([]*AiModelInfo, 0, len(list))
	for _, item := range list {
		result = append(result, l.toAiModelInfo(item))
	}

	return result, total, nil
}

// UpdateStatus 更新AI模型状态
func (l *AiModelLogic) UpdateStatus(id int64, status int32) error {
	q := query.Use(svc.Ctx.DB)
	m := q.TAiModel

	now := time.Now()
	_, err := m.WithContext(l.ctx).Where(m.ID.Eq(id), m.IsDelete.Is(false)).Updates(map[string]interface{}{
		"status":     status,
		"updated_at": now,
	})
	return err
}

// GetProviders 获取所有厂商列表（用于筛选）
func (l *AiModelLogic) GetProviders() ([]string, error) {
	q := query.Use(svc.Ctx.DB)
	m := q.TAiModel

	var providers []string
	err := m.WithContext(l.ctx).Where(m.IsDelete.Is(false)).Distinct(m.Provider).Pluck(m.Provider, &providers)
	if err != nil {
		return nil, err
	}

	return providers, nil
}

// toAiModelInfo 转换为返回信息（隐藏API Key）
func (l *AiModelLogic) toAiModelInfo(m *model.TAiModel) *AiModelInfo {
	info := &AiModelInfo{
		ID:        m.ID,
		CreatedAt: m.CreatedAt,
		UpdatedAt: m.UpdatedAt,
		CreatedBy: m.CreatedBy,
		Name:      m.Name,
		Provider:  m.Provider,
		ModelID:   m.ModelID,
		APIBaseURL: m.APIBaseURL,
		APIKeyMasked: maskAPIKey(m.APIKey),
	}

	if m.Version != nil {
		info.Version = *m.Version
	}
	if m.Description != nil {
		info.Description = *m.Description
	}
	if m.ContextLength != nil {
		info.ContextLength = *m.ContextLength
	}
	if m.ParamSize != nil {
		info.ParamSize = *m.ParamSize
	}
	if m.Sort != nil {
		info.Sort = *m.Sort
	}
	if m.Status != nil {
		info.Status = *m.Status
	}

	// 解析能力标签
	if m.CapabilityTags != nil && *m.CapabilityTags != "" {
		var tags []string
		if err := json.Unmarshal([]byte(*m.CapabilityTags), &tags); err == nil {
			info.CapabilityTags = tags
		}
	}
	if info.CapabilityTags == nil {
		info.CapabilityTags = []string{}
	}

	// 解析自定义标签
	if m.CustomTags != nil && *m.CustomTags != "" {
		var tags []string
		if err := json.Unmarshal([]byte(*m.CustomTags), &tags); err == nil {
			info.CustomTags = tags
		}
	}
	if info.CustomTags == nil {
		info.CustomTags = []string{}
	}

	return info
}

// maskAPIKey 掩码 API Key
func maskAPIKey(key string) string {
	if len(key) <= 8 {
		return "***"
	}
	prefix := key[:3]
	suffix := key[len(key)-4:]
	return prefix + strings.Repeat("*", 6) + suffix
}
