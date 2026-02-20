package logic

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	commonUtils "yqhp/common/utils"
	"yqhp/gulu/internal/model"
	"yqhp/gulu/internal/query"
	"yqhp/gulu/internal/svc"
	"yqhp/gulu/internal/workflow"
	"yqhp/workflow-engine/pkg/types"
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
	WorkflowID int64  `json:"workflow_id" validate:"required"`
	EnvID      int64  `json:"env_id" validate:"required"`
	ExecutorID int64  `json:"executor_id"` // 可选，指定执行机ID
	Mode       string `json:"mode"`        // 执行模式: debug, execute（默认 execute）
}

// ExecutionListReq 执行记录列表请求
type ExecutionListReq struct {
	Page       int    `query:"page" validate:"min=1"`
	PageSize   int    `query:"pageSize" validate:"min=1,max=100"`
	ProjectID  int64  `query:"projectId"`
	WorkflowID int64  `query:"workflowId"`
	EnvID      int64  `query:"envId"`
	Status     string `query:"status"`
	Mode       string `query:"mode"` // 执行模式过滤: debug, execute
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

	// 获取工作流类型
	workflowType := string(model.WorkflowTypeNormal)
	if wf.WorkflowType != nil {
		workflowType = *wf.WorkflowType
	}

	// 设置默认执行模式
	mode := req.Mode
	if mode == "" {
		mode = string(model.ExecutionModeExecute)
	}

	// 验证执行模式
	if mode != string(model.ExecutionModeDebug) && mode != string(model.ExecutionModeExecute) {
		return nil, errors.New("无效的执行模式，必须是 debug 或 execute")
	}

	// 普通流程只能调试，不能正式执行
	if mode == string(model.ExecutionModeExecute) && workflowType == string(model.WorkflowTypeNormal) {
		return nil, errors.New("普通流程仅支持调试模式，请使用调试接口")
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

	// 获取环境配置（包含 domains_json 和 vars_json）
	envLogic := NewEnvLogic(l.ctx)
	env, err := envLogic.GetByID(req.EnvID)
	if err != nil {
		return nil, errors.New("环境不存在")
	}

	// 验证环境属于同一项目
	if env.ProjectID != wf.ProjectID {
		return nil, errors.New("环境与工作流不属于同一项目")
	}

	// 合并环境配置到工作流（从 t_config_definition 和 t_config 表读取）
	merger := workflow.NewConfigMerger(l.ctx, req.EnvID)
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
		Mode:        mode,
		Status:      ExecutionStatusPending,
		StartTime:   &now,
		CreatedBy:   &userID,
	}

	q := query.Use(svc.Ctx.DB)
	err = q.TExecution.WithContext(l.ctx).Create(execution)
	if err != nil {
		return nil, err
	}

	// 解析引用工作流
	refResolver := workflow.NewRefWorkflowResolver(NewDBWorkflowLoader(l.ctx))
	if err := refResolver.Resolve(def.Steps); err != nil {
		return nil, fmt.Errorf("解析引用工作流失败: %w", err)
	}

	// 提交工作流到 workflow-engine 执行
	engine := workflow.GetEngine()
	if engine != nil {
		// 转换为 workflow-engine 的工作流类型
		weWorkflow := workflow.ConvertToEngineWorkflow(def, executionID)

		// 提交执行
		engineExecutionID, submitErr := engine.SubmitWorkflow(l.ctx, weWorkflow)
		if submitErr != nil {
			// 提交失败，更新状态为失败
			_, _ = q.TExecution.WithContext(l.ctx).Where(q.TExecution.ID.Eq(execution.ID)).Updates(map[string]interface{}{
				"status":     ExecutionStatusFailed,
				"updated_at": time.Now(),
			})
			return nil, fmt.Errorf("提交工作流执行失败: %v", submitErr)
		}

		fmt.Printf("工作流已提交，引擎执行ID: %s\n", engineExecutionID)

		// 启动后台协程监控执行状态（使用安全的 goroutine，防止 panic 导致服务崩溃）
		commonUtils.SafeGoWithName("monitor-execution-"+engineExecutionID, func() {
			l.monitorExecution(execution.ID, engineExecutionID, engine)
		})
	}

	// 更新状态为运行中
	_, err = q.TExecution.WithContext(l.ctx).Where(q.TExecution.ID.Eq(execution.ID)).Update(q.TExecution.Status, ExecutionStatusRunning)
	if err != nil {
		return nil, err
	}
	execution.Status = ExecutionStatusRunning

	return execution, nil
}

