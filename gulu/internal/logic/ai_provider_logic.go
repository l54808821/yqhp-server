package logic

import (
	"context"
	"errors"
	"time"

	"yqhp/gulu/internal/model"
	"yqhp/gulu/internal/svc"
)

// AiProviderLogic AI供应商逻辑
type AiProviderLogic struct {
	ctx context.Context
}

func NewAiProviderLogic(ctx context.Context) *AiProviderLogic {
	return &AiProviderLogic{ctx: ctx}
}

// ========== 请求/响应结构 ==========

type CreateAiProviderReq struct {
	Name         string `json:"name" validate:"required,max=100"`
	ProviderType string `json:"provider_type" validate:"required,max=50"`
	APIBaseURL   string `json:"api_base_url" validate:"required,max=500"`
	APIKey       string `json:"api_key" validate:"max=500"`
	Icon         string `json:"icon"`
	Description  string `json:"description"`
	Sort         int32  `json:"sort"`
	Status       int32  `json:"status"`
}

type UpdateAiProviderReq struct {
	Name         string `json:"name" validate:"max=100"`
	ProviderType string `json:"provider_type" validate:"max=50"`
	APIBaseURL   string `json:"api_base_url" validate:"max=500"`
	APIKey       string `json:"api_key" validate:"max=500"`
	Icon         string `json:"icon"`
	Description  string `json:"description"`
	Sort         int32  `json:"sort"`
	Status       int32  `json:"status"`
}

type AiProviderListReq struct {
	Page     int    `query:"page" validate:"min=1"`
	PageSize int    `query:"pageSize" validate:"min=1,max=100"`
	Name     string `query:"name"`
	Status   *int32 `query:"status"`
}

type AiProviderInfo struct {
	ID           int64      `json:"id"`
	CreatedAt    *time.Time `json:"created_at"`
	UpdatedAt    *time.Time `json:"updated_at"`
	Name         string     `json:"name"`
	ProviderType string     `json:"provider_type"`
	APIBaseURL   string     `json:"api_base_url"`
	APIKeyMasked string     `json:"api_key_masked"`
	Icon         string     `json:"icon"`
	Description  string     `json:"description"`
	Sort         int32      `json:"sort"`
	Status       int32      `json:"status"`
	ModelCount   int64      `json:"model_count"`
}

// ========== CRUD ==========

func (l *AiProviderLogic) Create(req *CreateAiProviderReq) (*AiProviderInfo, error) {
	now := time.Now()
	isDelete := false
	status := req.Status
	if status == 0 {
		status = 1
	}

	provider := &model.TAiProvider{
		CreatedAt:    &now,
		UpdatedAt:    &now,
		IsDelete:     &isDelete,
		Name:         req.Name,
		ProviderType: req.ProviderType,
		APIBaseURL:   req.APIBaseURL,
		APIKey:       req.APIKey,
		Sort:         &req.Sort,
		Status:       &status,
	}
	if req.Icon != "" {
		provider.Icon = &req.Icon
	}
	if req.Description != "" {
		provider.Description = &req.Description
	}

	if err := svc.Ctx.DB.WithContext(l.ctx).Create(provider).Error; err != nil {
		return nil, err
	}

	return l.toProviderInfo(provider), nil
}

func (l *AiProviderLogic) Update(id int64, req *UpdateAiProviderReq) error {
	db := svc.Ctx.DB.WithContext(l.ctx)

	var existing model.TAiProvider
	if err := db.Where("id = ? AND is_delete = 0", id).First(&existing).Error; err != nil {
		return errors.New("AI供应商不存在")
	}

	now := time.Now()
	updates := map[string]interface{}{
		"updated_at": now,
	}

	if req.Name != "" {
		updates["name"] = req.Name
	}
	if req.ProviderType != "" {
		updates["provider_type"] = req.ProviderType
	}
	if req.APIBaseURL != "" {
		updates["api_base_url"] = req.APIBaseURL
	}
	if req.APIKey != "" {
		updates["api_key"] = req.APIKey
	}
	updates["icon"] = req.Icon
	updates["description"] = req.Description
	updates["sort"] = req.Sort
	updates["status"] = req.Status

	return db.Model(&model.TAiProvider{}).Where("id = ?", id).Updates(updates).Error
}

