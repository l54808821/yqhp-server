package executor

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"yqhp/gulu/internal/websocket"
	"yqhp/gulu/internal/workflow"
	"yqhp/workflow-engine/pkg/types"
)

// DebugRequest 调试请求
type DebugRequest struct {
	SessionID string
	Workflow  *types.Workflow
	Variables map[string]interface{}
	Timeout   time.Duration
}

// DebugSummary 调试汇总
type DebugSummary struct {
	SessionID     string                  `json:"session_id"`
	TotalSteps    int                     `json:"total_steps"`
	SuccessSteps  int                     `json:"success_steps"`
	FailedSteps   int                     `json:"failed_steps"`
	TotalDuration int64                   `json:"total_duration_ms"`
	Status        string                  `json:"status"` // success, failed, timeout, stopped
	StepResults   []*websocket.StepResult `json:"step_results"`
	StartTime     time.Time               `json:"start_time"`
	EndTime       time.Time               `json:"end_time"`
}

// MasterExecutor Master 内置执行器
type MasterExecutor struct {
	hub     *websocket.Hub
	timeout time.Duration

	// 活跃的调试会话
	sessions map[string]*debugSession
	mu       sync.RWMutex
}

// debugSession 调试会话
type debugSession struct {
	sessionID string
	cancel    context.CancelFunc
	stopped   bool
}

// NewMasterExecutor 创建 Master 执行器
func NewMasterExecutor(hub *websocket.Hub, timeout time.Duration) *MasterExecutor {
	if timeout == 0 {
		timeout = 30 * time.Minute // 默认30分钟超时
	}
	return &MasterExecutor{
		hub:      hub,
		timeout:  timeout,
		sessions: make(map[string]*debugSession),
	}
}

