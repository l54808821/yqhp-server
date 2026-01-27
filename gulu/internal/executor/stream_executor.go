package executor

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"yqhp/gulu/internal/client"
	"yqhp/gulu/internal/sse"
	"yqhp/gulu/internal/workflow"
	"yqhp/workflow-engine/pkg/logger"
	"yqhp/workflow-engine/pkg/types"
)

// ExecutorType 执行器类型
type ExecutorType string

const (
	ExecutorTypeLocal  ExecutorType = "local"
	ExecutorTypeRemote ExecutorType = "remote"
)

// ExecuteRequest 执行请求
type ExecuteRequest struct {
	WorkflowID   int64                  `json:"workflow_id"`
	EnvID        int64                  `json:"env_id"`
	Variables    map[string]interface{} `json:"variables,omitempty"`
	Timeout      int                    `json:"timeout,omitempty"`
	ExecutorType ExecutorType           `json:"executor_type"`
	SlaveID      string                 `json:"slave_id,omitempty"`
}

// ExecutionSummary 执行汇总
type ExecutionSummary struct {
	SessionID     string                `json:"sessionId"`
	TotalSteps    int                   `json:"totalSteps"`
	SuccessSteps  int                   `json:"successSteps"`
	FailedSteps   int                   `json:"failedSteps"`
	TotalDuration int64                 `json:"totalDurationMs"`
	Status        string                `json:"status"`
	StartTime     time.Time             `json:"startTime"`
	EndTime       time.Time             `json:"endTime"`
	Steps         []StepExecutionResult `json:"steps,omitempty"` // 步骤执行详情
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
	sessionManager     *SessionManager
	slaveClientManager *client.SlaveClientManager
	engineClient       *client.WorkflowEngineClient
	callbackBaseURL    string
	defaultTimeout     time.Duration
}

// NewStreamExecutor 创建流式执行器
func NewStreamExecutor(sessionManager *SessionManager, defaultTimeout time.Duration) *StreamExecutor {
	if defaultTimeout == 0 {
		defaultTimeout = 30 * time.Minute
	}
	return &StreamExecutor{
		sessionManager:     sessionManager,
		slaveClientManager: client.NewSlaveClientManager(),
		engineClient:       client.NewWorkflowEngineClient(),
		defaultTimeout:     defaultTimeout,
	}
}

// SetCallbackBaseURL 设置回调基础 URL
func (e *StreamExecutor) SetCallbackBaseURL(url string) {
	e.callbackBaseURL = url
}

