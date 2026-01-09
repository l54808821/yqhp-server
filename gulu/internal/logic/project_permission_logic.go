package logic

import (
	"context"
	"errors"
	"time"

	"yqhp/gulu/internal/model"
	"yqhp/gulu/internal/query"
	"yqhp/gulu/internal/svc"
)

// ProjectPermissionLogic 项目权限逻辑
type ProjectPermissionLogic struct {
	ctx context.Context
}

// NewProjectPermissionLogic 创建项目权限逻辑
func NewProjectPermissionLogic(ctx context.Context) *ProjectPermissionLogic {
	return &ProjectPermissionLogic{ctx: ctx}
}

// 权限代码常量
const (
	PermissionWorkflowView    = "workflow:view"
	PermissionWorkflowCreate  = "workflow:create"
	PermissionWorkflowEdit    = "workflow:edit"
	PermissionWorkflowDelete  = "workflow:delete"
	PermissionWorkflowExecute = "workflow:execute"
	PermissionEnvView         = "env:view"
	PermissionEnvCreate       = "env:create"
	PermissionEnvEdit         = "env:edit"
	PermissionEnvDelete       = "env:delete"
	PermissionProjectAdmin    = "project:admin"
)

// GrantPermissionReq 授予权限请求
type GrantPermissionReq struct {
	UserID         int64  `json:"user_id" validate:"required"`
	PermissionCode string `json:"permission_code" validate:"required"`
}

// GrantPermission 授予权限
func (l *ProjectPermissionLogic) GrantPermission(projectID, userID int64, permissionCode string) (*model.TProjectPermission, error) {
	q := query.Use(svc.Ctx.DB)
	pp := q.TProjectPermission

	// 检查是否已有该权限
	exists, err := pp.WithContext(l.ctx).Where(
		pp.ProjectID.Eq(projectID),
		pp.UserID.Eq(userID),
		pp.PermissionCode.Eq(permissionCode),
	).Count()
	if err != nil {
		return nil, err
	}
	if exists > 0 {
		return nil, errors.New("用户已拥有该权限")
	}

	now := time.Now()
	permission := &model.TProjectPermission{
		CreatedAt:      &now,
		ProjectID:      projectID,
		UserID:         userID,
		PermissionCode: permissionCode,
	}

	err = pp.WithContext(l.ctx).Create(permission)
	if err != nil {
		return nil, err
	}

	return permission, nil
}

// RevokePermission 撤销权限
func (l *ProjectPermissionLogic) RevokePermission(projectID, userID int64, permissionCode string) error {
	q := query.Use(svc.Ctx.DB)
	pp := q.TProjectPermission

	_, err := pp.WithContext(l.ctx).Where(
		pp.ProjectID.Eq(projectID),
		pp.UserID.Eq(userID),
		pp.PermissionCode.Eq(permissionCode),
	).Delete()
	return err
}

// GetUserPermissions 获取用户在项目中的所有权限
func (l *ProjectPermissionLogic) GetUserPermissions(projectID, userID int64) ([]string, error) {
	q := query.Use(svc.Ctx.DB)
	pp := q.TProjectPermission

	permissions, err := pp.WithContext(l.ctx).Where(
		pp.ProjectID.Eq(projectID),
		pp.UserID.Eq(userID),
	).Find()
	if err != nil {
		return nil, err
	}

	codes := make([]string, len(permissions))
	for i, p := range permissions {
		codes[i] = p.PermissionCode
	}

	return codes, nil
}

// CheckPermission 检查用户是否有指定权限
func (l *ProjectPermissionLogic) CheckPermission(projectID, userID int64, permissionCode string) (bool, error) {
	q := query.Use(svc.Ctx.DB)
	pp := q.TProjectPermission

	// 先检查是否有项目管理员权限
	adminCount, err := pp.WithContext(l.ctx).Where(
		pp.ProjectID.Eq(projectID),
		pp.UserID.Eq(userID),
		pp.PermissionCode.Eq(PermissionProjectAdmin),
	).Count()
	if err != nil {
		return false, err
	}
	if adminCount > 0 {
		return true, nil // 管理员拥有所有权限
	}

	// 检查具体权限
	count, err := pp.WithContext(l.ctx).Where(
		pp.ProjectID.Eq(projectID),
		pp.UserID.Eq(userID),
		pp.PermissionCode.Eq(permissionCode),
	).Count()
	if err != nil {
		return false, err
	}

	return count > 0, nil
}

// GetProjectPermissions 获取项目的所有权限配置
func (l *ProjectPermissionLogic) GetProjectPermissions(projectID int64) ([]*model.TProjectPermission, error) {
	q := query.Use(svc.Ctx.DB)
	pp := q.TProjectPermission

	return pp.WithContext(l.ctx).Where(pp.ProjectID.Eq(projectID)).Order(pp.UserID.Asc()).Find()
}

// GrantAllPermissions 授予用户项目的所有权限
func (l *ProjectPermissionLogic) GrantAllPermissions(projectID, userID int64) error {
	allPermissions := []string{
		PermissionWorkflowView,
		PermissionWorkflowCreate,
		PermissionWorkflowEdit,
		PermissionWorkflowDelete,
		PermissionWorkflowExecute,
		PermissionEnvView,
		PermissionEnvCreate,
		PermissionEnvEdit,
		PermissionEnvDelete,
	}

	for _, code := range allPermissions {
		_, _ = l.GrantPermission(projectID, userID, code)
	}

	return nil
}

// RevokeAllPermissions 撤销用户在项目中的所有权限
func (l *ProjectPermissionLogic) RevokeAllPermissions(projectID, userID int64) error {
	q := query.Use(svc.Ctx.DB)
	pp := q.TProjectPermission

	_, err := pp.WithContext(l.ctx).Where(
		pp.ProjectID.Eq(projectID),
		pp.UserID.Eq(userID),
	).Delete()
	return err
}