// monitorExecution 监控执行状态并更新数据库
func (l *ExecutionLogic) monitorExecution(dbID int64, executionID string, engine *workflow.Engine) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	timeout := time.After(30 * time.Minute) // 最大监控30分钟

	for {
		select {
		case <-timeout:
			// 超时，标记为失败
			l.updateExecutionStatus(dbID, ExecutionStatusFailed, nil, nil)
			return
		case <-ticker.C:
			// 获取执行状态
			state, err := engine.GetExecutionStatus(context.Background(), executionID)
			if err != nil {
				fmt.Printf("获取执行状态失败: %v\n", err)
				continue
			}
			if state == nil {
				fmt.Printf("执行状态为空: %s\n", executionID)
				continue
			}

			// 根据状态更新数据库
			statusStr := string(state.Status)
			fmt.Printf("执行状态: %s -> %s\n", executionID, statusStr)

			var dbStatus string
			switch statusStr {
			case "pending":
				dbStatus = ExecutionStatusPending
			case "running":
				dbStatus = ExecutionStatusRunning
			case "completed":
				dbStatus = ExecutionStatusCompleted
			case "failed":
				dbStatus = ExecutionStatusFailed
			case "aborted", "stopped":
				dbStatus = ExecutionStatusStopped
			case "paused":
				dbStatus = ExecutionStatusPaused
			default:
				fmt.Printf("未知状态: %s\n", statusStr)
				continue
			}

			// 如果是终态，更新并退出
			if dbStatus == ExecutionStatusCompleted || dbStatus == ExecutionStatusFailed || dbStatus == ExecutionStatusStopped {
				fmt.Printf("执行完成: %s -> %s\n", executionID, dbStatus)
				l.updateExecutionStatus(dbID, dbStatus, state.EndTime, nil)
				return
			}
		}
	}
}

