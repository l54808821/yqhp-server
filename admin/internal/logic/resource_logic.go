package logic

import (
	"context"
	"errors"

	"yqhp/admin/internal/ctxutil"
	"yqhp/admin/internal/model"
	"yqhp/admin/internal/query"
	"yqhp/admin/internal/svc"
	"yqhp/admin/internal/types"

	"github.com/gofiber/fiber/v2"
)

// ResourceLogic 资源逻辑
type ResourceLogic struct {
	ctx context.Context
}

// NewResourceLogic 创建资源逻辑
func NewResourceLogic(c *fiber.Ctx) *ResourceLogic {
	return &ResourceLogic{ctx: c.UserContext()}
}

func (l *ResourceLogic) db() *query.Query {
	return svc.Ctx.Query
}

// CreateResource 创建资源
func (l *ResourceLogic) CreateResource(req *types.CreateResourceRequest) (*types.ResourceInfo, error) {
	r := l.db().SysResource
	if req.Code != "" {
		count, _ := r.WithContext(l.ctx).Where(r.AppID.Eq(int64(req.AppID)), r.Code.Eq(req.Code), r.IsDelete.Is(false)).Count()
		if count > 0 {
			return nil, errors.New("权限标识已存在")
		}
	}

	userID := ctxutil.GetUserID(l.ctx)
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
		CreatedBy: model.Int64Ptr(userID),
		UpdatedBy: model.Int64Ptr(userID),
	}

	if err := r.WithContext(l.ctx).Create(resource); err != nil {
		return nil, err
	}

	return types.ToResourceInfo(resource), nil
}

// UpdateResource 更新资源
func (l *ResourceLogic) UpdateResource(req *types.UpdateResourceRequest) error {
	r := l.db().SysResource
	original, err := r.WithContext(l.ctx).Where(r.ID.Eq(int64(req.ID))).First()
	if err != nil {
		return err
	}

	if req.Code != "" {
		count, _ := r.WithContext(l.ctx).Where(r.AppID.Eq(original.AppID), r.Code.Eq(req.Code), r.ID.Neq(int64(req.ID)), r.IsDelete.Is(false)).Count()
		if count > 0 {
			return errors.New("权限标识已存在")
		}
	}

	userID := ctxutil.GetUserID(l.ctx)
	_, err = r.WithContext(l.ctx).Where(r.ID.Eq(int64(req.ID))).Updates(map[string]any{
		"parent_id":  req.ParentID,
		"name":       req.Name,
		"code":       req.Code,
		"type":       req.Type,
		"path":       req.Path,
		"component":  req.Component,
		"redirect":   req.Redirect,
		"icon":       req.Icon,
		"sort":       req.Sort,
		"is_hidden":  req.IsHidden,
		"is_cache":   req.IsCache,
		"is_frame":   req.IsFrame,
		"status":     req.Status,
		"remark":     req.Remark,
		"updated_by": userID,
	})
	return err
}

// DeleteResource 删除资源
func (l *ResourceLogic) DeleteResource(id int64) error {
	r := l.db().SysResource
	count, _ := r.WithContext(l.ctx).Where(r.ParentID.Eq(id), r.IsDelete.Is(false)).Count()
	if count > 0 {
		return errors.New("该资源下有子资源，无法删除")
	}

	rr := l.db().SysRoleResource
	rr.WithContext(l.ctx).Where(rr.ResourceID.Eq(id), rr.IsDelete.Is(false)).Update(rr.IsDelete, true)

	_, err := r.WithContext(l.ctx).Where(r.ID.Eq(id)).Update(r.IsDelete, true)
	return err
}

// GetResource 获取资源详情
func (l *ResourceLogic) GetResource(id int64) (*types.ResourceInfo, error) {
	r := l.db().SysResource
	resource, err := r.WithContext(l.ctx).Where(r.ID.Eq(id), r.IsDelete.Is(false)).First()
	if err != nil {
		return nil, err
	}
	return types.ToResourceInfo(resource), nil
}

// GetResourceTree 获取资源树
func (l *ResourceLogic) GetResourceTree(includeDisabled bool) ([]types.ResourceTreeInfo, error) {
	r := l.db().SysResource
	q := r.WithContext(l.ctx).Where(r.IsDelete.Is(false))
	if !includeDisabled {
		q = q.Where(r.Status.Eq(1))
	}
	resources, err := q.Order(r.Sort).Find()
	if err != nil {
		return nil, err
	}

	return types.BuildResourceTree(resources, 0), nil
}

// GetResourceTreeByAppID 获取指定应用的资源树
func (l *ResourceLogic) GetResourceTreeByAppID(appID int64, includeDisabled bool) ([]types.ResourceTreeInfo, error) {
	r := l.db().SysResource
	q := r.WithContext(l.ctx).Where(r.AppID.Eq(appID), r.IsDelete.Is(false))
	if !includeDisabled {
		q = q.Where(r.Status.Eq(1))
	}
	resources, err := q.Order(r.Sort).Find()
	if err != nil {
		return nil, err
	}

	return types.BuildResourceTree(resources, 0), nil
}

