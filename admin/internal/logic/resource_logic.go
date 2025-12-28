package logic

import (
	"errors"

	"yqhp/admin/internal/model"
	"yqhp/admin/internal/types"

	"gorm.io/gorm"
)

// ResourceLogic 资源逻辑
type ResourceLogic struct {
	db *gorm.DB
}

// NewResourceLogic 创建资源逻辑
func NewResourceLogic(db *gorm.DB) *ResourceLogic {
	return &ResourceLogic{db: db}
}

// CreateResource 创建资源
func (l *ResourceLogic) CreateResource(req *types.CreateResourceRequest) (*model.Resource, error) {
	// 检查同一应用下权限标识是否存在
	if req.Code != "" {
		var count int64
		l.db.Model(&model.Resource{}).Where("app_id = ? AND code = ?", req.AppID, req.Code).Count(&count)
		if count > 0 {
			return nil, errors.New("权限标识已存在")
		}
	}

	resource := &model.Resource{
		AppID:     req.AppID,
		ParentID:  req.ParentID,
		Name:      req.Name,
		Code:      req.Code,
		Type:      req.Type,
		Path:      req.Path,
		Component: req.Component,
		Redirect:  req.Redirect,
		Icon:      req.Icon,
		Sort:      req.Sort,
		IsHidden:  req.IsHidden,
		IsCache:   req.IsCache,
		IsFrame:   req.IsFrame,
		Status:    req.Status,
		Remark:    req.Remark,
	}

	if err := l.db.Create(resource).Error; err != nil {
		return nil, err
	}

	return resource, nil
}

// UpdateResource 更新资源
func (l *ResourceLogic) UpdateResource(req *types.UpdateResourceRequest) error {
	// 获取原资源信息
	var original model.Resource
	if err := l.db.First(&original, req.ID).Error; err != nil {
		return err
	}

	// 检查同一应用下权限标识是否存在
	if req.Code != "" {
		var count int64
		l.db.Model(&model.Resource{}).Where("app_id = ? AND code = ? AND id != ?", original.AppID, req.Code, req.ID).Count(&count)
		if count > 0 {
			return errors.New("权限标识已存在")
		}
	}

	updates := map[string]any{
		"parent_id": req.ParentID,
		"name":      req.Name,
		"code":      req.Code,
		"type":      req.Type,
		"path":      req.Path,
		"component": req.Component,
		"redirect":  req.Redirect,
		"icon":      req.Icon,
		"sort":      req.Sort,
		"is_hidden": req.IsHidden,
		"is_cache":  req.IsCache,
		"is_frame":  req.IsFrame,
		"status":    req.Status,
		"remark":    req.Remark,
	}

	return l.db.Model(&model.Resource{}).Where("id = ?", req.ID).Updates(updates).Error
}

// DeleteResource 删除资源
func (l *ResourceLogic) DeleteResource(id uint) error {
	// 检查是否有子资源
	var count int64
	l.db.Model(&model.Resource{}).Where("parent_id = ?", id).Count(&count)
	if count > 0 {
		return errors.New("该资源下有子资源，无法删除")
	}

	// 删除角色资源关联
	l.db.Where("resource_id = ?", id).Delete(&model.RoleResource{})
	// 删除资源
	return l.db.Delete(&model.Resource{}, id).Error
}

// GetResource 获取资源详情
func (l *ResourceLogic) GetResource(id uint) (*model.Resource, error) {
	var resource model.Resource
	if err := l.db.First(&resource, id).Error; err != nil {
		return nil, err
	}
	return &resource, nil
}

// GetResourceTree 获取资源树
func (l *ResourceLogic) GetResourceTree() ([]model.Resource, error) {
	var resources []model.Resource
	if err := l.db.Where("status = 1").Order("sort ASC").Find(&resources).Error; err != nil {
		return nil, err
	}

	return buildResourceTree(resources, 0), nil
}

// GetResourceTreeByAppID 获取指定应用的资源树
func (l *ResourceLogic) GetResourceTreeByAppID(appID uint) ([]model.Resource, error) {
	var resources []model.Resource
	if err := l.db.Where("app_id = ? AND status = 1", appID).Order("sort ASC").Find(&resources).Error; err != nil {
		return nil, err
	}

	return buildResourceTree(resources, 0), nil
}

// GetAllResources 获取所有资源(平铺)
func (l *ResourceLogic) GetAllResources() ([]model.Resource, error) {
	var resources []model.Resource
	if err := l.db.Order("sort ASC").Find(&resources).Error; err != nil {
		return nil, err
	}
	return resources, nil
}

