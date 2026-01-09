package logic

import (
	"context"
	"errors"
	"time"

	"yqhp/gulu/internal/model"
	"yqhp/gulu/internal/query"
	"yqhp/gulu/internal/svc"
)

// ProjectMemberLogic 项目成员逻辑
type ProjectMemberLogic struct {
	ctx context.Context
}

// NewProjectMemberLogic 创建项目成员逻辑
func NewProjectMemberLogic(ctx context.Context) *ProjectMemberLogic {
	return &ProjectMemberLogic{ctx: ctx}
}

// AddProjectMemberReq 添加项目成员请求
type AddProjectMemberReq struct {
	UserID int64 `json:"user_id" validate:"required"`
}

// AddMember 添加项目成员
func (l *ProjectMemberLogic) AddMember(projectID, userID int64) (*model.TProjectMember, error) {
	q := query.Use(svc.Ctx.DB)
	pm := q.TProjectMember

	// 检查是否已经是成员
	exists, err := pm.WithContext(l.ctx).Where(pm.ProjectID.Eq(projectID), pm.UserID.Eq(userID)).Count()
	if err != nil {
		return nil, err
	}
	if exists > 0 {
		return nil, errors.New("用户已经是项目成员")
	}

	now := time.Now()
	member := &model.TProjectMember{
		CreatedAt: &now,
		ProjectID: projectID,
		UserID:    userID,
	}

	err = pm.WithContext(l.ctx).Create(member)
	if err != nil {
		return nil, err
	}

	return member, nil
}

// RemoveMember 移除项目成员
func (l *ProjectMemberLogic) RemoveMember(projectID, userID int64) error {
	q := query.Use(svc.Ctx.DB)
	pm := q.TProjectMember

	_, err := pm.WithContext(l.ctx).Where(pm.ProjectID.Eq(projectID), pm.UserID.Eq(userID)).Delete()
	return err
}

// GetProjectMembers 获取项目成员列表
func (l *ProjectMemberLogic) GetProjectMembers(projectID int64) ([]*model.TProjectMember, error) {
	q := query.Use(svc.Ctx.DB)
	pm := q.TProjectMember

	return pm.WithContext(l.ctx).Where(pm.ProjectID.Eq(projectID)).Order(pm.ID.Asc()).Find()
}

// IsMember 检查用户是否是项目成员
func (l *ProjectMemberLogic) IsMember(projectID, userID int64) (bool, error) {
	q := query.Use(svc.Ctx.DB)
	pm := q.TProjectMember

	count, err := pm.WithContext(l.ctx).Where(pm.ProjectID.Eq(projectID), pm.UserID.Eq(userID)).Count()
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// GetUserProjects 获取用户所属的项目ID列表
func (l *ProjectMemberLogic) GetUserProjects(userID int64) ([]int64, error) {
	q := query.Use(svc.Ctx.DB)
	pm := q.TProjectMember

	members, err := pm.WithContext(l.ctx).Where(pm.UserID.Eq(userID)).Find()
	if err != nil {
		return nil, err
	}

	projectIDs := make([]int64, len(members))
	for i, m := range members {
		projectIDs[i] = m.ProjectID
	}

	return projectIDs, nil
}
