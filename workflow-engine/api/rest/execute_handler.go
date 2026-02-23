// Package rest provides the REST API server for the workflow execution engine.
package rest

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"yqhp/workflow-engine/pkg/logger"
	"yqhp/workflow-engine/pkg/types"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

// ExecuteSession 执行会话
type ExecuteSession struct {
	ID            string
	ExecutionID   string
	Status        string
	TotalSteps    int
	SuccessSteps  int
	FailedSteps   int
	StartTime     time.Time
	Cancel        context.CancelFunc
	InteractionCh chan *types.InteractionResponse
	StepResults   []types.StepExecutionResult // 步骤执行结果（阻塞模式收集）
	mu            sync.RWMutex
}

// ExecuteSessionManager 执行会话管理器
type ExecuteSessionManager struct {
	sessions map[string]*ExecuteSession
	mu       sync.RWMutex
}

var globalSessionManager = &ExecuteSessionManager{
	sessions: make(map[string]*ExecuteSession),
}

// setupExecuteRoutes 设置执行路由
func (s *Server) setupExecuteRoutes() {
	api := s.app.Group("/api/v1")

	// 统一执行接口（同时支持 SSE 和阻塞，通过 stream 参数或 Accept 头控制）
	api.Post("/execute", s.executeWorkflow)
	// 提交交互响应
	api.Post("/execute/:sessionId/interaction", s.submitExecuteInteraction)
	// 停止执行
	api.Delete("/execute/:sessionId", s.stopExecuteSession)
	// 获取执行状态
	api.Get("/execute/:sessionId/status", s.getExecuteSessionStatus)
}

// executeWorkflow 统一执行工作流入口
// POST /api/v1/execute
// 通过 stream 参数或 Accept 头判断响应方式：
// - stream=true 或 Accept: text/event-stream → SSE 流式响应
// - 否则 → 阻塞式 JSON 响应
func (s *Server) executeWorkflow(c *fiber.Ctx) error {
	var req types.ExecuteWorkflowRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{
			Error:   "invalid_request",
			Message: "解析请求体失败: " + err.Error(),
		})
	}

	// 判断是否使用 SSE 流式响应
	isSSE := req.Stream || strings.Contains(c.Get("Accept"), "text/event-stream")

	// 解析工作流
	wf, err := s.parseWorkflow(&req)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{
			Error:   "invalid_workflow",
			Message: err.Error(),
		})
	}

	// 生成会话 ID
	sessionID := req.SessionID
	if sessionID == "" {
		sessionID = uuid.New().String()
	}
	wf.ID = sessionID

	// 创建执行会话
	session := &ExecuteSession{
		ID:            sessionID,
		Status:        "running",
		StartTime:     time.Now(),
		InteractionCh: make(chan *types.InteractionResponse, 1),
	}
	globalSessionManager.mu.Lock()
	globalSessionManager.sessions[sessionID] = session
	globalSessionManager.mu.Unlock()

	defer func() {
		globalSessionManager.mu.Lock()
		delete(globalSessionManager.sessions, sessionID)
		globalSessionManager.mu.Unlock()
	}()

	// 过滤选中的步骤
	if len(req.SelectedSteps) > 0 {
		wf.Steps = filterSelectedSteps(wf.Steps, req.SelectedSteps)
	}

	// 合并变量
	if req.Variables != nil {
		if wf.Variables == nil {
			wf.Variables = make(map[string]interface{})
		}
		for k, v := range req.Variables {
			wf.Variables[k] = v
		}
	}

	// 设置超时
	timeout := 30 * time.Minute
	if req.Timeout > 0 {
		timeout = time.Duration(req.Timeout) * time.Second
	}

	ctx, cancel := context.WithTimeout(c.UserContext(), timeout)
	defer cancel()
	session.Cancel = cancel

	if isSSE {
		return s.executeWithSSE(c, ctx, &req, wf, session)
	}
	return s.executeWithBlocking(c, ctx, &req, wf, session)
}

