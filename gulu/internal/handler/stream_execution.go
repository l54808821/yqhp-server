package handler

import (
	"bufio"
	"encoding/json"
	"strconv"
	"time"

	"yqhp/common/response"
	"yqhp/gulu/internal/executor"
	"yqhp/gulu/internal/logic"
	"yqhp/gulu/internal/middleware"
	"yqhp/gulu/internal/model"
	"yqhp/gulu/internal/scheduler"
	"yqhp/gulu/internal/sse"
	"yqhp/workflow-engine/pkg/logger"
	"yqhp/workflow-engine/pkg/types"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/valyala/fasthttp"
)

// StreamExecutionHandler 流式执行处理器
type StreamExecutionHandler struct {
	scheduler      scheduler.Scheduler
	streamExecutor *executor.StreamExecutor
	sessionManager *executor.SessionManager
}

// NewStreamExecutionHandler 创建流式执行处理器
func NewStreamExecutionHandler(sched scheduler.Scheduler, streamExec *executor.StreamExecutor, sessionMgr *executor.SessionManager) *StreamExecutionHandler {
	return &StreamExecutionHandler{
		scheduler:      sched,
		streamExecutor: streamExec,
		sessionManager: sessionMgr,
	}
}

// RunStreamRequest 流式执行请求
type RunStreamRequest struct {
	EnvID         int64                  `json:"env_id" query:"env_id"`
	Variables     map[string]interface{} `json:"variables,omitempty" query:"variables"`
	Timeout       int                    `json:"timeout,omitempty" query:"timeout"`
	ExecutorType  string                 `json:"executor_type,omitempty" query:"executor_type"`
	SlaveID       string                 `json:"slave_id,omitempty" query:"slave_id"`
	Definition    interface{}            `json:"definition,omitempty" query:"definition"`
	SelectedSteps []string               `json:"selected_steps,omitempty" query:"selected_steps"`
	Persist       *bool                  `json:"persist,omitempty" query:"persist"`
	Mode          string                 `json:"mode,omitempty" query:"mode"` // debug 或 normal
}

// RunBlockingRequest 阻塞式执行请求
type RunBlockingRequest struct {
	EnvID        int64                  `json:"env_id"`
	Variables    map[string]interface{} `json:"variables,omitempty"`
	Timeout      int                    `json:"timeout,omitempty"`
	ExecutorType string                 `json:"executor_type,omitempty"`
	SlaveID      string                 `json:"slave_id,omitempty"`
	Persist      *bool                  `json:"persist,omitempty"`
}

// InteractionRequest 交互请求
type InteractionRequest struct {
	Value   string `json:"value"`
	Skipped bool   `json:"skipped"`
}

// ExecutionContext 执行上下文（公共逻辑抽取）
type ExecutionContext struct {
	WorkflowID   int64
	SessionID    string
	EnvID        int64
	UserID       int64
	EngineWf     *types.Workflow
	Persist      bool
	ExecMode     scheduler.ExecutionMode
	ExecLogic    *logic.ExecutionLogic
	ScheduleRes  *scheduler.ScheduleResult
	ExecReq      *executor.ExecuteRequest
}

// shouldPersist 判断是否需要持久化
func shouldPersist(persist *bool) bool {
	if persist == nil {
		return true
	}
	return *persist
}