// Execute 执行调试
func (e *MasterExecutor) Execute(ctx context.Context, req *DebugRequest) (*DebugSummary, error) {
	// 创建带超时的上下文
	timeout := req.Timeout
	if timeout == 0 {
		timeout = e.timeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// 注册会话
	session := &debugSession{
		sessionID: req.SessionID,
		cancel:    cancel,
	}
	e.mu.Lock()
	e.sessions[req.SessionID] = session
	e.mu.Unlock()

	defer func() {
		e.mu.Lock()
		delete(e.sessions, req.SessionID)
		e.mu.Unlock()
	}()

	// 初始化汇总
	summary := &DebugSummary{
		SessionID:   req.SessionID,
		TotalSteps:  len(req.Workflow.Steps),
		StepResults: make([]*websocket.StepResult, 0),
		StartTime:   time.Now(),
	}

	// 获取工作流引擎
	engine := workflow.GetEngine()
	if engine == nil {
		return nil, fmt.Errorf("工作流引擎未初始化")
	}

	// 执行工作流步骤
	for i, step := range req.Workflow.Steps {
		// 检查是否被停止
		e.mu.RLock()
		stopped := session.stopped
		e.mu.RUnlock()
		if stopped {
			summary.Status = "stopped"
			break
		}

		// 检查上下文是否取消
		select {
		case <-ctx.Done():
			if ctx.Err() == context.DeadlineExceeded {
				summary.Status = "timeout"
			} else {
				summary.Status = "stopped"
			}
			break
		default:
		}

		// 广播进度
		e.hub.BroadcastProgress(req.SessionID, &websocket.ProgressData{
			CurrentStep: i + 1,
			TotalSteps:  len(req.Workflow.Steps),
			Percentage:  (i * 100) / len(req.Workflow.Steps),
			StepName:    step.Name,
		})

		// 广播步骤开始
		e.hub.BroadcastStepStarted(req.SessionID, step.ID, step.Name)

		// 执行步骤
		stepResult := e.executeStep(ctx, req.Workflow, &step, req.Variables)
		summary.StepResults = append(summary.StepResults, stepResult)

		if stepResult.Status == "success" {
			summary.SuccessSteps++
			e.hub.BroadcastStepCompleted(req.SessionID, stepResult)
		} else {
			summary.FailedSteps++
			e.hub.BroadcastStepFailed(req.SessionID, step.ID, step.Name, stepResult.Error)

			// 根据错误策略决定是否继续
			if step.OnError == types.ErrorStrategyAbort || step.OnError == "" {
				summary.Status = "failed"
				break
			}
		}
	}

	// 设置最终状态
	summary.EndTime = time.Now()
	summary.TotalDuration = summary.EndTime.Sub(summary.StartTime).Milliseconds()
	if summary.Status == "" {
		if summary.FailedSteps > 0 {
			summary.Status = "failed"
		} else {
			summary.Status = "success"
		}
	}

	// 广播完成
	e.hub.BroadcastDebugComplete(req.SessionID, &websocket.DebugSummary{
		SessionID:     summary.SessionID,
		TotalSteps:    summary.TotalSteps,
		SuccessSteps:  summary.SuccessSteps,
		FailedSteps:   summary.FailedSteps,
		TotalDuration: summary.TotalDuration,
		Status:        summary.Status,
		StartTime:     summary.StartTime,
		EndTime:       summary.EndTime,
	})

	return summary, nil
}

// executeStep 执行单个步骤
func (e *MasterExecutor) executeStep(ctx context.Context, wf *types.Workflow, step *types.Step, variables map[string]interface{}) *websocket.StepResult {
	startTime := time.Now()

	result := &websocket.StepResult{
		StepID:   step.ID,
		StepName: step.Name,
		Status:   "success",
		Output:   make(map[string]interface{}),
		Logs:     make([]string, 0),
	}

	// 获取工作流引擎
	engine := workflow.GetEngine()
	if engine == nil {
		result.Status = "failed"
		result.Error = "工作流引擎未初始化"
		result.Duration = time.Since(startTime).Milliseconds()
		return result
	}

	// 创建单步骤工作流用于执行
	singleStepWf := &types.Workflow{
		ID:        wf.ID + "_step_" + step.ID,
		Name:      step.Name,
		Variables: mergeVariables(wf.Variables, variables),
		Steps:     []types.Step{*step},
		Options: types.ExecutionOptions{
			VUs:           1,
			Iterations:    1,
			ExecutionMode: "constant-vus",
		},
	}

	// 提交执行
	execID, err := engine.SubmitWorkflow(ctx, singleStepWf)
	if err != nil {
		result.Status = "failed"
		result.Error = fmt.Sprintf("提交执行失败: %v", err)
		result.Duration = time.Since(startTime).Milliseconds()
		return result
	}

	// 等待执行完成
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			result.Status = "timeout"
			result.Error = "执行超时"
			result.Duration = time.Since(startTime).Milliseconds()
			return result
		case <-ticker.C:
			state, err := engine.GetExecutionStatus(ctx, execID)
			if err != nil {
				result.Logs = append(result.Logs, fmt.Sprintf("获取状态失败: %v", err))
				continue
			}

			switch state.Status {
			case types.ExecutionStatusCompleted:
				result.Status = "success"
				result.Duration = time.Since(startTime).Milliseconds()
				// 尝试获取输出
				if state.AggregatedMetrics != nil {
					result.Output["metrics"] = state.AggregatedMetrics
				}
				return result
			case types.ExecutionStatusFailed, types.ExecutionStatusAborted:
				result.Status = "failed"
				if len(state.Errors) > 0 {
					result.Error = state.Errors[0].Message
				} else {
					result.Error = "执行失败"
				}
				result.Duration = time.Since(startTime).Milliseconds()
				return result
			}
		}
	}
}

// mergeVariables 合并变量
func mergeVariables(base map[string]any, override map[string]interface{}) map[string]any {
	result := make(map[string]any)
	for k, v := range base {
		result[k] = v
	}
	for k, v := range override {
		result[k] = v
	}
	return result
}

// Stop 停止调试
func (e *MasterExecutor) Stop(sessionID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	session, ok := e.sessions[sessionID]
	if !ok {
		return fmt.Errorf("会话不存在: %s", sessionID)
	}

	session.stopped = true
	if session.cancel != nil {
		session.cancel()
	}

	return nil
}

// IsRunning 检查会话是否在运行
func (e *MasterExecutor) IsRunning(sessionID string) bool {
	e.mu.RLock()
	defer e.mu.RUnlock()

	session, ok := e.sessions[sessionID]
	return ok && !session.stopped
}

// GetActiveSessions 获取活跃会话数
func (e *MasterExecutor) GetActiveSessions() int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return len(e.sessions)
}

// SummaryToJSON 将汇总转换为 JSON
func (s *DebugSummary) ToJSON() (string, error) {
	data, err := json.Marshal(s)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
