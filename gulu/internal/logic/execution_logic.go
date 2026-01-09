package logic

import (
	"context"
	"errors"
	"fmt"
	"time"

	"yqhp/gulu/internal/model"
	"yqhp/gulu/internal/query"
	"yqhp/gulu/internal/svc"
	"yqhp/gulu/internal/workflow"
)

// ExecutionLogic 执行逻辑
type ExecutionLogic struct {
	ctx context.Context
}

// NewExecutionLogic 创建执行逻辑
func NewExecutionLogic(ctx context.Context) *ExecutionLogic {
	return &ExecutionLogic{ctx: ctx}
}

// ExecuteWorkflowReq 执行工作流请求
type ExecuteWorkflowReq struct {
	WorkflowID int64 `json:"workflow_id" validate:"required"`
	EnvID      int64 `json:"env_id" validate:"required"`
	ExecutorID int64 `json:"executor_id"` // 可选，指定执行机ID
}

// ExecutionListReq 执行记录列表请求
type ExecutionListReq struct {
	Page       int    `query:"page" validate:"min=1"`
	PageSize   int    `query:"pageSize" validate:"min=1,max=100"`
	ProjectID  int64  `query:"projectId"`
	WorkflowID int64  `query:"workflowId"`
	EnvID      int64  `query:"envId"`
	Status     string `query:"status"`
}

// ExecutionStatus 执行状态常量
const (
	ExecutionStatusPending   = "pending"
	ExecutionStatusRunning   = "running"
	ExecutionStatusCompleted = "completed"
	ExecutionStatusFailed    = "failed"
	ExecutionStatusStopped   = "stopped"
	ExecutionStatusPaused    = "paused"
)

// Execute 执行工作流
func (l *ExecutionLogic) Execute(req *ExecuteWorkflowReq, userID int64) (*model.TExecution, error) {
	// 获取工作流
	workflowLogic := NewWorkflowLogic(l.ctx)
	wf, err := workflowLogic.GetByID(req.WorkflowID)
	if err != nil {
		return nil, errors.New("工作流不存在")
	}

	// 解析工作流定义
	def, err := workflow.ParseJSON(wf.Definition)
	if err != nil {
		return nil, errors.New("工作流定义解析失败: " + err.Error())
	}

	// 执行前验证（要求至少一个步骤）
	validationResult := workflow.ValidateForExecution(def)
	if !validationResult.Valid {
		if len(validationResult.Errors) > 0 {
			return nil, errors.New(validationResult.Errors[0].Message)
		}
		return nil, errors.New("工作流定义验证失败")
	}

	// 获取环境配置
	envLogic := NewEnvLogic(l.ctx)
	env, err := envLogic.GetByID(req.EnvID)
	if err != nil {
		return nil, errors.New("环境不存在")
	}

	// 验证环境属于同一项目
	if env.ProjectID != wf.ProjectID {
		return nil, errors.New("环境与工作流不属于同一项目")
	}

	// 获取环境下的配置
	domainLogic := NewDomainLogic(l.ctx)
	domainResps, err := domainLogic.GetDomainsByEnvID(req.EnvID)
	if err != nil {
		return nil, err
	}
	// 转换为 model.TDomain
	domains := make([]*model.TDomain, len(domainResps))
	for i, dr := range domainResps {
		domains[i] = &model.TDomain{
			Code:    dr.Code,
			BaseURL: dr.BaseURL,
			Headers: dr.Headers,
		}
	}

	varLogic := NewVarLogic(l.ctx)
	vars, err := varLogic.GetVarsByEnvID(req.EnvID)
	if err != nil {
		return nil, err
	}

	dbConfigLogic := NewDatabaseConfigLogic(l.ctx)
	dbConfigs, err := dbConfigLogic.GetConfigsByEnvID(req.EnvID)
	if err != nil {
		return nil, err
	}

	mqConfigLogic := NewMQConfigLogic(l.ctx)
	mqConfigs, err := mqConfigLogic.GetConfigsByEnvID(req.EnvID)
	if err != nil {
		return nil, err
	}

	// 合并环境配置到工作流
	merger := workflow.NewConfigMerger().
		SetDomains(domains).
		SetVariables(vars).
		SetDatabases(dbConfigs).
		SetMQs(mqConfigs)

	_, err = merger.MergeToWorkflow(def)
	if err != nil {
		return nil, errors.New("配置合并失败: " + err.Error())
	}

	// 创建执行记录
	now := time.Now()
	executionID := generateExecutionID()

	// 处理 ExecutorID 转换
	var executorIDStr *string
	if req.ExecutorID > 0 {
		str := fmt.Sprintf("%d", req.ExecutorID)
		executorIDStr = &str
	}

	execution := &model.TExecution{
		CreatedAt:   &now,
		UpdatedAt:   &now,
		ProjectID:   wf.ProjectID,
		WorkflowID:  req.WorkflowID,
		EnvID:       req.EnvID,
		ExecutorID:  executorIDStr,
		ExecutionID: executionID,
		Status:      ExecutionStatusPending,
		StartTime:   &now,
		CreatedBy:   &userID,
	}

	q := query.Use(svc.Ctx.DB)
	err = q.TExecution.WithContext(l.ctx).Create(execution)
	if err != nil {
		return nil, err
	}

	// TODO: 调用 workflow-engine 提交执行
	// 这里需要实现与 workflow-engine 的集成
	// weClient := client.NewWorkflowEngineClient()
	// weClient.SubmitExecution(...)

	// 更新状态为运行中
	_, err = q.TExecution.WithContext(l.ctx).Where(q.TExecution.ID.Eq(execution.ID)).Update(q.TExecution.Status, ExecutionStatusRunning)
	if err != nil {
		return nil, err
	}
	execution.Status = ExecutionStatusRunning

	return execution, nil
}

