package logic

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	commonUtils "yqhp/common/utils"
	"yqhp/gulu/internal/model"
	"yqhp/gulu/internal/query"
	"yqhp/gulu/internal/svc"
	"yqhp/gulu/internal/workflow"
	"yqhp/workflow-engine/pkg/types"
)

// engineIDMap 维护 gulu executionID -> engine executionID 的映射（用于实时指标查询）
var engineIDMap sync.Map

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
	WorkflowID        int64              `json:"workflow_id" validate:"required"`
	EnvID             int64              `json:"env_id" validate:"required"`
	ExecutorID        int64              `json:"executor_id"`        // 可选，指定执行机ID
	Mode              string             `json:"mode"`               // 执行模式: debug, execute（默认 execute）
	PerformanceConfig *PerformanceConfig `json:"performance_config"` // 可选，压测配置（覆盖工作流定义中的配置）
}

// PerformanceConfig 压测配置
type PerformanceConfig struct {
	Mode       string            `json:"mode"`                 // constant-vus, ramping-vus 等
	VUs        int               `json:"vus,omitempty"`        // 虚拟用户数
	Duration   string            `json:"duration,omitempty"`   // 持续时间，如 "30s", "5m"
	Iterations int               `json:"iterations,omitempty"` // 迭代次数
	Stages     []PerfStage       `json:"stages,omitempty"`     // 阶梯配置
	Thresholds []PerfThreshold   `json:"thresholds,omitempty"` // 性能阈值
	HTTPEngine string            `json:"httpEngine,omitempty"` // fasthttp 或 standard
	Tags       map[string]string `json:"tags,omitempty"`       // 全局标签
}

// PerfStage 阶梯配置
type PerfStage struct {
	Duration string `json:"duration"`
	Target   int    `json:"target"`
	Name     string `json:"name,omitempty"`
}