// prepareExecution 准备执行上下文（公共逻辑）
func (h *StreamExecutionHandler) prepareExecution(c *fiber.Ctx, workflowID int64, envID int64, variables map[string]interface{}, definition interface{}, mode string, persist bool, executorType string, slaveID string, timeout int) (*ExecutionContext, error) {
	userID := middleware.GetCurrentUserID(c)

	// 获取工作流信息
	workflowLogic := logic.NewWorkflowLogic(c.UserContext())
	wf, err := workflowLogic.GetByID(workflowID)
	if err != nil {
		return nil, &executionError{code: "NOT_FOUND", message: "工作流不存在"}
	}

	// 生成会话 ID
	sessionID := uuid.New().String()

	// 获取工作流类型
	workflowType := string(model.WorkflowTypeNormal)
	if wf.WorkflowType != nil {
		workflowType = *wf.WorkflowType
	}

	// 确定执行模式
	execMode := scheduler.ModeDebug
	if mode == "normal" {
		execMode = scheduler.ModeExecute
	}

	// 创建调度请求并调度
	schedReq := &scheduler.ScheduleRequest{
		WorkflowID:   workflowID,
		WorkflowType: workflowType,
		Mode:         execMode,
		EnvID:        envID,
		SessionID:    sessionID,
		UserID:       userID,
		ExecutorID:   slaveID,
	}

	schedRes, err := h.scheduler.Schedule(c.UserContext(), schedReq)
	if err != nil {
		return nil, &executionError{code: "SCHEDULE_ERROR", message: "调度失败: " + err.Error()}
	}

	logger.Debug("调度完成", "target_type", schedRes.TargetType, "target_id", schedRes.TargetID)

	// 创建执行记录（仅当 persist=true 时）
	var execLogic *logic.ExecutionLogic
	if persist {
		execLogic = logic.NewExecutionLogic(c.UserContext())
		modeStr := string(model.ExecutionModeDebug)
		if mode == "normal" {
			modeStr = string(model.ExecutionModeExecute)
		}
		if err := execLogic.CreateStreamExecution(sessionID, wf.ProjectID, workflowID, envID, userID, modeStr); err != nil {
			return nil, &executionError{code: "DB_ERROR", message: "创建执行记录失败: " + err.Error()}
		}
	}

	// 确定工作流定义
	definitionToUse := wf.Definition
	if definition != nil {
		switch v := definition.(type) {
		case string:
			if v != "" {
				definitionToUse = v
				logger.Debug("使用请求中的工作流定义（字符串格式）")
			}
		case map[string]interface{}:
			defBytes, err := json.Marshal(v)
			if err != nil {
				return nil, &executionError{code: "PARSE_ERROR", message: "工作流定义序列化失败: " + err.Error()}
			}
			definitionToUse = string(defBytes)
			logger.Debug("使用请求中的工作流定义（对象格式）")
		}
	}

	// 转换工作流定义
	var engineWf *types.Workflow
	if mode == "normal" {
		engineWf, err = logic.ConvertToEngineWorkflow(definitionToUse, sessionID)
	} else {
		engineWf, err = logic.ConvertToEngineWorkflowStopOnError(definitionToUse, sessionID)
	}
	if err != nil {
		return nil, &executionError{code: "CONVERT_ERROR", message: "工作流转换失败: " + err.Error()}
	}

	logger.Debug("工作流转换完成", "id", engineWf.ID, "name", engineWf.Name, "steps", len(engineWf.Steps))

	// 创建执行请求
	execReq := &executor.ExecuteRequest{
		WorkflowID:   workflowID,
		EnvID:        envID,
		Variables:    variables,
		Timeout:      timeout,
		ExecutorType: executor.ExecutorType(executorType),
		SlaveID:      slaveID,
	}

	// 根据调度结果设置执行类型
	if schedRes.TargetType == "slave" && slaveID == "" {
		execReq.SlaveID = schedRes.TargetID
		execReq.ExecutorType = executor.ExecutorTypeRemote
	}

	return &ExecutionContext{
		WorkflowID:  workflowID,
		SessionID:   sessionID,
		EnvID:       envID,
		UserID:      userID,
		EngineWf:    engineWf,
		Persist:     persist,
		ExecMode:    execMode,
		ExecLogic:   execLogic,
		ScheduleRes: schedRes,
		ExecReq:     execReq,
	}, nil
}

// executionError 执行错误
type executionError struct {
	code    string
	message string
}

func (e *executionError) Error() string {
	return e.message
}

