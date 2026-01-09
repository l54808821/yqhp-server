package logic

import (
	"context"
	"errors"
	"time"

	"yqhp/gulu/internal/model"
	"yqhp/gulu/internal/query"
	"yqhp/gulu/internal/svc"
)

// CategoryWorkflowLogic 工作流分类逻辑
type CategoryWorkflowLogic struct {
	ctx context.Context
}

// NewCategoryWorkflowLogic 创建工作流分类逻辑
func NewCategoryWorkflowLogic(ctx context.Context) *CategoryWorkflowLogic {
	return &CategoryWorkflowLogic{ctx: ctx}
}

// 分类类型常量
const (
	CategoryTypeFolder   = "folder"
	CategoryTypeWorkflow = "workflow"
)

// CreateCategoryReq 创建分类请求
type CreateCategoryReq struct {
	ProjectID int64  `json:"project_id" validate:"required"`
	ParentID  int64  `json:"parent_id"`
	Name      string `json:"name" validate:"required,max=100"`
	Type      string `json:"type" validate:"required,oneof=folder workflow"`
	SourceID  *int64 `json:"source_id"` // 工作流ID，type=workflow时必填
}

// UpdateCategoryReq 更新分类请求
type UpdateCategoryReq struct {
	Name string `json:"name" validate:"max=100"`
}

// MoveCategoryReq 移动分类请求
type MoveCategoryReq struct {
	TargetID int64  `json:"target_id" validate:"required"`                          // 目标节点ID
	Position string `json:"position" validate:"required,oneof=before after inside"` // before|after|inside
}

// CategoryTreeNode 分类树节点
type CategoryTreeNode struct {
	ID        int64               `json:"id"`
	ProjectID int64               `json:"project_id"`
	ParentID  int64               `json:"parent_id"`
	Name      string              `json:"name"`
	Type      string              `json:"type"`
	SourceID  *int64              `json:"source_id,omitempty"`
	Sort      int32               `json:"sort"`
	Children  []*CategoryTreeNode `json:"children,omitempty"`
}

// Create 创建分类
func (l *CategoryWorkflowLogic) Create(req *CreateCategoryReq) (*model.TCategoryWorkflow, error) {
	// 如果是工作流类型，必须有 source_id
	if req.Type == CategoryTypeWorkflow && (req.SourceID == nil || *req.SourceID == 0) {
		return nil, errors.New("工作流类型必须指定关联的工作流ID")
	}

	now := time.Now()
	isDelete := false
	parentID := req.ParentID
	sort := int32(0)

	// 获取同级最大排序值
	q := query.Use(svc.Ctx.DB)
	cw := q.TCategoryWorkflow
	maxSort, _ := cw.WithContext(l.ctx).Where(
		cw.ProjectID.Eq(req.ProjectID),
		cw.ParentID.Eq(parentID),
		cw.IsDelete.Is(false),
	).Order(cw.Sort.Desc()).First()
	if maxSort != nil && maxSort.Sort != nil {
		sort = *maxSort.Sort + 1
	}

	category := &model.TCategoryWorkflow{
		CreatedAt: &now,
		UpdatedAt: &now,
		IsDelete:  &isDelete,
		ProjectID: req.ProjectID,
		ParentID:  &parentID,
		Name:      req.Name,
		Type:      req.Type,
		SourceID:  req.SourceID,
		Sort:      &sort,
	}

	err := cw.WithContext(l.ctx).Create(category)
	if err != nil {
		return nil, err
	}

	return category, nil
}

// Update 更新分类
func (l *CategoryWorkflowLogic) Update(id int64, req *UpdateCategoryReq) error {
	q := query.Use(svc.Ctx.DB)
	cw := q.TCategoryWorkflow

	_, err := cw.WithContext(l.ctx).Where(cw.ID.Eq(id), cw.IsDelete.Is(false)).First()
	if err != nil {
		return errors.New("分类不存在")
	}

	now := time.Now()
	updates := map[string]interface{}{
		"updated_at": now,
	}

	if req.Name != "" {
		updates["name"] = req.Name
	}

	_, err = cw.WithContext(l.ctx).Where(cw.ID.Eq(id)).Updates(updates)
	return err
}

