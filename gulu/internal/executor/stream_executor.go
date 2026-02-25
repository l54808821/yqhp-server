package executor

import (
	"context"
	"fmt"
	"time"

	"yqhp/gulu/internal/sse"
	"yqhp/gulu/internal/workflow"
	"yqhp/workflow-engine/pkg/logger"
	"yqhp/workflow-engine/pkg/runner"
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
type StreamExecutor struct {
	sessionManager *SessionManager
	defaultTimeout time.Duration
}

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

	execErr := e.executeViaRunner(ctx, wf, callback)

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

	execErr := e.executeViaRunner(ctx, wf, callback)

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

// executeViaRunner uses the unified runner.Run to execute the workflow.
func (e *StreamExecutor) executeViaRunner(ctx context.Context, wf *types.Workflow, callback *SSECallback) error {
	eng := workflow.GetEngine()
	if eng == nil {
		return fmt.Errorf("工作流引擎未初始化")
	}

	if wf.Options.TargetSlaves == nil {
		wf.Options.TargetSlaves = &types.SlaveSelector{
			Mode: types.SelectionModeLocal,
		}
	}

	wf.Callback = callback

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

	result, err := runner.Run(ctx, runner.RunOptions{
		Workflow: wf,
		Engine:   eng,
	})

	if result != nil {
		logger.Debug("执行完成", "exec_id", result.ExecutionID, "status", result.Status)
	}

	return err
}

func (e *StreamExecutor) withTimeout(ctx context.Context, timeoutSec int) (context.Context, context.CancelFunc) {
	timeout := e.defaultTimeout
	if timeoutSec > 0 {
		timeout = time.Duration(timeoutSec) * time.Second
	}
	return context.WithTimeout(ctx, timeout)
}

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

func (e *StreamExecutor) Stop(sessionID string) error {
	return e.sessionManager.StopSession(sessionID)
}

func (e *StreamExecutor) GetSession(sessionID string) (*Session, bool) {
	return e.sessionManager.GetSession(sessionID)
}

func (e *StreamExecutor) IsRunning(sessionID string) bool {
	return e.sessionManager.IsRunning(sessionID)
}

type discardWriter struct{}

func (d *discardWriter) Write(p []byte) (n int, err error) {
	return len(p), nil
}