// executeWithSSE SSE 流式执行
func (s *Server) executeWithSSE(c *fiber.Ctx, ctx context.Context, req *types.ExecuteWorkflowRequest, wf *types.Workflow, session *ExecuteSession) error {
	// 设置 SSE 响应头
	c.Set("Content-Type", "text/event-stream; charset=utf-8")
	c.Set("Cache-Control", "no-cache")
	c.Set("Connection", "keep-alive")
	c.Set("Transfer-Encoding", "chunked")
	c.Set("X-Accel-Buffering", "no")
	c.Set("X-Content-Type-Options", "nosniff")

	// 使用 StreamWriter 处理 SSE
	c.Context().SetBodyStreamWriter(func(w *bufio.Writer) {
		defer func() {
			if r := recover(); r != nil {
				logger.Error("SSE StreamWriter panic", "error", r)
			}
		}()

		writer := &sseWriter{w: w}

		// 发送连接成功事件
		writer.WriteEvent(string(types.EventTypeConnected), map[string]interface{}{
			"sessionId": session.ID,
			"message":   "SSE 连接成功",
		})

		// 根据执行器类型选择执行方式
		var execErr error
		if req.ExecutorType == "remote" && req.SlaveID != "" {
			execErr = s.executeRemoteStream(ctx, wf, req.SlaveID, session, writer)
		} else {
			execErr = s.executeLocalStream(ctx, wf, session, writer)
		}

		// 发送完成事件
		session.mu.RLock()
		status := "success"
		if execErr != nil {
			status = "failed"
		} else if session.FailedSteps > 0 {
			status = "failed"
		}
		completedEvent := &types.WorkflowCompletedEvent{
			Status:       status,
			TotalSteps:   session.TotalSteps,
			SuccessSteps: session.SuccessSteps,
			FailedSteps:  session.FailedSteps,
			DurationMs:   time.Since(session.StartTime).Milliseconds(),
		}
		session.mu.RUnlock()

		writer.WriteEvent(string(types.EventTypeWorkflowCompleted), completedEvent)

		if execErr != nil {
			writer.WriteEvent(string(types.EventTypeError), map[string]interface{}{
				"code":    "EXECUTION_ERROR",
				"message": execErr.Error(),
			})
		}
	})

	return nil
}

// executeWithBlocking 阻塞式执行
func (s *Server) executeWithBlocking(c *fiber.Ctx, ctx context.Context, req *types.ExecuteWorkflowRequest, wf *types.Workflow, session *ExecuteSession) error {
	// 根据执行器类型选择执行方式
	var execErr error
	if req.ExecutorType == "remote" && req.SlaveID != "" {
		execErr = s.executeRemoteBlocking(ctx, wf, req.SlaveID, session)
	} else {
		execErr = s.executeLocalBlocking(ctx, wf, session)
	}

	// 构建响应
	status := "success"
	if execErr != nil {
		status = "failed"
	} else if session.FailedSteps > 0 {
		status = "failed"
	}

	resp := &types.ExecuteWorkflowResponse{
		Success:     execErr == nil && session.FailedSteps == 0,
		ExecutionID: session.ExecutionID,
		SessionID:   session.ID,
		Summary: &types.ExecuteSummary{
			SessionID:     session.ID,
			TotalSteps:    session.TotalSteps,
			SuccessSteps:  session.SuccessSteps,
			FailedSteps:   session.FailedSteps,
			TotalDuration: time.Since(session.StartTime).Milliseconds(),
			Status:        status,
			Steps:         session.StepResults,
		},
	}

	if execErr != nil {
		resp.Error = execErr.Error()
	}

	return c.JSON(resp)
}

// submitExecuteInteraction 提交交互响应
// POST /api/v1/execute/:sessionId/interaction
func (s *Server) submitExecuteInteraction(c *fiber.Ctx) error {
	sessionID := c.Params("sessionId")
	if sessionID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{
			Error:   "invalid_request",
			Message: "Session ID is required",
		})
	}

	var req types.InteractionResponse
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{
			Error:   "invalid_request",
			Message: "Failed to parse request body: " + err.Error(),
		})
	}

	globalSessionManager.mu.RLock()
	session, ok := globalSessionManager.sessions[sessionID]
	globalSessionManager.mu.RUnlock()

	if !ok {
		return c.Status(fiber.StatusNotFound).JSON(ErrorResponse{
			Error:   "not_found",
			Message: "Session not found",
		})
	}

	// 发送交互响应到会话
	select {
	case session.InteractionCh <- &req:
		return c.JSON(map[string]interface{}{
			"success": true,
			"message": "Interaction submitted",
		})
	default:
		return c.Status(fiber.StatusConflict).JSON(ErrorResponse{
			Error:   "conflict",
			Message: "No pending interaction",
		})
	}
}