// RunStream 流式执行（SSE）
// GET/POST /api/workflows/:id/run/stream
func (h *StreamExecutionHandler) RunStream(c *fiber.Ctx) error {
	workflowID, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "无效的工作流ID")
	}

	var req RunStreamRequest
	// 优先解析 JSON body（POST），否则解析 query（GET）
	if err := c.BodyParser(&req); err != nil {
		if err := c.QueryParser(&req); err != nil {
			return response.Error(c, "参数解析失败: "+err.Error())
		}
	}

	if req.EnvID <= 0 {
		return response.Error(c, "环境ID不能为空")
	}

	// 准备执行上下文
	execCtx, err := h.prepareExecution(c, workflowID, req.EnvID, req.Variables, req.Definition, req.Mode, shouldPersist(req.Persist), req.ExecutorType, req.SlaveID, req.Timeout)
	if err != nil {
		if execErr, ok := err.(*executionError); ok {
			if execErr.code == "NOT_FOUND" {
				return response.NotFound(c, execErr.message)
			}
		}
		return response.Error(c, err.Error())
	}

	// 过滤选中的步骤
	if len(req.SelectedSteps) > 0 {
		logger.Debug("过滤选中的步骤", "count", len(req.SelectedSteps))
		execCtx.EngineWf.Steps = filterSteps(execCtx.EngineWf.Steps, req.SelectedSteps)
	}

	// 设置 SSE 响应头
	c.Set("Content-Type", "text/event-stream; charset=utf-8")
	c.Set("Cache-Control", "no-cache")
	c.Set("Connection", "keep-alive")
	c.Set("Transfer-Encoding", "chunked")
	c.Set("X-Accel-Buffering", "no")
	c.Set("X-Content-Type-Options", "nosniff")

	// 捕获上下文
	ctx := c.UserContext()

	// 使用 StreamWriter 处理 SSE
	c.Context().SetBodyStreamWriter(func(w *bufio.Writer) {
		defer func() {
			if r := recover(); r != nil {
				logger.Error("SSE StreamWriter panic", "error", r)
			}
		}()

		writer := sse.NewWriter(&flushWriter{w: w}, execCtx.SessionID)

		// 发送连接成功事件
		writer.WriteEvent(&sse.Event{
			Type: "connected",
			Data: map[string]interface{}{
				"session_id": execCtx.SessionID,
				"message":    "SSE 连接成功",
				"persist":    execCtx.Persist,
			},
		})

		// 执行工作流
		execErr := h.streamExecutor.ExecuteStream(ctx, execCtx.ExecReq, execCtx.EngineWf, writer)

		// 更新执行记录状态
		h.updateExecutionStatus(execCtx, execErr, writer)
	})

	return nil
}

// RunBlocking 阻塞式执行
// POST /api/workflows/:id/run
func (h *StreamExecutionHandler) RunBlocking(c *fiber.Ctx) error {
	workflowID, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "无效的工作流ID")
	}

	var req RunBlockingRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败: "+err.Error())
	}

	if req.EnvID <= 0 {
		return response.Error(c, "环境ID不能为空")
	}

	// 准备执行上下文
	execCtx, err := h.prepareExecution(c, workflowID, req.EnvID, req.Variables, nil, "normal", shouldPersist(req.Persist), req.ExecutorType, req.SlaveID, req.Timeout)
	if err != nil {
		if execErr, ok := err.(*executionError); ok {
			if execErr.code == "NOT_FOUND" {
				return response.NotFound(c, execErr.message)
			}
		}
		return response.Error(c, err.Error())
	}

	// 执行工作流（阻塞）
	summary, execErr := h.streamExecutor.ExecuteBlocking(c.UserContext(), execCtx.ExecReq, execCtx.EngineWf)

	// 更新执行记录状态
	if execCtx.Persist && execCtx.ExecLogic != nil {
		if execErr != nil {
			execCtx.ExecLogic.UpdateStreamExecutionStatus(execCtx.SessionID, string(model.ExecutionStatusFailed), nil)
			return response.Error(c, "执行失败: "+execErr.Error())
		}
		execCtx.ExecLogic.UpdateStreamExecutionStatus(execCtx.SessionID, summary.Status, summary)
	} else if execErr != nil {
		return response.Error(c, "执行失败: "+execErr.Error())
	}

	return response.Success(c, summary)
}

// updateExecutionStatus 更新执行状态（SSE 模式）
func (h *StreamExecutionHandler) updateExecutionStatus(execCtx *ExecutionContext, execErr error, writer *sse.Writer) {
	if execCtx.Persist && execCtx.ExecLogic != nil {
		if execErr != nil {
			writer.WriteErrorCode(sse.ErrExecutorError, "执行失败", execErr.Error())
			execCtx.ExecLogic.UpdateStreamExecutionStatus(execCtx.SessionID, string(model.ExecutionStatusFailed), nil)
		} else {
			session, ok := h.sessionManager.GetSession(execCtx.SessionID)
			if ok {
				total, success, failed := session.GetStats()
				status := "success"
				if failed > 0 {
					status = "failed"
				}
				execCtx.ExecLogic.UpdateStreamExecutionStatus(execCtx.SessionID, status, map[string]interface{}{
					"total_steps":   total,
					"success_steps": success,
					"failed_steps":  failed,
				})
			}
		}
	} else if execErr != nil {
		writer.WriteErrorCode(sse.ErrExecutorError, "执行失败", execErr.Error())
	}
}

