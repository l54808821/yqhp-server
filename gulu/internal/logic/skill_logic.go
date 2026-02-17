package logic

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"yqhp/gulu/internal/model"
	"yqhp/gulu/internal/query"
	"yqhp/gulu/internal/svc"
)

// SkillLogic Skill逻辑
type SkillLogic struct {
	ctx context.Context
}

// NewSkillLogic 创建Skill逻辑
func NewSkillLogic(ctx context.Context) *SkillLogic {
	return &SkillLogic{ctx: ctx}
}

// CreateSkillReq 创建Skill请求
type CreateSkillReq struct {
	Name                    string                  `json:"name" validate:"required,max=100"`
	Description             string                  `json:"description" validate:"max=1000"`
	Icon                    string                  `json:"icon" validate:"max=100"`
	Category                string                  `json:"category" validate:"max=50"`
	Tags                    []string                `json:"tags"`
	SystemPrompt            string                  `json:"system_prompt" validate:"required"`
	Variables               []SkillVariable         `json:"variables"`
	RecommendedModelParams  *RecommendedModelParams `json:"recommended_model_params"`
	RecommendedTools        []string                `json:"recommended_tools"`
	RecommendedMcpServerIDs []int64                 `json:"recommended_mcp_server_ids"`
	Sort                    int32                   `json:"sort"`
	Status                  int32                   `json:"status"`
	Slug                    string                  `json:"slug"`
	License                 string                  `json:"license"`
	Compatibility           string                  `json:"compatibility"`
	MetadataJSON            map[string]string       `json:"metadata_json"`
	AllowedTools            string                  `json:"allowed_tools"`
}

// UpdateSkillReq 更新Skill请求
type UpdateSkillReq struct {
	Name                    string                  `json:"name" validate:"max=100"`
	Description             string                  `json:"description" validate:"max=1000"`
	Icon                    string                  `json:"icon" validate:"max=100"`
	Category                string                  `json:"category" validate:"max=50"`
	Tags                    []string                `json:"tags"`
	SystemPrompt            string                  `json:"system_prompt"`
	Variables               []SkillVariable         `json:"variables"`
	RecommendedModelParams  *RecommendedModelParams `json:"recommended_model_params"`
	RecommendedTools        []string                `json:"recommended_tools"`
	RecommendedMcpServerIDs []int64                 `json:"recommended_mcp_server_ids"`
	Sort                    int32                   `json:"sort"`
	Status                  int32                   `json:"status"`
	Slug                    string                  `json:"slug"`
	License                 string                  `json:"license"`
	Compatibility           string                  `json:"compatibility"`
	MetadataJSON            map[string]string       `json:"metadata_json"`
	AllowedTools            string                  `json:"allowed_tools"`
}

// SkillListReq Skill列表请求
type SkillListReq struct {
	Page     int    `query:"page" validate:"min=1"`
	PageSize int    `query:"pageSize" validate:"min=1,max=100"`
	Name     string `query:"name"`
	Category string `query:"category"`
	Type     *int32 `query:"type"`
	Status   *int32 `query:"status"`
}

// SkillVariable 变量声明
type SkillVariable struct {
	Name     string `json:"name"`
	Label    string `json:"label"`
	Required bool   `json:"required"`
	Default  string `json:"default"`
}

// RecommendedModelParams 推荐模型参数
type RecommendedModelParams struct {
	Temperature *float64 `json:"temperature,omitempty"`
	MaxTokens   *int32   `json:"max_tokens,omitempty"`
	TopP        *float64 `json:"top_p,omitempty"`
}

// SkillResourceInfo Skill 资源返回信息
type SkillResourceInfo struct {
	ID         int64      `json:"id"`
	SkillID    int64      `json:"skill_id"`
	Category   string     `json:"category"`
	Filename   string     `json:"filename"`
	ContentType string    `json:"content_type"`
	Size       int32      `json:"size"`
	CreatedAt  *time.Time `json:"created_at"`
}

// CreateResourceReq 创建 Skill 资源请求
type CreateResourceReq struct {
	Category    string `json:"category"`
	Filename    string `json:"filename"`
	Content     string `json:"content"`
	ContentType string `json:"content_type"`
}