// ExecuteStream 流式执行（SSE）
func (e *StreamExecutor) ExecuteStream(ctx context.Context, req *ExecuteRequest, wf *types.Workflow, writer *sse.Writer) error {
	logger.Debug("ExecuteStream 开始", "workflow_id", req.WorkflowID, "steps", len(wf.Steps))

	// 创建会话
	session, err := e.sessionManager.CreateSession(req.WorkflowID, writer)
	if err != nil {
		return fmt.Errorf("创建会话失败: %w", err)
	}
	defer e.sessionManager.CleanupSession(session.ID)

	logger.Debug("会话创建成功", "session_id", session.ID)

	// 设置超时
	timeout := e.defaultTimeout
	if req.Timeout > 0 {
		timeout = time.Duration(req.Timeout) * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	e.sessionManager.SetCancel(session.ID, cancel)

	// 创建 SSE 回调
	callback := NewSSECallback(writer, session)
	wf.Callback = callback

	// 合并变量
	if req.Variables != nil {
		if wf.Variables == nil {
			wf.Variables = make(map[string]any)
		}
		for k, v := range req.Variables {
			wf.Variables[k] = v
		}
	}

	// 根据执行器类型选择执行方式
	var execErr error
	switch req.ExecutorType {
	case ExecutorTypeRemote:
		execErr = e.executeRemote(ctx, req, wf, callback)
	default:
		logger.Debug("开始本地执行")
		execErr = e.executeLocal(ctx, wf, session, callback)
		logger.Debug("本地执行完成", "error", execErr)
	}

	// 更新会话状态并发送完成事件
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

	// 设置超时
	timeout := e.defaultTimeout
	if req.Timeout > 0 {
		timeout = time.Duration(req.Timeout) * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	e.sessionManager.SetCancel(session.ID, cancel)

	callback := NewSSECallback(writer, session)
	wf.Callback = callback

	// 合并变量
	if req.Variables != nil {
		if wf.Variables == nil {
			wf.Variables = make(map[string]any)
		}
		for k, v := range req.Variables {
			wf.Variables[k] = v
		}
	}

	// 执行
	var execErr error
	switch req.ExecutorType {
	case ExecutorTypeRemote:
		summary, err := e.executeRemoteBlocking(ctx, req, wf)
		if err != nil {
			execErr = err
		} else {
			return summary, nil
		}
	default:
		execErr = e.executeLocal(ctx, wf, session, callback)
	}

	// 构建汇总
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
	}, execErr
}

// executeLocal 本地执行（通过 workflow-engine）
func (e *StreamExecutor) executeLocal(ctx context.Context, wf *types.Workflow, session *Session, callback *SSECallback) error {
	logger.Debug("executeLocal 开始", "workflow_id", wf.ID, "steps", len(wf.Steps))

	// 优先使用 workflow-engine API
	engine := workflow.GetEngine()
	if engine == nil {
		logger.Error("工作流引擎未初始化")
		return fmt.Errorf("工作流引擎未初始化")
	}

	// 设置回调到工作流
	wf.Callback = callback

	execID, err := engine.SubmitWorkflow(ctx, wf)
	if err != nil {
		logger.Error("提交执行失败", "error", err)
		return fmt.Errorf("提交执行失败: %w", err)
	}
	logger.Debug("工作流已提交", "exec_id", execID)

	// 启动心跳
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

// executeRemote 远程流式执行
func (e *StreamExecutor) executeRemote(ctx context.Context, req *ExecuteRequest, wf *types.Workflow, callback *SSECallback) error {
	logger.Debug("executeRemote 开始", "workflow_id", wf.ID, "slave_id", req.SlaveID)

	slaveStatus, err := e.engineClient.GetExecutorStatus(req.SlaveID)
	if err != nil {
		return fmt.Errorf("获取 Slave 状态失败: %w", err)
	}

	if slaveStatus.State != "online" {
		return fmt.Errorf("Slave 不可用: %s (状态: %s)", req.SlaveID, slaveStatus.State)
	}

	slaveClient := e.slaveClientManager.GetClient(req.SlaveID, slaveStatus.Address)

	if err := slaveClient.Ping(ctx); err != nil {
		return fmt.Errorf("Slave 连接失败: %w", err)
	}

	session, ok := e.sessionManager.GetSession(wf.ID)
	if !ok {
		return fmt.Errorf("会话不存在: %s", wf.ID)
	}

	interactionURL := fmt.Sprintf("%s/api/executions/%s/interaction", e.callbackBaseURL, wf.ID)

	slaveReq := &client.SlaveStreamExecuteRequest{
		Workflow:       wf,
		Variables:      req.Variables,
		SessionID:      wf.ID,
		Timeout:        req.Timeout,
		InteractionURL: interactionURL,
	}

	logger.Debug("开始 SSE 流式执行", "slave_id", req.SlaveID, "session_id", wf.ID)

	// 启动心跳
	heartbeatDone := make(chan struct{})
	go func() {
		ticker := time.NewTicker(30 * time.Second)
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

	var execErr error
	err = slaveClient.ExecuteStream(ctx, slaveReq, func(eventType string, data []byte) error {
		logger.Debug("收到 Slave SSE 事件", "type", eventType)

		switch eventType {
		case "workflow_completed":
			var completedData struct {
				Status       string `json:"status"`
				TotalSteps   int    `json:"totalSteps"`
				SuccessSteps int    `json:"successSteps"`
				FailedSteps  int    `json:"failedSteps"`
			}
			if err := json.Unmarshal(data, &completedData); err == nil {
				if completedData.FailedSteps > 0 || completedData.Status == "failed" {
					session.SetStatus(SessionStatusFailed)
				} else {
					session.SetStatus(SessionStatusCompleted)
				}
			}

		case "ai_interaction_required":
			session.SetStatus(SessionStatusWaiting)

			if err := client.ForwardSSEEvent(callback.writer, eventType, data); err != nil {
				return err
			}

			var interactionData struct {
				StepID  string `json:"stepId"`
				Timeout int    `json:"timeout"`
			}
			if err := json.Unmarshal(data, &interactionData); err != nil {
				return fmt.Errorf("解析交互数据失败: %w", err)
			}

			timeout := time.Duration(interactionData.Timeout) * time.Second
			if timeout == 0 {
				timeout = 5 * time.Minute
			}

			resp, err := session.WaitForInteraction(ctx, timeout)
			if err != nil {
				return fmt.Errorf("等待交互响应失败: %w", err)
			}

			interactionReq := &client.InteractionSubmitRequest{
				SessionID: wf.ID,
				StepID:    interactionData.StepID,
				Value:     resp.Value,
				Skipped:   resp.Skipped,
			}
			if err := slaveClient.SubmitInteraction(ctx, interactionReq); err != nil {
				return fmt.Errorf("提交交互响应到 Slave 失败: %w", err)
			}

			session.SetStatus(SessionStatusRunning)
			return nil

		case "error":
			var errorData struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			}
			if err := json.Unmarshal(data, &errorData); err == nil {
				execErr = fmt.Errorf("%s: %s", errorData.Code, errorData.Message)
			}
		}

		return client.ForwardSSEEvent(callback.writer, eventType, data)
	})

	if err != nil {
		return fmt.Errorf("SSE 流执行失败: %w", err)
	}

	return execErr
}

// executeRemoteBlocking 远程阻塞式执行
func (e *StreamExecutor) executeRemoteBlocking(ctx context.Context, req *ExecuteRequest, wf *types.Workflow) (*ExecutionSummary, error) {
	logger.Debug("executeRemoteBlocking 开始", "workflow_id", wf.ID, "slave_id", req.SlaveID)

	slaveStatus, err := e.engineClient.GetExecutorStatus(req.SlaveID)
	if err != nil {
		return nil, fmt.Errorf("获取 Slave 状态失败: %w", err)
	}

	if slaveStatus.State != "online" {
		return nil, fmt.Errorf("Slave 不可用: %s (状态: %s)", req.SlaveID, slaveStatus.State)
	}

	slaveClient := e.slaveClientManager.GetClient(req.SlaveID, slaveStatus.Address)

	if err := slaveClient.Ping(ctx); err != nil {
		return nil, fmt.Errorf("Slave 连接失败: %w", err)
	}

	slaveReq := &client.SlaveBlockingExecuteRequest{
		Workflow:  wf,
		Variables: req.Variables,
		SessionID: wf.ID,
		Timeout:   req.Timeout,
	}

	resp, err := slaveClient.ExecuteBlocking(ctx, slaveReq)
	if err != nil {
		return nil, fmt.Errorf("阻塞式执行失败: %w", err)
	}

	status := resp.Status
	if resp.FailedSteps > 0 && status == "" {
		status = "failed"
	} else if status == "" {
		status = "success"
	}

	return &ExecutionSummary{
		SessionID:     resp.SessionID,
		TotalSteps:    resp.TotalSteps,
		SuccessSteps:  resp.SuccessSteps,
		FailedSteps:   resp.FailedSteps,
		TotalDuration: resp.TotalDuration,
		Status:        status,
		StartTime:     time.Now(),
		EndTime:       time.Now(),
	}, nil
}

// waitForCompletion 等待执行完成
func (e *StreamExecutor) waitForCompletion(ctx context.Context, engine *workflow.Engine, execID string, session *Session) error {
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	pollCount := 0
	for {
		select {
		case <-ctx.Done():
			logger.Debug("waitForCompletion: 上下文取消")
			return ctx.Err()
		case <-ticker.C:
			pollCount++

			if session.GetStatus() == SessionStatusStopped {
				logger.Debug("waitForCompletion: 会话被停止")
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

			if pollCount%25 == 0 {
				logger.Debug("waitForCompletion: 状态检查", "status", state.Status, "progress", state.Progress)
			}

			switch state.Status {
			case types.ExecutionStatusCompleted:
				logger.Debug("waitForCompletion: 执行完成")
				return nil
			case types.ExecutionStatusFailed:
				logger.Debug("waitForCompletion: 执行失败")
				if len(state.Errors) > 0 {
					return fmt.Errorf(state.Errors[0].Message)
				}
				return fmt.Errorf("执行失败")
			case types.ExecutionStatusAborted:
				logger.Debug("waitForCompletion: 执行被中止")
				return fmt.Errorf("执行被中止")
			}
		}
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

// discardWriter 丢弃写入的数据
type discardWriter struct{}

func (d *discardWriter) Write(p []byte) (n int, err error) {
	return len(p), nil
}
