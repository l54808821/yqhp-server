package handler

import (
	"context"
	"encoding/json"
	"strconv"
	"time"

	"yqhp/common/response"
	"yqhp/gulu/internal/executor"
	"yqhp/gulu/internal/logic"
	"yqhp/gulu/internal/middleware"
	"yqhp/gulu/internal/model"
	"yqhp/gulu/internal/scheduler"
	"yqhp/gulu/internal/websocket"

	ws "github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

// DebugHandler 调试处理器
type DebugHandler struct {
	hub            *websocket.Hub
	scheduler      scheduler.Scheduler
	masterExecutor *executor.MasterExecutor
}

// NewDebugHandler 创建调试处理器
func NewDebugHandler(hub *websocket.Hub, sched scheduler.Scheduler, masterExec *executor.MasterExecutor) *DebugHandler {
	return &DebugHandler{
		hub:            hub,
		scheduler:      sched,
		masterExecutor: masterExec,
	}
}

// StartDebugRequest 开始调试请求
type StartDebugRequest struct {
	EnvID     int64                  `json:"env_id"`
	Variables map[string]interface{} `json:"variables,omitempty"`
	Timeout   int                    `json:"timeout,omitempty"` // 超时时间（秒）
}

// StartDebugResponse 开始调试响应
type StartDebugResponse struct {
	SessionID    string `json:"session_id"`
	WebSocketURL string `json:"websocket_url"`
}

// StartDebug 开始调试
// POST /api/workflows/:id/debug
func (h *DebugHandler) StartDebug(c *fiber.Ctx) error {
	workflowID, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "无效的工作流ID")
	}

	var req StartDebugRequest
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

	// 创建执行记录（使用统一的 t_execution 表）
	debugLogic := logic.NewDebugLogic(c.UserContext())
	if err := debugLogic.CreateDebugSession(sessionID, wf.ProjectID, workflowID, req.EnvID, userID); err != nil {
		return response.Error(c, "创建调试会话失败: "+err.Error())
	}

	// 异步执行调试
	go h.executeDebug(sessionID, wf, req)

	return response.Success(c, &StartDebugResponse{
		SessionID:    sessionID,
		WebSocketURL: "/ws/debug/" + sessionID,
	})
}

// executeDebug 执行调试（异步）
func (h *DebugHandler) executeDebug(sessionID string, wf *model.TWorkflow, req StartDebugRequest) {
	// 等待 WebSocket 连接
	time.Sleep(500 * time.Millisecond)

	// 转换工作流定义（调试模式：失败立即停止）
	engineWf, err := logic.ConvertToEngineWorkflowForDebug(wf.Definition, sessionID)
	if err != nil {
		h.hub.BroadcastError(sessionID, "CONVERSION_ERROR", "工作流转换失败", err.Error())
		return
	}

	// 设置超时
	timeout := 30 * time.Minute
	if req.Timeout > 0 {
		timeout = time.Duration(req.Timeout) * time.Second
	}

	// 创建调试请求
	debugReq := &executor.DebugRequest{
		SessionID: sessionID,
		Workflow:  engineWf,
		Variables: req.Variables,
		Timeout:   timeout,
	}

	// 执行调试
	ctx := context.Background()
	summary, err := h.masterExecutor.Execute(ctx, debugReq)

	// 更新执行记录状态
	debugLogic := logic.NewDebugLogic(ctx)
	if err != nil {
		debugLogic.UpdateExecutionStatus(sessionID, string(model.ExecutionStatusFailed), nil)
		h.hub.BroadcastError(sessionID, "EXECUTION_ERROR", "执行失败", err.Error())
		return
	}

	// 保存结果
	debugLogic.UpdateExecutionStatus(sessionID, summary.Status, summary)
}

// StopDebug 停止调试
// DELETE /api/debug/:sessionId
func (h *DebugHandler) StopDebug(c *fiber.Ctx) error {
	sessionID := c.Params("sessionId")
	if sessionID == "" {
		return response.Error(c, "会话ID不能为空")
	}

	// 停止执行
	if err := h.masterExecutor.Stop(sessionID); err != nil {
		return response.Error(c, "停止调试失败: "+err.Error())
	}

	// 更新执行记录状态
	debugLogic := logic.NewDebugLogic(c.UserContext())
	debugLogic.UpdateExecutionStatus(sessionID, string(model.ExecutionStatusStopped), nil)

	return response.Success(c, nil)
}

// GetDebugSession 获取调试会话信息
// GET /api/debug/:sessionId
func (h *DebugHandler) GetDebugSession(c *fiber.Ctx) error {
	sessionID := c.Params("sessionId")
	if sessionID == "" {
		return response.Error(c, "会话ID不能为空")
	}

	debugLogic := logic.NewDebugLogic(c.UserContext())
	session, err := debugLogic.GetDebugSession(sessionID)
	if err != nil {
		return response.NotFound(c, "调试会话不存在")
	}

	return response.Success(c, session)
}

// ListDebugSessions 获取调试会话列表
// GET /api/debug/sessions
func (h *DebugHandler) ListDebugSessions(c *fiber.Ctx) error {
	workflowID, _ := strconv.ParseInt(c.Query("workflow_id"), 10, 64)
	userID := middleware.GetCurrentUserID(c)

	debugLogic := logic.NewDebugLogic(c.UserContext())
	sessions, err := debugLogic.ListDebugSessions(workflowID, userID)
	if err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, sessions)
}

// WebSocketHandler WebSocket 连接处理
// GET /ws/debug/:sessionId
func (h *DebugHandler) WebSocketHandler(conn *ws.Conn) {
	sessionID := conn.Params("sessionId")
	if sessionID == "" {
		conn.Close()
		return
	}

	// 注册连接
	h.hub.Register(sessionID, conn)
	defer h.hub.Unregister(sessionID, conn)

	// 发送连接成功消息
	conn.WriteJSON(map[string]interface{}{
		"type":       "connected",
		"session_id": sessionID,
		"timestamp":  time.Now(),
	})

	// 读取消息循环
	for {
		messageType, message, err := conn.ReadMessage()
		if err != nil {
			break
		}

		// 处理 ping/pong
		if messageType == ws.PingMessage {
			conn.WriteMessage(ws.PongMessage, nil)
			continue
		}

		// 处理文本消息（如客户端发送的 ping）
		if messageType == ws.TextMessage {
			var msg map[string]interface{}
			if err := json.Unmarshal(message, &msg); err == nil {
				if msg["type"] == "ping" {
					conn.WriteJSON(map[string]interface{}{
						"type":       "pong",
						"session_id": sessionID,
						"timestamp":  time.Now(),
					})
				}
			}
		}
	}
}

// WebSocketUpgrade WebSocket 升级中间件
func (h *DebugHandler) WebSocketUpgrade(c *fiber.Ctx) error {
	if ws.IsWebSocketUpgrade(c) {
		return c.Next()
	}
	return fiber.ErrUpgradeRequired
}
