package logic

import (
	"context"
	"errors"
	"time"

	"yqhp/gulu/internal/model"
	"yqhp/gulu/internal/query"
	"yqhp/gulu/internal/svc"

	"gorm.io/gorm"
)

// EnvLogic 环境逻辑
type EnvLogic struct {
	ctx context.Context
}

// NewEnvLogic 创建环境逻辑
func NewEnvLogic(ctx context.Context) *EnvLogic {
	return &EnvLogic{ctx: ctx}
}

// CreateEnvReq 创建环境请求
type CreateEnvReq struct {
	ProjectID   int64  `json:"project_id" validate:"required"`
	Name        string `json:"name" validate:"required,max=100"`
	Description string `json:"description" validate:"max=500"`
	Sort        int64  `json:"sort"`
	Status      int32  `json:"status"`
}

// UpdateEnvReq 更新环境请求
type UpdateEnvReq struct {
	Name        string `json:"name" validate:"max=100"`
	Description string `json:"description" validate:"max=500"`
	Sort        int64  `json:"sort"`
	Status      int32  `json:"status"`
}

// EnvListReq 环境列表请求
type EnvListReq struct {
	Page      int    `query:"page" validate:"min=1"`
	PageSize  int    `query:"pageSize" validate:"min=1,max=100"`
	ProjectID int64  `query:"project_id"`
	Name      string `query:"name"`
	Status    *int32 `query:"status"`
}

// EnvUpdateSortReq 环境排序请求
type EnvUpdateSortReq struct {
	ID       int64  `json:"id" validate:"required"`        // 被拖动的环境ID
	TargetID int64  `json:"target_id" validate:"required"` // 目标位置的环境ID
	Position string `json:"position" validate:"required"`  // before 或 after
}

// Create 创建环境
func (l *EnvLogic) Create(req *CreateEnvReq, userID int64) (*model.TEnv, error) {
	// 检查项目是否存在
	projectLogic := NewProjectLogic(l.ctx)
	_, err := projectLogic.GetByID(req.ProjectID)
	if err != nil {
		return nil, errors.New("项目不存在")
	}

	now := time.Now()
	isDelete := false
	status := req.Status
	if status == 0 {
		status = 1
	}

	env := &model.TEnv{
		CreatedAt:   &now,
		UpdatedAt:   &now,
		IsDelete:    &isDelete,
		CreatedBy:   &userID,
		UpdatedBy:   &userID,
		ProjectID:   req.ProjectID,
		Name:        req.Name,
		Description: &req.Description,
		Sort:        &req.Sort,
		Status:      &status,
	}

	q := query.Use(svc.Ctx.DB)
	err = q.TEnv.WithContext(l.ctx).Create(env)
	if err != nil {
		return nil, err
	}

	return env, nil
}

// Update 更新环境
func (l *EnvLogic) Update(id int64, req *UpdateEnvReq, userID int64) error {
	q := query.Use(svc.Ctx.DB)
	e := q.TEnv

	env, err := e.WithContext(l.ctx).Where(e.ID.Eq(id), e.IsDelete.Is(false)).First()
	if err != nil {
		return errors.New("环境不存在")
	}

	now := time.Now()
	updates := map[string]interface{}{
		"updated_at": now,
		"updated_by": userID,
	}

	if req.Name != "" {
		updates["name"] = req.Name
	}
	if req.Description != "" {
		updates["description"] = req.Description
	}
	updates["sort"] = req.Sort
	updates["status"] = req.Status

	_, err = e.WithContext(l.ctx).Where(e.ID.Eq(env.ID)).Updates(updates)
	return err
}

// Delete 删除环境（软删除）
func (l *EnvLogic) Delete(id int64) error {
	q := query.Use(svc.Ctx.DB)
	e := q.TEnv

	isDelete := true
	_, err := e.WithContext(l.ctx).Where(e.ID.Eq(id)).Update(e.IsDelete, isDelete)
	return err
}

// GetByID 根据ID获取环境
func (l *EnvLogic) GetByID(id int64) (*model.TEnv, error) {
	q := query.Use(svc.Ctx.DB)
	e := q.TEnv

	return e.WithContext(l.ctx).Where(e.ID.Eq(id), e.IsDelete.Is(false)).First()
}

// List 获取环境列表
func (l *EnvLogic) List(req *EnvListReq) ([]*model.TEnv, int64, error) {
	q := query.Use(svc.Ctx.DB)
	e := q.TEnv

	qry := e.WithContext(l.ctx).Where(e.IsDelete.Is(false))

	if req.ProjectID > 0 {
		qry = qry.Where(e.ProjectID.Eq(req.ProjectID))
	}
	if req.Name != "" {
		qry = qry.Where(e.Name.Like("%" + req.Name + "%"))
	}
	if req.Status != nil {
		qry = qry.Where(e.Status.Eq(*req.Status))
	}

	total, err := qry.Count()
	if err != nil {
		return nil, 0, err
	}

	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 10
	}

	offset := (req.Page - 1) * req.PageSize
	list, err := qry.Order(e.Sort.Desc(), e.ID.Desc()).Offset(offset).Limit(req.PageSize).Find()
	if err != nil {
		return nil, 0, err
	}

	return list, total, nil
}

