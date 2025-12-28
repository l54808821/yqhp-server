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
func (l *ResourceLogic) CreateResource(req *types.CreateResourceRequest) (*model.SysResource, error) {
	// 检查同一应用下权限标识是否存在
	if req.Code != "" {
		var count int64
		l.db.Model(&model.SysResource{}).Where("app_id = ? AND code = ? AND is_delete = ?", req.AppID, req.Code, false).Count(&count)
		if count > 0 {
			return nil, errors.New("权限标识已存在")
		}
	}

	resource := &model.SysResource{
		AppID:     int64(req.AppID),
		ParentID:  model.Int64Ptr(int64(req.ParentID)),
		Name:      req.Name,
		Code:      model.StringPtr(req.Code),
		Type:      int32(req.Type),
		Path:      model.StringPtr(req.Path),
		Component: model.StringPtr(req.Component),
		Redirect:  model.StringPtr(req.Redirect),
		Icon:      model.StringPtr(req.Icon),
		Sort:      model.Int64Ptr(int64(req.Sort)),
		IsHidden:  model.BoolPtr(req.IsHidden),
		IsCache:   model.BoolPtr(req.IsCache),
		IsFrame:   model.BoolPtr(req.IsFrame),
		Status:    model.Int32Ptr(int32(req.Status)),
		Remark:    model.StringPtr(req.Remark),
		IsDelete:  model.BoolPtr(false),
	}

	if err := l.db.Create(resource).Error; err != nil {
		return nil, err
	}

	return resource, nil
}