// SkillInfo Skill返回信息
type SkillInfo struct {
	ID                      int64                   `json:"id"`
	CreatedAt               *time.Time              `json:"created_at"`
	UpdatedAt               *time.Time              `json:"updated_at"`
	CreatedBy               *int64                  `json:"created_by"`
	Name                    string                  `json:"name"`
	Description             string                  `json:"description"`
	Icon                    string                  `json:"icon"`
	Category                string                  `json:"category"`
	Tags                    []string                `json:"tags"`
	SystemPrompt            string                  `json:"system_prompt"`
	Variables               []SkillVariable         `json:"variables"`
	RecommendedModelParams  *RecommendedModelParams `json:"recommended_model_params"`
	RecommendedTools        []string                `json:"recommended_tools"`
	RecommendedMcpServerIDs []int64                 `json:"recommended_mcp_server_ids"`
	Type                    int32                   `json:"type"`
	IsPublic                int32                   `json:"is_public"`
	Version                 string                  `json:"version"`
	Sort                    int32                   `json:"sort"`
	Status                  int32                   `json:"status"`
	Slug                    string                  `json:"slug"`
	License                 string                  `json:"license"`
	Compatibility           string                  `json:"compatibility"`
	MetadataJSON            map[string]string       `json:"metadata_json"`
	AllowedTools            string                  `json:"allowed_tools"`
}

// Create 创建Skill
func (l *SkillLogic) Create(req *CreateSkillReq) (*SkillInfo, error) {
	now := time.Now()
	isDelete := false
	status := req.Status
	if status == 0 {
		status = 1
	}
	skillType := int32(0) // 用户自建

	skill := &model.TSkill{
		CreatedAt:    &now,
		UpdatedAt:    &now,
		IsDelete:     &isDelete,
		Name:         req.Name,
		Description:  strPtr(req.Description),
		Icon:         strPtr(req.Icon),
		Category:     strPtr(req.Category),
		SystemPrompt: req.SystemPrompt,
		Type:         &skillType,
		Sort:         &req.Sort,
		Status:       &status,
	}

	// 序列化 JSON 字段
	if len(req.Tags) > 0 {
		skill.Tags = marshalJSONPtr(req.Tags)
	}
	if len(req.Variables) > 0 {
		skill.Variables = marshalJSONPtr(req.Variables)
	}
	if req.RecommendedModelParams != nil {
		skill.RecommendedModelParams = marshalJSONPtr(req.RecommendedModelParams)
	}
	if len(req.RecommendedTools) > 0 {
		skill.RecommendedTools = marshalJSONPtr(req.RecommendedTools)
	}
	if len(req.RecommendedMcpServerIDs) > 0 {
		skill.RecommendedMcpServerIDs = marshalJSONPtr(req.RecommendedMcpServerIDs)
	}

	if req.Slug != "" {
		skill.Slug = strPtr(req.Slug)
	}
	skill.License = strPtr(req.License)
	skill.Compatibility = strPtr(req.Compatibility)
	skill.AllowedTools = strPtr(req.AllowedTools)
	if req.MetadataJSON != nil {
		skill.MetadataJSON = marshalJSONPtr(req.MetadataJSON)
	}

	q := query.Use(svc.Ctx.DB)
	err := q.TSkill.WithContext(l.ctx).Create(skill)
	if err != nil {
		return nil, err
	}

	return l.toSkillInfo(skill), nil
}

// Update 更新Skill
func (l *SkillLogic) Update(id int64, req *UpdateSkillReq) error {
	q := query.Use(svc.Ctx.DB)
	m := q.TSkill

	existing, err := m.WithContext(l.ctx).Where(m.ID.Eq(id), m.IsDelete.Is(false)).First()
	if err != nil {
		return errors.New("Skill不存在")
	}

	// 系统内置 Skill 不允许修改
	if existing.Type != nil && *existing.Type == 1 {
		return errors.New("系统内置Skill不允许修改")
	}

	now := time.Now()
	updates := map[string]interface{}{
		"updated_at": now,
	}

	if req.Name != "" {
		updates["name"] = req.Name
	}
	updates["description"] = req.Description
	updates["icon"] = req.Icon
	updates["category"] = req.Category
	if req.SystemPrompt != "" {
		updates["system_prompt"] = req.SystemPrompt
	}
	updates["sort"] = req.Sort
	updates["status"] = req.Status

	if req.Tags != nil {
		b, _ := json.Marshal(req.Tags)
		updates["tags"] = string(b)
	}
	if req.Variables != nil {
		b, _ := json.Marshal(req.Variables)
		updates["variables"] = string(b)
	}
	if req.RecommendedModelParams != nil {
		b, _ := json.Marshal(req.RecommendedModelParams)
		updates["recommended_model_params"] = string(b)
	} else {
		updates["recommended_model_params"] = nil
	}
	if req.RecommendedTools != nil {
		b, _ := json.Marshal(req.RecommendedTools)
		updates["recommended_tools"] = string(b)
	}
	if req.RecommendedMcpServerIDs != nil {
		b, _ := json.Marshal(req.RecommendedMcpServerIDs)
		updates["recommended_mcp_server_ids"] = string(b)
	}

	if req.Slug != "" {
		updates["slug"] = req.Slug
	}
	updates["license"] = req.License
	updates["compatibility"] = req.Compatibility
	updates["allowed_tools"] = req.AllowedTools
	if req.MetadataJSON != nil {
		b, _ := json.Marshal(req.MetadataJSON)
		updates["metadata_json"] = string(b)
	}

	_, err = m.WithContext(l.ctx).Where(m.ID.Eq(id)).Updates(updates)
	return err
}

