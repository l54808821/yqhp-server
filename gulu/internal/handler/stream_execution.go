package handler

import (
	"bufio"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"yqhp/common/response"
	"yqhp/gulu/internal/executor"
	"yqhp/gulu/internal/logic"
	"yqhp/gulu/internal/middleware"
	"yqhp/gulu/internal/model"
	"yqhp/gulu/internal/scheduler"
	"yqhp/gulu/internal/sse"
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
	Definition    interface{}            `json:"definition,omitempty" query:"definition"`         // 工作流定义（用于执行未保存的工作流），可以是字符串或对象
	SelectedSteps []string               `json:"selected_steps,omitempty" query:"selected_steps"` // 选中的步骤 ID（用于选择性执行）
	Persist       *bool                  `json:"persist,omitempty" query:"persist"`               // 是否持久化执行记录，默认 true
	Mode          string                 `json:"mode,omitempty" query:"mode"`                     // 执行模式：debug（失败即停止）或 normal（继续执行）
}

// RunBlockingRequest 阻塞式执行请求
type RunBlockingRequest struct {
	EnvID        int64                  `json:"env_id"`
	Variables    map[string]interface{} `json:"variables,omitempty"`
	Timeout      int                    `json:"timeout,omitempty"`
	ExecutorType string                 `json:"executor_type,omitempty"`
	SlaveID      string                 `json:"slave_id,omitempty"`
	Persist      *bool                  `json:"persist,omitempty"` // 是否持久化执行记录，默认 true
}

// InteractionRequest 交互请求
type InteractionRequest struct {
	Value   string `json:"value"`
	Skipped bool   `json:"skipped"`
}

// shouldPersist 判断是否需要持久化
func shouldPersist(persist *bool) bool {
	if persist == nil {
		return true // 默认持久化
	}
	return *persist
}

