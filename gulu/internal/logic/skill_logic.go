package logic

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"yqhp/gulu/internal/model"
	"yqhp/gulu/internal/query"
	"yqhp/gulu/internal/svc"

	"gopkg.in/yaml.v3"
)

type SkillLogic struct {
	ctx context.Context
}

func NewSkillLogic(ctx context.Context) *SkillLogic {
	return &SkillLogic{ctx: ctx}
}

// ---------- Request / Response types ----------

type CreateSkillReq struct {
	Name         string            `json:"name" validate:"required,max=100"`
	Description  string            `json:"description" validate:"max=1000"`
	Icon         string            `json:"icon" validate:"max=100"`
	Category     string            `json:"category" validate:"max=50"`
	Tags         []string          `json:"tags"`
	Sort         int32             `json:"sort"`
	Status       int32             `json:"status"`
	Slug         string            `json:"slug"`
	License      string            `json:"license"`
	Compatibility string           `json:"compatibility"`
	MetadataJSON map[string]string `json:"metadata_json"`
	AllowedTools string            `json:"allowed_tools"`
	Author       string            `json:"author"`
	SourceURL    string            `json:"source_url"`
	InstallCount int32             `json:"install_count"`
	SkillMD      string            `json:"skill_md"`
}

type UpdateSkillReq struct {
	Name         string            `json:"name" validate:"max=100"`
	Description  string            `json:"description" validate:"max=1000"`
	Icon         string            `json:"icon" validate:"max=100"`
	Category     string            `json:"category" validate:"max=50"`
	Tags         []string          `json:"tags"`
	Sort         int32             `json:"sort"`
	Status       int32             `json:"status"`
	Slug         string            `json:"slug"`
	License      string            `json:"license"`
	Compatibility string           `json:"compatibility"`
	MetadataJSON map[string]string `json:"metadata_json"`
	AllowedTools string            `json:"allowed_tools"`
	Author       string            `json:"author"`
	SourceURL    string            `json:"source_url"`
}

type SkillListReq struct {
	Page     int    `query:"page" validate:"min=1"`
	PageSize int    `query:"pageSize" validate:"min=1,max=100"`
	Name     string `query:"name"`
	Category string `query:"category"`
	Type     *int32 `query:"type"`
	Status   *int32 `query:"status"`
}

type SkillResourceInfo struct {
	ID          int64      `json:"id"`
	SkillID     int64      `json:"skill_id"`
	Path        string     `json:"path"`
	ContentType string     `json:"content_type"`
	Size        int32      `json:"size"`
	CreatedAt   *time.Time `json:"created_at"`
}

type CreateResourceReq struct {
	Path        string `json:"path"`
	Content     string `json:"content"`
	ContentType string `json:"content_type"`
}

type UpdateResourceReq struct {
	Content     string `json:"content"`
	ContentType string `json:"content_type"`
}

type SkillInfo struct {
	ID           int64             `json:"id"`
	CreatedAt    *time.Time        `json:"created_at"`
	UpdatedAt    *time.Time        `json:"updated_at"`
	CreatedBy    *int64            `json:"created_by"`
	Name         string            `json:"name"`
	Description  string            `json:"description"`
	Icon         string            `json:"icon"`
	Category     string            `json:"category"`
	Tags         []string          `json:"tags"`
	Type         int32             `json:"type"`
	IsPublic     int32             `json:"is_public"`
	Version      string            `json:"version"`
	Sort         int32             `json:"sort"`
	Status       int32             `json:"status"`
	Slug         string            `json:"slug"`
	License      string            `json:"license"`
	Compatibility string           `json:"compatibility"`
	MetadataJSON map[string]string `json:"metadata_json"`
	AllowedTools string            `json:"allowed_tools"`
	Author       string            `json:"author"`
	SourceURL    string            `json:"source_url"`
	InstallCount int32             `json:"install_count"`
}

// ---------- CRUD ----------

