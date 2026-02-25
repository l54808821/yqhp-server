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
	Mode         string            `json:"mode"`                   // constant-vus, ramping-vus 等
	VUs          int               `json:"vus,omitempty"`          // 虚拟用户数
	Duration     string            `json:"duration,omitempty"`     // 持续时间，如 "30s", "5m"
	Iterations   int               `json:"iterations,omitempty"`   // 迭代次数
	Stages       []PerfStage       `json:"stages,omitempty"`       // 阶梯配置
	Thresholds   []PerfThreshold   `json:"thresholds,omitempty"`   // 性能阈值
	HTTPEngine   string            `json:"httpEngine,omitempty"`   // fasthttp 或 standard
	SamplingMode string            `json:"samplingMode,omitempty"` // 采样模式: none, errors, smart
	Tags         map[string]string `json:"tags,omitempty"`         // 全局标签
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
	mergedDef, err := merger.MergeToWorkflow(def)
	if err != nil {
		return nil, errors.New("配置合并失败: " + err.Error())
	}
	def = mergedDef

	// 解析步骤中的环境配置引用（域名、数据库等）
	// 调试流程在 stream_execution.go 中做了这一步，压测流程也必须做
	mergedConfig, _ := merger.Merge()
	if mergedConfig != nil {
		workflow.ResolveEnvConfigReferences(def.Steps, mergedConfig)
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

// monitorExecution monitors execution status and stores the final report.
// Now simplified: the engine handles all metrics/report generation internally.
// Gulu just monitors status and saves the final report to DB.
func (l *ExecutionLogic) monitorExecution(dbID int64, executionID string, engine *workflow.Engine) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	timeout := time.After(30 * time.Minute)

	for {
		select {
		case <-timeout:
			l.updateExecutionStatus(dbID, ExecutionStatusFailed, nil, nil)
			return
		case <-ticker.C:
			state, err := engine.GetExecutionStatus(context.Background(), executionID)
			if err != nil || state == nil {
				continue
			}

			dbStatus := mapEngineStatus(string(state.Status))
			if dbStatus == "" {
				continue
			}

			// Terminal state: fetch the final report from the engine and save to DB
			if isTerminalStatus(dbStatus) {
				// Try to get the full performance report from engine
				report, reportErr := engine.GetPerformanceReport(context.Background(), executionID)

				var resultStr *string
				if reportErr == nil && report != nil {
					if reportJSON, jsonErr := json.Marshal(report); jsonErr == nil {
						s := string(reportJSON)
						resultStr = &s
					}
				} else {
					// Fallback: save basic status info
					fallback := map[string]interface{}{"status": dbStatus}
					if len(state.Errors) > 0 {
						msgs := make([]string, len(state.Errors))
						for i, e := range state.Errors {
							msgs[i] = e.Message
						}
						fallback["errors"] = msgs
					}
					if j, e := json.Marshal(fallback); e == nil {
						s := string(j)
						resultStr = &s
					}
				}

				l.updateExecutionStatus(dbID, dbStatus, state.EndTime, resultStr)

				// 持久化采样日志到数据库
				l.persistSampleLogs(executionID, engine)

				return
			}
		}
	}
}

