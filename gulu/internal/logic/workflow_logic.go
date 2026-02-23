package logic

import (
	"context"
	"errors"
	"time"

	"yqhp/gulu/internal/model"
	"yqhp/gulu/internal/query"
	"yqhp/gulu/internal/svc"
	"yqhp/gulu/internal/workflow"
)

// WorkflowLogic 工作流逻辑
type WorkflowLogic struct {
	ctx context.Context
}

// NewWorkflowLogic 创建工作流逻辑
func NewWorkflowLogic(ctx context.Context) *WorkflowLogic {
	return &WorkflowLogic{ctx: ctx}
}

// CreateWorkflowReq 创建工作流请求
type CreateWorkflowReq struct {
	ProjectID      int64  `json:"project_id" validate:"required"`
	Name           string `json:"name" validate:"required,max=100"`
	Description    string `json:"description" validate:"max=500"`
	Definition     string `json:"definition" validate:"required"` // JSON 格式
	Status         int32  `json:"status"`
	WorkflowType   string `json:"workflow_type"`   // normal, performance, data_generation
	ExecutorConfig string `json:"executor_config"` // JSON: {"strategy":"auto|manual|local","executor_id":null,"labels":{}}
}

// UpdateWorkflowReq 更新工作流请求
type UpdateWorkflowReq struct {
	Name           string `json:"name" validate:"max=100"`
	Description    string `json:"description" validate:"max=500"`
	Definition     string `json:"definition"` // JSON 格式
	Status         int32  `json:"status"`
	WorkflowType   string `json:"workflow_type"`   // normal, performance, data_generation
	ExecutorConfig string `json:"executor_config"` // JSON: {"strategy":"auto|manual|local","executor_id":null,"labels":{}}
}

// WorkflowListReq 工作流列表请求
type WorkflowListReq struct {
	Page      int    `query:"page" validate:"min=1"`
	PageSize  int    `query:"pageSize" validate:"min=1,max=100"`
	ProjectID int64  `query:"projectId"`
	Name      string `query:"name"`
	Status    *int32 `query:"status"`
}

// Create 创建工作流
func (l *WorkflowLogic) Create(req *CreateWorkflowReq, userID int64) (*model.TWorkflow, error) {
	// 验证工作流定义
	if err := l.validateDefinition(req.Definition); err != nil {
		return nil, err
	}

	now := time.Now()
	isDelete := false
	status := req.Status
	if status == 0 {
		status = 1 // 默认启用
	}
	version := int32(1)

	// 设置工作流类型，默认为 normal
	workflowType := req.WorkflowType
	if workflowType == "" {
		workflowType = string(model.WorkflowTypeNormal)
	}
	// 验证工作流类型
	if !model.WorkflowType(workflowType).IsValid() {
		return nil, errors.New("无效的工作流类型")
	}

	var executorConfig *string
	if req.ExecutorConfig != "" {
		executorConfig = &req.ExecutorConfig
	}

	wf := &model.TWorkflow{
		CreatedAt:      &now,
		UpdatedAt:      &now,
		IsDelete:       &isDelete,
		CreatedBy:      &userID,
		UpdatedBy:      &userID,
		ProjectID:      req.ProjectID,
		Name:           req.Name,
		Description:    &req.Description,
		Version:        &version,
		Definition:     req.Definition,
		Status:         &status,
		WorkflowType:   &workflowType,
		ExecutorConfig: executorConfig,
	}

	q := query.Use(svc.Ctx.DB)
	err := q.TWorkflow.WithContext(l.ctx).Create(wf)
	if err != nil {
		return nil, err
	}

	return wf, nil
}

// Update 更新工作流（版本号自动递增）
func (l *WorkflowLogic) Update(id int64, req *UpdateWorkflowReq, userID int64) error {
	q := query.Use(svc.Ctx.DB)
	w := q.TWorkflow

	// 检查工作流是否存在
	wf, err := w.WithContext(l.ctx).Where(w.ID.Eq(id), w.IsDelete.Is(false)).First()
	if err != nil {
		return errors.New("工作流不存在")
	}

	// 如果更新了定义，需要验证
	if req.Definition != "" {
		if err := l.validateDefinition(req.Definition); err != nil {
			return err
		}
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
	if req.Definition != "" {
		updates["definition"] = req.Definition
		// 版本号递增
		newVersion := int32(1)
		if wf.Version != nil {
			newVersion = *wf.Version + 1
		}
		updates["version"] = newVersion
	}
	if req.WorkflowType != "" {
		// 验证工作流类型
		if !model.WorkflowType(req.WorkflowType).IsValid() {
			return errors.New("无效的工作流类型")
		}
		updates["workflow_type"] = req.WorkflowType
	}
	if req.ExecutorConfig != "" {
		updates["executor_config"] = req.ExecutorConfig
	}
	updates["status"] = req.Status

	_, err = w.WithContext(l.ctx).Where(w.ID.Eq(id)).Updates(updates)
	if err != nil {
		return err
	}

	// 如果名称被修改，同步更新关联的分类名称
	if req.Name != "" && req.Name != wf.Name {
		c := q.TCategoryWorkflow
		_, _ = c.WithContext(l.ctx).
			Where(c.SourceID.Eq(id), c.Type.Eq("workflow"), c.IsDelete.Is(false)).
			Update(c.Name, req.Name)
	}

	return nil
}

// Delete 删除工作流（软删除）
func (l *WorkflowLogic) Delete(id int64) error {
	q := query.Use(svc.Ctx.DB)
	w := q.TWorkflow

	isDelete := true
	_, err := w.WithContext(l.ctx).Where(w.ID.Eq(id)).Update(w.IsDelete, isDelete)
	return err
}

// GetByID 根据ID获取工作流
func (l *WorkflowLogic) GetByID(id int64) (*model.TWorkflow, error) {
	q := query.Use(svc.Ctx.DB)
	w := q.TWorkflow

	return w.WithContext(l.ctx).Where(w.ID.Eq(id), w.IsDelete.Is(false)).First()
}

// List 获取工作流列表
func (l *WorkflowLogic) List(req *WorkflowListReq) ([]*model.TWorkflow, int64, error) {
	q := query.Use(svc.Ctx.DB)
	w := q.TWorkflow

	// 构建查询条件
	queryBuilder := w.WithContext(l.ctx).Where(w.IsDelete.Is(false))

	if req.ProjectID > 0 {
		queryBuilder = queryBuilder.Where(w.ProjectID.Eq(req.ProjectID))
	}
	if req.Name != "" {
		queryBuilder = queryBuilder.Where(w.Name.Like("%" + req.Name + "%"))
	}
	if req.Status != nil {
		queryBuilder = queryBuilder.Where(w.Status.Eq(*req.Status))
	}

	// 获取总数
	total, err := queryBuilder.Count()
	if err != nil {
		return nil, 0, err
	}

	// 分页查询
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 10
	}

	offset := (req.Page - 1) * req.PageSize
	list, err := queryBuilder.Order(w.ID.Desc()).Offset(offset).Limit(req.PageSize).Find()
	if err != nil {
		return nil, 0, err
	}

	return list, total, nil
}

