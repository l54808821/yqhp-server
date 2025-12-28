package logic

import (
	"context"
	"errors"

	"yqhp/admin/internal/model"
	"yqhp/admin/internal/query"
	"yqhp/admin/internal/svc"
	"yqhp/admin/internal/types"

	"github.com/gofiber/fiber/v2"
)

// RoleLogic 角色逻辑
type RoleLogic struct {
	ctx context.Context
}

// NewRoleLogic 创建角色逻辑
func NewRoleLogic(c *fiber.Ctx) *RoleLogic {
	return &RoleLogic{ctx: c.Context()}
}

func (l *RoleLogic) db() *query.Query {
	return svc.Ctx.Query
}

// CreateRole 创建角色
func (l *RoleLogic) CreateRole(req *types.CreateRoleRequest) (*types.RoleInfo, error) {
	r := l.db().SysRole
	count, _ := r.WithContext(l.ctx).Where(r.AppID.Eq(int64(req.AppID)), r.Code.Eq(req.Code), r.IsDelete.Is(false)).Count()
	if count > 0 {
		return nil, errors.New("角色编码已存在")
	}

	role := &model.SysRole{
		AppID:    int64(req.AppID),
		Name:     req.Name,
		Code:     req.Code,
		Sort:     model.Int64Ptr(int64(req.Sort)),
		Status:   model.Int32Ptr(int32(req.Status)),
		Remark:   model.StringPtr(req.Remark),
		IsDelete: model.BoolPtr(false),
	}

	if err := r.WithContext(l.ctx).Create(role); err != nil {
		return nil, err
	}

	// 关联资源
	if len(req.ResourceIDs) > 0 {
		rr := l.db().SysRoleResource
		for _, resourceID := range req.ResourceIDs {
			rr.WithContext(l.ctx).Create(&model.SysRoleResource{
				RoleID:     role.ID,
				ResourceID: int64(resourceID),
				IsDelete:   model.BoolPtr(false),
			})
		}
	}

	return types.ToRoleInfo(role), nil
}

// UpdateRole 更新角色
func (l *RoleLogic) UpdateRole(req *types.UpdateRoleRequest) error {
	r := l.db().SysRole
	_, err := r.WithContext(l.ctx).Where(r.ID.Eq(int64(req.ID))).Updates(map[string]any{
		"name":   req.Name,
		"sort":   req.Sort,
		"status": req.Status,
		"remark": req.Remark,
	})
	if err != nil {
		return err
	}

	// 更新资源关联
	rr := l.db().SysRoleResource
	rr.WithContext(l.ctx).Where(rr.RoleID.Eq(int64(req.ID)), rr.IsDelete.Is(false)).Update(rr.IsDelete, true)
	if len(req.ResourceIDs) > 0 {
		for _, resourceID := range req.ResourceIDs {
			rr.WithContext(l.ctx).Create(&model.SysRoleResource{
				RoleID:     int64(req.ID),
				ResourceID: int64(resourceID),
				IsDelete:   model.BoolPtr(false),
			})
		}
	}

	return nil
}

// DeleteRole 删除角色
func (l *RoleLogic) DeleteRole(id int64) error {
	ur := l.db().SysUserRole
	count, _ := ur.WithContext(l.ctx).Where(ur.RoleID.Eq(id), ur.IsDelete.Is(false)).Count()
	if count > 0 {
		return errors.New("该角色下有用户，无法删除")
	}

	rr := l.db().SysRoleResource
	rr.WithContext(l.ctx).Where(rr.RoleID.Eq(id), rr.IsDelete.Is(false)).Update(rr.IsDelete, true)

	r := l.db().SysRole
	_, err := r.WithContext(l.ctx).Where(r.ID.Eq(id)).Update(r.IsDelete, true)
	return err
}

// GetRole 获取角色详情
func (l *RoleLogic) GetRole(id int64) (*types.RoleInfo, error) {
	r := l.db().SysRole
	role, err := r.WithContext(l.ctx).Where(r.ID.Eq(id), r.IsDelete.Is(false)).First()
	if err != nil {
		return nil, err
	}
	return types.ToRoleInfo(role), nil
}

// ListRoles 获取角色列表
func (l *RoleLogic) ListRoles(req *types.ListRolesRequest) ([]*types.RoleInfo, int64, error) {
	r := l.db().SysRole
	q := r.WithContext(l.ctx).Where(r.IsDelete.Is(false))

	if req.AppID > 0 {
		q = q.Where(r.AppID.Eq(int64(req.AppID)))
	}
	if req.Name != "" {
		q = q.Where(r.Name.Like("%" + req.Name + "%"))
	}
	if req.Code != "" {
		q = q.Where(r.Code.Like("%" + req.Code + "%"))
	}
	if req.Status != nil {
		q = q.Where(r.Status.Eq(int32(*req.Status)))
	}

	total, _ := q.Count()

	if req.Page > 0 && req.PageSize > 0 {
		q = q.Offset((req.Page - 1) * req.PageSize).Limit(req.PageSize)
	}

	roles, err := q.Order(r.Sort).Find()
	if err != nil {
		return nil, 0, err
	}
	return types.ToRoleInfoList(roles), total, nil
}

// GetAllRoles 获取所有角色
func (l *RoleLogic) GetAllRoles() ([]*types.RoleInfo, error) {
	r := l.db().SysRole
	roles, err := r.WithContext(l.ctx).Where(r.Status.Eq(1), r.IsDelete.Is(false)).Order(r.Sort).Find()
	if err != nil {
		return nil, err
	}
	return types.ToRoleInfoList(roles), nil
}

// GetRoleResourceIDs 获取角色的资源ID列表
func (l *RoleLogic) GetRoleResourceIDs(roleID int64) ([]int64, error) {
	rr := l.db().SysRoleResource
	resources, err := rr.WithContext(l.ctx).Where(rr.RoleID.Eq(roleID), rr.IsDelete.Is(false)).Find()
	if err != nil {
		return nil, err
	}

	ids := make([]int64, len(resources))
	for i, r := range resources {
		ids[i] = r.ResourceID
	}
	return ids, nil
}