// GetByID 根据ID获取执行记录
func (l *ExecutionLogic) GetByID(id int64) (*model.TExecution, error) {
	q := query.Use(svc.Ctx.DB)
	e := q.TExecution

	return e.WithContext(l.ctx).Where(e.ID.Eq(id)).First()
}

// GetByExecutionID 根据执行ID获取执行记录
func (l *ExecutionLogic) GetByExecutionID(executionID string) (*model.TExecution, error) {
	q := query.Use(svc.Ctx.DB)
	e := q.TExecution

	return e.WithContext(l.ctx).Where(e.ExecutionID.Eq(executionID)).First()
}

// List 获取执行记录列表
func (l *ExecutionLogic) List(req *ExecutionListReq) ([]*model.TExecution, int64, error) {
	q := query.Use(svc.Ctx.DB)
	e := q.TExecution

	// 构建查询条件
	queryBuilder := e.WithContext(l.ctx)

	if req.ProjectID > 0 {
		queryBuilder = queryBuilder.Where(e.ProjectID.Eq(req.ProjectID))
	}
	if req.WorkflowID > 0 {
		queryBuilder = queryBuilder.Where(e.WorkflowID.Eq(req.WorkflowID))
	}
	if req.EnvID > 0 {
		queryBuilder = queryBuilder.Where(e.EnvID.Eq(req.EnvID))
	}
	if req.Status != "" {
		queryBuilder = queryBuilder.Where(e.Status.Eq(req.Status))
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
	list, err := queryBuilder.Order(e.ID.Desc()).Offset(offset).Limit(req.PageSize).Find()
	if err != nil {
		return nil, 0, err
	}

	return list, total, nil
}

// GetLogs 获取执行日志
func (l *ExecutionLogic) GetLogs(id int64) (string, error) {
	execution, err := l.GetByID(id)
	if err != nil {
		return "", errors.New("执行记录不存在")
	}

	// TODO: 从 workflow-engine 获取实时日志
	// weClient := client.NewWorkflowEngineClient()
	// logs, err := weClient.GetExecutionLogs(execution.ExecutionID)

	if execution.Logs != nil {
		return *execution.Logs, nil
	}
	return "", nil
}

// Stop 停止执行
func (l *ExecutionLogic) Stop(id int64) error {
	execution, err := l.GetByID(id)
	if err != nil {
		return errors.New("执行记录不存在")
	}

	if execution.Status != ExecutionStatusRunning && execution.Status != ExecutionStatusPaused {
		return errors.New("只能停止运行中或暂停的执行")
	}

	// TODO: 调用 workflow-engine 停止执行
	// weClient := client.NewWorkflowEngineClient()
	// weClient.StopExecution(execution.ExecutionID)

	now := time.Now()
	q := query.Use(svc.Ctx.DB)
	e := q.TExecution

	var duration *int64
	if execution.StartTime != nil {
		d := now.Sub(*execution.StartTime).Milliseconds()
		duration = &d
	}

	_, err = e.WithContext(l.ctx).Where(e.ID.Eq(id)).Updates(map[string]interface{}{
		"status":     ExecutionStatusStopped,
		"end_time":   now,
		"duration":   duration,
		"updated_at": now,
	})
	return err
}

// Pause 暂停执行
func (l *ExecutionLogic) Pause(id int64) error {
	execution, err := l.GetByID(id)
	if err != nil {
		return errors.New("执行记录不存在")
	}

	if execution.Status != ExecutionStatusRunning {
		return errors.New("只能暂停运行中的执行")
	}

	// TODO: 调用 workflow-engine 暂停执行
	// weClient := client.NewWorkflowEngineClient()
	// weClient.PauseExecution(execution.ExecutionID)

	now := time.Now()
	q := query.Use(svc.Ctx.DB)
	e := q.TExecution

	_, err = e.WithContext(l.ctx).Where(e.ID.Eq(id)).Updates(map[string]interface{}{
		"status":     ExecutionStatusPaused,
		"updated_at": now,
	})
	return err
}

// Resume 恢复执行
func (l *ExecutionLogic) Resume(id int64) error {
	execution, err := l.GetByID(id)
	if err != nil {
		return errors.New("执行记录不存在")
	}

	if execution.Status != ExecutionStatusPaused {
		return errors.New("只能恢复暂停的执行")
	}

	// TODO: 调用 workflow-engine 恢复执行
	// weClient := client.NewWorkflowEngineClient()
	// weClient.ResumeExecution(execution.ExecutionID)

	now := time.Now()
	q := query.Use(svc.Ctx.DB)
	e := q.TExecution

	_, err = e.WithContext(l.ctx).Where(e.ID.Eq(id)).Updates(map[string]interface{}{
		"status":     ExecutionStatusRunning,
		"updated_at": now,
	})
	return err
}

// UpdateStatus 更新执行状态（内部使用，用于 webhook 回调）
func (l *ExecutionLogic) UpdateStatus(executionID string, status string, result string, logs string) error {
	execution, err := l.GetByExecutionID(executionID)
	if err != nil {
		return errors.New("执行记录不存在")
	}

	now := time.Now()
	q := query.Use(svc.Ctx.DB)
	e := q.TExecution

	updates := map[string]interface{}{
		"status":     status,
		"updated_at": now,
	}

	if result != "" {
		updates["result"] = result
	}
	if logs != "" {
		updates["logs"] = logs
	}

	// 如果是终态，设置结束时间和持续时间
	if status == ExecutionStatusCompleted || status == ExecutionStatusFailed || status == ExecutionStatusStopped {
		updates["end_time"] = now
		if execution.StartTime != nil {
			duration := now.Sub(*execution.StartTime).Milliseconds()
			updates["duration"] = duration
		}
	}

	_, err = e.WithContext(l.ctx).Where(e.ID.Eq(execution.ID)).Updates(updates)
	return err
}

// generateExecutionID 生成执行ID
func generateExecutionID() string {
	return "exec_" + time.Now().Format("20060102150405") + "_" + randomString(8)
}

// randomString 生成随机字符串
func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[time.Now().UnixNano()%int64(len(letters))]
		time.Sleep(time.Nanosecond)
	}
	return string(b)
}