// GetAllResources 获取所有资源
func (l *ResourceLogic) GetAllResources() ([]*types.ResourceInfo, error) {
	r := l.db().SysResource
	resources, err := r.WithContext(l.ctx).Where(r.IsDelete.Is(false)).Order(r.Sort).Find()
	if err != nil {
		return nil, err
	}
	return types.ToResourceInfoList(resources), nil
}

// GetAllResourcesByAppID 获取指定应用的所有资源
func (l *ResourceLogic) GetAllResourcesByAppID(appID int64) ([]*types.ResourceInfo, error) {
	r := l.db().SysResource
	resources, err := r.WithContext(l.ctx).Where(r.AppID.Eq(appID), r.IsDelete.Is(false)).Order(r.Sort).Find()
	if err != nil {
		return nil, err
	}
	return types.ToResourceInfoList(resources), nil
}

// GetUserMenus 获取用户菜单
func (l *ResourceLogic) GetUserMenus(userID int64) ([]types.ResourceTreeInfo, error) {
	ur := l.db().SysUserRole
	userRoles, err := ur.WithContext(l.ctx).Where(ur.UserID.Eq(userID), ur.IsDelete.Is(false)).Find()
	if err != nil || len(userRoles) == 0 {
		return []types.ResourceTreeInfo{}, nil
	}

	roleIDs := make([]int64, len(userRoles))
	for i, r := range userRoles {
		roleIDs[i] = r.RoleID
	}

	rr := l.db().SysRoleResource
	roleResources, err := rr.WithContext(l.ctx).Where(rr.RoleID.In(roleIDs...), rr.IsDelete.Is(false)).Find()
	if err != nil || len(roleResources) == 0 {
		return []types.ResourceTreeInfo{}, nil
	}

	resourceIDs := make([]int64, len(roleResources))
	for i, r := range roleResources {
		resourceIDs[i] = r.ResourceID
	}

	r := l.db().SysResource
	resources, err := r.WithContext(l.ctx).Where(r.ID.In(resourceIDs...), r.Type.In(1, 2), r.Status.Eq(1), r.IsDelete.Is(false)).Order(r.Sort).Find()
	if err != nil {
		return nil, err
	}

	resources = l.completeParentMenus(resources)
	return types.BuildResourceTree(resources, 0), nil
}

// completeParentMenus 补全父级菜单
func (l *ResourceLogic) completeParentMenus(resources []*model.SysResource) []*model.SysResource {
	existingIDs := make(map[int64]bool)
	for _, r := range resources {
		existingIDs[r.ID] = true
	}

	needParentIDs := make(map[int64]bool)
	for _, r := range resources {
		parentID := model.GetInt64(r.ParentID)
		if parentID != 0 && !existingIDs[parentID] {
			needParentIDs[parentID] = true
		}
	}

	res := l.db().SysResource
	for len(needParentIDs) > 0 {
		var parentIDs []int64
		for id := range needParentIDs {
			parentIDs = append(parentIDs, id)
		}

		parents, _ := res.WithContext(l.ctx).Where(res.ID.In(parentIDs...), res.Status.Eq(1), res.IsDelete.Is(false)).Find()

		needParentIDs = make(map[int64]bool)
		for _, p := range parents {
			if !existingIDs[p.ID] {
				resources = append(resources, p)
				existingIDs[p.ID] = true
			}
			parentID := model.GetInt64(p.ParentID)
			if parentID != 0 && !existingIDs[parentID] {
				needParentIDs[parentID] = true
			}
		}
	}

	return resources
}

// buildResourceTree 构建资源树
func buildResourceTree(resources []*model.SysResource, parentID int64) []model.ResourceWithChildren {
	var tree []model.ResourceWithChildren
	for _, resource := range resources {
		resParentID := model.GetInt64(resource.ParentID)
		if resParentID == parentID {
			node := model.ResourceWithChildren{
				SysResource: *resource,
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

// GetUserPermissionCodes 获取用户所有权限码
func (l *ResourceLogic) GetUserPermissionCodes(userID int64) ([]string, error) {
	ur := l.db().SysUserRole
	userRoles, err := ur.WithContext(l.ctx).Where(ur.UserID.Eq(userID), ur.IsDelete.Is(false)).Find()
	if err != nil || len(userRoles) == 0 {
		return []string{}, nil
	}

	roleIDs := make([]int64, len(userRoles))
	for i, r := range userRoles {
		roleIDs[i] = r.RoleID
	}

	rr := l.db().SysRoleResource
	roleResources, err := rr.WithContext(l.ctx).Where(rr.RoleID.In(roleIDs...), rr.IsDelete.Is(false)).Find()
	if err != nil || len(roleResources) == 0 {
		return []string{}, nil
	}

	resourceIDs := make([]int64, len(roleResources))
	for i, r := range roleResources {
		resourceIDs[i] = r.ResourceID
	}

	r := l.db().SysResource
	resources, err := r.WithContext(l.ctx).Where(r.ID.In(resourceIDs...), r.Code.IsNotNull(), r.Code.Neq(""), r.Status.Eq(1), r.IsDelete.Is(false)).Find()
	if err != nil {
		return nil, err
	}

	codes := make([]string, 0, len(resources))
	for _, res := range resources {
		if res.Code != nil && *res.Code != "" {
			codes = append(codes, *res.Code)
		}
	}
	return codes, nil
}