// Delete 删除Skill（软删除）
func (l *SkillLogic) Delete(id int64) error {
	q := query.Use(svc.Ctx.DB)
	m := q.TSkill

	existing, err := m.WithContext(l.ctx).Where(m.ID.Eq(id), m.IsDelete.Is(false)).First()
	if err != nil {
		return errors.New("Skill不存在")
	}

	if existing.Type != nil && *existing.Type == 1 {
		return errors.New("系统内置Skill不允许删除")
	}

	isDelete := true
	_, err = m.WithContext(l.ctx).Where(m.ID.Eq(id)).Update(m.IsDelete, isDelete)
	return err
}

// GetByID 根据ID获取Skill
func (l *SkillLogic) GetByID(id int64) (*SkillInfo, error) {
	q := query.Use(svc.Ctx.DB)
	m := q.TSkill

	skill, err := m.WithContext(l.ctx).Where(m.ID.Eq(id), m.IsDelete.Is(false)).First()
	if err != nil {
		return nil, err
	}

	return l.toSkillInfo(skill), nil
}

// List 获取Skill列表
func (l *SkillLogic) List(req *SkillListReq) ([]*SkillInfo, int64, error) {
	q := query.Use(svc.Ctx.DB)
	m := q.TSkill

	queryBuilder := m.WithContext(l.ctx).Where(m.IsDelete.Is(false))

	if req.Name != "" {
		queryBuilder = queryBuilder.Where(m.Name.Like("%" + req.Name + "%"))
	}
	if req.Category != "" {
		queryBuilder = queryBuilder.Where(m.Category.Eq(req.Category))
	}
	if req.Type != nil {
		queryBuilder = queryBuilder.Where(m.Type.Eq(*req.Type))
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
		req.PageSize = 20
	}

	offset := (req.Page - 1) * req.PageSize
	list, err := queryBuilder.Order(m.Sort.Desc(), m.ID.Desc()).Offset(offset).Limit(req.PageSize).Find()
	if err != nil {
		return nil, 0, err
	}

	result := make([]*SkillInfo, 0, len(list))
	for _, item := range list {
		result = append(result, l.toSkillInfo(item))
	}

	return result, total, nil
}

// UpdateStatus 更新Skill状态
func (l *SkillLogic) UpdateStatus(id int64, status int32) error {
	q := query.Use(svc.Ctx.DB)
	m := q.TSkill

	now := time.Now()
	_, err := m.WithContext(l.ctx).Where(m.ID.Eq(id), m.IsDelete.Is(false)).Updates(map[string]interface{}{
		"status":     status,
		"updated_at": now,
	})
	return err
}

// GetCategories 获取所有分类列表
func (l *SkillLogic) GetCategories() ([]string, error) {
	q := query.Use(svc.Ctx.DB)
	m := q.TSkill

	var categories []string
	err := m.WithContext(l.ctx).Where(m.IsDelete.Is(false)).Where(m.Category.IsNotNull()).Distinct(m.Category).Pluck(m.Category, &categories)
	if err != nil {
		return nil, err
	}

	return categories, nil
}

// ListResources 获取 Skill 资源列表
func (l *SkillLogic) ListResources(skillID int64) ([]*SkillResourceInfo, error) {
	q := query.Use(svc.Ctx.DB)
	m := q.TSkillResource

	list, err := m.WithContext(l.ctx).Where(m.SkillID.Eq(skillID)).Find()
	if err != nil {
		return nil, err
	}

	result := make([]*SkillResourceInfo, 0, len(list))
	for _, item := range list {
		result = append(result, toSkillResourceInfo(item))
	}
	return result, nil
}