// Delete 删除分类（软删除）
func (l *CategoryWorkflowLogic) Delete(id int64) error {
	q := query.Use(svc.Ctx.DB)
	cw := q.TCategoryWorkflow

	// 检查是否有子分类
	category, err := cw.WithContext(l.ctx).Where(cw.ID.Eq(id), cw.IsDelete.Is(false)).First()
	if err != nil {
		return errors.New("分类不存在")
	}

	// 如果是文件夹，检查是否有子节点
	if category.Type == CategoryTypeFolder {
		childCount, err := cw.WithContext(l.ctx).Where(cw.ParentID.Eq(id), cw.IsDelete.Is(false)).Count()
		if err != nil {
			return err
		}
		if childCount > 0 {
			return errors.New("不能删除非空文件夹")
		}
	}

	isDelete := true
	_, err = cw.WithContext(l.ctx).Where(cw.ID.Eq(id)).Update(cw.IsDelete, isDelete)
	return err
}

// GetByID 根据ID获取分类
func (l *CategoryWorkflowLogic) GetByID(id int64) (*model.TCategoryWorkflow, error) {
	q := query.Use(svc.Ctx.DB)
	cw := q.TCategoryWorkflow

	return cw.WithContext(l.ctx).Where(cw.ID.Eq(id), cw.IsDelete.Is(false)).First()
}

// GetTree 获取项目的分类树
func (l *CategoryWorkflowLogic) GetTree(projectID int64) ([]*CategoryTreeNode, error) {
	q := query.Use(svc.Ctx.DB)
	cw := q.TCategoryWorkflow

	// 获取所有分类
	categories, err := cw.WithContext(l.ctx).Where(
		cw.ProjectID.Eq(projectID),
		cw.IsDelete.Is(false),
	).Order(cw.Sort.Asc(), cw.ID.Asc()).Find()
	if err != nil {
		return nil, err
	}

	// 构建树
	return l.buildTree(categories, 0), nil
}

// buildTree 构建树结构
func (l *CategoryWorkflowLogic) buildTree(categories []*model.TCategoryWorkflow, parentID int64) []*CategoryTreeNode {
	var nodes []*CategoryTreeNode

	for _, c := range categories {
		pid := int64(0)
		if c.ParentID != nil {
			pid = *c.ParentID
		}
		if pid == parentID {
			sort := int32(0)
			if c.Sort != nil {
				sort = *c.Sort
			}
			node := &CategoryTreeNode{
				ID:        c.ID,
				ProjectID: c.ProjectID,
				ParentID:  pid,
				Name:      c.Name,
				Type:      c.Type,
				SourceID:  c.SourceID,
				Sort:      sort,
				Children:  l.buildTree(categories, c.ID),
			}
			nodes = append(nodes, node)
		}
	}

	return nodes
}