// UpdateResource 更新资源
func (l *ResourceLogic) UpdateResource(req *types.UpdateResourceRequest) error {
	// 获取原资源信息
	var original model.SysResource
	if err := l.db.First(&original, req.ID).Error; err != nil {
		return err
	}

	// 检查同一应用下权限标识是否存在
	if req.Code != "" {
		var count int64
		l.db.Model(&model.SysResource{}).Where("app_id = ? AND code = ? AND id != ? AND is_delete = ?", original.AppID, req.Code, req.ID, false).Count(&count)
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

	return l.db.Model(&model.SysResource{}).Where("id = ?", req.ID).Updates(updates).Error
}

// DeleteResource 删除资源（软删除）
func (l *ResourceLogic) DeleteResource(id int64) error {
	// 检查是否有子资源
	var count int64
	l.db.Model(&model.SysResource{}).Where("parent_id = ? AND is_delete = ?", id, false).Count(&count)
	if count > 0 {
		return errors.New("该资源下有子资源，无法删除")
	}

	// 软删除角色资源关联
	l.db.Model(&model.SysRoleResource{}).Where("resource_id = ? AND is_delete = ?", id, false).Update("is_delete", true)
	// 软删除资源
	return l.db.Model(&model.SysResource{}).Where("id = ?", id).Update("is_delete", true).Error
}

// GetResource 获取资源详情
func (l *ResourceLogic) GetResource(id int64) (*model.SysResource, error) {
	var resource model.SysResource
	if err := l.db.Where("is_delete = ?", false).First(&resource, id).Error; err != nil {
		return nil, err
	}
	return &resource, nil
}

// GetResourceTree 获取资源树
func (l *ResourceLogic) GetResourceTree() ([]model.ResourceWithChildren, error) {
	var resources []model.SysResource
	if err := l.db.Where("status = 1 AND is_delete = ?", false).Order("sort ASC").Find(&resources).Error; err != nil {
		return nil, err
	}

	return buildResourceTree(resources, 0), nil
}

// GetResourceTreeByAppID 获取指定应用的资源树
func (l *ResourceLogic) GetResourceTreeByAppID(appID int64) ([]model.ResourceWithChildren, error) {
	var resources []model.SysResource
	if err := l.db.Where("app_id = ? AND status = 1 AND is_delete = ?", appID, false).Order("sort ASC").Find(&resources).Error; err != nil {
		return nil, err
	}

	return buildResourceTree(resources, 0), nil
}

// GetAllResources 获取所有资源(平铺)
func (l *ResourceLogic) GetAllResources() ([]model.SysResource, error) {
	var resources []model.SysResource
	if err := l.db.Where("is_delete = ?", false).Order("sort ASC").Find(&resources).Error; err != nil {
		return nil, err
	}
	return resources, nil
}

// GetAllResourcesByAppID 获取指定应用的所有资源(平铺)
func (l *ResourceLogic) GetAllResourcesByAppID(appID int64) ([]model.SysResource, error) {
	var resources []model.SysResource
	if err := l.db.Where("app_id = ? AND is_delete = ?", appID, false).Order("sort ASC").Find(&resources).Error; err != nil {
		return nil, err
	}
	return resources, nil
}

// GetUserMenus 获取用户菜单
func (l *ResourceLogic) GetUserMenus(userID int64) ([]model.ResourceWithChildren, error) {
	// 获取用户的所有角色
	var roleIDs []int64
	err := l.db.Model(&model.SysUserRole{}).
		Where("user_id = ? AND is_delete = ?", userID, false).
		Pluck("role_id", &roleIDs).Error
	if err != nil {
		return nil, err
	}

	if len(roleIDs) == 0 {
		return []model.ResourceWithChildren{}, nil
	}

	// 获取角色关联的资源
	var resourceIDs []int64
	err = l.db.Model(&model.SysRoleResource{}).
		Where("role_id IN ? AND is_delete = ?", roleIDs, false).
		Pluck("resource_id", &resourceIDs).Error
	if err != nil {
		return nil, err
	}

	if len(resourceIDs) == 0 {
		return []model.ResourceWithChildren{}, nil
	}

	// 获取菜单类型的资源(1:目录 2:菜单)
	var resources []model.SysResource
	err = l.db.Where("id IN ? AND type IN (1, 2) AND status = 1 AND is_delete = ?", resourceIDs, false).
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
func (l *ResourceLogic) completeParentMenus(resources []model.SysResource) []model.SysResource {
	// 创建已有资源ID的映射
	existingIDs := make(map[int64]bool)
	for _, r := range resources {
		existingIDs[r.ID] = true
	}

	// 收集需要补全的父级ID
	needParentIDs := make(map[int64]bool)
	for _, r := range resources {
		parentID := model.GetInt64(r.ParentID)
		if parentID != 0 && !existingIDs[parentID] {
			needParentIDs[parentID] = true
		}
	}

	// 递归获取所有缺失的父级
	for len(needParentIDs) > 0 {
		var parentIDs []int64
		for id := range needParentIDs {
			parentIDs = append(parentIDs, id)
		}

		var parents []model.SysResource
		l.db.Where("id IN ? AND status = 1 AND is_delete = ?", parentIDs, false).Find(&parents)

		// 清空待查找列表
		needParentIDs = make(map[int64]bool)

		for _, p := range parents {
			if !existingIDs[p.ID] {
				resources = append(resources, p)
				existingIDs[p.ID] = true
			}
			// 检查这个父级是否还有更上级的父级需要补全
			parentID := model.GetInt64(p.ParentID)
			if parentID != 0 && !existingIDs[parentID] {
				needParentIDs[parentID] = true
			}
		}
	}

	return resources
}

// buildResourceTree 构建资源树
func buildResourceTree(resources []model.SysResource, parentID int64) []model.ResourceWithChildren {
	var tree []model.ResourceWithChildren
	for _, resource := range resources {
		resParentID := model.GetInt64(resource.ParentID)
		if resParentID == parentID {
			node := model.ResourceWithChildren{
				SysResource: resource,
			}
			children := buildResourceTree(resources, resource.ID)
			if len(children) > 0 {
				node.Children = children
			}
			tree = append(tree, node)
		}
	}
	return tree
}

// GetUserPermissionCodes 获取用户所有权限码（包括按钮权限）
func (l *ResourceLogic) GetUserPermissionCodes(userID int64) ([]string, error) {
	// 获取用户的所有角色
	var roleIDs []int64
	err := l.db.Model(&model.SysUserRole{}).
		Where("user_id = ? AND is_delete = ?", userID, false).
		Pluck("role_id", &roleIDs).Error
	if err != nil {
		return nil, err
	}

	if len(roleIDs) == 0 {
		return []string{}, nil
	}

	// 获取角色关联的资源
	var resourceIDs []int64
	err = l.db.Model(&model.SysRoleResource{}).
		Where("role_id IN ? AND is_delete = ?", roleIDs, false).
		Pluck("resource_id", &resourceIDs).Error
	if err != nil {
		return nil, err
	}

	if len(resourceIDs) == 0 {
		return []string{}, nil
	}

	// 获取所有资源的权限码
	var codes []string
	err = l.db.Model(&model.SysResource{}).
		Where("id IN ? AND code != '' AND code IS NOT NULL AND status = 1 AND is_delete = ?", resourceIDs, false).
		Pluck("code", &codes).Error
	if err != nil {
		return nil, err
	}

	return codes, nil
}
