package executor

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"yqhp/gulu/internal/client"
	"yqhp/gulu/internal/sse"
	"yqhp/gulu/internal/workflow"
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
	SessionID     string    `json:"session_id"`
	TotalSteps    int       `json:"total_steps"`
	SuccessSteps  int       `json:"success_steps"`
	FailedSteps   int       `json:"failed_steps"`
	TotalDuration int64     `json:"total_duration_ms"`
	Status        string    `json:"status"`
	StartTime     time.Time `json:"start_time"`
	EndTime       time.Time `json:"end_time"`
}

// StreamExecutor 流式执行器
type StreamExecutor struct {
	sessionManager     *SessionManager
	slaveClientManager *client.SlaveClientManager
	engineClient       *client.WorkflowEngineClient
	callbackBaseURL    string // 回调基础 URL
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
	fmt.Printf("[DEBUG] ExecuteStream 开始: WorkflowID=%d, Steps=%d\n", req.WorkflowID, len(wf.Steps))

	// 创建会话
	session, err := e.sessionManager.CreateSession(req.WorkflowID, writer)
	if err != nil {
		return fmt.Errorf("创建会话失败: %w", err)
	}
	defer e.sessionManager.CleanupSession(session.ID)

	fmt.Printf("[DEBUG] 会话创建成功: SessionID=%s\n", session.ID)

	// 设置超时
	timeout := e.defaultTimeout
	if req.Timeout > 0 {
		timeout = time.Duration(req.Timeout) * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// 保存取消函数
	e.sessionManager.SetCancel(session.ID, cancel)

	// 创建 SSE 回调
	callback := NewSSECallback(writer, session)

	// 设置回调到工作流
	wf.Callback = callback
	fmt.Printf("[DEBUG] 回调已设置到工作流\n")

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
		fmt.Printf("[DEBUG] 开始本地执行\n")
		execErr = e.executeLocal(ctx, wf, session, callback)
		fmt.Printf("[DEBUG] 本地执行完成, err=%v\n", execErr)
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

	// 发送执行完成事件
	callback.OnExecutionComplete(ctx, nil)

	return execErr
}

// ExecuteBlocking 阻塞式执行
func (e *StreamExecutor) ExecuteBlocking(ctx context.Context, req *ExecuteRequest, wf *types.Workflow) (*ExecutionSummary, error) {
	// 创建一个临时的 writer（不实际写入）
	sessionID := fmt.Sprintf("blocking-%d-%d", req.WorkflowID, time.Now().UnixNano())
	writer := sse.NewWriter(&discardWriter{}, sessionID)

	// 创建会话
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

	// 保存取消函数
	e.sessionManager.SetCancel(session.ID, cancel)

	// 创建回调（不实际发送 SSE）
	callback := NewSSECallback(writer, session)

	// 设置回调到工作流
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
		// 远程阻塞式执行
		summary, err := e.executeRemoteBlocking(ctx, req, wf)
		if err != nil {
			execErr = err
		} else {
			// 直接返回远程执行结果
			return summary, nil
		}
	default:
		execErr = e.executeLocal(ctx, wf, session, callback)
	}

	// 构建汇总
	total, success, failed := session.GetStats()
	status := "success"
	if execErr != nil {
		if ctx.Err() == context.DeadlineExceeded {
			status = "timeout"
		} else if ctx.Err() == context.Canceled {
			status = "stopped"
		} else {
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
	}, execErr
}

// executeLocal 本地执行
func (e *StreamExecutor) executeLocal(ctx context.Context, wf *types.Workflow, session *Session, callback *SSECallback) error {
	fmt.Printf("[DEBUG] executeLocal 开始: WorkflowID=%s, Steps=%d\n", wf.ID, len(wf.Steps))

	// 获取工作流引擎
	engine := workflow.GetEngine()
	if engine == nil {
		fmt.Printf("[DEBUG] 工作流引擎未初始化!\n")
		return fmt.Errorf("工作流引擎未初始化")
	}
	fmt.Printf("[DEBUG] 工作流引擎已获取\n")

	// 提交给引擎执行
	execID, err := engine.SubmitWorkflow(ctx, wf)
	if err != nil {
		fmt.Printf("[DEBUG] 提交执行失败: %v\n", err)
		return fmt.Errorf("提交执行失败: %w", err)
	}
	fmt.Printf("[DEBUG] 工作流已提交, execID=%s\n", execID)

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

	// 等待执行完成
	fmt.Printf("[DEBUG] 开始等待执行完成\n")
	return e.waitForCompletion(ctx, engine, execID, session)
}

// executeRemote 远程流式执行
// Slave 返回 SSE 流，Gulu 作为 SSE 客户端接收事件并转发给前端
func (e *StreamExecutor) executeRemote(ctx context.Context, req *ExecuteRequest, wf *types.Workflow, callback *SSECallback) error {
	fmt.Printf("[DEBUG] executeRemote 开始: WorkflowID=%s, SlaveID=%s\n", wf.ID, req.SlaveID)

	// 获取 Slave 信息
	slaveStatus, err := e.engineClient.GetExecutorStatus(req.SlaveID)
	if err != nil {
		return fmt.Errorf("获取 Slave 状态失败: %w", err)
	}

	if slaveStatus.State != "online" {
		return fmt.Errorf("Slave 不可用: %s (状态: %s)", req.SlaveID, slaveStatus.State)
	}

	// 获取或创建 Slave 客户端
	slaveClient := e.slaveClientManager.GetClient(req.SlaveID, slaveStatus.Address)

	// 检查 Slave 连接
	if err := slaveClient.Ping(ctx); err != nil {
		return fmt.Errorf("Slave 连接失败: %w", err)
	}

	// 获取会话
	session, ok := e.sessionManager.GetSession(wf.ID)
	if !ok {
		return fmt.Errorf("会话不存在: %s", wf.ID)
	}

	// 构建交互 URL（用于人机交互时 Slave 等待 Gulu 的响应）
	interactionURL := fmt.Sprintf("%s/api/executions/%s/interaction", e.callbackBaseURL, wf.ID)

	// 构建流式执行请求
	slaveReq := &client.SlaveStreamExecuteRequest{
		Workflow:       wf,
		Variables:      req.Variables,
		SessionID:      wf.ID,
		Timeout:        req.Timeout,
		InteractionURL: interactionURL,
	}

	fmt.Printf("[DEBUG] 开始 SSE 流式执行: SlaveID=%s, SessionID=%s\n", req.SlaveID, wf.ID)

	// 启动心跳（在 SSE 流中可能不需要，但保留以防万一）
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
				// 只在没有收到事件时发送心跳
				callback.WriteHeartbeat()
			}
		}
	}()
	defer close(heartbeatDone)

	// 执行流式请求，接收 SSE 事件并转发
	var execErr error
	err = slaveClient.ExecuteStream(ctx, slaveReq, func(eventType string, data []byte) error {
		fmt.Printf("[DEBUG] 收到 Slave SSE 事件: type=%s\n", eventType)

		// 处理特殊事件
		switch eventType {
		case "workflow_completed":
			// 解析完成数据，更新会话状态
			var completedData struct {
				Status       string `json:"status"`
				TotalSteps   int    `json:"total_steps"`
				SuccessSteps int    `json:"success_steps"`
				FailedSteps  int    `json:"failed_steps"`
			}
			if err := json.Unmarshal(data, &completedData); err == nil {
				if completedData.FailedSteps > 0 || completedData.Status == "failed" {
					session.SetStatus(SessionStatusFailed)
				} else {
					session.SetStatus(SessionStatusCompleted)
				}
			}

		case "ai_interaction_required":
			// 人机交互：设置会话状态为等待
			session.SetStatus(SessionStatusWaiting)

			// 转发事件到前端
			if err := client.ForwardSSEEvent(callback.writer, eventType, data); err != nil {
				return err
			}

			// 等待前端响应（通过 session.InteractionCh）
			// 注意：这里会阻塞直到收到响应或超时
			var interactionData struct {
				StepID  string `json:"step_id"`
				Timeout int    `json:"timeout"`
			}
			if err := json.Unmarshal(data, &interactionData); err != nil {
				return fmt.Errorf("解析交互数据失败: %w", err)
			}

			timeout := time.Duration(interactionData.Timeout) * time.Second
			if timeout == 0 {
				timeout = 5 * time.Minute // 默认 5 分钟
			}

			resp, err := session.WaitForInteraction(ctx, timeout)
			if err != nil {
				return fmt.Errorf("等待交互响应失败: %w", err)
			}

			// 将响应发送给 Slave
			interactionReq := &client.InteractionSubmitRequest{
				SessionID: wf.ID,
				StepID:    interactionData.StepID,
				Value:     resp.Value,
				Skipped:   resp.Skipped,
			}
			if err := slaveClient.SubmitInteraction(ctx, interactionReq); err != nil {
				return fmt.Errorf("提交交互响应到 Slave 失败: %w", err)
			}

			// 恢复运行状态
			session.SetStatus(SessionStatusRunning)
			return nil

		case "error":
			// 解析错误，设置执行错误
			var errorData struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			}
			if err := json.Unmarshal(data, &errorData); err == nil {
				execErr = fmt.Errorf("%s: %s", errorData.Code, errorData.Message)
			}
		}

		// 转发事件到前端
		return client.ForwardSSEEvent(callback.writer, eventType, data)
	})

	if err != nil {
		return fmt.Errorf("SSE 流执行失败: %w", err)
	}

	return execErr
}

