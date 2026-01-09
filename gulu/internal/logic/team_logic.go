package logic

import (
	"context"
	"errors"
	"time"

	"yqhp/gulu/internal/model"
	"yqhp/gulu/internal/query"
	"yqhp/gulu/internal/svc"
)

// TeamLogic 团队逻辑
type TeamLogic struct {
	ctx context.Context
}

// NewTeamLogic 创建团队逻辑
func NewTeamLogic(ctx context.Context) *TeamLogic {
	return &TeamLogic{ctx: ctx}
}

// CreateTeamReq 创建团队请求
type CreateTeamReq struct {
	Name        string `json:"name" validate:"required,max=100"`
	Description string `json:"description" validate:"max=500"`
}

// UpdateTeamReq 更新团队请求
type UpdateTeamReq struct {
	Name        string `json:"name" validate:"max=100"`
	Description string `json:"description" validate:"max=500"`
	Status      *int32 `json:"status"`
}

// TeamListReq 团队列表请求
type TeamListReq struct {
	Page     int    `query:"page" validate:"min=1"`
	PageSize int    `query:"pageSize" validate:"min=1,max=100"`
	Name     string `query:"name"`
	Status   *int32 `query:"status"`
}

// Create 创建团队
func (l *TeamLogic) Create(req *CreateTeamReq, userID int64) (*model.TTeam, error) {
	now := time.Now()
	isDelete := false
	status := int32(1)

	team := &model.TTeam{
		CreatedAt:   &now,
		UpdatedAt:   &now,
		IsDelete:    &isDelete,
		CreatedBy:   &userID,
		UpdatedBy:   &userID,
		Name:        req.Name,
		Description: &req.Description,
		Status:      &status,
	}

	q := query.Use(svc.Ctx.DB)
	err := q.TTeam.WithContext(l.ctx).Create(team)
	if err != nil {
		return nil, err
	}

	// 创建者自动成为团队 owner
	memberLogic := NewTeamMemberLogic(l.ctx)
	_, err = memberLogic.AddMember(team.ID, userID, "owner")
	if err != nil {
		return nil, err
	}

	return team, nil
}

// Update 更新团队
func (l *TeamLogic) Update(id int64, req *UpdateTeamReq, userID int64) error {
	q := query.Use(svc.Ctx.DB)
	t := q.TTeam

	// 检查团队是否存在
	_, err := t.WithContext(l.ctx).Where(t.ID.Eq(id), t.IsDelete.Is(false)).First()
	if err != nil {
		return errors.New("团队不存在")
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
	if req.Status != nil {
		updates["status"] = *req.Status
	}

	_, err = t.WithContext(l.ctx).Where(t.ID.Eq(id)).Updates(updates)
	return err
}

// Delete 删除团队（软删除）
func (l *TeamLogic) Delete(id int64) error {
	q := query.Use(svc.Ctx.DB)
	t := q.TTeam

	isDelete := true
	_, err := t.WithContext(l.ctx).Where(t.ID.Eq(id)).Update(t.IsDelete, isDelete)
	return err
}

// GetByID 根据ID获取团队
func (l *TeamLogic) GetByID(id int64) (*model.TTeam, error) {
	q := query.Use(svc.Ctx.DB)
	t := q.TTeam

	return t.WithContext(l.ctx).Where(t.ID.Eq(id), t.IsDelete.Is(false)).First()
}

// List 获取团队列表
func (l *TeamLogic) List(req *TeamListReq) ([]*model.TTeam, int64, error) {
	q := query.Use(svc.Ctx.DB)
	t := q.TTeam

	queryBuilder := t.WithContext(l.ctx).Where(t.IsDelete.Is(false))

	if req.Name != "" {
		queryBuilder = queryBuilder.Where(t.Name.Like("%" + req.Name + "%"))
	}
	if req.Status != nil {
		queryBuilder = queryBuilder.Where(t.Status.Eq(*req.Status))
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
	list, err := queryBuilder.Order(t.ID.Desc()).Offset(offset).Limit(req.PageSize).Find()
	if err != nil {
		return nil, 0, err
	}

	return list, total, nil
}

// GetUserTeams 获取用户所属的团队列表
func (l *TeamLogic) GetUserTeams(userID int64) ([]*model.TTeam, error) {
	q := query.Use(svc.Ctx.DB)
	t := q.TTeam
	tm := q.TTeamMember

	// 先获取用户所属的团队ID列表
	members, err := tm.WithContext(l.ctx).Where(tm.UserID.Eq(userID)).Find()
	if err != nil {
		return nil, err
	}

	if len(members) == 0 {
		return []*model.TTeam{}, nil
	}

	teamIDs := make([]int64, len(members))
	for i, m := range members {
		teamIDs[i] = m.TeamID
	}

	// 获取团队信息
	return t.WithContext(l.ctx).Where(t.ID.In(teamIDs...), t.IsDelete.Is(false)).Order(t.ID.Desc()).Find()
}