// persistSampleLogs 将引擎内存中的采样日志持久化到数据库
func (l *ExecutionLogic) persistSampleLogs(executionID string, engine *workflow.Engine) {
	engineExecID := l.resolveEngineID(executionID)
	rawLogs, err := engine.GetSampleLogs(context.Background(), engineExecID)
	if err != nil || rawLogs == nil {
		return
	}

	logsJSON, err := json.Marshal(rawLogs)
	if err != nil {
		return
	}

	var entries []struct {
		ExecutionID     string            `json:"execution_id"`
		StepID          string            `json:"step_id"`
		StepName        string            `json:"step_name"`
		Timestamp       time.Time         `json:"timestamp"`
		Status          string            `json:"status"`
		DurationMs      int64             `json:"duration_ms"`
		RequestMethod   string            `json:"request_method"`
		RequestURL      string            `json:"request_url"`
		RequestHeaders  map[string]string `json:"request_headers,omitempty"`
		RequestBody     string            `json:"request_body,omitempty"`
		ResponseStatus  int               `json:"response_status"`
		ResponseHeaders map[string]string `json:"response_headers,omitempty"`
		ResponseBody    string            `json:"response_body,omitempty"`
		ErrorMessage    string            `json:"error_message,omitempty"`
	}
	if err := json.Unmarshal(logsJSON, &entries); err != nil || len(entries) == 0 {
		return
	}

	var dbLogs []*model.TSampleLog
	for _, e := range entries {
		ts := e.Timestamp
		log := &model.TSampleLog{
			ExecutionID:    executionID,
			StepID:         e.StepID,
			StepName:       e.StepName,
			Timestamp:      &ts,
			Status:         e.Status,
			DurationMs:     e.DurationMs,
			RequestMethod:  e.RequestMethod,
			RequestURL:     e.RequestURL,
			ResponseStatus: e.ResponseStatus,
		}
		if e.RequestBody != "" {
			log.RequestBody = &e.RequestBody
		}
		if e.ResponseBody != "" {
			log.ResponseBody = &e.ResponseBody
		}
		if e.ErrorMessage != "" {
			log.ErrorMessage = &e.ErrorMessage
		}
		if len(e.RequestHeaders) > 0 {
			if j, err := json.Marshal(e.RequestHeaders); err == nil {
				s := string(j)
				log.RequestHeaders = &s
			}
		}
		if len(e.ResponseHeaders) > 0 {
			if j, err := json.Marshal(e.ResponseHeaders); err == nil {
				s := string(j)
				log.ResponseHeaders = &s
			}
		}
		dbLogs = append(dbLogs, log)
	}

	sampleLogLogic := NewSampleLogLogic(l.ctx)
	if err := sampleLogLogic.SaveSampleLogs(dbLogs); err != nil {
		fmt.Printf("[persistSampleLogs] 保存采样日志失败: %v\n", err)
	} else {
		fmt.Printf("[persistSampleLogs] 已保存 %d 条采样日志\n", len(dbLogs))
	}
}

func mapEngineStatus(s string) string {
	switch s {
	case "pending":
		return ExecutionStatusPending
	case "running":
		return ExecutionStatusRunning
	case "completed":
		return ExecutionStatusCompleted
	case "failed":
		return ExecutionStatusFailed
	case "aborted", "stopped":
		return ExecutionStatusStopped
	case "paused":
		return ExecutionStatusPaused
	default:
		return ""
	}
}