// stopExecuteSession 停止执行会话
// DELETE /api/v1/execute/:sessionId
func (s *Server) stopExecuteSession(c *fiber.Ctx) error {
	sessionID := c.Params("sessionId")
	if sessionID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{
			Error:   "invalid_request",
			Message: "Session ID is required",
		})
	}

	globalSessionManager.mu.RLock()
	session, ok := globalSessionManager.sessions[sessionID]
	globalSessionManager.mu.RUnlock()

	if !ok {
		return c.Status(fiber.StatusNotFound).JSON(ErrorResponse{
			Error:   "not_found",
			Message: "Session not found",
		})
	}

	if session.Cancel != nil {
		session.Cancel()
	}

	session.mu.Lock()
	session.Status = "stopped"
	session.mu.Unlock()

	return c.JSON(map[string]interface{}{
		"success": true,
		"message": "Execution stopped",
	})
}

// getExecuteSessionStatus 获取执行会话状态
// GET /api/v1/execute/:sessionId/status
func (s *Server) getExecuteSessionStatus(c *fiber.Ctx) error {
	sessionID := c.Params("sessionId")
	if sessionID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{
			Error:   "invalid_request",
			Message: "Session ID is required",
		})
	}

	globalSessionManager.mu.RLock()
	session, ok := globalSessionManager.sessions[sessionID]
	globalSessionManager.mu.RUnlock()

	if !ok {
		return c.Status(fiber.StatusNotFound).JSON(ErrorResponse{
			Error:   "not_found",
			Message: "Session not found",
		})
	}

	session.mu.RLock()
	defer session.mu.RUnlock()

	return c.JSON(map[string]interface{}{
		"sessionId":    session.ID,
		"executionId":  session.ExecutionID,
		"status":       session.Status,
		"totalSteps":   session.TotalSteps,
		"successSteps": session.SuccessSteps,
		"failedSteps":  session.FailedSteps,
		"startTime":    session.StartTime,
		"durationMs":   time.Since(session.StartTime).Milliseconds(),
	})
}

// parseWorkflow 解析工作流
func (s *Server) parseWorkflow(req *types.ExecuteWorkflowRequest) (*types.Workflow, error) {
	// 优先使用 Workflow
	if req.Workflow != nil {
		return req.Workflow, nil
	}

	// 其次使用 WorkflowJSON
	if req.WorkflowJSON != "" {
		var wf types.Workflow
		if err := json.Unmarshal([]byte(req.WorkflowJSON), &wf); err != nil {
			return nil, fmt.Errorf("解析工作流 JSON 失败: %w", err)
		}
		return &wf, nil
	}

	// 最后处理 Step 快捷方式：将单个步骤包装为工作流
	if req.Step != nil {
		wf := &types.Workflow{
			ID:   uuid.New().String(),
			Name: "单步执行",
			Steps: []types.Step{
				*req.Step,
			},
		}
		return wf, nil
	}

	return nil, fmt.Errorf("工作流定义不能为空（需要提供 workflow、workflowJson 或 step）")
}

// executeLocalStream 本地流式执行
func (s *Server) executeLocalStream(ctx context.Context, wf *types.Workflow, session *ExecuteSession, writer *sseWriter) error {
	logger.Debug("executeLocalStream 开始", "workflow_id", wf.ID, "steps", len(wf.Steps))

	if s.master == nil {
		return fmt.Errorf("Master 未初始化")
	}

	// 创建回调
	callback := &streamCallback{
		writer:  writer,
		session: session,
	}
	wf.Callback = callback

	// 提交执行
	execID, err := s.master.SubmitWorkflow(ctx, wf)
	if err != nil {
		return fmt.Errorf("提交执行失败: %w", err)
	}

	session.mu.Lock()
	session.ExecutionID = execID
	session.mu.Unlock()

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
				writer.WriteEvent(string(types.EventTypeHeartbeat), map[string]interface{}{
					"timestamp": time.Now().Unix(),
				})
			}
		}
	}()
	defer close(heartbeatDone)

	// 等待执行完成
	return s.waitForExecutionCompletion(ctx, execID, session)
}