func (l *AiProviderLogic) Delete(id int64) error {
	db := svc.Ctx.DB.WithContext(l.ctx)
	return db.Model(&model.TAiProvider{}).Where("id = ?", id).Update("is_delete", true).Error
}

func (l *AiProviderLogic) GetByID(id int64) (*AiProviderInfo, error) {
	db := svc.Ctx.DB.WithContext(l.ctx)

	var provider model.TAiProvider
	if err := db.Where("id = ? AND is_delete = 0", id).First(&provider).Error; err != nil {
		return nil, err
	}

	info := l.toProviderInfo(&provider)
	// 查询该供应商下的模型数量
	var count int64
	db.Model(&model.TAiModel{}).Where("provider_id = ? AND is_delete = 0", id).Count(&count)
	info.ModelCount = count

	return info, nil
}

func (l *AiProviderLogic) GetByIDWithKey(id int64) (*model.TAiProvider, error) {
	db := svc.Ctx.DB.WithContext(l.ctx)

	var provider model.TAiProvider
	if err := db.Where("id = ? AND is_delete = 0", id).First(&provider).Error; err != nil {
		return nil, err
	}

	return &provider, nil
}

func (l *AiProviderLogic) List(req *AiProviderListReq) ([]*AiProviderInfo, int64, error) {
	db := svc.Ctx.DB.WithContext(l.ctx).Model(&model.TAiProvider{}).Where("is_delete = 0")

	if req.Name != "" {
		db = db.Where("name LIKE ?", "%"+req.Name+"%")
	}
	if req.Status != nil {
		db = db.Where("status = ?", *req.Status)
	}

	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 20
	}

	var providers []model.TAiProvider
	offset := (req.Page - 1) * req.PageSize
	if err := db.Order("sort DESC, id DESC").Offset(offset).Limit(req.PageSize).Find(&providers).Error; err != nil {
		return nil, 0, err
	}

	// 批量查询每个供应商的模型数量
	providerIDs := make([]int64, len(providers))
	for i, p := range providers {
		providerIDs[i] = p.ID
	}
	modelCounts := make(map[int64]int64)
	if len(providerIDs) > 0 {
		type countResult struct {
			ProviderID int64 `gorm:"column:provider_id"`
			Count      int64 `gorm:"column:cnt"`
		}
		var counts []countResult
		svc.Ctx.DB.WithContext(l.ctx).Model(&model.TAiModel{}).
			Select("provider_id, COUNT(*) as cnt").
			Where("provider_id IN ? AND is_delete = 0", providerIDs).
			Group("provider_id").
			Find(&counts)
		for _, c := range counts {
			modelCounts[c.ProviderID] = c.Count
		}
	}

	result := make([]*AiProviderInfo, 0, len(providers))
	for _, p := range providers {
		info := l.toProviderInfo(&p)
		info.ModelCount = modelCounts[p.ID]
		result = append(result, info)
	}

	return result, total, nil
}

func (l *AiProviderLogic) UpdateStatus(id int64, status int32) error {
	db := svc.Ctx.DB.WithContext(l.ctx)
	now := time.Now()
	return db.Model(&model.TAiProvider{}).Where("id = ? AND is_delete = 0", id).
		Updates(map[string]interface{}{"status": status, "updated_at": now}).Error
}

// ========== 辅助方法 ==========

func (l *AiProviderLogic) toProviderInfo(p *model.TAiProvider) *AiProviderInfo {
	info := &AiProviderInfo{
		ID:           p.ID,
		CreatedAt:    p.CreatedAt,
		UpdatedAt:    p.UpdatedAt,
		Name:         p.Name,
		ProviderType: p.ProviderType,
		APIBaseURL:   p.APIBaseURL,
		APIKeyMasked: maskAPIKey(p.APIKey),
	}
	if p.Icon != nil {
		info.Icon = *p.Icon
	}
	if p.Description != nil {
		info.Description = *p.Description
	}
	if p.Sort != nil {
		info.Sort = *p.Sort
	}
	if p.Status != nil {
		info.Status = *p.Status
	}
	return info
}

