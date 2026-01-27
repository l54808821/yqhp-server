package handler

import (
	"bufio"
	"encoding/json"
	"strings"
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

// ExecuteRequest 统一执行请求
type ExecuteRequest struct {
	// 工作流定义（完整工作流）
	Workflow interface{} `json:"workflow,omitempty"`
	// 单步快捷方式：传入单个步骤，自动包装为工作流
	Step *StepConfig `json:"step,omitempty"`
	// 环境 ID
	EnvID int64 `json:"envId,omitempty"`
	// 变量
	Variables map[string]interface{} `json:"variables,omitempty"`
	// 执行模式：debug（失败即停止）或 normal（继续执行）
	Mode string `json:"mode,omitempty"`
	// 会话 ID
	SessionID string `json:"sessionId,omitempty"`
	// 选中的步骤 ID
	SelectedSteps []string `json:"selectedSteps,omitempty"`
	// 超时时间（秒）
	Timeout int `json:"timeout,omitempty"`
	// 执行器类型
	ExecutorType string `json:"executorType,omitempty"`
	// 指定的 Slave ID
	SlaveID string `json:"slaveId,omitempty"`
	// 是否使用 SSE 流式响应
	Stream bool `json:"stream,omitempty"`
	// 是否持久化执行记录
	Persist *bool `json:"persist,omitempty"`
}

// StepConfig 步骤配置（单步执行快捷方式）
type StepConfig struct {
	ID             string                 `json:"id"`
	Type           string                 `json:"type"`
	Name           string                 `json:"name"`
	Config         map[string]interface{} `json:"config"`
	PreProcessors  []ProcessorConfig      `json:"preProcessors,omitempty"`
	PostProcessors []ProcessorConfig      `json:"postProcessors,omitempty"`
}

// ProcessorConfig 处理器配置
type ProcessorConfig struct {
	ID      string                 `json:"id"`
	Type    string                 `json:"type"`
	Enabled bool                   `json:"enabled"`
	Name    string                 `json:"name,omitempty"`
	Config  map[string]interface{} `json:"config"`
}

// Execute 统一执行入口
// POST /api/execute
// 通过 stream 参数或 Accept 头判断响应方式：
// - stream=true 或 Accept: text/event-stream → SSE 流式响应
// - 否则 → 阻塞式 JSON 响应
func (h *StreamExecutionHandler) Execute(c *fiber.Ctx) error {
	var req ExecuteRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败: "+err.Error())
	}

	// 判断是否使用 SSE 流式响应
	isSSE := req.Stream || strings.Contains(c.Get("Accept"), "text/event-stream")

	// 处理 Step 快捷方式：将单个步骤包装为工作流
	var workflowDef interface{}
	if req.Step != nil {
		// 构建单步工作流
		workflowDef = map[string]interface{}{
			"steps": []interface{}{
				map[string]interface{}{
					"id":             req.Step.ID,
					"type":           req.Step.Type,
					"name":           req.Step.Name,
					"config":         req.Step.Config,
					"preProcessors":  req.Step.PreProcessors,
					"postProcessors": req.Step.PostProcessors,
				},
			},
		}
	} else if req.Workflow != nil {
		workflowDef = req.Workflow
	} else {
		return response.Error(c, "工作流定义不能为空（需要提供 workflow 或 step）")
	}

	// 流程执行时环境ID必填，单步调试时可选
	if req.Workflow != nil && req.EnvID <= 0 {
		return response.Error(c, "流程执行时环境ID不能为空")
	}

	// 默认调试模式
	mode := req.Mode
	if mode == "" {
		mode = "debug"
	}

	// 准备执行上下文（无 workflowID，直接使用定义）
	execCtx, err := h.prepareExecutionFromDefinition(c, workflowDef, req.EnvID, req.Variables, mode, shouldPersist(req.Persist), req.ExecutorType, req.SlaveID, req.Timeout, req.SessionID)
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

	if isSSE {
		return h.executeSSE(c, execCtx)
	}
	return h.executeBlocking(c, execCtx)
}