func (l *SkillLogic) Create(req *CreateSkillReq) (*SkillInfo, error) {
	now := time.Now()
	isDelete := false
	status := req.Status
	if status == 0 {
		status = 1
	}
	skillType := int32(0)

	skill := &model.TSkill{
		CreatedAt:    &now,
		UpdatedAt:    &now,
		IsDelete:     &isDelete,
		Name:         req.Name,
		Description:  strPtr(req.Description),
		Icon:         strPtr(req.Icon),
		Category:     strPtr(req.Category),
		Type:         &skillType,
		Sort:         &req.Sort,
		Status:       &status,
		Author:       strPtr(req.Author),
		SourceURL:    strPtr(req.SourceURL),
		InstallCount: &req.InstallCount,
	}

	if len(req.Tags) > 0 {
		skill.Tags = marshalJSONPtr(req.Tags)
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
	if err := q.TSkill.WithContext(l.ctx).Create(skill); err != nil {
		return nil, err
	}

	if req.SkillMD != "" {
		l.upsertResource(skill.ID, "SKILL.md", req.SkillMD, "text/markdown")
	}

	return l.toSkillInfo(skill), nil
}

func (l *SkillLogic) Update(id int64, req *UpdateSkillReq) error {
	q := query.Use(svc.Ctx.DB)
	m := q.TSkill

	existing, err := m.WithContext(l.ctx).Where(m.ID.Eq(id), m.IsDelete.Is(false)).First()
	if err != nil {
		return errors.New("Skill不存在")
	}
	if existing.Type != nil && *existing.Type == 1 {
		return errors.New("系统内置Skill不允许修改")
	}

	now := time.Now()
	updates := map[string]interface{}{"updated_at": now}
	if req.Name != "" {
		updates["name"] = req.Name
	}
	updates["description"] = req.Description
	updates["icon"] = req.Icon
	updates["category"] = req.Category
	updates["sort"] = req.Sort
	updates["status"] = req.Status
	updates["author"] = req.Author
	updates["source_url"] = req.SourceURL
	if req.Tags != nil {
		b, _ := json.Marshal(req.Tags)
		updates["tags"] = string(b)
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

func (l *SkillLogic) GetByID(id int64) (*SkillInfo, error) {
	q := query.Use(svc.Ctx.DB)
	m := q.TSkill
	skill, err := m.WithContext(l.ctx).Where(m.ID.Eq(id), m.IsDelete.Is(false)).First()
	if err != nil {
		return nil, err
	}
	return l.toSkillInfo(skill), nil
}

func (l *SkillLogic) List(req *SkillListReq) ([]*SkillInfo, int64, error) {
	q := query.Use(svc.Ctx.DB)
	m := q.TSkill
	qb := m.WithContext(l.ctx).Where(m.IsDelete.Is(false))
	if req.Name != "" {
		qb = qb.Where(m.Name.Like("%" + req.Name + "%"))
	}
	if req.Category != "" {
		qb = qb.Where(m.Category.Eq(req.Category))
	}
	if req.Type != nil {
		qb = qb.Where(m.Type.Eq(*req.Type))
	}
	if req.Status != nil {
		qb = qb.Where(m.Status.Eq(*req.Status))
	}
	total, err := qb.Count()
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
	list, err := qb.Order(m.Sort.Desc(), m.ID.Desc()).Offset(offset).Limit(req.PageSize).Find()
	if err != nil {
		return nil, 0, err
	}
	result := make([]*SkillInfo, 0, len(list))
	for _, item := range list {
		result = append(result, l.toSkillInfo(item))
	}
	return result, total, nil
}

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

func (l *SkillLogic) GetCategories() ([]string, error) {
	q := query.Use(svc.Ctx.DB)
	m := q.TSkill
	var categories []string
	err := m.WithContext(l.ctx).Where(m.IsDelete.Is(false)).Where(m.Category.IsNotNull()).Distinct(m.Category).Pluck(m.Category, &categories)
	return categories, err
}

// ---------- 搜索（供 workflow-engine find_skills 调用） ----------

// SkillSearchReq 内部搜索请求
type SkillSearchReq struct {
	Query    string `query:"q"`
	Category string `query:"category"`
	Limit    int    `query:"limit"`
}

// SkillSearchItem 搜索结果（轻量，不含 body）
type SkillSearchItem struct {
	ID          int64    `json:"id"`
	Name        string   `json:"name"`
	Slug        string   `json:"slug"`
	Description string   `json:"description"`
	Category    string   `json:"category"`
	Tags        []string `json:"tags"`
	Type        int32    `json:"type"`
	Author      string   `json:"author"`
}

// Search 模糊搜索已启用的 Skill（name / description / tags / slug）
func (l *SkillLogic) Search(req *SkillSearchReq) ([]*SkillSearchItem, error) {
	q := query.Use(svc.Ctx.DB)
	m := q.TSkill
	status := int32(1)
	qb := m.WithContext(l.ctx).Where(m.IsDelete.Is(false), m.Status.Eq(status))

	if req.Query != "" {
		keyword := "%" + req.Query + "%"
		qb = qb.Where(
			m.WithContext(l.ctx).Where(m.Name.Like(keyword)).
				Or(m.Description.Like(keyword)).
				Or(m.Slug.Like(keyword)).
				Or(m.Tags.Like(keyword)),
		)
	}
	if req.Category != "" {
		qb = qb.Where(m.Category.Eq(req.Category))
	}

	limit := req.Limit
	if limit <= 0 || limit > 50 {
		limit = 10
	}

	list, err := qb.Order(m.Sort.Desc(), m.InstallCount.Desc()).Limit(limit).Find()
	if err != nil {
		return nil, err
	}

	result := make([]*SkillSearchItem, 0, len(list))
	for _, item := range list {
		si := &SkillSearchItem{
			ID:   item.ID,
			Name: item.Name,
		}
		if item.Description != nil {
			si.Description = *item.Description
		}
		if item.Slug != nil {
			si.Slug = *item.Slug
		}
		if item.Category != nil {
			si.Category = *item.Category
		}
		if item.Type != nil {
			si.Type = *item.Type
		}
		if item.Author != nil {
			si.Author = *item.Author
		}
		si.Tags = unmarshalStringSlice(item.Tags)
		result = append(result, si)
	}
	return result, nil
}

// GetSkillBody 获取单个 Skill 的摘要信息 + SKILL.md body（供 use_skill 工具调用）
func (l *SkillLogic) GetSkillBody(id int64) (*SkillSearchItem, string, error) {
	q := query.Use(svc.Ctx.DB)
	m := q.TSkill
	skill, err := m.WithContext(l.ctx).Where(m.ID.Eq(id), m.IsDelete.Is(false)).First()
	if err != nil {
		return nil, "", errors.New("Skill 不存在")
	}
	if skill.Status != nil && *skill.Status != 1 {
		return nil, "", errors.New("Skill 已禁用")
	}
	si := &SkillSearchItem{
		ID:   skill.ID,
		Name: skill.Name,
	}
	if skill.Description != nil {
		si.Description = *skill.Description
	}
	if skill.Slug != nil {
		si.Slug = *skill.Slug
	}
	if skill.Category != nil {
		si.Category = *skill.Category
	}
	body, _ := l.GetSKILLMDContent(id)
	return si, body, nil
}

// ---------- SKILL.md 内容读取 ----------

// GetSKILLMDContent 从 t_skill_resource 获取 SKILL.md 的 body（去掉 frontmatter）
func (l *SkillLogic) GetSKILLMDContent(skillID int64) (string, error) {
	q := query.Use(svc.Ctx.DB)
	m := q.TSkillResource
	res, err := m.WithContext(l.ctx).Where(m.SkillID.Eq(skillID), m.Path.Eq("SKILL.md")).First()
	if err != nil {
		return "", err
	}
	if res.Content == nil {
		return "", nil
	}
	_, body, parseErr := parseSkillMD(*res.Content)
	if parseErr != nil {
		return *res.Content, nil
	}
	return body, nil
}

// GetSKILLMDRaw 从 t_skill_resource 获取 SKILL.md 完整原文
func (l *SkillLogic) GetSKILLMDRaw(skillID int64) (string, error) {
	q := query.Use(svc.Ctx.DB)
	m := q.TSkillResource
	res, err := m.WithContext(l.ctx).Where(m.SkillID.Eq(skillID), m.Path.Eq("SKILL.md")).First()
	if err != nil {
		return "", err
	}
	if res.Content == nil {
		return "", nil
	}
	return *res.Content, nil
}

// SyncMetadataFromSKILLMD 解析 SKILL.md frontmatter 并同步到 t_skill 元数据
func (l *SkillLogic) SyncMetadataFromSKILLMD(skillID int64, skillMDContent string) error {
	fm, _, err := parseSkillMD(skillMDContent)
	if err != nil {
		return nil
	}

	q := query.Use(svc.Ctx.DB)
	m := q.TSkill
	updates := map[string]interface{}{"updated_at": time.Now()}

	if fm.Name != "" {
		updates["slug"] = fm.Name
	}
	if fm.Description != "" {
		updates["description"] = fm.Description
	}
	if fm.License != "" {
		updates["license"] = fm.License
	}
	if fm.Compatibility != "" {
		updates["compatibility"] = fm.Compatibility
	}
	if fm.AllowedTools != "" {
		updates["allowed_tools"] = fm.AllowedTools
	}
	if len(fm.Metadata) > 0 {
		b, _ := json.Marshal(fm.Metadata)
		updates["metadata_json"] = string(b)
	}

	_, err = m.WithContext(l.ctx).Where(m.ID.Eq(skillID)).Updates(updates)
	return err
}

// ---------- 资源文件 CRUD ----------

func (l *SkillLogic) ListResources(skillID int64) ([]*SkillResourceInfo, error) {
	q := query.Use(svc.Ctx.DB)
	m := q.TSkillResource
	list, err := m.WithContext(l.ctx).Where(m.SkillID.Eq(skillID)).Order(m.Path).Find()
	if err != nil {
		return nil, err
	}
	result := make([]*SkillResourceInfo, 0, len(list))
	for _, item := range list {
		result = append(result, toSkillResourceInfo(item))
	}
	return result, nil
}

func (l *SkillLogic) GetResourceContent(resourceID int64) (string, error) {
	q := query.Use(svc.Ctx.DB)
	m := q.TSkillResource
	res, err := m.WithContext(l.ctx).Where(m.ID.Eq(resourceID)).First()
	if err != nil {
		return "", err
	}
	if res.Content == nil {
		return "", nil
	}
	return *res.Content, nil
}

func (l *SkillLogic) CreateResource(skillID int64, req *CreateResourceReq) (*SkillResourceInfo, error) {
	return l.upsertResource(skillID, req.Path, req.Content, req.ContentType)
}

func (l *SkillLogic) UpdateResource(resourceID int64, req *UpdateResourceReq) error {
	q := query.Use(svc.Ctx.DB)
	m := q.TSkillResource

	res, err := m.WithContext(l.ctx).Where(m.ID.Eq(resourceID)).First()
	if err != nil {
		return errors.New("资源不存在")
	}

	now := time.Now()
	size := int32(len(req.Content))
	contentType := req.ContentType
	if contentType == "" {
		contentType = "text/plain"
	}

	_, err = m.WithContext(l.ctx).Where(m.ID.Eq(resourceID)).Updates(map[string]interface{}{
		"updated_at":   now,
		"content":      req.Content,
		"content_type": contentType,
		"size":         size,
	})
	if err != nil {
		return err
	}

	if res.Path == "SKILL.md" {
		_ = l.SyncMetadataFromSKILLMD(res.SkillID, req.Content)
	}

	return nil
}

func (l *SkillLogic) DeleteResource(resourceID int64) error {
	q := query.Use(svc.Ctx.DB)
	m := q.TSkillResource
	_, err := m.WithContext(l.ctx).Where(m.ID.Eq(resourceID)).Delete()
	return err
}

func (l *SkillLogic) upsertResource(skillID int64, path, content, contentType string) (*SkillResourceInfo, error) {
	now := time.Now()
	size := int32(len(content))
	if contentType == "" {
		contentType = guessContentType(path)
	}

	q := query.Use(svc.Ctx.DB)
	m := q.TSkillResource

	existing, _ := m.WithContext(l.ctx).Where(m.SkillID.Eq(skillID), m.Path.Eq(path)).First()
	if existing != nil {
		_, err := m.WithContext(l.ctx).Where(m.ID.Eq(existing.ID)).Updates(map[string]interface{}{
			"updated_at":   now,
			"content":      content,
			"content_type": contentType,
			"size":         size,
		})
		if err != nil {
			return nil, err
		}
		existing.Content = &content
		existing.ContentType = &contentType
		existing.Size = &size
		return toSkillResourceInfo(existing), nil
	}

	resource := &model.TSkillResource{
		CreatedAt:   &now,
		UpdatedAt:   &now,
		SkillID:     skillID,
		Path:        path,
		Content:     &content,
		ContentType: &contentType,
		Size:        &size,
	}
	if err := q.TSkillResource.WithContext(l.ctx).Create(resource); err != nil {
		return nil, err
	}
	return toSkillResourceInfo(resource), nil
}

// ---------- Converters ----------

func toSkillResourceInfo(m *model.TSkillResource) *SkillResourceInfo {
	info := &SkillResourceInfo{
		ID:      m.ID,
		SkillID: m.SkillID,
		Path:    m.Path,
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

func (l *SkillLogic) toSkillInfo(m *model.TSkill) *SkillInfo {
	info := &SkillInfo{
		ID:        m.ID,
		CreatedAt: m.CreatedAt,
		UpdatedAt: m.UpdatedAt,
		CreatedBy: m.CreatedBy,
		Name:      m.Name,
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
	if m.Author != nil {
		info.Author = *m.Author
	}
	if m.SourceURL != nil {
		info.SourceURL = *m.SourceURL
	}
	if m.InstallCount != nil {
		info.InstallCount = *m.InstallCount
	}
	if m.MetadataJSON != nil {
		var metadata map[string]string
		if err := json.Unmarshal([]byte(*m.MetadataJSON), &metadata); err == nil {
			info.MetadataJSON = metadata
		}
	}
	info.Tags = unmarshalStringSlice(m.Tags)
	return info
}

// ---------- Helpers ----------

func strPtr(s string) *string { return &s }

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

func guessContentType(filename string) string {
	lower := strings.ToLower(filename)
	idx := strings.LastIndex(lower, ".")
	if idx < 0 {
		return "text/plain"
	}
	switch lower[idx:] {
	case ".py":
		return "text/x-python"
	case ".sh", ".bash":
		return "text/x-shellscript"
	case ".js":
		return "text/javascript"
	case ".ts":
		return "text/typescript"
	case ".md":
		return "text/markdown"
	case ".json":
		return "application/json"
	case ".yaml", ".yml":
		return "text/yaml"
	case ".txt":
		return "text/plain"
	case ".pptx":
		return "application/vnd.openxmlformats-officedocument.presentationml.presentation"
	case ".docx":
		return "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	case ".xlsx":
		return "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	default:
		return "text/plain"
	}
}

// generateSKILLMD generates SKILL.md content from metadata and body
func generateSKILLMD(skill *model.TSkill, body string) string {
	fm := SkillMDFrontmatter{}
	if skill.Slug != nil && *skill.Slug != "" {
		fm.Name = *skill.Slug
	}
	if skill.Description != nil {
		fm.Description = *skill.Description
	}
	if skill.License != nil && *skill.License != "" {
		fm.License = *skill.License
	}
	if skill.Compatibility != nil && *skill.Compatibility != "" {
		fm.Compatibility = *skill.Compatibility
	}
	if skill.AllowedTools != nil && *skill.AllowedTools != "" {
		fm.AllowedTools = *skill.AllowedTools
	}
	if skill.MetadataJSON != nil && *skill.MetadataJSON != "" {
		_ = json.Unmarshal([]byte(*skill.MetadataJSON), &fm.Metadata)
	}
	fmBytes, _ := yaml.Marshal(&fm)
	return fmt.Sprintf("---\n%s---\n\n%s\n", string(fmBytes), body)
}