// GetAllResourcesByAppID 获取指定应用的所有资源(平铺)
func (l *ResourceLogic) GetAllResourcesByAppID(appID uint) ([]model.Resource, error) {
	var resources []model.Resource
	if err := l.db.Where("app_id = ?", appID).Order("sort ASC").Find(&resources).Error; err != nil {
		return nil, err
	}
	return resources, nil
}

// GetUserMenus 获取用户菜单
func (l *ResourceLogic) GetUserMenus(userID uint) ([]model.Resource, error) {
	// 获取用户的所有角色
	var roleIDs []uint
	err := l.db.Model(&model.UserRole{}).
		Where("user_id = ?", userID).
		Pluck("role_id", &roleIDs).Error
	if err != nil {
		return nil, err
	}

	if len(roleIDs) == 0 {
		return []model.Resource{}, nil
	}

	// 获取角色关联的资源
	var resourceIDs []uint
	err = l.db.Model(&model.RoleResource{}).
		Where("role_id IN ?", roleIDs).
		Pluck("resource_id", &resourceIDs).Error
	if err != nil {
		return nil, err
	}

	if len(resourceIDs) == 0 {
		return []model.Resource{}, nil
	}

	// 获取菜单类型的资源(1:目录 2:菜单)
	var resources []model.Resource
	err = l.db.Where("id IN ? AND type IN (1, 2) AND status = 1", resourceIDs).
		Order("sort ASC").
		Find(&resources).Error
	if err != nil {
		return nil, err
	}

	// 补全父级目录：确保所有子菜单的父目录都存在
	resources = l.completeParentMenus(resources)

	return buildResourceTree(resources, 0), nil
}

// completeParentMenus 补全父级菜单
func (l *ResourceLogic) completeParentMenus(resources []model.Resource) []model.Resource {
	// 创建已有资源ID的映射
	existingIDs := make(map[uint]bool)
	for _, r := range resources {
		existingIDs[r.ID] = true
	}

	// 收集需要补全的父级ID
	needParentIDs := make(map[uint]bool)
	for _, r := range resources {
		if r.ParentID != 0 && !existingIDs[r.ParentID] {
			needParentIDs[r.ParentID] = true
		}
	}

	// 递归获取所有缺失的父级
	for len(needParentIDs) > 0 {
		var parentIDs []uint
		for id := range needParentIDs {
			parentIDs = append(parentIDs, id)
		}

		var parents []model.Resource
		l.db.Where("id IN ? AND status = 1", parentIDs).Find(&parents)

		// 清空待查找列表
		needParentIDs = make(map[uint]bool)

		for _, p := range parents {
			if !existingIDs[p.ID] {
				resources = append(resources, p)
				existingIDs[p.ID] = true
			}
			// 检查这个父级是否还有更上级的父级需要补全
			if p.ParentID != 0 && !existingIDs[p.ParentID] {
				needParentIDs[p.ParentID] = true
			}
		}
	}

	return resources
}

// buildResourceTree 构建资源树
func buildResourceTree(resources []model.Resource, parentID uint) []model.Resource {
	var tree []model.Resource
	for _, resource := range resources {
		if resource.ParentID == parentID {
			children := buildResourceTree(resources, resource.ID)
			if len(children) > 0 {
				resource.Children = children
			}
			tree = append(tree, resource)
		}
	}
	return tree
}

// GetUserPermissionCodes 获取用户所有权限码（包括按钮权限）
func (l *ResourceLogic) GetUserPermissionCodes(userID uint) ([]string, error) {
	// 获取用户的所有角色
	var roleIDs []uint
	err := l.db.Model(&model.UserRole{}).
		Where("user_id = ?", userID).
		Pluck("role_id", &roleIDs).Error
	if err != nil {
		return nil, err
	}

	if len(roleIDs) == 0 {
		return []string{}, nil
	}

	// 获取角色关联的资源
	var resourceIDs []uint
	err = l.db.Model(&model.RoleResource{}).
		Where("role_id IN ?", roleIDs).
		Pluck("resource_id", &resourceIDs).Error
	if err != nil {
		return nil, err
	}

	if len(resourceIDs) == 0 {
		return []string{}, nil
	}

	// 获取所有资源的权限码
	var codes []string
	err = l.db.Model(&model.Resource{}).
		Where("id IN ? AND code != '' AND status = 1", resourceIDs).
		Pluck("code", &codes).Error
	if err != nil {
		return nil, err
	}

	return codes, nil
}
