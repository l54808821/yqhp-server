package executor

import (
	"context"
	"fmt"
	"time"

	"yqhp/gulu/internal/sse"
	"yqhp/gulu/internal/workflow"
	"yqhp/workflow-engine/pkg/logger"
	"yqhp/workflow-engine/pkg/types"
)

// ExecuteRequest 执行请求
type ExecuteRequest struct {
	WorkflowID int64                  `json:"workflow_id"`
	EnvID      int64                  `json:"env_id"`
	Variables  map[string]interface{} `json:"variables,omitempty"`
	Timeout    int                    `json:"timeout,omitempty"`
}

// ExecutionSummary 执行汇总
type ExecutionSummary struct {
	SessionID     string                 `json:"sessionId"`
	TotalSteps    int                    `json:"totalSteps"`
	SuccessSteps  int                    `json:"successSteps"`
	FailedSteps   int                    `json:"failedSteps"`
	TotalDuration int64                  `json:"totalDurationMs"`
	Status        string                 `json:"status"`
	StartTime     time.Time              `json:"startTime"`
	EndTime       time.Time              `json:"endTime"`
	Steps         []StepExecutionResult  `json:"steps,omitempty"`
	Variables     map[string]interface{} `json:"variables,omitempty"`
	EnvVariables  map[string]interface{} `json:"envVariables,omitempty"`
}

// StepExecutionResult 步骤执行结果
type StepExecutionResult struct {
	StepID     string      `json:"stepId"`
	StepName   string      `json:"stepName"`
	StepType   string      `json:"stepType"`
	Success    bool        `json:"success"`
	DurationMs int64       `json:"durationMs"`
	Result     interface{} `json:"result,omitempty"`
	Error      string      `json:"error,omitempty"`
}

// StreamExecutor 流式执行器
// 所有执行统一通过内置 master 提交，由 master 根据 TargetSlaves 决定本地执行或分发到远程 Slave。
type StreamExecutor struct {
	sessionManager *SessionManager
	defaultTimeout time.Duration
}

// NewStreamExecutor 创建流式执行器
func NewStreamExecutor(sessionManager *SessionManager, defaultTimeout time.Duration) *StreamExecutor {
	if defaultTimeout == 0 {
		defaultTimeout = 30 * time.Minute
	}
	return &StreamExecutor{
		sessionManager: sessionManager,
		defaultTimeout: defaultTimeout,
	}
}

// ExecuteStream 流式执行（SSE）
func (e *StreamExecutor) ExecuteStream(ctx context.Context, req *ExecuteRequest, wf *types.Workflow, writer *sse.Writer) error {
	logger.Debug("ExecuteStream 开始", "workflow_id", req.WorkflowID, "steps", len(wf.Steps))

	session, err := e.sessionManager.CreateSession(req.WorkflowID, writer)
	if err != nil {
		return fmt.Errorf("创建会话失败: %w", err)
	}
	defer e.sessionManager.CleanupSession(session.ID)

	ctx, cancel := e.withTimeout(ctx, req.Timeout)
	defer cancel()
	e.sessionManager.SetCancel(session.ID, cancel)

	callback := NewSSECallback(writer, session)
	wf.Callback = callback

	mergeVariables(wf, req.Variables)

	execErr := e.executeViaEngine(ctx, wf, session, callback)

	if execErr != nil {
		if ctx.Err() == context.DeadlineExceeded {
			session.SetStatus(SessionStatusFailed)
			callback.WriteError("TIMEOUT", "执行超时", execErr.Error(), false)
		} else if ctx.Err() == context.Canceled {
			session.SetStatus(SessionStatusStopped)
		} else {
			session.SetStatus(SessionStatusFailed)
			callback.WriteError("EXECUTION_ERROR", "执行失败", execErr.Error(), false)
		}
	} else {
		session.SetStatus(SessionStatusCompleted)
	}

	if wf.FinalVariables != nil {
		session.SetVariables(wf.FinalVariables)
	}
	if wf.EnvVariables != nil {
		session.SetEnvVariables(wf.EnvVariables)
	}

	callback.OnExecutionComplete(ctx, nil)

	return execErr
}

