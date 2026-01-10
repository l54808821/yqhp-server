package logic

import (
	"context"
	"encoding/json"
	"time"

	"yqhp/gulu/internal/model"
	"yqhp/gulu/internal/query"
	"yqhp/gulu/internal/workflow"
	"yqhp/workflow-engine/pkg/types"
)

// DebugLogic 调试业务逻辑
type DebugLogic struct {
	ctx context.Context
}

// NewDebugLogic 创建调试业务逻辑
func NewDebugLogic(ctx context.Context) *DebugLogic {
	return &DebugLogic{ctx: ctx}
}

// ExecutionDetail 执行记录详情（统一调试和正式执行）
type ExecutionDetail struct {
	ID           int64      `json:"id"`
	ExecutionID  string     `json:"execution_id"`
	ProjectID    int64      `json:"project_id"`
	WorkflowID   int64      `json:"workflow_id"`
	EnvID        int64      `json:"env_id"`
	Mode         string     `json:"mode"`
	Status       string     `json:"status"`
	StartTime    *time.Time `json:"start_time"`
	EndTime      *time.Time `json:"end_time"`
	Duration     *int64     `json:"duration"`
	TotalSteps   *int       `json:"total_steps"`
	SuccessSteps *int       `json:"success_steps"`
	FailedSteps  *int       `json:"failed_steps"`
	Result       string     `json:"result,omitempty"`
	CreatedBy    *int64     `json:"created_by"`
	CreatedAt    *time.Time `json:"created_at"`
}

// CreateDebugSession 创建调试会话（使用 t_execution 表）
func (l *DebugLogic) CreateDebugSession(sessionID string, projectID, workflowID, envID, userID int64) error {
	now := time.Now()
	execution := &model.TExecution{
		ProjectID:   projectID,
		WorkflowID:  workflowID,
		EnvID:       envID,
		ExecutionID: sessionID,
		Mode:        string(model.ExecutionModeDebug),
		Status:      string(model.ExecutionStatusRunning),
		StartTime:   &now,
		CreatedBy:   &userID,
	}

	return query.TExecution.WithContext(l.ctx).Create(execution)
}

// UpdateExecutionStatus 更新执行状态
func (l *DebugLogic) UpdateExecutionStatus(executionID, status string, result interface{}) error {
	q := query.TExecution
	updates := map[string]interface{}{
		"status": status,
	}

	// 如果是终态，设置结束时间
	if model.ExecutionStatus(status).IsTerminal() {
		now := time.Now()
		updates["end_time"] = now
	}

	// 如果有结果，序列化并保存
	if result != nil {
		resultJSON, err := json.Marshal(result)
		if err == nil {
			resultStr := string(resultJSON)
			updates["result"] = resultStr
		}
	}

	_, err := q.WithContext(l.ctx).
		Where(q.ExecutionID.Eq(executionID)).
		Updates(updates)
	return err
}

// UpdateStepStats 更新步骤统计
func (l *DebugLogic) UpdateStepStats(executionID string, totalSteps, successSteps, failedSteps int) error {
	q := query.TExecution
	_, err := q.WithContext(l.ctx).
		Where(q.ExecutionID.Eq(executionID)).
		Updates(map[string]interface{}{
			"total_steps":   totalSteps,
			"success_steps": successSteps,
			"failed_steps":  failedSteps,
		})
	return err
}

// GetExecution 获取执行记录
func (l *DebugLogic) GetExecution(executionID string) (*ExecutionDetail, error) {
	q := query.TExecution
	execution, err := q.WithContext(l.ctx).
		Where(q.ExecutionID.Eq(executionID)).
		First()
	if err != nil {
		return nil, err
	}

	return l.toExecutionDetail(execution), nil
}

// GetDebugSession 获取调试会话（兼容旧接口）
func (l *DebugLogic) GetDebugSession(sessionID string) (*ExecutionDetail, error) {
	return l.GetExecution(sessionID)
}

// ListExecutions 获取执行记录列表
func (l *DebugLogic) ListExecutions(workflowID int64, mode string, userID int64) ([]*ExecutionDetail, error) {
	q := query.TExecution
	qb := q.WithContext(l.ctx)

	if workflowID > 0 {
		qb = qb.Where(q.WorkflowID.Eq(workflowID))
	}
	if mode != "" {
		qb = qb.Where(q.Mode.Eq(mode))
	}
	if userID > 0 {
		qb = qb.Where(q.CreatedBy.Eq(userID))
	}

	executions, err := qb.Order(q.CreatedAt.Desc()).Find()
	if err != nil {
		return nil, err
	}

	result := make([]*ExecutionDetail, len(executions))
	for i, e := range executions {
		result[i] = l.toExecutionDetail(e)
	}

	return result, nil
}

// ListDebugSessions 获取调试会话列表（兼容旧接口）
func (l *DebugLogic) ListDebugSessions(workflowID, userID int64) ([]*ExecutionDetail, error) {
	return l.ListExecutions(workflowID, string(model.ExecutionModeDebug), userID)
}

// toExecutionDetail 转换为详情
func (l *DebugLogic) toExecutionDetail(e *model.TExecution) *ExecutionDetail {
	detail := &ExecutionDetail{
		ID:           e.ID,
		ExecutionID:  e.ExecutionID,
		ProjectID:    e.ProjectID,
		WorkflowID:   e.WorkflowID,
		EnvID:        e.EnvID,
		Mode:         e.Mode,
		Status:       e.Status,
		StartTime:    e.StartTime,
		EndTime:      e.EndTime,
		Duration:     e.Duration,
		TotalSteps:   e.TotalSteps,
		SuccessSteps: e.SuccessSteps,
		FailedSteps:  e.FailedSteps,
		CreatedBy:    e.CreatedBy,
		CreatedAt:    e.CreatedAt,
	}

	if e.Result != nil {
		detail.Result = *e.Result
	}

	return detail
}

// ConvertToEngineWorkflow 将工作流定义转换为引擎工作流
func ConvertToEngineWorkflow(definition string, executionID string) (*types.Workflow, error) {
	// 解析工作流定义
	var def workflow.WorkflowDefinition
	if err := json.Unmarshal([]byte(definition), &def); err != nil {
		return nil, err
	}

	// 使用 workflow 包的转换函数
	return workflow.ConvertToEngineWorkflow(&def, executionID), nil
}
