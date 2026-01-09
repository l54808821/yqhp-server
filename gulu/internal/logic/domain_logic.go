package logic

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"time"

	"yqhp/gulu/internal/model"
	"yqhp/gulu/internal/query"
	"yqhp/gulu/internal/svc"
)

// DomainLogic 域名逻辑
type DomainLogic struct {
	ctx context.Context
}

// NewDomainLogic 创建域名逻辑
func NewDomainLogic(ctx context.Context) *DomainLogic {
	return &DomainLogic{ctx: ctx}
}

// HeaderItem 请求头项
type HeaderItem struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// CreateDomainReq 创建域名请求
type CreateDomainReq struct {
	ProjectID   int64        `json:"project_id" validate:"required"`
	EnvID       int64        `json:"env_id" validate:"required"`
	Name        string       `json:"name" validate:"required,max=100"`
	Code        string       `json:"code" validate:"required,max=50"`
	BaseURL     string       `json:"base_url" validate:"required,max=500"`
	Headers     []HeaderItem `json:"headers"`
	Description string       `json:"description" validate:"max=500"`
	Sort        int64        `json:"sort"`
	Status      int32        `json:"status"`
}

// UpdateDomainReq 更新域名请求
type UpdateDomainReq struct {
	Name        string       `json:"name" validate:"max=100"`
	BaseURL     string       `json:"base_url" validate:"max=500"`
	Headers     []HeaderItem `json:"headers"`
	Description string       `json:"description" validate:"max=500"`
	Sort        int64        `json:"sort"`
	Status      int32        `json:"status"`
}

// DomainListReq 域名列表请求
type DomainListReq struct {
	Page      int    `query:"page" validate:"min=1"`
	PageSize  int    `query:"pageSize" validate:"min=1,max=100"`
	ProjectID int64  `query:"project_id"`
	EnvID     int64  `query:"env_id"`
	Name      string `query:"name"`
	Code      string `query:"code"`
	Status    *int32 `query:"status"`
}

// DomainResp 域名响应（包含解析后的Headers）
type DomainResp struct {
	*model.TDomain
	HeaderList []HeaderItem `json:"header_list"`
}

// ValidateURL 验证URL格式
func ValidateURL(rawURL string) error {
	if rawURL == "" {
		return errors.New("URL不能为空")
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return errors.New("URL格式无效")
	}

	if parsed.Scheme == "" {
		return errors.New("URL必须包含协议（如 http:// 或 https://）")
	}

	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return errors.New("URL协议必须是 http 或 https")
	}

	if parsed.Host == "" {
		return errors.New("URL必须包含主机名")
	}

	return nil
}

// Create 创建域名
func (l *DomainLogic) Create(req *CreateDomainReq) (*model.TDomain, error) {
	// 验证URL格式
	if err := ValidateURL(req.BaseURL); err != nil {
		return nil, err
	}

	// 检查环境是否存在
	envLogic := NewEnvLogic(l.ctx)
	env, err := envLogic.GetByID(req.EnvID)
	if err != nil {
		return nil, errors.New("环境不存在")
	}

	// 确保项目ID与环境的项目ID一致
	if req.ProjectID != env.ProjectID {
		return nil, errors.New("项目ID与环境所属项目不匹配")
	}

	// 检查域名代码在环境内是否唯一
	exists, err := l.CheckCodeExists(req.EnvID, req.Code, 0)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, errors.New("域名代码在该环境中已存在")
	}

	// 序列化Headers
	var headersJSON *string
	if len(req.Headers) > 0 {
		data, err := json.Marshal(req.Headers)
		if err != nil {
			return nil, errors.New("Headers格式无效")
		}
		str := string(data)
		headersJSON = &str
	}

	now := time.Now()
	isDelete := false
	status := req.Status
	if status == 0 {
		status = 1
	}

	domain := &model.TDomain{
		CreatedAt:   &now,
		UpdatedAt:   &now,
		IsDelete:    &isDelete,
		ProjectID:   req.ProjectID,
		EnvID:       req.EnvID,
		Name:        req.Name,
		Code:        req.Code,
		BaseURL:     req.BaseURL,
		Headers:     headersJSON,
		Description: &req.Description,
		Sort:        &req.Sort,
		Status:      &status,
	}

	q := query.Use(svc.Ctx.DB)
	err = q.TDomain.WithContext(l.ctx).Create(domain)
	if err != nil {
		return nil, err
	}

	return domain, nil
}