func isTerminalStatus(s string) bool {
	return s == ExecutionStatusCompleted || s == ExecutionStatusFailed || s == ExecutionStatusStopped
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

// GetRealtimeMetrics proxies realtime metrics from the engine.
// The engine's MetricsEngine handles all aggregation; gulu just forwards.
func (l *ExecutionLogic) GetRealtimeMetrics(id int64) (interface{}, error) {
	execution, err := l.GetByID(id)
	if err != nil {
		return nil, errors.New("执行记录不存在")
	}

	engine := workflow.GetEngine()
	if engine == nil {
		return nil, errors.New("引擎未启动")
	}

	engineExecID := l.resolveEngineID(execution.ExecutionID)
	return engine.GetRealtimeMetrics(context.Background(), engineExecID)
}

// GetReport retrieves the final performance test report from the engine.
func (l *ExecutionLogic) GetReport(id int64) (*types.PerformanceTestReport, error) {
	execution, err := l.GetByID(id)
	if err != nil {
		return nil, errors.New("执行记录不存在")
	}

	// First try to get from DB (stored result)
	if execution.Result != nil && *execution.Result != "" {
		var report types.PerformanceTestReport
		if err := json.Unmarshal([]byte(*execution.Result), &report); err == nil && report.ExecutionID != "" {
			return &report, nil
		}
	}

	// Fallback: get from engine (if still running or recently completed)
	engine := workflow.GetEngine()
	if engine == nil {
		return nil, errors.New("报告尚未生成")
	}

	engineExecID := l.resolveEngineID(execution.ExecutionID)
	return engine.GetPerformanceReport(context.Background(), engineExecID)
}

// ScaleVUs adjusts the VU count for a running execution.
func (l *ExecutionLogic) ScaleVUs(id int64, vus int) error {
	execution, err := l.GetByID(id)
	if err != nil {
		return errors.New("执行记录不存在")
	}

	if execution.Status != ExecutionStatusRunning {
		return errors.New("只能调整运行中的执行")
	}

	engine := workflow.GetEngine()
	if engine == nil {
		return errors.New("引擎未启动")
	}

	engineExecID := l.resolveEngineID(execution.ExecutionID)
	return engine.ScaleVUs(context.Background(), engineExecID, vus)
}

// GetTimeSeries retrieves the time-series data for charting.
func (l *ExecutionLogic) GetTimeSeries(id int64) (interface{}, error) {
	execution, err := l.GetByID(id)
	if err != nil {
		return nil, errors.New("执行记录不存在")
	}

	engine := workflow.GetEngine()
	if engine == nil {
		return nil, errors.New("引擎未启动")
	}

	engineExecID := l.resolveEngineID(execution.ExecutionID)
	return engine.GetTimeSeries(context.Background(), engineExecID)
}

// resolveEngineID maps a gulu execution ID to the engine's internal execution ID.
func (l *ExecutionLogic) resolveEngineID(guluExecID string) string {
	if mapped, ok := engineIDMap.Load(guluExecID); ok {
		return mapped.(string)
	}
	return guluExecID
}

// Stop stops a running execution via the engine API.
func (l *ExecutionLogic) Stop(id int64) error {
	execution, err := l.GetByID(id)
	if err != nil {
		return errors.New("执行记录不存在")
	}

	if execution.Status != ExecutionStatusRunning && execution.Status != ExecutionStatusPaused {
		return errors.New("只能停止运行中或暂停的执行")
	}

	engine := workflow.GetEngine()
	if engine != nil {
		engineExecID := l.resolveEngineID(execution.ExecutionID)
		_ = engine.StopExecution(context.Background(), engineExecID)
	}

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

// Pause pauses a running execution via the engine API.
func (l *ExecutionLogic) Pause(id int64) error {
	execution, err := l.GetByID(id)
	if err != nil {
		return errors.New("执行记录不存在")
	}

	if execution.Status != ExecutionStatusRunning {
		return errors.New("只能暂停运行中的执行")
	}

	engine := workflow.GetEngine()
	if engine != nil {
		engineExecID := l.resolveEngineID(execution.ExecutionID)
		_ = engine.PauseExecution(context.Background(), engineExecID)
	}

	now := time.Now()
	q := query.Use(svc.Ctx.DB)
	e := q.TExecution

	_, err = e.WithContext(l.ctx).Where(e.ID.Eq(id)).Updates(map[string]interface{}{
		"status":     ExecutionStatusPaused,
		"updated_at": now,
	})
	return err
}

// Resume resumes a paused execution via the engine API.
func (l *ExecutionLogic) Resume(id int64) error {
	execution, err := l.GetByID(id)
	if err != nil {
		return errors.New("执行记录不存在")
	}

	if execution.Status != ExecutionStatusPaused {
		return errors.New("只能恢复暂停的执行")
	}

	engine := workflow.GetEngine()
	if engine != nil {
		engineExecID := l.resolveEngineID(execution.ExecutionID)
		_ = engine.ResumeExecution(context.Background(), engineExecID)
	}

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

	// 设置采样模式
	if cfg.SamplingMode != "" {
		wf.Options.SamplingMode = types.SamplingMode(cfg.SamplingMode)
	}

	// 设置标签
	if len(cfg.Tags) > 0 {
		wf.Options.Tags = cfg.Tags
	}
}