// StopExecution 停止执行
// DELETE /api/executions/:sessionId/stop
func (h *StreamExecutionHandler) StopExecution(c *fiber.Ctx) error {
	sessionID := c.Params("sessionId")
	if sessionID == "" {
		return response.Error(c, "会话ID不能为空")
	}

	if err := h.streamExecutor.Stop(sessionID); err != nil {
		return response.Error(c, "停止执行失败: "+err.Error())
	}

	executionLogic := logic.NewExecutionLogic(c.UserContext())
	executionLogic.UpdateStreamExecutionStatus(sessionID, string(model.ExecutionStatusStopped), nil)

	return response.Success(c, nil)
}

// SubmitInteraction 提交交互响应
// POST /api/executions/:sessionId/interaction
func (h *StreamExecutionHandler) SubmitInteraction(c *fiber.Ctx) error {
	sessionID := c.Params("sessionId")
	if sessionID == "" {
		return response.Error(c, "会话ID不能为空")
	}

	var req InteractionRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败: "+err.Error())
	}

	resp := &executor.InteractionResponse{
		Value:   req.Value,
		Skipped: req.Skipped,
	}

	if err := h.sessionManager.SubmitInteraction(sessionID, resp); err != nil {
		return response.Error(c, "提交交互响应失败: "+err.Error())
	}

	return response.Success(c, nil)
}

// GetExecutionStatus 获取执行状态
// GET /api/executions/:sessionId/status
func (h *StreamExecutionHandler) GetExecutionStatus(c *fiber.Ctx) error {
	sessionID := c.Params("sessionId")
	if sessionID == "" {
		return response.Error(c, "会话ID不能为空")
	}

	// 先检查内存中的会话
	session, ok := h.sessionManager.GetSession(sessionID)
	if ok {
		total, success, failed := session.GetStats()
		return response.Success(c, map[string]interface{}{
			"session_id":    sessionID,
			"status":        session.GetStatus(),
			"total_steps":   total,
			"success_steps": success,
			"failed_steps":  failed,
			"start_time":    session.StartTime,
			"duration_ms":   time.Since(session.StartTime).Milliseconds(),
		})
	}

	// 从数据库获取
	executionLogic := logic.NewExecutionLogic(c.UserContext())
	dbSession, err := executionLogic.GetStreamExecution(sessionID)
	if err != nil {
		return response.NotFound(c, "执行记录不存在")
	}

	return response.Success(c, dbSession)
}

// flushWriter 包装 bufio.Writer 以支持 Flush
type flushWriter struct {
	w *bufio.Writer
}

func (fw *flushWriter) Write(p []byte) (n int, err error) {
	n, err = fw.w.Write(p)
	if err != nil {
		return
	}
	err = fw.w.Flush()
	return
}

func (fw *flushWriter) Flush() {
	fw.w.Flush()
}

// fasthttpFlushWriter 用于 fasthttp 的 flush writer
type fasthttpFlushWriter struct {
	ctx *fasthttp.RequestCtx
}

func (fw *fasthttpFlushWriter) Write(p []byte) (n int, err error) {
	n, err = fw.ctx.Write(p)
	if err != nil {
		return
	}
	fw.ctx.Response.ImmediateHeaderFlush = true
	return
}

func (fw *fasthttpFlushWriter) Flush() {}

// filterSteps 过滤选中的步骤
func filterSteps(steps []types.Step, selectedIDs []string) []types.Step {
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
			// 对于循环步骤，递归过滤子步骤
			if step.Loop != nil && len(step.Loop.Steps) > 0 {
				filteredChildren := filterSteps(step.Loop.Steps, selectedIDs)
				if len(filteredChildren) > 0 {
					newStep := step
					newStep.Loop = &types.Loop{
						Mode:              step.Loop.Mode,
						Count:             step.Loop.Count,
						Items:             step.Loop.Items,
						ItemVar:           step.Loop.ItemVar,
						Condition:         step.Loop.Condition,
						MaxIterations:     step.Loop.MaxIterations,
						BreakCondition:    step.Loop.BreakCondition,
						ContinueCondition: step.Loop.ContinueCondition,
						Steps:             filteredChildren,
					}
					filtered = append(filtered, newStep)
				}
			}
			// 对于条件步骤，递归过滤 children
			if len(step.Children) > 0 {
				filteredChildren := filterSteps(step.Children, selectedIDs)
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
