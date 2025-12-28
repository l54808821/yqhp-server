package logic

import (
	"errors"

	"yqhp/admin/internal/model"
	"yqhp/admin/internal/types"

	"gorm.io/gorm"
)

// RoleLogic 角色逻辑
type RoleLogic struct {
	db *gorm.DB
}

// NewRoleLogic 创建角色逻辑
func NewRoleLogic(db *gorm.DB) *RoleLogic {
	return &RoleLogic{db: db}
}

// CreateRole 创建角色
func (l *RoleLogic) CreateRole(req *types.CreateRoleRequest) (*model.SysRole, error) {
	// 检查同一应用下角色编码是否存在
	var count int64
	l.db.Model(&model.SysRole{}).Where("app_id = ? AND code = ? AND is_delete = ?", req.AppID, req.Code, false).Count(&count)
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

	if err := l.db.Create(role).Error; err != nil {
		return nil, err
	}

	// 关联资源
	if len(req.ResourceIDs) > 0 {
		for _, resourceID := range req.ResourceIDs {
			l.db.Create(&model.SysRoleResource{
				RoleID:     role.ID,
				ResourceID: int64(resourceID),
				IsDelete:   model.BoolPtr(false),
			})
		}
	}

	return role, nil
}

// UpdateRole 更新角色
func (l *RoleLogic) UpdateRole(req *types.UpdateRoleRequest) error {
	updates := map[string]any{
		"name":   req.Name,
		"sort":   req.Sort,
		"status": req.Status,
		"remark": req.Remark,
	}

	if err := l.db.Model(&model.SysRole{}).Where("id = ?", req.ID).Updates(updates).Error; err != nil {
		return err
	}

	// 更新资源关联（软删除旧的，创建新的）
	l.db.Model(&model.SysRoleResource{}).Where("role_id = ? AND is_delete = ?", req.ID, false).Update("is_delete", true)
	if len(req.ResourceIDs) > 0 {
		for _, resourceID := range req.ResourceIDs {
			l.db.Create(&model.SysRoleResource{
				RoleID:     int64(req.ID),
				ResourceID: int64(resourceID),
				IsDelete:   model.BoolPtr(false),
			})
		}
	}

	return nil
}

// DeleteRole 删除角色（软删除）
func (l *RoleLogic) DeleteRole(id int64) error {
	// 检查是否有用户使用该角色
	var count int64
	l.db.Model(&model.SysUserRole{}).Where("role_id = ? AND is_delete = ?", id, false).Count(&count)
	if count > 0 {
		return errors.New("该角色下有用户，无法删除")
	}

	// 软删除角色资源关联
	l.db.Model(&model.SysRoleResource{}).Where("role_id = ? AND is_delete = ?", id, false).Update("is_delete", true)
	// 软删除角色
	return l.db.Model(&model.SysRole{}).Where("id = ?", id).Update("is_delete", true).Error
}

// GetRole 获取角色详情
func (l *RoleLogic) GetRole(id int64) (*model.SysRole, error) {
	var role model.SysRole
	if err := l.db.Where("is_delete = ?", false).First(&role, id).Error; err != nil {
		return nil, err
	}
	return &role, nil
}

// ListRoles 获取角色列表
func (l *RoleLogic) ListRoles(req *types.ListRolesRequest) ([]model.SysRole, int64, error) {
	var roles []model.SysRole
	var total int64

	query := l.db.Model(&model.SysRole{}).Where("is_delete = ?", false)

	if req.AppID > 0 {
		query = query.Where("app_id = ?", req.AppID)
	}
	if req.Name != "" {
		query = query.Where("name LIKE ?", "%"+req.Name+"%")
	}
	if req.Code != "" {
		query = query.Where("code LIKE ?", "%"+req.Code+"%")
	}
	if req.Status != nil {
		query = query.Where("status = ?", *req.Status)
	}

	query.Count(&total)

	if req.Page > 0 && req.PageSize > 0 {
		offset := (req.Page - 1) * req.PageSize
		query = query.Offset(offset).Limit(req.PageSize)
	}

	if err := query.Order("sort ASC").Find(&roles).Error; err != nil {
		return nil, 0, err
	}

	return roles, total, nil
}

// GetAllRoles 获取所有角色(用于下拉选择)
func (l *RoleLogic) GetAllRoles() ([]model.SysRole, error) {
	var roles []model.SysRole
	if err := l.db.Where("status = 1 AND is_delete = ?", false).Order("sort ASC").Find(&roles).Error; err != nil {
		return nil, err
	}
	return roles, nil
}

// GetRolesByAppID 获取指定应用的所有角色
func (l *RoleLogic) GetRolesByAppID(appID int64) ([]model.SysRole, error) {
	var roles []model.SysRole
	if err := l.db.Where("app_id = ? AND status = 1 AND is_delete = ?", appID, false).Order("sort ASC").Find(&roles).Error; err != nil {
		return nil, err
	}
	return roles, nil
}

// GetRoleResourceIDs 获取角色的资源ID列表
func (l *RoleLogic) GetRoleResourceIDs(roleID int64) ([]int64, error) {
	var resourceIDs []int64
	err := l.db.Model(&model.SysRoleResource{}).
		Where("role_id = ? AND is_delete = ?", roleID, false).
		Pluck("resource_id", &resourceIDs).Error
	return resourceIDs, err
}
