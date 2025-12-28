package service

import (
	"errors"

	"yqhp/admin/internal/model"

	"gorm.io/gorm"
)

// RoleService 角色服务
type RoleService struct {
	db *gorm.DB
}

// NewRoleService 创建角色服务
func NewRoleService(db *gorm.DB) *RoleService {
	return &RoleService{db: db}
}

// CreateRoleRequest 创建角色请求
type CreateRoleRequest struct {
	Name        string `json:"name" validate:"required"`
	Code        string `json:"code" validate:"required"`
	Sort        int    `json:"sort"`
	Status      int8   `json:"status"`
	Remark      string `json:"remark"`
	ResourceIDs []uint `json:"resourceIds"`
}

// CreateRole 创建角色
func (s *RoleService) CreateRole(req *CreateRoleRequest) (*model.Role, error) {
	// 检查角色编码是否存在
	var count int64
	s.db.Model(&model.Role{}).Where("code = ?", req.Code).Count(&count)
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

	if err := s.db.Create(role).Error; err != nil {
		return nil, err
	}

	// 关联资源
	if len(req.ResourceIDs) > 0 {
		for _, resourceID := range req.ResourceIDs {
			s.db.Create(&model.RoleResource{
				RoleID:     role.ID,
				ResourceID: resourceID,
			})
		}
	}

	return role, nil
}

// UpdateRoleRequest 更新角色请求
type UpdateRoleRequest struct {
	ID          uint   `json:"id" validate:"required"`
	Name        string `json:"name"`
	Sort        int    `json:"sort"`
	Status      int8   `json:"status"`
	Remark      string `json:"remark"`
	ResourceIDs []uint `json:"resourceIds"`
}

// UpdateRole 更新角色
func (s *RoleService) UpdateRole(req *UpdateRoleRequest) error {
	updates := map[string]any{
		"name":   req.Name,
		"sort":   req.Sort,
		"status": req.Status,
		"remark": req.Remark,
	}

	if err := s.db.Model(&model.Role{}).Where("id = ?", req.ID).Updates(updates).Error; err != nil {
		return err
	}

	// 更新资源关联
	s.db.Where("role_id = ?", req.ID).Delete(&model.RoleResource{})
	if len(req.ResourceIDs) > 0 {
		for _, resourceID := range req.ResourceIDs {
			s.db.Create(&model.RoleResource{
				RoleID:     req.ID,
				ResourceID: resourceID,
			})
		}
	}

	return nil
}

// DeleteRole 删除角色
func (s *RoleService) DeleteRole(id uint) error {
	// 检查是否有用户使用该角色
	var count int64
	s.db.Model(&model.UserRole{}).Where("role_id = ?", id).Count(&count)
	if count > 0 {
		return errors.New("该角色下有用户，无法删除")
	}

	// 删除角色资源关联
	s.db.Where("role_id = ?", id).Delete(&model.RoleResource{})
	// 删除角色
	return s.db.Delete(&model.Role{}, id).Error
}

// GetRole 获取角色详情
func (s *RoleService) GetRole(id uint) (*model.Role, error) {
	var role model.Role
	if err := s.db.Preload("Resources").First(&role, id).Error; err != nil {
		return nil, err
	}
	return &role, nil
}

// ListRolesRequest 角色列表请求
type ListRolesRequest struct {
	Page     int    `json:"page"`
	PageSize int    `json:"pageSize"`
	Name     string `json:"name"`
	Code     string `json:"code"`
	Status   *int8  `json:"status"`
}

// ListRoles 获取角色列表
func (s *RoleService) ListRoles(req *ListRolesRequest) ([]model.Role, int64, error) {
	var roles []model.Role
	var total int64

	query := s.db.Model(&model.Role{})

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
func (s *RoleService) GetAllRoles() ([]model.Role, error) {
	var roles []model.Role
	if err := s.db.Where("status = 1").Order("sort ASC").Find(&roles).Error; err != nil {
		return nil, err
	}
	return roles, nil
}

// GetRoleResourceIDs 获取角色的资源ID列表
func (s *RoleService) GetRoleResourceIDs(roleID uint) ([]uint, error) {
	var resourceIDs []uint
	err := s.db.Model(&model.RoleResource{}).
		Where("role_id = ?", roleID).
		Pluck("resource_id", &resourceIDs).Error
	return resourceIDs, err
}