// PerfThreshold 性能阈值
type PerfThreshold struct {
	Metric    string `json:"metric"`
	Condition string `json:"condition"`
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

		// 应用压测配置
		perfConfig := req.PerformanceConfig
		if perfConfig == nil {
			perfConfig = extractPerformanceConfigFromDefinition(wf.Definition)
		}
		if perfConfig != nil {
			applyPerformanceConfig(weWorkflow, perfConfig)
			fmt.Printf("[Execute] 已应用压测配置: mode=%s, vus=%d, duration=%v, iterations=%d, stages=%d\n",
				weWorkflow.Options.ExecutionMode, weWorkflow.Options.VUs, weWorkflow.Options.Duration, weWorkflow.Options.Iterations, len(weWorkflow.Options.Stages))
		} else {
			fmt.Printf("[Execute] 未找到压测配置，使用默认值: vus=%d, iterations=%d\n", weWorkflow.Options.VUs, weWorkflow.Options.Iterations)
		}

		// 设置执行机指定（TargetSlaves）
		if req.ExecutorID > 0 {
			executorLogic := NewExecutorLogic(l.ctx)
			execInfo, execErr := executorLogic.GetByID(req.ExecutorID)
			if execErr != nil {
				return nil, fmt.Errorf("指定的执行机不存在: %v", execErr)
			}
			if execInfo.SlaveID != "" {
				weWorkflow.Options.TargetSlaves = &types.SlaveSelector{
					Mode:     types.SelectionModeManual,
					SlaveIDs: []string{execInfo.SlaveID},
				}
			}
		}

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

		// 保存 gulu executionID -> engine executionID 的映射
		engineIDMap.Store(executionID, engineExecutionID)

		// 启动后台协程监控执行状态
		commonUtils.SafeGoWithName("monitor-execution-"+engineExecutionID, func() {
			l.monitorExecution(execution.ID, engineExecutionID, engine)
			// 延迟清除映射，给查询留足时间
			time.AfterFunc(10*time.Minute, func() {
				engineIDMap.Delete(executionID)
			})
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

			// 如果是终态，获取最终指标并存储报告
			if dbStatus == ExecutionStatusCompleted || dbStatus == ExecutionStatusFailed || dbStatus == ExecutionStatusStopped {
				fmt.Printf("执行完成: %s -> %s\n", executionID, dbStatus)

				reportData := map[string]interface{}{
					"status": dbStatus,
				}

				// 收集引擎错误信息
				if len(state.Errors) > 0 {
					errMsgs := make([]string, len(state.Errors))
					for i, e := range state.Errors {
						errMsgs[i] = e.Message
					}
					reportData["errors"] = errMsgs
					fmt.Printf("执行错误: %v\n", errMsgs)
				}

				// 收集最终指标
				metrics, metricsErr := engine.GetMetrics(context.Background(), executionID)
				if metricsErr == nil && metrics != nil {
					reportData["total_vus"] = metrics.TotalVUs
					reportData["total_iterations"] = metrics.TotalIterations
					reportData["duration"] = formatEngineDuration(metrics.Duration)
					reportData["step_metrics"] = metrics.StepMetrics
				}

				var resultStr *string
				if reportJSON, jsonErr := json.Marshal(reportData); jsonErr == nil {
					s := string(reportJSON)
					resultStr = &s
				}

				l.updateExecutionStatus(dbID, dbStatus, state.EndTime, resultStr)
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

// ExecutionMetricsResp 执行指标响应
type ExecutionMetricsResp struct {
	ExecutionID     string                       `json:"execution_id"`
	Status          string                       `json:"status"`
	TotalVUs        int                          `json:"total_vus"`
	TotalIterations int64                        `json:"total_iterations"`
	Duration        string                       `json:"duration"`
	StartTime       *time.Time                   `json:"start_time"`
	EndTime         *time.Time                   `json:"end_time"`
	DurationMs      *int64                       `json:"duration_ms"`
	StepMetrics     map[string]*StepMetricsResp  `json:"step_metrics,omitempty"`
	Errors          []string                     `json:"errors,omitempty"`
}

// StepMetricsResp 步骤指标响应
type StepMetricsResp struct {
	StepID       string            `json:"step_id"`
	Count        int64             `json:"count"`
	SuccessCount int64             `json:"success_count"`
	FailureCount int64             `json:"failure_count"`
	Duration     *DurationMetrics  `json:"duration,omitempty"`
}

// DurationMetrics 耗时指标
type DurationMetrics struct {
	Min string `json:"min"`
	Max string `json:"max"`
	Avg string `json:"avg"`
	P50 string `json:"p50"`
	P90 string `json:"p90"`
	P95 string `json:"p95"`
	P99 string `json:"p99"`
}

// GetMetrics 获取执行的实时指标
func (l *ExecutionLogic) GetMetrics(id int64) (*ExecutionMetricsResp, error) {
	execution, err := l.GetByID(id)
	if err != nil {
		return nil, errors.New("执行记录不存在")
	}

	resp := &ExecutionMetricsResp{
		ExecutionID: execution.ExecutionID,
		Status:      execution.Status,
		StartTime:   execution.StartTime,
		EndTime:     execution.EndTime,
		DurationMs:  execution.Duration,
	}

	// 如果已有 DB 存储的 result，解析其中的错误信息
	if execution.Result != nil && *execution.Result != "" {
		var savedResult map[string]interface{}
		if jsonErr := json.Unmarshal([]byte(*execution.Result), &savedResult); jsonErr == nil {
			if errs, ok := savedResult["errors"].([]interface{}); ok {
				for _, e := range errs {
					if s, ok := e.(string); ok {
						resp.Errors = append(resp.Errors, s)
					}
				}
			}
		}
	}

	engine := workflow.GetEngine()
	if engine == nil {
		return resp, nil
	}

	// 查找引擎真实执行 ID（gulu ID 和 engine ID 不同）
	engineExecID := execution.ExecutionID
	if mapped, ok := engineIDMap.Load(execution.ExecutionID); ok {
		engineExecID = mapped.(string)
	}

	// 从引擎获取实时执行状态（包含错误信息）
	state, _ := engine.GetExecutionStatus(context.Background(), engineExecID)
	if state != nil && len(state.Errors) > 0 {
		resp.Errors = make([]string, len(state.Errors))
		for i, e := range state.Errors {
			resp.Errors[i] = e.Message
		}
	}

	metrics, err := engine.GetMetrics(context.Background(), engineExecID)
	if err != nil || metrics == nil {
		return resp, nil
	}

	resp.TotalVUs = metrics.TotalVUs
	resp.TotalIterations = metrics.TotalIterations
	resp.Duration = formatEngineDuration(metrics.Duration)

	if len(metrics.StepMetrics) > 0 {
		resp.StepMetrics = make(map[string]*StepMetricsResp)
		for id, sm := range metrics.StepMetrics {
			stepResp := &StepMetricsResp{
				StepID:       sm.StepID,
				Count:        sm.Count,
				SuccessCount: sm.SuccessCount,
				FailureCount: sm.FailureCount,
			}
			if sm.Duration != nil {
				stepResp.Duration = &DurationMetrics{
					Min: formatEngineDuration(sm.Duration.Min),
					Max: formatEngineDuration(sm.Duration.Max),
					Avg: formatEngineDuration(sm.Duration.Avg),
					P50: formatEngineDuration(sm.Duration.P50),
					P90: formatEngineDuration(sm.Duration.P90),
					P95: formatEngineDuration(sm.Duration.P95),
					P99: formatEngineDuration(sm.Duration.P99),
				}
			}
			resp.StepMetrics[id] = stepResp
		}
	}

	return resp, nil
}

// formatEngineDuration 格式化引擎返回的 Duration
func formatEngineDuration(d time.Duration) string {
	if d == 0 {
		return "0s"
	}
	if d < time.Millisecond {
		return fmt.Sprintf("%.2fµs", float64(d)/float64(time.Microsecond))
	}
	if d < time.Second {
		return fmt.Sprintf("%.2fms", float64(d)/float64(time.Millisecond))
	}
	return fmt.Sprintf("%.2fs", float64(d)/float64(time.Second))
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

// extractPerformanceConfigFromDefinition 从工作流定义 JSON 中提取压测配置
func extractPerformanceConfigFromDefinition(definition string) *PerformanceConfig {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal([]byte(definition), &raw); err != nil {
		return nil
	}

	perfRaw, ok := raw["performanceConfig"]
	if !ok {
		return nil
	}

	var cfg PerformanceConfig
	if err := json.Unmarshal(perfRaw, &cfg); err != nil {
		return nil
	}

	return &cfg
}

// applyPerformanceConfig 将压测配置应用到引擎工作流的 Options 上
func applyPerformanceConfig(wf *types.Workflow, cfg *PerformanceConfig) {
	if wf == nil || cfg == nil {
		return
	}

	// 设置执行模式
	if cfg.Mode != "" {
		wf.Options.ExecutionMode = types.ExecutionMode(cfg.Mode)
	}

	// 设置 VU 数
	if cfg.VUs > 0 {
		wf.Options.VUs = cfg.VUs
	}

	// 解析并设置持续时间
	if cfg.Duration != "" {
		if d, err := time.ParseDuration(cfg.Duration); err == nil {
			wf.Options.Duration = d
			// 基于时长的执行，iterations 设为 0（无限迭代，靠 duration 控制结束）
			wf.Options.Iterations = 0
		}
	}

	// 显式设置迭代次数（覆盖上面的 duration 默认值）
	if cfg.Iterations > 0 {
		wf.Options.Iterations = cfg.Iterations
	}

	// 设置阶梯配置
	if len(cfg.Stages) > 0 {
		stages := make([]types.Stage, len(cfg.Stages))
		for i, s := range cfg.Stages {
			var d time.Duration
			if s.Duration != "" {
				d, _ = time.ParseDuration(s.Duration)
			}
			stages[i] = types.Stage{
				Duration: d,
				Target:   s.Target,
				Name:     s.Name,
			}
		}
		wf.Options.Stages = stages
	}

	// 设置阈值
	if len(cfg.Thresholds) > 0 {
		thresholds := make([]types.Threshold, len(cfg.Thresholds))
		for i, t := range cfg.Thresholds {
			thresholds[i] = types.Threshold{
				Metric:    t.Metric,
				Condition: t.Condition,
			}
		}
		wf.Options.Thresholds = thresholds
	}

	// 设置 HTTP 引擎类型
	if cfg.HTTPEngine != "" {
		wf.Options.HTTPEngine = types.HTTPEngineType(cfg.HTTPEngine)
	}

	// 设置标签
	if len(cfg.Tags) > 0 {
		wf.Options.Tags = cfg.Tags
	}
}