// CreateResource 创建 Skill 资源
func (l *SkillLogic) CreateResource(skillID int64, req *CreateResourceReq) (*SkillResourceInfo, error) {
	now := time.Now()
	size := int32(len(req.Content))
	content := req.Content
	contentType := req.ContentType
	if contentType == "" {
		contentType = "text/plain"
	}

	resource := &model.TSkillResource{
		CreatedAt:   &now,
		UpdatedAt:   &now,
		SkillID:     skillID,
		Category:    req.Category,
		Filename:    req.Filename,
		Content:     &content,
		ContentType: &contentType,
		Size:        &size,
	}

	q := query.Use(svc.Ctx.DB)
	err := q.TSkillResource.WithContext(l.ctx).Create(resource)
	if err != nil {
		return nil, err
	}
	return toSkillResourceInfo(resource), nil
}

// DeleteResource 删除 Skill 资源
func (l *SkillLogic) DeleteResource(resourceID int64) error {
	q := query.Use(svc.Ctx.DB)
	m := q.TSkillResource
	_, err := m.WithContext(l.ctx).Where(m.ID.Eq(resourceID)).Delete()
	return err
}

func toSkillResourceInfo(m *model.TSkillResource) *SkillResourceInfo {
	info := &SkillResourceInfo{
		ID:       m.ID,
		SkillID:  m.SkillID,
		Category: m.Category,
		Filename: m.Filename,
	}
	if m.ContentType != nil {
		info.ContentType = *m.ContentType
	}
	if m.Size != nil {
		info.Size = *m.Size
	}
	info.CreatedAt = m.CreatedAt
	return info
}

// toSkillInfo 转换为返回信息
func (l *SkillLogic) toSkillInfo(m *model.TSkill) *SkillInfo {
	info := &SkillInfo{
		ID:           m.ID,
		CreatedAt:    m.CreatedAt,
		UpdatedAt:    m.UpdatedAt,
		CreatedBy:    m.CreatedBy,
		Name:         m.Name,
		SystemPrompt: m.SystemPrompt,
	}

	if m.Description != nil {
		info.Description = *m.Description
	}
	if m.Icon != nil {
		info.Icon = *m.Icon
	}
	if m.Category != nil {
		info.Category = *m.Category
	}
	if m.Type != nil {
		info.Type = *m.Type
	}
	if m.IsPublic != nil {
		info.IsPublic = *m.IsPublic
	}
	if m.Version != nil {
		info.Version = *m.Version
	}
	if m.Sort != nil {
		info.Sort = *m.Sort
	}
	if m.Status != nil {
		info.Status = *m.Status
	}
	if m.Slug != nil {
		info.Slug = *m.Slug
	}
	if m.License != nil {
		info.License = *m.License
	}
	if m.Compatibility != nil {
		info.Compatibility = *m.Compatibility
	}
	if m.AllowedTools != nil {
		info.AllowedTools = *m.AllowedTools
	}
	if m.MetadataJSON != nil {
		var metadata map[string]string
		if err := json.Unmarshal([]byte(*m.MetadataJSON), &metadata); err == nil {
			info.MetadataJSON = metadata
		}
	}

	// 解析 JSON 字段
	info.Tags = unmarshalStringSlice(m.Tags)
	info.Variables = unmarshalVariables(m.Variables)
	info.RecommendedModelParams = unmarshalModelParams(m.RecommendedModelParams)
	info.RecommendedTools = unmarshalStringSlice(m.RecommendedTools)
	info.RecommendedMcpServerIDs = unmarshalInt64Slice(m.RecommendedMcpServerIDs)

	return info
}

// 辅助函数

func strPtr(s string) *string {
	return &s
}

func marshalJSONPtr(v interface{}) *string {
	b, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	s := string(b)
	return &s
}

func unmarshalStringSlice(s *string) []string {
	if s == nil || *s == "" {
		return []string{}
	}
	var result []string
	if err := json.Unmarshal([]byte(*s), &result); err != nil {
		return []string{}
	}
	return result
}

func unmarshalInt64Slice(s *string) []int64 {
	if s == nil || *s == "" {
		return []int64{}
	}
	var result []int64
	if err := json.Unmarshal([]byte(*s), &result); err != nil {
		return []int64{}
	}
	return result
}

func unmarshalVariables(s *string) []SkillVariable {
	if s == nil || *s == "" {
		return []SkillVariable{}
	}
	var result []SkillVariable
	if err := json.Unmarshal([]byte(*s), &result); err != nil {
		return []SkillVariable{}
	}
	return result
}

func unmarshalModelParams(s *string) *RecommendedModelParams {
	if s == nil || *s == "" {
		return nil
	}
	var result RecommendedModelParams
	if err := json.Unmarshal([]byte(*s), &result); err != nil {
		return nil
	}
	return &result
}