// prepareExecutionFromDefinition 从工作流定义准备执行上下文
func (h *StreamExecutionHandler) prepareExecutionFromDefinition(c *fiber.Ctx, definition interface{}, envID int64, variables map[string]interface{}, mode string, persist bool, executorType string, slaveID string, timeout int, sessionID string) (*ExecutionContext, error) {
	userID := middleware.GetCurrentUserID(c)

	// 生成会话 ID
	if sessionID == "" {
		sessionID = uuid.New().String()
	}

	// 确定执行模式
	execMode := scheduler.ModeDebug
	if mode == "normal" {
		execMode = scheduler.ModeExecute
	}

	// 转换工作流定义
	var definitionStr string
	switch v := definition.(type) {
	case string:
		definitionStr = v
	case map[string]interface{}:
		defBytes, err := json.Marshal(v)
		if err != nil {
			return nil, &executionError{code: "PARSE_ERROR", message: "工作流定义序列化失败: " + err.Error()}
		}
		definitionStr = string(defBytes)
	default:
		defBytes, err := json.Marshal(v)
		if err != nil {
			return nil, &executionError{code: "PARSE_ERROR", message: "工作流定义序列化失败: " + err.Error()}
		}
		definitionStr = string(defBytes)
	}

	// 转换工作流
	var engineWf *types.Workflow
	var err error
	if mode == "normal" {
		engineWf, err = logic.ConvertToEngineWorkflow(definitionStr, sessionID)
	} else {
		engineWf, err = logic.ConvertToEngineWorkflowStopOnError(definitionStr, sessionID)
	}
	if err != nil {
		return nil, &executionError{code: "CONVERT_ERROR", message: "工作流转换失败: " + err.Error()}
	}

	logger.Debug("工作流转换完成", "id", engineWf.ID, "name", engineWf.Name, "steps", len(engineWf.Steps))

	// 创建执行记录（仅当 persist=true 时）
	var execLogic *logic.ExecutionLogic
	if persist {
		execLogic = logic.NewExecutionLogic(c.UserContext())
		modeStr := string(model.ExecutionModeDebug)
		if mode == "normal" {
			modeStr = string(model.ExecutionModeExecute)
		}
		// 无 workflowID 时使用 0
		if err := execLogic.CreateStreamExecution(sessionID, 0, 0, envID, userID, modeStr); err != nil {
			return nil, &executionError{code: "DB_ERROR", message: "创建执行记录失败: " + err.Error()}
		}
	}

	// 创建执行请求
	execReq := &executor.ExecuteRequest{
		WorkflowID:   0,
		EnvID:        envID,
		Variables:    variables,
		Timeout:      timeout,
		ExecutorType: executor.ExecutorType(executorType),
		SlaveID:      slaveID,
	}

	return &ExecutionContext{
		WorkflowID:  0,
		SessionID:   sessionID,
		EnvID:       envID,
		UserID:      userID,
		EngineWf:    engineWf,
		Persist:     persist,
		ExecMode:    execMode,
		ExecLogic:   execLogic,
		ScheduleRes: nil,
		ExecReq:     execReq,
	}, nil
}

// executeSSE SSE 流式执行
func (h *StreamExecutionHandler) executeSSE(c *fiber.Ctx, execCtx *ExecutionContext) error {
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
				"sessionId": execCtx.SessionID,
				"message":   "SSE 连接成功",
				"persist":   execCtx.Persist,
			},
		})

		// 执行工作流
		execErr := h.streamExecutor.ExecuteStream(ctx, execCtx.ExecReq, execCtx.EngineWf, writer)

		// 更新执行记录状态
		h.updateExecutionStatus(execCtx, execErr, writer)
	})

	return nil
}

// executeBlocking 阻塞式执行
func (h *StreamExecutionHandler) executeBlocking(c *fiber.Ctx, execCtx *ExecutionContext) error {
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


// executionError 执行错误
type executionError struct {
	code    string
	message string
}

func (e *executionError) Error() string {
	return e.message
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
					"totalSteps":   total,
					"successSteps": success,
					"failedSteps":  failed,
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
			"sessionId":    sessionID,
			"status":       session.GetStatus(),
			"totalSteps":   total,
			"successSteps": success,
			"failedSteps":  failed,
			"startTime":    session.StartTime,
			"durationMs":   time.Since(session.StartTime).Milliseconds(),
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