// executeLocalBlocking 本地阻塞式执行
func (s *Server) executeLocalBlocking(ctx context.Context, wf *types.Workflow, session *ExecuteSession) error {
	logger.Debug("executeLocalBlocking 开始", "workflow_id", wf.ID, "steps", len(wf.Steps))

	if s.master == nil {
		return fmt.Errorf("Master 未初始化")
	}

	// 创建回调（记录统计信息）
	callback := &blockingCallback{session: session}
	wf.Callback = callback

	// 提交执行
	execID, err := s.master.SubmitWorkflow(ctx, wf)
	if err != nil {
		return fmt.Errorf("提交执行失败: %w", err)
	}

	session.mu.Lock()
	session.ExecutionID = execID
	session.mu.Unlock()

	// 等待执行完成
	return s.waitForExecutionCompletion(ctx, execID, session)
}

// executeRemoteStream 远程流式执行
func (s *Server) executeRemoteStream(ctx context.Context, wf *types.Workflow, slaveID string, session *ExecuteSession, writer *sseWriter) error {
	logger.Debug("executeRemoteStream 开始", "workflow_id", wf.ID, "slave_id", slaveID)

	// 获取 Slave 信息
	_, err := s.registry.GetSlave(ctx, slaveID)
	if err != nil {
		return fmt.Errorf("获取 Slave 失败: %w", err)
	}

	slaveStatus, err := s.registry.GetSlaveStatus(ctx, slaveID)
	if err != nil || slaveStatus.State != types.SlaveStateOnline {
		state := "unknown"
		if slaveStatus != nil {
			state = string(slaveStatus.State)
		}
		return fmt.Errorf("Slave 不可用: %s (状态: %s)", slaveID, state)
	}

	// 创建任务分配
	task := &TaskAssignment{
		TaskID:      uuid.New().String(),
		ExecutionID: wf.ID,
		Workflow:    wf,
		Segment: &ExecutionSegment{
			Start: 0,
			End:   1,
		},
	}

	// 发送任务到 Slave
	s.taskQueuesMu.Lock()
	queue, ok := s.taskQueues[slaveID]
	if !ok {
		queue = make(chan *TaskAssignment, 100)
		s.taskQueues[slaveID] = queue
	}
	s.taskQueuesMu.Unlock()

	select {
	case queue <- task:
		logger.Debug("任务已发送到 Slave", "slave_id", slaveID, "task_id", task.TaskID)
	case <-ctx.Done():
		return ctx.Err()
	default:
		return fmt.Errorf("Slave 任务队列已满: %s", slaveID)
	}

	// 等待执行完成（通过轮询状态或 WebSocket）
	return s.waitForRemoteCompletion(ctx, task.TaskID, session, writer)
}