// Move 移动分类
func (l *CategoryWorkflowLogic) Move(id int64, req *MoveCategoryReq) error {
	q := query.Use(svc.Ctx.DB)
	cw := q.TCategoryWorkflow

	// 获取被拖拽的节点
	dragNode, err := cw.WithContext(l.ctx).Where(cw.ID.Eq(id), cw.IsDelete.Is(false)).First()
	if err != nil {
		return errors.New("分类不存在")
	}

	// 获取目标节点
	targetNode, err := cw.WithContext(l.ctx).Where(cw.ID.Eq(req.TargetID), cw.IsDelete.Is(false)).First()
	if err != nil {
		return errors.New("目标分类不存在")
	}

	var newParentID int64
	var newSort int32
	now := time.Now()

	switch req.Position {
	case "inside":
		// 拖入目标节点内部（目标必须是文件夹）
		if targetNode.Type != CategoryTypeFolder {
			return errors.New("只能拖入文件夹")
		}
		// 不能移动到自己的子节点下
		if l.isDescendant(id, req.TargetID) {
			return errors.New("不能移动到自己的子节点下")
		}
		newParentID = req.TargetID
		// 获取目标文件夹内最大排序值
		maxSort, _ := cw.WithContext(l.ctx).Where(
			cw.ParentID.Eq(newParentID),
			cw.IsDelete.Is(false),
		).Order(cw.Sort.Desc()).First()
		if maxSort != nil && maxSort.Sort != nil {
			newSort = *maxSort.Sort + 1
		} else {
			newSort = 0
		}

	case "before", "after":
		// 拖到目标节点前面或后面
		targetParentID := int64(0)
		if targetNode.ParentID != nil {
			targetParentID = *targetNode.ParentID
		}
		newParentID = targetParentID

		// 不能移动到自己的子节点下
		if newParentID != 0 && l.isDescendant(id, newParentID) {
			return errors.New("不能移动到自己的子节点下")
		}

		targetSort := int32(0)
		if targetNode.Sort != nil {
			targetSort = *targetNode.Sort
		}

		// 获取同级所有节点
		siblings, err := cw.WithContext(l.ctx).Where(
			cw.ProjectID.Eq(dragNode.ProjectID),
			cw.ParentID.Eq(newParentID),
			cw.IsDelete.Is(false),
			cw.ID.Neq(id), // 排除自己
		).Order(cw.Sort.Asc()).Find()
		if err != nil {
			return err
		}

		// 重新计算排序
		if req.Position == "before" {
			newSort = targetSort
		} else {
			newSort = targetSort + 1
		}

		// 更新其他节点的排序
		for _, sibling := range siblings {
			siblingSort := int32(0)
			if sibling.Sort != nil {
				siblingSort = *sibling.Sort
			}
			if siblingSort >= newSort {
				_, err := cw.WithContext(l.ctx).Where(cw.ID.Eq(sibling.ID)).Updates(map[string]interface{}{
					"sort":       siblingSort + 1,
					"updated_at": now,
				})
				if err != nil {
					return err
				}
			}
		}
	}

	// 更新被拖拽节点
	_, err = cw.WithContext(l.ctx).Where(cw.ID.Eq(id)).Updates(map[string]interface{}{
		"parent_id":  newParentID,
		"sort":       newSort,
		"updated_at": now,
	})
	return err
}

// isDescendant 检查 targetID 是否是 parentID 的后代
func (l *CategoryWorkflowLogic) isDescendant(parentID, targetID int64) bool {
	q := query.Use(svc.Ctx.DB)
	cw := q.TCategoryWorkflow

	target, err := cw.WithContext(l.ctx).Where(cw.ID.Eq(targetID), cw.IsDelete.Is(false)).First()
	if err != nil {
		return false
	}

	if target.ParentID == nil || *target.ParentID == 0 {
		return false
	}

	if *target.ParentID == parentID {
		return true
	}

	return l.isDescendant(parentID, *target.ParentID)
}

// Search 搜索工作流
func (l *CategoryWorkflowLogic) Search(projectID int64, keyword string) ([]*model.TCategoryWorkflow, error) {
	q := query.Use(svc.Ctx.DB)
	cw := q.TCategoryWorkflow

	return cw.WithContext(l.ctx).Where(
		cw.ProjectID.Eq(projectID),
		cw.IsDelete.Is(false),
		cw.Type.Eq(CategoryTypeWorkflow),
		cw.Name.Like("%"+keyword+"%"),
	).Order(cw.Sort.Asc()).Find()
}

// GetBySourceID 根据工作流ID获取分类
func (l *CategoryWorkflowLogic) GetBySourceID(sourceID int64) (*model.TCategoryWorkflow, error) {
	q := query.Use(svc.Ctx.DB)
	cw := q.TCategoryWorkflow

	return cw.WithContext(l.ctx).Where(cw.SourceID.Eq(sourceID), cw.IsDelete.Is(false)).First()
}

// UpdateSort 批量更新排序
func (l *CategoryWorkflowLogic) UpdateSort(items []struct {
	ID   int64 `json:"id"`
	Sort int32 `json:"sort"`
}) error {
	q := query.Use(svc.Ctx.DB)
	cw := q.TCategoryWorkflow

	now := time.Now()
	for _, item := range items {
		_, err := cw.WithContext(l.ctx).Where(cw.ID.Eq(item.ID)).Updates(map[string]interface{}{
			"sort":       item.Sort,
			"updated_at": now,
		})
		if err != nil {
			return err
		}
	}

	return nil
}