// GetByProjectID 根据项目ID获取工作流列表
func (l *WorkflowLogic) GetByProjectID(projectID int64) ([]*model.TWorkflow, error) {
	q := query.Use(svc.Ctx.DB)
	w := q.TWorkflow

	return w.WithContext(l.ctx).Where(w.ProjectID.Eq(projectID), w.IsDelete.Is(false)).Order(w.ID.Desc()).Find()
}

// Copy 复制工作流
func (l *WorkflowLogic) Copy(id int64, newName string, userID int64) (*model.TWorkflow, error) {
	// 获取原工作流
	original, err := l.GetByID(id)
	if err != nil {
		return nil, errors.New("原工作流不存在")
	}

	now := time.Now()
	isDelete := false
	status := int32(1)
	version := int32(1)

	newWf := &model.TWorkflow{
		CreatedAt:   &now,
		UpdatedAt:   &now,
		IsDelete:    &isDelete,
		CreatedBy:   &userID,
		UpdatedBy:   &userID,
		ProjectID:   original.ProjectID,
		Name:        newName,
		Description: original.Description,
		Version:     &version,
		Definition:  original.Definition,
		Status:      &status,
	}

	q := query.Use(svc.Ctx.DB)
	err = q.TWorkflow.WithContext(l.ctx).Create(newWf)
	if err != nil {
		return nil, err
	}

	return newWf, nil
}

// ExportYAML 导出工作流为 YAML
func (l *WorkflowLogic) ExportYAML(id int64) (string, error) {
	wf, err := l.GetByID(id)
	if err != nil {
		return "", errors.New("工作流不存在")
	}

	return workflow.JSONToYAML(wf.Definition)
}

// ImportYAML 从 YAML 导入工作流
func (l *WorkflowLogic) ImportYAML(projectID int64, yamlContent string, userID int64) (*model.TWorkflow, error) {
	// 解析 YAML
	def, err := workflow.ParseYAML(yamlContent)
	if err != nil {
		return nil, err
	}

	// 验证工作流定义
	if err := workflow.ValidateDefinition(def); err != nil {
		return nil, err
	}

	// 转换为 JSON
	jsonContent, err := workflow.ToJSON(def)
	if err != nil {
		return nil, err
	}

	// 使用定义中的名称
	name := def.Name
	if name == "" {
		name = "imported_workflow_" + time.Now().Format("20060102150405")
	}

	req := &CreateWorkflowReq{
		ProjectID:   projectID,
		Name:        name,
		Description: def.Description,
		Definition:  jsonContent,
		Status:      1,
	}

	return l.Create(req, userID)
}

// Validate 验证工作流定义
func (l *WorkflowLogic) Validate(id int64) (*workflow.ValidationResult, error) {
	wf, err := l.GetByID(id)
	if err != nil {
		return nil, errors.New("工作流不存在")
	}

	return workflow.ValidateJSON(wf.Definition)
}

// ValidateDefinition 验证工作流定义字符串
func (l *WorkflowLogic) ValidateDefinition(definition string) (*workflow.ValidationResult, error) {
	return workflow.ValidateJSON(definition)
}

// UpdateStatus 更新工作流状态
func (l *WorkflowLogic) UpdateStatus(id int64, status int32, userID int64) error {
	q := query.Use(svc.Ctx.DB)
	w := q.TWorkflow

	now := time.Now()
	_, err := w.WithContext(l.ctx).Where(w.ID.Eq(id), w.IsDelete.Is(false)).Updates(map[string]interface{}{
		"status":     status,
		"updated_at": now,
		"updated_by": userID,
	})
	return err
}

// validateDefinition 验证工作流定义
func (l *WorkflowLogic) validateDefinition(definition string) error {
	result, err := workflow.ValidateJSON(definition)
	if err != nil {
		return err
	}
	if !result.Valid {
		if len(result.Errors) > 0 {
			return errors.New(result.Errors[0].Message)
		}
		return errors.New("工作流定义验证失败")
	}
	return nil
}