// executeRemoteBlocking 远程阻塞式执行
func (s *Server) executeRemoteBlocking(ctx context.Context, wf *types.Workflow, slaveID string, session *ExecuteSession) error {
	logger.Debug("executeRemoteBlocking 开始", "workflow_id", wf.ID, "slave_id", slaveID)

	// 获取 Slave 信息
	_, err := s.registry.GetSlave(ctx, slaveID)
	if err != nil {
		return fmt.Errorf("获取 Slave 失败: %w", err)
	}

	slaveStatus, err := s.registry.GetSlaveStatus(ctx, slaveID)
	if err != nil || slaveStatus.State != types.SlaveStateOnline {
		state := "unknown"
		if slaveStatus != nil {
			state = string(slaveStatus.State)
		}
		return fmt.Errorf("Slave 不可用: %s (状态: %s)", slaveID, state)
	}

	// 创建任务分配
	task := &TaskAssignment{
		TaskID:      uuid.New().String(),
		ExecutionID: wf.ID,
		Workflow:    wf,
		Segment: &ExecutionSegment{
			Start: 0,
			End:   1,
		},
	}

	// 发送任务到 Slave
	s.taskQueuesMu.Lock()
	queue, ok := s.taskQueues[slaveID]
	if !ok {
		queue = make(chan *TaskAssignment, 100)
		s.taskQueues[slaveID] = queue
	}
	s.taskQueuesMu.Unlock()

	select {
	case queue <- task:
		logger.Debug("任务已发送到 Slave", "slave_id", slaveID, "task_id", task.TaskID)
	case <-ctx.Done():
		return ctx.Err()
	default:
		return fmt.Errorf("Slave 任务队列已满: %s", slaveID)
	}

	// 等待执行完成
	return s.waitForRemoteBlockingCompletion(ctx, task.TaskID, session)
}

// waitForExecutionCompletion 等待执行完成
func (s *Server) waitForExecutionCompletion(ctx context.Context, execID string, session *ExecuteSession) error {
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			session.mu.RLock()
			status := session.Status
			session.mu.RUnlock()

			if status == "stopped" {
				s.master.StopExecution(ctx, execID)
				return fmt.Errorf("执行被停止")
			}

			state, err := s.master.GetExecutionStatus(ctx, execID)
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

// waitForRemoteCompletion 等待远程执行完成（流式）
func (s *Server) waitForRemoteCompletion(ctx context.Context, taskID string, session *ExecuteSession, writer *sseWriter) error {
	// 简化实现：通过轮询等待
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			session.mu.RLock()
			status := session.Status
			session.mu.RUnlock()

			if status == "completed" || status == "failed" || status == "stopped" {
				return nil
			}

			// 发送心跳
			writer.WriteEvent(string(types.EventTypeHeartbeat), map[string]interface{}{
				"timestamp": time.Now().Unix(),
			})
		}
	}
}

// waitForRemoteBlockingCompletion 等待远程执行完成（阻塞）
func (s *Server) waitForRemoteBlockingCompletion(ctx context.Context, taskID string, session *ExecuteSession) error {
	// 简化实现：通过轮询等待
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			session.mu.RLock()
			status := session.Status
			session.mu.RUnlock()

			if status == "completed" {
				return nil
			}
			if status == "failed" {
				return fmt.Errorf("执行失败")
			}
			if status == "stopped" {
				return fmt.Errorf("执行被停止")
			}
		}
	}
}

// filterSelectedSteps 过滤选中的步骤
func filterSelectedSteps(steps []types.Step, selectedIDs []string) []types.Step {
	if len(selectedIDs) == 0 {
		return steps
	}

	idSet := make(map[string]bool)
	for _, id := range selectedIDs {
		idSet[id] = true
	}

	var filtered []types.Step
	for _, step := range steps {
		if idSet[step.ID] {
			filtered = append(filtered, step)
		} else {
			// 递归过滤子步骤
			if step.Loop != nil && len(step.Loop.Steps) > 0 {
				filteredChildren := filterSelectedSteps(step.Loop.Steps, selectedIDs)
				if len(filteredChildren) > 0 {
					newStep := step
					newStep.Loop.Steps = filteredChildren
					filtered = append(filtered, newStep)
				}
			}
			if len(step.Children) > 0 {
				filteredChildren := filterSelectedSteps(step.Children, selectedIDs)
				if len(filteredChildren) > 0 {
					newStep := step
					newStep.Children = filteredChildren
					filtered = append(filtered, newStep)
				}
			}
		}
	}

	return filtered
}

// sseWriter SSE 写入器
type sseWriter struct {
	w  *bufio.Writer
	mu sync.Mutex
}

func (w *sseWriter) WriteEvent(eventType string, data interface{}) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	dataBytes, err := json.Marshal(data)
	if err != nil {
		return err
	}

	fmt.Fprintf(w.w, "event: %s\n", eventType)
	fmt.Fprintf(w.w, "data: %s\n\n", string(dataBytes))
	return w.w.Flush()
}