// executeRemoteBlocking 远程阻塞式执行
func (e *StreamExecutor) executeRemoteBlocking(ctx context.Context, req *ExecuteRequest, wf *types.Workflow) (*ExecutionSummary, error) {
	fmt.Printf("[DEBUG] executeRemoteBlocking 开始: WorkflowID=%s, SlaveID=%s\n", wf.ID, req.SlaveID)

	// 获取 Slave 信息
	slaveStatus, err := e.engineClient.GetExecutorStatus(req.SlaveID)
	if err != nil {
		return nil, fmt.Errorf("获取 Slave 状态失败: %w", err)
	}

	if slaveStatus.State != "online" {
		return nil, fmt.Errorf("Slave 不可用: %s (状态: %s)", req.SlaveID, slaveStatus.State)
	}

	// 获取或创建 Slave 客户端
	slaveClient := e.slaveClientManager.GetClient(req.SlaveID, slaveStatus.Address)

	// 检查 Slave 连接
	if err := slaveClient.Ping(ctx); err != nil {
		return nil, fmt.Errorf("Slave 连接失败: %w", err)
	}

	// 构建阻塞式执行请求
	slaveReq := &client.SlaveBlockingExecuteRequest{
		Workflow:  wf,
		Variables: req.Variables,
		SessionID: wf.ID,
		Timeout:   req.Timeout,
	}

	// 执行阻塞式请求
	resp, err := slaveClient.ExecuteBlocking(ctx, slaveReq)
	if err != nil {
		return nil, fmt.Errorf("阻塞式执行失败: %w", err)
	}

	// 转换响应
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
		StartTime:     time.Now(), // Slave 应该返回实际时间
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
			fmt.Printf("[DEBUG] waitForCompletion: 上下文取消\n")
			return ctx.Err()
		case <-ticker.C:
			pollCount++
			// 检查是否被停止
			if session.GetStatus() == SessionStatusStopped {
				fmt.Printf("[DEBUG] waitForCompletion: 会话被停止\n")
				engine.StopExecution(ctx, execID)
				return fmt.Errorf("执行被停止")
			}

			// 获取执行状态
			state, err := engine.GetExecutionStatus(ctx, execID)
			if err != nil {
				if pollCount%25 == 0 { // 每5秒打印一次
					fmt.Printf("[DEBUG] waitForCompletion: 获取状态失败: %v\n", err)
				}
				continue
			}

			if pollCount%25 == 0 { // 每5秒打印一次
				fmt.Printf("[DEBUG] waitForCompletion: 状态=%s, 进度=%.2f\n", state.Status, state.Progress)
			}

			switch state.Status {
			case types.ExecutionStatusCompleted:
				fmt.Printf("[DEBUG] waitForCompletion: 执行完成\n")
				return nil
			case types.ExecutionStatusFailed:
				fmt.Printf("[DEBUG] waitForCompletion: 执行失败\n")
				if len(state.Errors) > 0 {
					return fmt.Errorf(state.Errors[0].Message)
				}
				return fmt.Errorf("执行失败")
			case types.ExecutionStatusAborted:
				fmt.Printf("[DEBUG] waitForCompletion: 执行被中止\n")
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
