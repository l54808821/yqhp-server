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

// SSEDebugHandler SSE 调试处理器
type SSEDebugHandler struct {
	scheduler      scheduler.Scheduler
	streamExecutor *executor.StreamExecutor
	sessionManager *executor.SessionManager
}

// NewSSEDebugHandler 创建 SSE 调试处理器
func NewSSEDebugHandler(sched scheduler.Scheduler, streamExec *executor.StreamExecutor, sessionMgr *executor.SessionManager) *SSEDebugHandler {
	return &SSEDebugHandler{
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
	Definition    interface{}            `json:"definition,omitempty" query:"definition"`         // 工作流定义（用于调试未保存的工作流），可以是字符串或对象
	SelectedSteps []string               `json:"selected_steps,omitempty" query:"selected_steps"` // 选中的步骤 ID（用于选择性调试）
}

// RunBlockingRequest 阻塞式执行请求
type RunBlockingRequest struct {
	EnvID        int64                  `json:"env_id"`
	Variables    map[string]interface{} `json:"variables,omitempty"`
	Timeout      int                    `json:"timeout,omitempty"`
	ExecutorType string                 `json:"executor_type,omitempty"`
	SlaveID      string                 `json:"slave_id,omitempty"`
}

// InteractionRequest 交互请求
type InteractionRequest struct {
	Value   string `json:"value"`
	Skipped bool   `json:"skipped"`
}

// RunStream 流式执行（SSE）
// POST /api/workflows/:id/run/stream
func (h *SSEDebugHandler) RunStream(c *fiber.Ctx) error {
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
		Mode:         scheduler.ModeDebug,
		EnvID:        req.EnvID,
		SessionID:    sessionID,
		UserID:       userID,
	}

	// 调度到 Master
	_, err = h.scheduler.Schedule(c.UserContext(), schedReq)
	if err != nil {
		return response.Error(c, "调度失败: "+err.Error())
	}

	// 创建执行记录
	debugLogic := logic.NewDebugLogic(c.UserContext())
	if err := debugLogic.CreateDebugSession(sessionID, wf.ProjectID, workflowID, req.EnvID, userID); err != nil {
		return response.Error(c, "创建调试会话失败: "+err.Error())
	}

	// 确定使用的工作流定义：优先使用请求中的 definition，否则使用数据库中的
	definitionToUse := wf.Definition
	if req.Definition != nil {
		// 处理 definition：可能是字符串或对象
		switch v := req.Definition.(type) {
		case string:
			if v != "" {
				definitionToUse = v
				fmt.Printf("[DEBUG] 使用请求中的工作流定义（字符串格式）\n")
			}
		case map[string]interface{}:
			// 对象格式，转换为 JSON 字符串
			defBytes, err := json.Marshal(v)
			if err != nil {
				return response.Error(c, "工作流定义序列化失败: "+err.Error())
			}
			definitionToUse = string(defBytes)
			fmt.Printf("[DEBUG] 使用请求中的工作流定义（对象格式）\n")
		default:
			fmt.Printf("[DEBUG] 未知的 definition 类型: %T\n", v)
		}
	}
	if definitionToUse == wf.Definition {
		fmt.Printf("[DEBUG] 使用数据库中的工作流定义\n")
	}

	// 转换工作流定义（调试模式：失败立即停止）
	fmt.Printf("[DEBUG] 原始工作流定义: %s\n", definitionToUse)
	engineWf, err := logic.ConvertToEngineWorkflowForDebug(definitionToUse, sessionID)
	if err != nil {
		// 返回 HTTP 错误，因为 SSE 连接还未建立
		return response.Error(c, "工作流转换失败: "+err.Error())
	}

	// 过滤选中的步骤
	if len(req.SelectedSteps) > 0 {
		fmt.Printf("[DEBUG] 过滤选中的步骤: %v\n", req.SelectedSteps)
		engineWf.Steps = filterSteps(engineWf.Steps, req.SelectedSteps)
	}

	// 调试日志：打印工作流信息
	fmt.Printf("[DEBUG] 工作流转换完成: ID=%s, Name=%s, Steps=%d\n", engineWf.ID, engineWf.Name, len(engineWf.Steps))
	for i, step := range engineWf.Steps {
		fmt.Printf("[DEBUG] 步骤[%d]: ID=%s, Type=%s, Name=%s\n", i, step.ID, step.Type, step.Name)
	}

	// 设置 SSE 响应头
	c.Set("Content-Type", "text/event-stream; charset=utf-8")
	c.Set("Cache-Control", "no-cache")
	c.Set("Connection", "keep-alive")
	c.Set("Transfer-Encoding", "chunked")
	c.Set("X-Accel-Buffering", "no") // 禁用 nginx 缓冲
	c.Set("X-Content-Type-Options", "nosniff")

	// 【重要】在 SetBodyStreamWriter 之前捕获上下文
	// SetBodyStreamWriter 的回调在独立 goroutine 中运行，此时 Fiber context 已失效
	ctx := c.UserContext()

	// 创建执行请求（在回调外部创建，避免闭包问题）
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
		// Panic 恢复，防止服务器崩溃
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
			},
		})

		// 执行工作流（使用预先捕获的 ctx 和 execReq）
		execErr := h.streamExecutor.ExecuteStream(ctx, execReq, engineWf, writer)

		// 更新执行记录状态
		if execErr != nil {
			// 发送错误事件
			writer.WriteErrorCode(sse.ErrExecutorError, "执行失败", execErr.Error())
			debugLogic.UpdateExecutionStatus(sessionID, string(model.ExecutionStatusFailed), nil)
		} else {
			session, ok := h.sessionManager.GetSession(sessionID)
			if ok {
				total, success, failed := session.GetStats()
				status := "success"
				if failed > 0 {
					status = "failed"
				}
				debugLogic.UpdateExecutionStatus(sessionID, status, map[string]interface{}{
					"total_steps":   total,
					"success_steps": success,
					"failed_steps":  failed,
				})
			}
		}
	})

	return nil
}

