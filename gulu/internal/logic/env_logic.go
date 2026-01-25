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
	Code        string `json:"code" validate:"required,max=50"`
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
	Code      string `query:"code"`
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

	// 检查环境代码在项目内是否唯一
	exists, err := l.CheckCodeExists(req.ProjectID, req.Code, 0)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, errors.New("环境代码在该项目中已存在")
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
		Code:        req.Code,
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
	if req.Code != "" {
		qry = qry.Where(e.Code.Like("%" + req.Code + "%"))
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

// CheckCodeExists 检查环境代码在项目内是否存在
func (l *EnvLogic) CheckCodeExists(projectID int64, code string, excludeID int64) (bool, error) {
	q := query.Use(svc.Ctx.DB)
	e := q.TEnv

	qry := e.WithContext(l.ctx).Where(e.ProjectID.Eq(projectID), e.Code.Eq(code), e.IsDelete.Is(false))
	if excludeID > 0 {
		qry = qry.Where(e.ID.Neq(excludeID))
	}

	count, err := qry.Count()
	if err != nil {
		return false, err
	}

	return count > 0, nil
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
func (l *EnvLogic) CopyEnv(sourceEnvID int64, newName, newCode string, userID int64) (*model.TEnv, error) {
	// 获取源环境
	sourceEnv, err := l.GetByID(sourceEnvID)
	if err != nil {
		return nil, errors.New("源环境不存在")
	}

	// 检查新代码是否已存在
	exists, err := l.CheckCodeExists(sourceEnv.ProjectID, newCode, 0)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, errors.New("环境代码在该项目中已存在")
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
			Code:        newCode,
			Description: sourceEnv.Description,
			Sort:        sourceEnv.Sort,
			Status:      &status,
		}
		if err := q.TEnv.WithContext(l.ctx).Create(newEnv); err != nil {
			return err
		}

		// 2. 复制域名配置
		domains, err := q.TDomain.WithContext(l.ctx).Where(
			q.TDomain.EnvID.Eq(sourceEnvID),
			q.TDomain.IsDelete.Is(false),
		).Find()
		if err != nil {
			return err
		}
		for _, d := range domains {
			newDomain := &model.TDomain{
				CreatedAt:   &now,
				UpdatedAt:   &now,
				IsDelete:    &isDelete,
				ProjectID:   d.ProjectID,
				EnvID:       newEnv.ID,
				Name:        d.Name,
				Code:        d.Code,
				BaseURL:     d.BaseURL,
				Headers:     d.Headers,
				Description: d.Description,
				Sort:        d.Sort,
				Status:      d.Status,
			}
			if err := q.TDomain.WithContext(l.ctx).Create(newDomain); err != nil {
				return err
			}
		}

		// 3. 复制变量配置
		vars, err := q.TVar.WithContext(l.ctx).Where(
			q.TVar.EnvID.Eq(sourceEnvID),
			q.TVar.IsDelete.Is(false),
		).Find()
		if err != nil {
			return err
		}
		for _, v := range vars {
			newVar := &model.TVar{
				CreatedAt:   &now,
				UpdatedAt:   &now,
				IsDelete:    &isDelete,
				ProjectID:   v.ProjectID,
				EnvID:       newEnv.ID,
				Name:        v.Name,
				Key:         v.Key,
				Value:       v.Value,
				Type:        v.Type,
				IsSensitive: v.IsSensitive,
				Description: v.Description,
			}
			if err := q.TVar.WithContext(l.ctx).Create(newVar); err != nil {
				return err
			}
		}

		// 4. 复制数据库配置
		dbConfigs, err := q.TDatabaseConfig.WithContext(l.ctx).Where(
			q.TDatabaseConfig.EnvID.Eq(sourceEnvID),
			q.TDatabaseConfig.IsDelete.Is(false),
		).Find()
		if err != nil {
			return err
		}
		for _, db := range dbConfigs {
			newDB := &model.TDatabaseConfig{
				CreatedAt:   &now,
				UpdatedAt:   &now,
				IsDelete:    &isDelete,
				ProjectID:   db.ProjectID,
				EnvID:       newEnv.ID,
				Name:        db.Name,
				Code:        db.Code,
				Type:        db.Type,
				Host:        db.Host,
				Port:        db.Port,
				Database:    db.Database,
				Username:    db.Username,
				Password:    db.Password,
				Options:     db.Options,
				Description: db.Description,
				Status:      db.Status,
			}
			if err := q.TDatabaseConfig.WithContext(l.ctx).Create(newDB); err != nil {
				return err
			}
		}

		// 5. 复制MQ配置
		mqConfigs, err := q.TMqConfig.WithContext(l.ctx).Where(
			q.TMqConfig.EnvID.Eq(sourceEnvID),
			q.TMqConfig.IsDelete.Is(false),
		).Find()
		if err != nil {
			return err
		}
		for _, mq := range mqConfigs {
			newMQ := &model.TMqConfig{
				CreatedAt:   &now,
				UpdatedAt:   &now,
				IsDelete:    &isDelete,
				ProjectID:   mq.ProjectID,
				EnvID:       newEnv.ID,
				Name:        mq.Name,
				Code:        mq.Code,
				Type:        mq.Type,
				Host:        mq.Host,
				Port:        mq.Port,
				Username:    mq.Username,
				Password:    mq.Password,
				Vhost:       mq.Vhost,
				Options:     mq.Options,
				Description: mq.Description,
				Status:      mq.Status,
			}
			if err := q.TMqConfig.WithContext(l.ctx).Create(newMQ); err != nil {
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