// ExecuteBlocking 阻塞式执行
func (e *StreamExecutor) ExecuteBlocking(ctx context.Context, req *ExecuteRequest, wf *types.Workflow) (*ExecutionSummary, error) {
	sessionID := fmt.Sprintf("blocking-%d-%d", req.WorkflowID, time.Now().UnixNano())
	writer := sse.NewWriter(&discardWriter{}, sessionID)

	session, err := e.sessionManager.CreateSession(req.WorkflowID, writer)
	if err != nil {
		return nil, fmt.Errorf("创建会话失败: %w", err)
	}
	defer e.sessionManager.CleanupSession(session.ID)

	ctx, cancel := e.withTimeout(ctx, req.Timeout)
	defer cancel()
	e.sessionManager.SetCancel(session.ID, cancel)

	callback := NewSSECallback(writer, session)
	wf.Callback = callback

	mergeVariables(wf, req.Variables)

	execErr := e.executeViaEngine(ctx, wf, session, callback)

	total, success, failed := session.GetStats()
	status := "success"
	if execErr != nil {
		switch ctx.Err() {
		case context.DeadlineExceeded:
			status = "timeout"
		case context.Canceled:
			status = "stopped"
		default:
			status = "failed"
		}
	} else if failed > 0 {
		status = "failed"
	}

	return &ExecutionSummary{
		SessionID:     session.ID,
		TotalSteps:    total,
		SuccessSteps:  success,
		FailedSteps:   failed,
		TotalDuration: time.Since(session.StartTime).Milliseconds(),
		Status:        status,
		StartTime:     session.StartTime,
		EndTime:       time.Now(),
		Steps:         session.GetStepResults(),
		Variables:     wf.FinalVariables,
		EnvVariables:  wf.EnvVariables,
	}, nil
}

// executeViaEngine 通过内置 master 提交执行。
// TargetSlaves 已在调用方设好：nil 表示本地执行，非 nil 表示分发到指定 Slave。
func (e *StreamExecutor) executeViaEngine(ctx context.Context, wf *types.Workflow, session *Session, callback *SSECallback) error {
	engine := workflow.GetEngine()
	if engine == nil {
		return fmt.Errorf("工作流引擎未初始化")
	}

	if wf.Options.TargetSlaves == nil {
		wf.Options.TargetSlaves = &types.SlaveSelector{
			Mode: types.SelectionModeLocal,
		}
	}

	wf.Callback = callback

	execID, err := engine.SubmitWorkflow(ctx, wf)
	if err != nil {
		return fmt.Errorf("提交执行失败: %w", err)
	}
	logger.Debug("工作流已提交", "exec_id", execID, "target", wf.Options.TargetSlaves.Mode)

	heartbeatDone := make(chan struct{})
	go func() {
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-heartbeatDone:
				return
			case <-ticker.C:
				callback.WriteHeartbeat()
			}
		}
	}()
	defer close(heartbeatDone)

	return e.waitForCompletion(ctx, engine, execID, session)
}

// waitForCompletion 轮询等待执行完成
func (e *StreamExecutor) waitForCompletion(ctx context.Context, engine *workflow.Engine, execID string, session *Session) error {
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	pollCount := 0
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			pollCount++

			if session.GetStatus() == SessionStatusStopped {
				engine.StopExecution(ctx, execID)
				return fmt.Errorf("执行被停止")
			}

			state, err := engine.GetExecutionStatus(ctx, execID)
			if err != nil {
				if pollCount%25 == 0 {
					logger.Warn("waitForCompletion: 获取状态失败", "error", err)
				}
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

// withTimeout 创建带超时的上下文
func (e *StreamExecutor) withTimeout(ctx context.Context, timeoutSec int) (context.Context, context.CancelFunc) {
	timeout := e.defaultTimeout
	if timeoutSec > 0 {
		timeout = time.Duration(timeoutSec) * time.Second
	}
	return context.WithTimeout(ctx, timeout)
}

// mergeVariables 将请求变量合并到工作流中
func mergeVariables(wf *types.Workflow, variables map[string]interface{}) {
	if variables == nil {
		return
	}
	if wf.Variables == nil {
		wf.Variables = make(map[string]any)
	}
	for k, v := range variables {
		wf.Variables[k] = v
	}
}

// Stop 停止执行
func (e *StreamExecutor) Stop(sessionID string) error {
	return e.sessionManager.StopSession(sessionID)
}

// GetSession 获取会话
func (e *StreamExecutor) GetSession(sessionID string) (*Session, bool) {
	return e.sessionManager.GetSession(sessionID)
}

// IsRunning 检查会话是否在运行
func (e *StreamExecutor) IsRunning(sessionID string) bool {
	return e.sessionManager.IsRunning(sessionID)
}

// discardWriter 丢弃写入的数据（阻塞式执行用）
type discardWriter struct{}

func (d *discardWriter) Write(p []byte) (n int, err error) {
	return len(p), nil
}