// RunStream 流式执行（SSE）
// POST /api/workflows/:id/run/stream
func (h *StreamExecutionHandler) RunStream(c *fiber.Ctx) error {
	workflowID, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "无效的工作流ID")
	}

	var req RunStreamRequest
	// 优先尝试解析 JSON body（POST 方式）
	if err := c.BodyParser(&req); err != nil {
		// 如果 body 解析失败，尝试解析 query 参数（GET 方式，向后兼容）
		if err := c.QueryParser(&req); err != nil {
			return response.Error(c, "参数解析失败: "+err.Error())
		}
	}

	if req.EnvID <= 0 {
		return response.Error(c, "环境ID不能为空")
	}

	userID := middleware.GetCurrentUserID(c)
	persist := shouldPersist(req.Persist)

	// 获取工作流信息
	workflowLogic := logic.NewWorkflowLogic(c.UserContext())
	wf, err := workflowLogic.GetByID(workflowID)
	if err != nil {
		return response.NotFound(c, "工作流不存在")
	}

	// 生成会话ID
	sessionID := uuid.New().String()

	// 获取工作流类型
	workflowType := string(model.WorkflowTypeNormal)
	if wf.WorkflowType != nil {
		workflowType = *wf.WorkflowType
	}

	// 确定执行模式
	execMode := scheduler.ModeDebug
	if req.Mode == "normal" {
		execMode = scheduler.ModeExecute
	}

	// 创建调度请求
	schedReq := &scheduler.ScheduleRequest{
		WorkflowID:   workflowID,
		WorkflowType: workflowType,
		Mode:         execMode,
		EnvID:        req.EnvID,
		SessionID:    sessionID,
		UserID:       userID,
	}

	// 调度到 Master
	_, err = h.scheduler.Schedule(c.UserContext(), schedReq)
	if err != nil {
		return response.Error(c, "调度失败: "+err.Error())
	}

	// 创建执行记录（仅当 persist=true 时）
	var executionLogic *logic.ExecutionLogic
	if persist {
		executionLogic = logic.NewExecutionLogic(c.UserContext())
		mode := string(model.ExecutionModeDebug)
		if req.Mode == "normal" {
			mode = string(model.ExecutionModeExecute)
		}
		if err := executionLogic.CreateStreamExecution(sessionID, wf.ProjectID, workflowID, req.EnvID, userID, mode); err != nil {
			return response.Error(c, "创建执行记录失败: "+err.Error())
		}
	}

	// 确定使用的工作流定义：优先使用请求中的 definition，否则使用数据库中的
	definitionToUse := wf.Definition
	if req.Definition != nil {
		// 处理 definition：可能是字符串或对象
		switch v := req.Definition.(type) {
		case string:
			if v != "" {
				definitionToUse = v
				fmt.Printf("[EXECUTION] 使用请求中的工作流定义（字符串格式）\n")
			}
		case map[string]interface{}:
			// 对象格式，转换为 JSON 字符串
			defBytes, err := json.Marshal(v)
			if err != nil {
				return response.Error(c, "工作流定义序列化失败: "+err.Error())
			}
			definitionToUse = string(defBytes)
			fmt.Printf("[EXECUTION] 使用请求中的工作流定义（对象格式）\n")
		default:
			fmt.Printf("[EXECUTION] 未知的 definition 类型: %T\n", v)
		}
	}
	if definitionToUse == wf.Definition {
		fmt.Printf("[EXECUTION] 使用数据库中的工作流定义\n")
	}

	// 转换工作流定义
	fmt.Printf("[EXECUTION] 原始工作流定义: %s\n", definitionToUse)
	var engineWf *types.Workflow
	if req.Mode == "normal" {
		// 普通模式：失败后继续执行
		engineWf, err = logic.ConvertToEngineWorkflow(definitionToUse, sessionID)
	} else {
		// 调试模式（默认）：失败立即停止
		engineWf, err = logic.ConvertToEngineWorkflowStopOnError(definitionToUse, sessionID)
	}
	if err != nil {
		return response.Error(c, "工作流转换失败: "+err.Error())
	}

	// 过滤选中的步骤
	if len(req.SelectedSteps) > 0 {
		fmt.Printf("[EXECUTION] 过滤选中的步骤: %v\n", req.SelectedSteps)
		engineWf.Steps = filterSteps(engineWf.Steps, req.SelectedSteps)
	}

	// 日志：打印工作流信息
	fmt.Printf("[EXECUTION] 工作流转换完成: ID=%s, Name=%s, Steps=%d, Persist=%v\n", engineWf.ID, engineWf.Name, len(engineWf.Steps), persist)
	for i, step := range engineWf.Steps {
		fmt.Printf("[EXECUTION] 步骤[%d]: ID=%s, Type=%s, Name=%s\n", i, step.ID, step.Type, step.Name)
	}

	// 设置 SSE 响应头
	c.Set("Content-Type", "text/event-stream; charset=utf-8")
	c.Set("Cache-Control", "no-cache")
	c.Set("Connection", "keep-alive")
	c.Set("Transfer-Encoding", "chunked")
	c.Set("X-Accel-Buffering", "no") // 禁用 nginx 缓冲
	c.Set("X-Content-Type-Options", "nosniff")

	// 【重要】在 SetBodyStreamWriter 之前捕获上下文
	ctx := c.UserContext()

	// 创建执行请求
	execReq := &executor.ExecuteRequest{
		WorkflowID:   workflowID,
		EnvID:        req.EnvID,
		Variables:    req.Variables,
		Timeout:      req.Timeout,
		ExecutorType: executor.ExecutorType(req.ExecutorType),
		SlaveID:      req.SlaveID,
	}

	// 使用 StreamWriter 处理 SSE
	c.Context().SetBodyStreamWriter(func(w *bufio.Writer) {
		// Panic 恢复
		defer func() {
			if r := recover(); r != nil {
				fmt.Printf("[ERROR] SSE StreamWriter panic recovered: %v\n", r)
			}
		}()

		// 创建 SSE Writer
		writer := sse.NewWriter(&flushWriter{w: w}, sessionID)

		// 发送连接成功事件
		writer.WriteEvent(&sse.Event{
			Type: "connected",
			Data: map[string]interface{}{
				"session_id": sessionID,
				"message":    "SSE 连接成功",
				"persist":    persist,
			},
		})

		// 执行工作流
		execErr := h.streamExecutor.ExecuteStream(ctx, execReq, engineWf, writer)

		// 更新执行记录状态（仅当 persist=true 时）
		if persist && executionLogic != nil {
			if execErr != nil {
				writer.WriteErrorCode(sse.ErrExecutorError, "执行失败", execErr.Error())
				executionLogic.UpdateStreamExecutionStatus(sessionID, string(model.ExecutionStatusFailed), nil)
			} else {
				session, ok := h.sessionManager.GetSession(sessionID)
				if ok {
					total, success, failed := session.GetStats()
					status := "success"
					if failed > 0 {
						status = "failed"
					}
					executionLogic.UpdateStreamExecutionStatus(sessionID, status, map[string]interface{}{
						"total_steps":   total,
						"success_steps": success,
						"failed_steps":  failed,
					})
				}
			}
		} else if execErr != nil {
			// 不持久化时，仍然发送错误事件
			writer.WriteErrorCode(sse.ErrExecutorError, "执行失败", execErr.Error())
		}
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

	userID := middleware.GetCurrentUserID(c)
	persist := shouldPersist(req.Persist)

	// 获取工作流信息
	workflowLogic := logic.NewWorkflowLogic(c.UserContext())
	wf, err := workflowLogic.GetByID(workflowID)
	if err != nil {
		return response.NotFound(c, "工作流不存在")
	}

	// 生成会话ID
	sessionID := uuid.New().String()

	// 获取工作流类型
	workflowType := string(model.WorkflowTypeNormal)
	if wf.WorkflowType != nil {
		workflowType = *wf.WorkflowType
	}

	// 创建调度请求
	schedReq := &scheduler.ScheduleRequest{
		WorkflowID:   workflowID,
		WorkflowType: workflowType,
		Mode:         scheduler.ModeExecute,
		EnvID:        req.EnvID,
		SessionID:    sessionID,
		UserID:       userID,
	}

	// 调度到 Master
	_, err = h.scheduler.Schedule(c.UserContext(), schedReq)
	if err != nil {
		return response.Error(c, "调度失败: "+err.Error())
	}

	// 创建执行记录（仅当 persist=true 时）
	var executionLogic *logic.ExecutionLogic
	if persist {
		executionLogic = logic.NewExecutionLogic(c.UserContext())
		if err := executionLogic.CreateStreamExecution(sessionID, wf.ProjectID, workflowID, req.EnvID, userID, string(model.ExecutionModeExecute)); err != nil {
			return response.Error(c, "创建执行记录失败: "+err.Error())
		}
	}

	// 转换工作流定义
	engineWf, err := logic.ConvertToEngineWorkflow(wf.Definition, sessionID)
	if err != nil {
		return response.Error(c, "工作流转换失败: "+err.Error())
	}

	// 创建执行请求
	execReq := &executor.ExecuteRequest{
		WorkflowID:   workflowID,
		EnvID:        req.EnvID,
		Variables:    req.Variables,
		Timeout:      req.Timeout,
		ExecutorType: executor.ExecutorType(req.ExecutorType),
		SlaveID:      req.SlaveID,
	}

	// 执行工作流（阻塞）
	summary, execErr := h.streamExecutor.ExecuteBlocking(c.UserContext(), execReq, engineWf)

	// 更新执行记录状态（仅当 persist=true 时）
	if persist && executionLogic != nil {
		if execErr != nil {
			executionLogic.UpdateStreamExecutionStatus(sessionID, string(model.ExecutionStatusFailed), nil)
			return response.Error(c, "执行失败: "+execErr.Error())
		}
		executionLogic.UpdateStreamExecutionStatus(sessionID, summary.Status, summary)
	} else if execErr != nil {
		return response.Error(c, "执行失败: "+execErr.Error())
	}

	return response.Success(c, summary)
}

// StopExecution 停止执行
// DELETE /api/executions/:sessionId/stop
func (h *StreamExecutionHandler) StopExecution(c *fiber.Ctx) error {
	sessionID := c.Params("sessionId")
	if sessionID == "" {
		return response.Error(c, "会话ID不能为空")
	}

	// 停止执行
	if err := h.streamExecutor.Stop(sessionID); err != nil {
		return response.Error(c, "停止执行失败: "+err.Error())
	}

	// 更新执行记录状态
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

	// 提交交互响应
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

// Flush 实现 http.Flusher 接口
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

func (fw *fasthttpFlushWriter) Flush() {
	// fasthttp 会自动 flush
}

// filterSteps 过滤选中的步骤
func filterSteps(steps []types.Step, selectedIDs []string) []types.Step {
	if len(selectedIDs) == 0 {
		return steps
	}

	// 构建选中 ID 集合
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