// updateExecutionStatus 更新执行状态
func (l *ExecutionLogic) updateExecutionStatus(id int64, status string, endTime *time.Time, result *string) {
	q := query.Use(svc.Ctx.DB)
	e := q.TExecution

	updates := map[string]interface{}{
		"status":     status,
		"updated_at": time.Now(),
	}

	if endTime != nil {
		updates["end_time"] = *endTime
		// 计算持续时间
		execution, err := e.WithContext(context.Background()).Where(e.ID.Eq(id)).First()
		if err == nil && execution.StartTime != nil {
			duration := endTime.Sub(*execution.StartTime).Milliseconds()
			updates["duration"] = duration
		}
	}

	if result != nil {
		updates["result"] = *result
	}

	_, _ = e.WithContext(context.Background()).Where(e.ID.Eq(id)).Updates(updates)
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
	if req.Mode != "" {
		queryBuilder = queryBuilder.Where(e.Mode.Eq(req.Mode))
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

// ============== 流式执行（SSE）相关方法 ==============

// StreamExecutionDetail 流式执行记录详情
type StreamExecutionDetail struct {
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
	TotalSteps   *int32     `json:"total_steps"`
	SuccessSteps *int32     `json:"success_steps"`
	FailedSteps  *int32     `json:"failed_steps"`
	Result       string     `json:"result,omitempty"`
	CreatedBy    *int64     `json:"created_by"`
	CreatedAt    *time.Time `json:"created_at"`
}

// CreateStreamExecution 创建流式执行记录
func (l *ExecutionLogic) CreateStreamExecution(sessionID string, projectID, workflowID, envID, userID int64, mode string) error {
	now := time.Now()
	execution := &model.TExecution{
		ProjectID:   projectID,
		WorkflowID:  workflowID,
		EnvID:       envID,
		ExecutionID: sessionID,
		Mode:        mode,
		Status:      string(model.ExecutionStatusRunning),
		StartTime:   &now,
		CreatedBy:   &userID,
	}

	return query.TExecution.WithContext(l.ctx).Create(execution)
}

// UpdateStreamExecutionStatus 更新流式执行状态
func (l *ExecutionLogic) UpdateStreamExecutionStatus(executionID, status string, result interface{}) error {
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

// UpdateStreamStepStats 更新流式执行步骤统计
func (l *ExecutionLogic) UpdateStreamStepStats(executionID string, totalSteps, successSteps, failedSteps int) error {
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

// GetStreamExecution 获取流式执行记录
func (l *ExecutionLogic) GetStreamExecution(executionID string) (*StreamExecutionDetail, error) {
	q := query.TExecution
	execution, err := q.WithContext(l.ctx).
		Where(q.ExecutionID.Eq(executionID)).
		First()
	if err != nil {
		return nil, err
	}

	return l.toStreamExecutionDetail(execution), nil
}

// ListStreamExecutions 获取流式执行记录列表
func (l *ExecutionLogic) ListStreamExecutions(workflowID int64, mode string, userID int64) ([]*StreamExecutionDetail, error) {
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

	result := make([]*StreamExecutionDetail, len(executions))
	for i, e := range executions {
		result[i] = l.toStreamExecutionDetail(e)
	}

	return result, nil
}

// toStreamExecutionDetail 转换为流式执行详情
func (l *ExecutionLogic) toStreamExecutionDetail(e *model.TExecution) *StreamExecutionDetail {
	detail := &StreamExecutionDetail{
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

// ============== WorkflowLoader 实现 ==============

// dbWorkflowLoader 基于数据库的工作流加载器，实现 workflow.WorkflowLoader 接口
type dbWorkflowLoader struct {
	ctx context.Context
}

// NewDBWorkflowLoader 创建基于数据库的工作流加载器
func NewDBWorkflowLoader(ctx context.Context) workflow.WorkflowLoader {
	return &dbWorkflowLoader{ctx: ctx}
}

func (l *dbWorkflowLoader) LoadDefinition(id int64) (string, string, error) {
	wfLogic := NewWorkflowLogic(l.ctx)
	wf, err := wfLogic.GetByID(id)
	if err != nil {
		return "", "", err
	}
	return wf.Name, wf.Definition, nil
}

// ============== 工作流转换函数 ==============

// ConvertToEngineWorkflow 将工作流定义转换为引擎工作流
func ConvertToEngineWorkflow(definition string, executionID string) (*types.Workflow, error) {
	return ConvertToEngineWorkflowWithContext(context.Background(), definition, executionID)
}

// ConvertToEngineWorkflowWithContext 将工作流定义转换为引擎工作流（带上下文，支持解析引用工作流）
func ConvertToEngineWorkflowWithContext(ctx context.Context, definition string, executionID string) (*types.Workflow, error) {
	var def workflow.WorkflowDefinition
	if err := json.Unmarshal([]byte(definition), &def); err != nil {
		return nil, err
	}

	// 解析引用工作流：将 ref_workflow 步骤展开为完整定义
	resolver := workflow.NewRefWorkflowResolver(NewDBWorkflowLoader(ctx))
	if err := resolver.Resolve(def.Steps); err != nil {
		return nil, fmt.Errorf("解析引用工作流失败: %w", err)
	}

	return workflow.ConvertToEngineWorkflow(&def, executionID), nil
}

// ConvertToEngineWorkflowStopOnError 将工作流定义转换为"失败即停止"模式的引擎工作流
func ConvertToEngineWorkflowStopOnError(definition string, executionID string) (*types.Workflow, error) {
	return ConvertToEngineWorkflowStopOnErrorWithContext(context.Background(), definition, executionID)
}

// ConvertToEngineWorkflowStopOnErrorWithContext 将工作流定义转换为"失败即停止"模式的引擎工作流（带上下文）
func ConvertToEngineWorkflowStopOnErrorWithContext(ctx context.Context, definition string, executionID string) (*types.Workflow, error) {
	var def workflow.WorkflowDefinition
	if err := json.Unmarshal([]byte(definition), &def); err != nil {
		return nil, err
	}

	// 解析引用工作流
	resolver := workflow.NewRefWorkflowResolver(NewDBWorkflowLoader(ctx))
	if err := resolver.Resolve(def.Steps); err != nil {
		return nil, fmt.Errorf("解析引用工作流失败: %w", err)
	}

	return workflow.ConvertToEngineWorkflowForDebug(&def, executionID), nil
}
