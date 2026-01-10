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
	summary   *DebugSummary
	mu        sync.Mutex
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

	// 初始化汇总
	summary := &DebugSummary{
		SessionID:   req.SessionID,
		TotalSteps:  0,
		StepResults: make([]*websocket.StepResult, 0),
		StartTime:   time.Now(),
	}

	// 注册会话
	session := &debugSession{
		sessionID: req.SessionID,
		cancel:    cancel,
		summary:   summary,
	}
	e.mu.Lock()
	e.sessions[req.SessionID] = session
	e.mu.Unlock()

	defer func() {
		e.mu.Lock()
		delete(e.sessions, req.SessionID)
		e.mu.Unlock()
	}()

	// 获取工作流引擎
	engine := workflow.GetEngine()
	if engine == nil {
		return nil, fmt.Errorf("工作流引擎未初始化")
	}

	// 创建回调实现
	callback := &debugCallback{
		hub:       e.hub,
		sessionID: req.SessionID,
		session:   session,
	}

	// 设置回调到工作流
	req.Workflow.Callback = callback

	// 合并变量
	if req.Variables != nil {
		if req.Workflow.Variables == nil {
			req.Workflow.Variables = make(map[string]any)
		}
		for k, v := range req.Variables {
			req.Workflow.Variables[k] = v
		}
	}

	// 提交给引擎执行
	execID, err := engine.SubmitWorkflow(ctx, req.Workflow)
	if err != nil {
		return nil, fmt.Errorf("提交执行失败: %w", err)
	}

	// 等待执行完成
	err = e.waitForCompletion(ctx, engine, execID, session)

	// 设置最终状态
	session.mu.Lock()
	summary.EndTime = time.Now()
	summary.TotalDuration = summary.EndTime.Sub(summary.StartTime).Milliseconds()
	if summary.Status == "" {
		if err != nil {
			if ctx.Err() == context.DeadlineExceeded {
				summary.Status = "timeout"
			} else if session.stopped {
				summary.Status = "stopped"
			} else {
				summary.Status = "failed"
			}
		} else if summary.FailedSteps > 0 {
			summary.Status = "failed"
		} else {
			summary.Status = "success"
		}
	}
	session.mu.Unlock()

	// 广播最终进度100%
	e.hub.BroadcastProgress(req.SessionID, &websocket.ProgressData{
		CurrentStep: summary.TotalSteps,
		TotalSteps:  summary.TotalSteps,
		Percentage:  100,
		StepName:    "完成",
	})

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

// waitForCompletion 等待执行完成
func (e *MasterExecutor) waitForCompletion(ctx context.Context, engine *workflow.Engine, execID string, session *debugSession) error {
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			// 检查是否被停止
			e.mu.RLock()
			stopped := session.stopped
			e.mu.RUnlock()
			if stopped {
				engine.StopExecution(ctx, execID)
				return fmt.Errorf("执行被停止")
			}

			// 获取执行状态
			state, err := engine.GetExecutionStatus(ctx, execID)
			if err != nil {
				continue
			}

			switch state.Status {
			case types.ExecutionStatusCompleted:
				return nil
			case types.ExecutionStatusFailed:
				if len(state.Errors) > 0 {
					return fmt.Errorf(state.Errors[0].Message)
				}
				return fmt.Errorf("执行失败")
			case types.ExecutionStatusAborted:
				return fmt.Errorf("执行被中止")
			}
		}
	}
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

// debugCallback 调试回调实现
type debugCallback struct {
	hub       *websocket.Hub
	sessionID string
	session   *debugSession
}

// OnStepStart 步骤开始
func (c *debugCallback) OnStepStart(ctx context.Context, step *types.Step, parentID string, iteration int) {
	c.hub.BroadcastStepStarted(c.sessionID, &websocket.StepStartedData{
		StepID:    step.ID,
		StepName:  step.Name,
		StepType:  step.Type,
		ParentID:  parentID,
		Iteration: iteration,
	})
}

// OnStepComplete 步骤完成
func (c *debugCallback) OnStepComplete(ctx context.Context, step *types.Step, result *types.StepResult, parentID string, iteration int) {
	c.session.mu.Lock()
	c.session.summary.TotalSteps++
	c.session.summary.SuccessSteps++
	c.session.mu.Unlock()

	wsResult := &websocket.StepResult{
		StepID:    step.ID,
		StepName:  step.Name,
		StepType:  step.Type,
		ParentID:  parentID,
		Iteration: iteration,
		Status:    "success",
		Duration:  result.Duration.Milliseconds(),
	}

	// 将 output 转换为 map[string]interface{}
	if result.Output != nil {
		// 先尝试直接类型断言
		if outputMap, ok := result.Output.(map[string]interface{}); ok {
			wsResult.Output = outputMap
		} else {
			// 如果不是 map，尝试通过 JSON 序列化/反序列化转换
			// 这样可以处理结构体类型（如 HTTPResponse、FastHTTPResponse）
			jsonBytes, err := json.Marshal(result.Output)
			if err == nil {
				var outputMap map[string]interface{}
				if err := json.Unmarshal(jsonBytes, &outputMap); err == nil {
					wsResult.Output = outputMap
				}
			}
		}
	}

	c.session.mu.Lock()
	c.session.summary.StepResults = append(c.session.summary.StepResults, wsResult)
	c.session.mu.Unlock()

	c.hub.BroadcastStepCompleted(c.sessionID, wsResult)
}

// OnStepFailed 步骤失败
func (c *debugCallback) OnStepFailed(ctx context.Context, step *types.Step, err error, duration time.Duration, parentID string, iteration int) {
	c.session.mu.Lock()
	c.session.summary.TotalSteps++
	c.session.summary.FailedSteps++
	c.session.mu.Unlock()

	errMsg := ""
	if err != nil {
		errMsg = err.Error()
	}

	wsResult := &websocket.StepResult{
		StepID:    step.ID,
		StepName:  step.Name,
		StepType:  step.Type,
		ParentID:  parentID,
		Iteration: iteration,
		Status:    "failed",
		Duration:  duration.Milliseconds(),
		Error:     errMsg,
	}

	c.session.mu.Lock()
	c.session.summary.StepResults = append(c.session.summary.StepResults, wsResult)
	c.session.mu.Unlock()

	c.hub.BroadcastStepFailed(c.sessionID, step.ID, step.Name, errMsg)
}

// OnProgress 进度更新
func (c *debugCallback) OnProgress(ctx context.Context, current, total int, stepName string) {
	percentage := 0
	if total > 0 {
		percentage = current * 100 / total
	}

	c.hub.BroadcastProgress(c.sessionID, &websocket.ProgressData{
		CurrentStep: current,
		TotalSteps:  total,
		Percentage:  percentage,
		StepName:    stepName,
	})
}

// OnExecutionComplete 执行完成
func (c *debugCallback) OnExecutionComplete(ctx context.Context, summary *types.ExecutionSummary) {
	// 这个回调在 waitForCompletion 之后由 Execute 方法处理
	// 这里不需要做任何事情
}

// 确保 debugCallback 实现了 ExecutionCallback 接口
var _ types.ExecutionCallback = (*debugCallback)(nil)