// Update 更新域名
func (l *DomainLogic) Update(id int64, req *UpdateDomainReq) error {
	// 验证URL格式
	if req.BaseURL != "" {
		if err := ValidateURL(req.BaseURL); err != nil {
			return err
		}
	}

	q := query.Use(svc.Ctx.DB)
	d := q.TDomain

	domain, err := d.WithContext(l.ctx).Where(d.ID.Eq(id), d.IsDelete.Is(false)).First()
	if err != nil {
		return errors.New("域名不存在")
	}

	now := time.Now()
	updates := map[string]interface{}{
		"updated_at": now,
	}

	if req.Name != "" {
		updates["name"] = req.Name
	}
	if req.BaseURL != "" {
		updates["base_url"] = req.BaseURL
	}
	if req.Description != "" {
		updates["description"] = req.Description
	}

	// 更新Headers
	if req.Headers != nil {
		data, err := json.Marshal(req.Headers)
		if err != nil {
			return errors.New("Headers格式无效")
		}
		updates["headers"] = string(data)
	}

	updates["sort"] = req.Sort
	updates["status"] = req.Status

	_, err = d.WithContext(l.ctx).Where(d.ID.Eq(domain.ID)).Updates(updates)
	return err
}

// Delete 删除域名（软删除）
func (l *DomainLogic) Delete(id int64) error {
	q := query.Use(svc.Ctx.DB)
	d := q.TDomain

	isDelete := true
	_, err := d.WithContext(l.ctx).Where(d.ID.Eq(id)).Update(d.IsDelete, isDelete)
	return err
}

// GetByID 根据ID获取域名
func (l *DomainLogic) GetByID(id int64) (*DomainResp, error) {
	q := query.Use(svc.Ctx.DB)
	d := q.TDomain

	domain, err := d.WithContext(l.ctx).Where(d.ID.Eq(id), d.IsDelete.Is(false)).First()
	if err != nil {
		return nil, err
	}

	return l.toDomainResp(domain), nil
}

// List 获取域名列表
func (l *DomainLogic) List(req *DomainListReq) ([]*DomainResp, int64, error) {
	q := query.Use(svc.Ctx.DB)
	d := q.TDomain

	qry := d.WithContext(l.ctx).Where(d.IsDelete.Is(false))

	if req.ProjectID > 0 {
		qry = qry.Where(d.ProjectID.Eq(req.ProjectID))
	}
	if req.EnvID > 0 {
		qry = qry.Where(d.EnvID.Eq(req.EnvID))
	}
	if req.Name != "" {
		qry = qry.Where(d.Name.Like("%" + req.Name + "%"))
	}
	if req.Code != "" {
		qry = qry.Where(d.Code.Like("%" + req.Code + "%"))
	}
	if req.Status != nil {
		qry = qry.Where(d.Status.Eq(*req.Status))
	}

	total, err := qry.Count()
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
	list, err := qry.Order(d.Sort.Desc(), d.ID.Desc()).Offset(offset).Limit(req.PageSize).Find()
	if err != nil {
		return nil, 0, err
	}

	// 转换为响应格式
	respList := make([]*DomainResp, len(list))
	for i, item := range list {
		respList[i] = l.toDomainResp(item)
	}

	return respList, total, nil
}

// CheckCodeExists 检查域名代码在环境内是否存在
func (l *DomainLogic) CheckCodeExists(envID int64, code string, excludeID int64) (bool, error) {
	q := query.Use(svc.Ctx.DB)
	d := q.TDomain

	qry := d.WithContext(l.ctx).Where(d.EnvID.Eq(envID), d.Code.Eq(code), d.IsDelete.Is(false))
	if excludeID > 0 {
		qry = qry.Where(d.ID.Neq(excludeID))
	}

	count, err := qry.Count()
	if err != nil {
		return false, err
	}

	return count > 0, nil
}

// GetDomainsByEnvID 获取环境下所有启用的域名
func (l *DomainLogic) GetDomainsByEnvID(envID int64) ([]*DomainResp, error) {
	q := query.Use(svc.Ctx.DB)
	d := q.TDomain

	status := int32(1)
	list, err := d.WithContext(l.ctx).Where(
		d.EnvID.Eq(envID),
		d.IsDelete.Is(false),
		d.Status.Eq(status),
	).Order(d.Sort.Desc(), d.ID.Desc()).Find()
	if err != nil {
		return nil, err
	}

	respList := make([]*DomainResp, len(list))
	for i, item := range list {
		respList[i] = l.toDomainResp(item)
	}

	return respList, nil
}

// toDomainResp 转换为域名响应
func (l *DomainLogic) toDomainResp(domain *model.TDomain) *DomainResp {
	resp := &DomainResp{
		TDomain:    domain,
		HeaderList: []HeaderItem{},
	}

	// 解析Headers
	if domain.Headers != nil && *domain.Headers != "" {
		var headers []HeaderItem
		if err := json.Unmarshal([]byte(*domain.Headers), &headers); err == nil {
			resp.HeaderList = headers
		}
	}

	return resp
}