// GetEnvsByProjectID 获取项目下所有环境
func (l *EnvLogic) GetEnvsByProjectID(projectID int64) ([]*model.TEnv, error) {
	q := query.Use(svc.Ctx.DB)
	e := q.TEnv

	return e.WithContext(l.ctx).Where(
		e.ProjectID.Eq(projectID),
		e.IsDelete.Is(false),
	).Order(e.Sort, e.ID).Find()
}

// UpdateSort 更新环境排序
func (l *EnvLogic) UpdateSort(req *EnvUpdateSortReq) error {
	q := query.Use(svc.Ctx.DB)
	e := q.TEnv

	// 获取被拖动的环境
	draggedEnv, err := e.WithContext(l.ctx).Where(e.ID.Eq(req.ID), e.IsDelete.Is(false)).First()
	if err != nil {
		return errors.New("被拖动的环境不存在")
	}

	// 获取目标位置的环境
	targetEnv, err := e.WithContext(l.ctx).Where(e.ID.Eq(req.TargetID), e.IsDelete.Is(false)).First()
	if err != nil {
		return errors.New("目标环境不存在")
	}

	// 确保是同一个项目
	if draggedEnv.ProjectID != targetEnv.ProjectID {
		return errors.New("只能在同一项目内排序")
	}

	// 获取项目下所有环境，按 sort 排序
	envs, err := e.WithContext(l.ctx).Where(
		e.ProjectID.Eq(draggedEnv.ProjectID),
		e.IsDelete.Is(false),
	).Order(e.Sort, e.ID).Find()
	if err != nil {
		return err
	}

	// 找到拖动项和目标项的索引
	var draggedIdx, targetIdx int
	for i, env := range envs {
		if env.ID == req.ID {
			draggedIdx = i
		}
		if env.ID == req.TargetID {
			targetIdx = i
		}
	}

	// 从列表中移除拖动项
	envs = append(envs[:draggedIdx], envs[draggedIdx+1:]...)

	// 重新计算目标索引（因为移除了一项）
	if draggedIdx < targetIdx {
		targetIdx--
	}

	// 根据 position 插入到正确位置
	var insertIdx int
	if req.Position == "before" {
		insertIdx = targetIdx
	} else {
		insertIdx = targetIdx + 1
	}

	// 插入拖动项到新位置
	envs = append(envs[:insertIdx], append([]*model.TEnv{draggedEnv}, envs[insertIdx:]...)...)

	// 批量更新排序值
	for i, env := range envs {
		sort := int64(i)
		_, err := e.WithContext(l.ctx).Where(e.ID.Eq(env.ID)).Update(e.Sort, sort)
		if err != nil {
			return err
		}
	}

	return nil
}

// CopyEnv 复制环境（包含所有配置）
func (l *EnvLogic) CopyEnv(sourceEnvID int64, newName string, userID int64) (*model.TEnv, error) {
	// 获取源环境
	sourceEnv, err := l.GetByID(sourceEnvID)
	if err != nil {
		return nil, errors.New("源环境不存在")
	}

	// 开启事务
	var newEnv *model.TEnv
	err = svc.Ctx.DB.Transaction(func(tx *gorm.DB) error {
		q := query.Use(tx)

		// 1. 创建新环境
		now := time.Now()
		isDelete := false
		status := int32(1)
		newEnv = &model.TEnv{
			CreatedAt:   &now,
			UpdatedAt:   &now,
			IsDelete:    &isDelete,
			CreatedBy:   &userID,
			UpdatedBy:   &userID,
			ProjectID:   sourceEnv.ProjectID,
			Name:        newName,
			Description: sourceEnv.Description,
			Sort:        sourceEnv.Sort,
			Status:      &status,
		}
		if err := q.TEnv.WithContext(l.ctx).Create(newEnv); err != nil {
			return err
		}

		// 2. 复制源环境的配置值到新环境
		sourceConfigs, err := q.TConfig.WithContext(l.ctx).
			Where(q.TConfig.EnvID.Eq(sourceEnvID)).
			Find()
		if err != nil {
			return err
		}

		if len(sourceConfigs) > 0 {
			newConfigs := make([]*model.TConfig, len(sourceConfigs))
			for i, cfg := range sourceConfigs {
				newConfigs[i] = &model.TConfig{
					ProjectID: cfg.ProjectID,
					EnvID:     newEnv.ID,
					Type:      cfg.Type,
					Code:      cfg.Code,
					Value:     cfg.Value,
				}
			}
			if err := q.TConfig.WithContext(l.ctx).CreateInBatches(newConfigs, 100); err != nil {
				return err
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return newEnv, nil
}
