package logic

import (
	"context"
	"errors"
	"time"

	"yqhp/gulu/internal/model"
	"yqhp/gulu/internal/query"
	"yqhp/gulu/internal/svc"
)

// ProjectLogic 项目逻辑
type ProjectLogic struct {
	ctx context.Context
}

// NewProjectLogic 创建项目逻辑
func NewProjectLogic(ctx context.Context) *ProjectLogic {
	return &ProjectLogic{ctx: ctx}
}

// CreateProjectReq 创建项目请求
type CreateProjectReq struct {
	TeamID      int64  `json:"team_id" validate:"required"`
	Name        string `json:"name" validate:"required,max=100"`
	Description string `json:"description" validate:"max=500"`
	Icon        string `json:"icon" validate:"max=255"`
	Sort        int64  `json:"sort"`
	Status      int32  `json:"status"`
}

// UpdateProjectReq 更新项目请求
type UpdateProjectReq struct {
	Name        string `json:"name" validate:"max=100"`
	Description string `json:"description" validate:"max=500"`
	Icon        string `json:"icon" validate:"max=255"`
	Sort        int64  `json:"sort"`
	Status      int32  `json:"status"`
}

// ProjectListReq 项目列表请求
type ProjectListReq struct {
	Page     int    `query:"page" validate:"min=1"`
	PageSize int    `query:"pageSize" validate:"min=1,max=100"`
	TeamID   int64  `query:"teamId"`
	Name     string `query:"name"`
	Status   *int32 `query:"status"`
}

// Create 创建项目
func (l *ProjectLogic) Create(req *CreateProjectReq, userID int64) (*model.TProject, error) {
	now := time.Now()
	isDelete := false
	status := req.Status
	if status == 0 {
		status = 1 // 默认启用
	}

	project := &model.TProject{
		CreatedAt:   &now,
		UpdatedAt:   &now,
		IsDelete:    &isDelete,
		CreatedBy:   &userID,
		UpdatedBy:   &userID,
		TeamID:      req.TeamID,
		Name:        req.Name,
		Description: &req.Description,
		Icon:        &req.Icon,
		Sort:        &req.Sort,
		Status:      &status,
	}

	q := query.Use(svc.Ctx.DB)
	err := q.TProject.WithContext(l.ctx).Create(project)
	if err != nil {
		return nil, err
	}

	return project, nil
}

// Update 更新项目
func (l *ProjectLogic) Update(id int64, req *UpdateProjectReq, userID int64) error {
	q := query.Use(svc.Ctx.DB)
	p := q.TProject

	// 检查项目是否存在
	project, err := p.WithContext(l.ctx).Where(p.ID.Eq(id), p.IsDelete.Is(false)).First()
	if err != nil {
		return errors.New("项目不存在")
	}

	now := time.Now()
	updates := map[string]interface{}{
		"updated_at": now,
		"updated_by": userID,
	}

	if req.Name != "" {
		updates["name"] = req.Name
	}
	if req.Description != "" {
		updates["description"] = req.Description
	}
	if req.Icon != "" {
		updates["icon"] = req.Icon
	}
	updates["sort"] = req.Sort
	updates["status"] = req.Status

	_, err = p.WithContext(l.ctx).Where(p.ID.Eq(project.ID)).Updates(updates)
	return err
}

// Delete 删除项目（软删除）
func (l *ProjectLogic) Delete(id int64) error {
	q := query.Use(svc.Ctx.DB)
	p := q.TProject

	isDelete := true
	_, err := p.WithContext(l.ctx).Where(p.ID.Eq(id)).Update(p.IsDelete, isDelete)
	return err
}

// GetByID 根据ID获取项目
func (l *ProjectLogic) GetByID(id int64) (*model.TProject, error) {
	q := query.Use(svc.Ctx.DB)
	p := q.TProject

	return p.WithContext(l.ctx).Where(p.ID.Eq(id), p.IsDelete.Is(false)).First()
}

// List 获取项目列表
func (l *ProjectLogic) List(req *ProjectListReq) ([]*model.TProject, int64, error) {
	q := query.Use(svc.Ctx.DB)
	p := q.TProject

	// 构建查询条件
	queryBuilder := p.WithContext(l.ctx).Where(p.IsDelete.Is(false))

	if req.TeamID > 0 {
		queryBuilder = queryBuilder.Where(p.TeamID.Eq(req.TeamID))
	}
	if req.Name != "" {
		queryBuilder = queryBuilder.Where(p.Name.Like("%" + req.Name + "%"))
	}
	if req.Status != nil {
		queryBuilder = queryBuilder.Where(p.Status.Eq(*req.Status))
	}

	// 获取总数
	total, err := queryBuilder.Count()
	if err != nil {
		return nil, 0, err
	}

	// 分页查询
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 10
	}

	offset := (req.Page - 1) * req.PageSize
	list, err := queryBuilder.Order(p.Sort.Desc(), p.ID.Desc()).Offset(offset).Limit(req.PageSize).Find()
	if err != nil {
		return nil, 0, err
	}

	return list, total, nil
}

// GetByTeamID 根据团队ID获取项目列表
func (l *ProjectLogic) GetByTeamID(teamID int64) ([]*model.TProject, error) {
	q := query.Use(svc.Ctx.DB)
	p := q.TProject

	return p.WithContext(l.ctx).Where(p.TeamID.Eq(teamID), p.IsDelete.Is(false)).Order(p.Sort.Desc(), p.ID.Desc()).Find()
}

// GetAllProjects 获取所有启用的项目（用于下拉选择）
func (l *ProjectLogic) GetAllProjects() ([]*model.TProject, error) {
	q := query.Use(svc.Ctx.DB)
	p := q.TProject

	status := int32(1)
	return p.WithContext(l.ctx).Where(p.IsDelete.Is(false), p.Status.Eq(status)).Order(p.Sort.Desc(), p.ID.Desc()).Find()
}
