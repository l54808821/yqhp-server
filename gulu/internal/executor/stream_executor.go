package executor

import (
	"context"
	"fmt"
	"time"

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
		execErr = e.executeRemote(ctx, req, wf, callback)
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

// executeRemote 远程执行
func (e *StreamExecutor) executeRemote(ctx context.Context, req *ExecuteRequest, wf *types.Workflow, callback *SSECallback) error {
	// TODO: 实现远程 Slave 执行
	// 这里需要调用 SlaveClient 发送执行请求
	// 并通过回调 URL 接收执行事件
	return fmt.Errorf("远程执行暂未实现")
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