// streamCallback 流式回调（实现 types.ExecutionCallback 接口）
type streamCallback struct {
	writer  *sseWriter
	session *ExecuteSession
}

func (c *streamCallback) OnStepStart(ctx context.Context, step *types.Step, parentID string, iteration int) {
	c.session.mu.Lock()
	c.session.TotalSteps++
	c.session.mu.Unlock()

	c.writer.WriteEvent(string(types.EventTypeStepStarted), &types.StepStartedEvent{
		StepID:   step.ID,
		StepName: step.Name,
		StepType: step.Type,
	})
}

func (c *streamCallback) OnStepComplete(ctx context.Context, step *types.Step, result *types.StepResult, parentID string, iteration int) {
	isSuccess := result.Status == types.ResultStatusSuccess

	c.session.mu.Lock()
	if isSuccess {
		c.session.SuccessSteps++
	} else {
		c.session.FailedSteps++
	}
	c.session.mu.Unlock()

	c.writer.WriteEvent(string(types.EventTypeStepCompleted), &types.StepCompletedEvent{
		StepID:     step.ID,
		StepName:   step.Name,
		Success:    isSuccess,
		DurationMs: result.Duration.Milliseconds(),
		Result:     result.Output, // 失败时也有 Output（如 HTTPResponseData 含错误信息）
	})
}

func (c *streamCallback) OnStepFailed(ctx context.Context, step *types.Step, err error, duration time.Duration, parentID string, iteration int) {
	// 结果已在 OnStepComplete 中处理，这里不需要重复
}

func (c *streamCallback) OnStepSkipped(ctx context.Context, step *types.Step, reason string, parentID string, iteration int) {
	// 跳过的步骤不影响统计
}

func (c *streamCallback) OnProgress(ctx context.Context, current, total int, stepName string) {
	// 流式模式已通过事件推送进度
}

func (c *streamCallback) OnExecutionComplete(ctx context.Context, summary *types.ExecutionSummary) {
	// 由外部处理
}

// blockingCallback 阻塞式回调（实现 types.ExecutionCallback 接口）
type blockingCallback struct {
	session *ExecuteSession
}

func (c *blockingCallback) OnStepStart(ctx context.Context, step *types.Step, parentID string, iteration int) {
	c.session.mu.Lock()
	c.session.TotalSteps++
	c.session.mu.Unlock()
}

func (c *blockingCallback) OnStepComplete(ctx context.Context, step *types.Step, result *types.StepResult, parentID string, iteration int) {
	c.session.mu.Lock()

	isSuccess := result.Status == types.ResultStatusSuccess
	if isSuccess {
		c.session.SuccessSteps++
	} else {
		c.session.FailedSteps++
	}

	// 收集步骤执行结果（不管成功还是失败都收集，含 Output 和 Error）
	stepResult := types.StepExecutionResult{
		StepID:     step.ID,
		StepName:   step.Name,
		StepType:   step.Type,
		Success:    isSuccess,
		DurationMs: result.Duration.Milliseconds(),
		Result:     result.Output, // 失败时也有 Output（如 HTTPResponseData 含错误信息和请求上下文）
	}
	if result.Error != nil {
		stepResult.Error = result.Error.Error()
	}
	c.session.StepResults = append(c.session.StepResults, stepResult)

	c.session.mu.Unlock()
}

func (c *blockingCallback) OnStepFailed(ctx context.Context, step *types.Step, err error, duration time.Duration, parentID string, iteration int) {
	// 结果已在 OnStepComplete 中收集，这里不需要重复处理
	// OnStepFailed 仅作为额外的失败通知，可用于日志或报警等场景
}

func (c *blockingCallback) OnStepSkipped(ctx context.Context, step *types.Step, reason string, parentID string, iteration int) {
	// 跳过的步骤不影响统计
}

func (c *blockingCallback) OnProgress(ctx context.Context, current, total int, stepName string) {
	// 阻塞模式不需要进度
}

func (c *blockingCallback) OnExecutionComplete(ctx context.Context, summary *types.ExecutionSummary) {
	c.session.mu.Lock()
	c.session.Status = "completed"
	c.session.mu.Unlock()
}
