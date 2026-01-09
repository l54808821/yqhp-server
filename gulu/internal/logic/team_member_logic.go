package logic

import (
	"context"
	"errors"
	"time"

	"yqhp/gulu/internal/model"
	"yqhp/gulu/internal/query"
	"yqhp/gulu/internal/svc"
)

// TeamMemberLogic 团队成员逻辑
type TeamMemberLogic struct {
	ctx context.Context
}

// NewTeamMemberLogic 创建团队成员逻辑
func NewTeamMemberLogic(ctx context.Context) *TeamMemberLogic {
	return &TeamMemberLogic{ctx: ctx}
}

// AddMemberReq 添加成员请求
type AddMemberReq struct {
	UserID int64  `json:"user_id" validate:"required"`
	Role   string `json:"role" validate:"required,oneof=owner admin member"`
}

// UpdateRoleReq 更新角色请求
type UpdateRoleReq struct {
	Role string `json:"role" validate:"required,oneof=owner admin member"`
}

// TeamMemberInfo 团队成员信息（包含用户信息）
type TeamMemberInfo struct {
	ID        int64      `json:"id"`
	TeamID    int64      `json:"team_id"`
	UserID    int64      `json:"user_id"`
	Role      string     `json:"role"`
	CreatedAt *time.Time `json:"created_at"`
	Username  string     `json:"username"`
	Nickname  string     `json:"nickname"`
}

// AddMember 添加团队成员
func (l *TeamMemberLogic) AddMember(teamID, userID int64, role string) (*model.TTeamMember, error) {
	q := query.Use(svc.Ctx.DB)
	tm := q.TTeamMember

	// 检查是否已经是成员
	exists, err := tm.WithContext(l.ctx).Where(tm.TeamID.Eq(teamID), tm.UserID.Eq(userID)).Count()
	if err != nil {
		return nil, err
	}
	if exists > 0 {
		return nil, errors.New("用户已经是团队成员")
	}

	now := time.Now()
	member := &model.TTeamMember{
		CreatedAt: &now,
		TeamID:    teamID,
		UserID:    userID,
		Role:      role,
	}

	err = tm.WithContext(l.ctx).Create(member)
	if err != nil {
		return nil, err
	}

	return member, nil
}

// RemoveMember 移除团队成员
func (l *TeamMemberLogic) RemoveMember(teamID, userID int64) error {
	q := query.Use(svc.Ctx.DB)
	tm := q.TTeamMember

	// 检查是否是 owner，owner 不能被移除
	member, err := tm.WithContext(l.ctx).Where(tm.TeamID.Eq(teamID), tm.UserID.Eq(userID)).First()
	if err != nil {
		return errors.New("成员不存在")
	}
	if member.Role == "owner" {
		return errors.New("不能移除团队所有者")
	}

	_, err = tm.WithContext(l.ctx).Where(tm.TeamID.Eq(teamID), tm.UserID.Eq(userID)).Delete()
	return err
}

// UpdateRole 更新成员角色
func (l *TeamMemberLogic) UpdateRole(teamID, userID int64, role string) error {
	q := query.Use(svc.Ctx.DB)
	tm := q.TTeamMember

	// 检查成员是否存在
	member, err := tm.WithContext(l.ctx).Where(tm.TeamID.Eq(teamID), tm.UserID.Eq(userID)).First()
	if err != nil {
		return errors.New("成员不存在")
	}

	// 如果是 owner 且要改成其他角色，需要确保至少有一个 owner
	if member.Role == "owner" && role != "owner" {
		ownerCount, err := tm.WithContext(l.ctx).Where(tm.TeamID.Eq(teamID), tm.Role.Eq("owner")).Count()
		if err != nil {
			return err
		}
		if ownerCount <= 1 {
			return errors.New("团队必须至少有一个所有者")
		}
	}

	_, err = tm.WithContext(l.ctx).Where(tm.TeamID.Eq(teamID), tm.UserID.Eq(userID)).Update(tm.Role, role)
	return err
}

// GetTeamMembers 获取团队成员列表
func (l *TeamMemberLogic) GetTeamMembers(teamID int64) ([]*model.TTeamMember, error) {
	q := query.Use(svc.Ctx.DB)
	tm := q.TTeamMember

	return tm.WithContext(l.ctx).Where(tm.TeamID.Eq(teamID)).Order(tm.ID.Asc()).Find()
}

// GetMember 获取单个成员信息
func (l *TeamMemberLogic) GetMember(teamID, userID int64) (*model.TTeamMember, error) {
	q := query.Use(svc.Ctx.DB)
	tm := q.TTeamMember

	return tm.WithContext(l.ctx).Where(tm.TeamID.Eq(teamID), tm.UserID.Eq(userID)).First()
}

// IsMember 检查用户是否是团队成员
func (l *TeamMemberLogic) IsMember(teamID, userID int64) (bool, error) {
	q := query.Use(svc.Ctx.DB)
	tm := q.TTeamMember

	count, err := tm.WithContext(l.ctx).Where(tm.TeamID.Eq(teamID), tm.UserID.Eq(userID)).Count()
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// IsOwnerOrAdmin 检查用户是否是团队所有者或管理员
func (l *TeamMemberLogic) IsOwnerOrAdmin(teamID, userID int64) (bool, error) {
	q := query.Use(svc.Ctx.DB)
	tm := q.TTeamMember

	count, err := tm.WithContext(l.ctx).Where(
		tm.TeamID.Eq(teamID),
		tm.UserID.Eq(userID),
		tm.Role.In("owner", "admin"),
	).Count()
	if err != nil {
		return false, err
	}
	return count > 0, nil
}
