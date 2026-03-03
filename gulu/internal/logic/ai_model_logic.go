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

func NewAiModelLogic(ctx context.Context) *AiModelLogic {
	return &AiModelLogic{ctx: ctx}
}

// ========== 请求/响应结构 ==========

type CreateAiModelReq struct {
	ProviderID     int64    `json:"provider_id"`
	Name           string   `json:"name" validate:"required,max=100"`
	Provider       string   `json:"provider" validate:"max=100"`
	ModelID        string   `json:"model_id" validate:"required,max=200"`
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

type UpdateAiModelReq struct {
	ProviderID     *int64   `json:"provider_id"`
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

// BatchCreateModelReq 批量创建模型请求（用于供应商下批量添加）
type BatchCreateModelReq struct {
	ProviderID int64              `json:"provider_id" validate:"required"`
	Models     []BatchModelEntry  `json:"models" validate:"required,min=1"`
}

type BatchModelEntry struct {
	Name           string   `json:"name" validate:"required,max=100"`
	ModelID        string   `json:"model_id" validate:"required,max=200"`
	Description    string   `json:"description"`
	ContextLength  int32    `json:"context_length"`
	ParamSize      string   `json:"param_size"`
	CapabilityTags []string `json:"capability_tags"`
	CustomTags     []string `json:"custom_tags"`
}

type AiModelListReq struct {
	Page       int    `query:"page" validate:"min=1"`
	PageSize   int    `query:"pageSize" validate:"min=1,max=100"`
	Name       string `query:"name"`
	Provider   string `query:"provider"`
	ProviderID *int64 `query:"provider_id"`
	Status     *int32 `query:"status"`
}

type AiModelInfo struct {
	ID             int64      `json:"id"`
	CreatedAt      *time.Time `json:"created_at"`
	UpdatedAt      *time.Time `json:"updated_at"`
	CreatedBy      *int64     `json:"created_by"`
	ProviderID     int64      `json:"provider_id"`
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

// ModelWithCredentials 包含完整凭证的模型信息（仅内部使用）
type ModelWithCredentials struct {
	ID         int64
	Name       string
	Provider   string
	ModelID    string
	APIBaseURL string
	APIKey     string
	Status     *int32
}

// ========== CRUD ==========

func (l *AiModelLogic) Create(req *CreateAiModelReq) (*AiModelInfo, error) {
	now := time.Now()
	isDelete := false
	status := req.Status
	if status == 0 {
		status = 1
	}

	capabilityTagsJSON := serializeTags(req.CapabilityTags)
	customTagsJSON := serializeTags(req.CustomTags)

	providerName := req.Provider
	if req.ProviderID > 0 && providerName == "" {
		pl := NewAiProviderLogic(l.ctx)
		p, err := pl.GetByIDWithKey(req.ProviderID)
		if err == nil {
			providerName = p.Name
		}
	}

	aiModel := &model.TAiModel{
		CreatedAt:      &now,
		UpdatedAt:      &now,
		IsDelete:       &isDelete,
		ProviderID:     req.ProviderID,
		Name:           req.Name,
		Provider:       providerName,
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

// BatchCreate 批量创建模型（在某个供应商下）
func (l *AiModelLogic) BatchCreate(req *BatchCreateModelReq) ([]*AiModelInfo, error) {
	pl := NewAiProviderLogic(l.ctx)
	provider, err := pl.GetByIDWithKey(req.ProviderID)
	if err != nil {
		return nil, errors.New("供应商不存在")
	}

	now := time.Now()
	isDelete := false
	status := int32(1)

	var models []*model.TAiModel
	for _, entry := range req.Models {
		capJSON := serializeTags(entry.CapabilityTags)
		customJSON := serializeTags(entry.CustomTags)

		m := &model.TAiModel{
			CreatedAt:      &now,
			UpdatedAt:      &now,
			IsDelete:       &isDelete,
			ProviderID:     req.ProviderID,
			Name:           entry.Name,
			Provider:       provider.Name,
			ModelID:        entry.ModelID,
			Description:    &entry.Description,
			ContextLength:  &entry.ContextLength,
			ParamSize:      &entry.ParamSize,
			CapabilityTags: capJSON,
			CustomTags:     customJSON,
			Sort:           new(int32),
			Status:         &status,
		}
		models = append(models, m)
	}

	q := query.Use(svc.Ctx.DB)
	if err := q.TAiModel.WithContext(l.ctx).Create(models...); err != nil {
		return nil, err
	}

	result := make([]*AiModelInfo, 0, len(models))
	for _, m := range models {
		result = append(result, l.toAiModelInfo(m))
	}
	return result, nil
}

func (l *AiModelLogic) Update(id int64, req *UpdateAiModelReq) error {
	q := query.Use(svc.Ctx.DB)
	m := q.TAiModel

	_, err := m.WithContext(l.ctx).Where(m.ID.Eq(id), m.IsDelete.Is(false)).First()
	if err != nil {
		return errors.New("AI模型不存在")
	}

	now := time.Now()
	updates := map[string]interface{}{
		"updated_at": now,
	}

	if req.ProviderID != nil {
		updates["provider_id"] = *req.ProviderID
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
	if req.APIKey != "" {
		updates["api_key"] = req.APIKey
	}
	updates["context_length"] = req.ContextLength
	updates["param_size"] = req.ParamSize
	updates["sort"] = req.Sort
	updates["status"] = req.Status

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

func (l *AiModelLogic) Delete(id int64) error {
	q := query.Use(svc.Ctx.DB)
	m := q.TAiModel

	isDelete := true
	_, err := m.WithContext(l.ctx).Where(m.ID.Eq(id)).Update(m.IsDelete, isDelete)
	return err
}

func (l *AiModelLogic) GetByID(id int64) (*AiModelInfo, error) {
	q := query.Use(svc.Ctx.DB)
	m := q.TAiModel

	aiModel, err := m.WithContext(l.ctx).Where(m.ID.Eq(id), m.IsDelete.Is(false)).First()
	if err != nil {
		return nil, err
	}

	return l.toAiModelInfo(aiModel), nil
}

// GetByIDWithKey 根据ID获取AI模型（含完整凭证，仅内部使用）
// 如果模型关联了供应商（provider_id > 0），从供应商获取 api_key 和 api_base_url
func (l *AiModelLogic) GetByIDWithKey(id int64) (*ModelWithCredentials, error) {
	q := query.Use(svc.Ctx.DB)
	m := q.TAiModel

	aiModel, err := m.WithContext(l.ctx).Where(m.ID.Eq(id), m.IsDelete.Is(false)).First()
	if err != nil {
		return nil, err
	}

	result := &ModelWithCredentials{
		ID:         aiModel.ID,
		Name:       aiModel.Name,
		Provider:   aiModel.Provider,
		ModelID:    aiModel.ModelID,
		APIBaseURL: aiModel.APIBaseURL,
		APIKey:     aiModel.APIKey,
		Status:     aiModel.Status,
	}

	// 优先从供应商获取凭证
	if aiModel.ProviderID > 0 {
		pl := NewAiProviderLogic(l.ctx)
		provider, err := pl.GetByIDWithKey(aiModel.ProviderID)
		if err == nil {
			result.APIBaseURL = provider.APIBaseURL
			result.APIKey = provider.APIKey
			if result.Provider == "" {
				result.Provider = provider.Name
			}
		}
	}

	return result, nil
}

func (l *AiModelLogic) List(req *AiModelListReq) ([]*AiModelInfo, int64, error) {
	q := query.Use(svc.Ctx.DB)
	m := q.TAiModel

	queryBuilder := m.WithContext(l.ctx).Where(m.IsDelete.Is(false))

	if req.Name != "" {
		queryBuilder = queryBuilder.Where(m.Name.Like("%" + req.Name + "%"))
	}
	if req.Provider != "" {
		providers := strings.Split(req.Provider, ",")
		if len(providers) == 1 {
			queryBuilder = queryBuilder.Where(m.Provider.Eq(providers[0]))
		} else {
			queryBuilder = queryBuilder.Where(m.Provider.In(providers...))
		}
	}
	if req.ProviderID != nil && *req.ProviderID > 0 {
		queryBuilder = queryBuilder.Where(m.ProviderID.Eq(*req.ProviderID))
	}
	if req.Status != nil {
		queryBuilder = queryBuilder.Where(m.Status.Eq(*req.Status))
	}

	total, err := queryBuilder.Count()
	if err != nil {
		return nil, 0, err
	}

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

// ========== 辅助方法 ==========

func (l *AiModelLogic) toAiModelInfo(m *model.TAiModel) *AiModelInfo {
	info := &AiModelInfo{
		ID:           m.ID,
		CreatedAt:    m.CreatedAt,
		UpdatedAt:    m.UpdatedAt,
		CreatedBy:    m.CreatedBy,
		ProviderID:   m.ProviderID,
		Name:         m.Name,
		Provider:     m.Provider,
		ModelID:      m.ModelID,
		APIBaseURL:   m.APIBaseURL,
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

	if m.CapabilityTags != nil && *m.CapabilityTags != "" {
		var tags []string
		if err := json.Unmarshal([]byte(*m.CapabilityTags), &tags); err == nil {
			info.CapabilityTags = tags
		}
	}
	if info.CapabilityTags == nil {
		info.CapabilityTags = []string{}
	}

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

func serializeTags(tags []string) *string {
	if len(tags) == 0 {
		return nil
	}
	b, err := json.Marshal(tags)
	if err != nil {
		return nil
	}
	s := string(b)
	return &s
}

func maskAPIKey(key string) string {
	if len(key) <= 8 {
		return "***"
	}
	prefix := key[:3]
	suffix := key[len(key)-4:]
	return prefix + strings.Repeat("*", 6) + suffix
}