// RunBlocking 阻塞式执行
// POST /api/workflows/:id/run
func (h *SSEDebugHandler) RunBlocking(c *fiber.Ctx) error {
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

	// 创建执行记录
	debugLogic := logic.NewDebugLogic(c.UserContext())
	if err := debugLogic.CreateDebugSession(sessionID, wf.ProjectID, workflowID, req.EnvID, userID); err != nil {
		return response.Error(c, "创建执行会话失败: "+err.Error())
	}

	// 转换工作流定义
	engineWf, err := logic.ConvertToEngineWorkflowForDebug(wf.Definition, sessionID)
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

	// 更新执行记录状态
	if execErr != nil {
		debugLogic.UpdateExecutionStatus(sessionID, string(model.ExecutionStatusFailed), nil)
		return response.Error(c, "执行失败: "+execErr.Error())
	}

	debugLogic.UpdateExecutionStatus(sessionID, summary.Status, summary)

	return response.Success(c, summary)
}

// StopExecution 停止执行
// DELETE /api/executions/:sessionId
func (h *SSEDebugHandler) StopExecution(c *fiber.Ctx) error {
	sessionID := c.Params("sessionId")
	if sessionID == "" {
		return response.Error(c, "会话ID不能为空")
	}

	// 停止执行
	if err := h.streamExecutor.Stop(sessionID); err != nil {
		return response.Error(c, "停止执行失败: "+err.Error())
	}

	// 更新执行记录状态
	debugLogic := logic.NewDebugLogic(c.UserContext())
	debugLogic.UpdateExecutionStatus(sessionID, string(model.ExecutionStatusStopped), nil)

	return response.Success(c, nil)
}

// SubmitInteraction 提交交互响应
// POST /api/executions/:sessionId/interaction
func (h *SSEDebugHandler) SubmitInteraction(c *fiber.Ctx) error {
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
// GET /api/executions/:sessionId
func (h *SSEDebugHandler) GetExecutionStatus(c *fiber.Ctx) error {
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
	debugLogic := logic.NewDebugLogic(c.UserContext())
	dbSession, err := debugLogic.GetDebugSession(sessionID)
	if err != nil {
		return response.NotFound(c, "执行会话不存在")
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
// 当选中父步骤（循环/条件）时，自动包含其子步骤
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
			// 步骤被选中，添加到结果（包含其所有子步骤）
			filtered = append(filtered, step)
		} else {
			// 步骤未被选中，但需要检查其子步骤是否被选中
			// 对于循环步骤，递归过滤子步骤
			if step.Loop != nil && len(step.Loop.Steps) > 0 {
				filteredChildren := filterSteps(step.Loop.Steps, selectedIDs)
				if len(filteredChildren) > 0 {
					// 有子步骤被选中，创建新的步骤副本并替换子步骤
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
