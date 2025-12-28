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
func (l *RoleLogic) CreateRole(req *types.CreateRoleRequest) (*model.Role, error) {
	// 检查角色编码是否存在
	var count int64
	l.db.Model(&model.Role{}).Where("code = ?", req.Code).Count(&count)
	if count > 0 {
		return nil, errors.New("角色编码已存在")
	}

	role := &model.Role{
		Name:   req.Name,
		Code:   req.Code,
		Sort:   req.Sort,
		Status: req.Status,
		Remark: req.Remark,
	}

	if err := l.db.Create(role).Error; err != nil {
		return nil, err
	}

	// 关联资源
	if len(req.ResourceIDs) > 0 {
		for _, resourceID := range req.ResourceIDs {
			l.db.Create(&model.RoleResource{
				RoleID:     role.ID,
				ResourceID: resourceID,
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

	if err := l.db.Model(&model.Role{}).Where("id = ?", req.ID).Updates(updates).Error; err != nil {
		return err
	}

	// 更新资源关联
	l.db.Where("role_id = ?", req.ID).Delete(&model.RoleResource{})
	if len(req.ResourceIDs) > 0 {
		for _, resourceID := range req.ResourceIDs {
			l.db.Create(&model.RoleResource{
				RoleID:     req.ID,
				ResourceID: resourceID,
			})
		}
	}

	return nil
}

// DeleteRole 删除角色
func (l *RoleLogic) DeleteRole(id uint) error {
	// 检查是否有用户使用该角色
	var count int64
	l.db.Model(&model.UserRole{}).Where("role_id = ?", id).Count(&count)
	if count > 0 {
		return errors.New("该角色下有用户，无法删除")
	}

	// 删除角色资源关联
	l.db.Where("role_id = ?", id).Delete(&model.RoleResource{})
	// 删除角色
	return l.db.Delete(&model.Role{}, id).Error
}

// GetRole 获取角色详情
func (l *RoleLogic) GetRole(id uint) (*model.Role, error) {
	var role model.Role
	if err := l.db.Preload("Resources").First(&role, id).Error; err != nil {
		return nil, err
	}
	return &role, nil
}

// ListRoles 获取角色列表
func (l *RoleLogic) ListRoles(req *types.ListRolesRequest) ([]model.Role, int64, error) {
	var roles []model.Role
	var total int64

	query := l.db.Model(&model.Role{})

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
func (l *RoleLogic) GetAllRoles() ([]model.Role, error) {
	var roles []model.Role
	if err := l.db.Where("status = 1").Order("sort ASC").Find(&roles).Error; err != nil {
		return nil, err
	}
	return roles, nil
}

// GetRoleResourceIDs 获取角色的资源ID列表
func (l *RoleLogic) GetRoleResourceIDs(roleID uint) ([]uint, error) {
	var resourceIDs []uint
	err := l.db.Model(&model.RoleResource{}).
		Where("role_id = ?", roleID).
		Pluck("resource_id", &resourceIDs).Error
	return resourceIDs, err
}
