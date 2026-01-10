package websocket

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/gofiber/contrib/websocket"
)

// MessageType 消息类型
type MessageType string

const (
	MsgTypeStepStarted   MessageType = "step_started"
	MsgTypeStepCompleted MessageType = "step_completed"
	MsgTypeStepFailed    MessageType = "step_failed"
	MsgTypeProgress      MessageType = "progress"
	MsgTypeDebugComplete MessageType = "debug_completed"
	MsgTypeError         MessageType = "error"
	MsgTypePing          MessageType = "ping"
	MsgTypePong          MessageType = "pong"
)

// Message WebSocket 消息结构
type Message struct {
	Type      MessageType `json:"type"`
	SessionID string      `json:"session_id"`
	Timestamp time.Time   `json:"timestamp"`
	Data      interface{} `json:"data"`
}

// StepStartedData 步骤开始数据
type StepStartedData struct {
	StepID   string `json:"step_id"`
	StepName string `json:"step_name"`
}

// StepResult 步骤执行结果
type StepResult struct {
	StepID   string                 `json:"step_id"`
	StepName string                 `json:"step_name"`
	Status   string                 `json:"status"`
	Duration int64                  `json:"duration_ms"`
	Output   map[string]interface{} `json:"output,omitempty"`
	Error    string                 `json:"error,omitempty"`
	Logs     []string               `json:"logs,omitempty"`
}

// ProgressData 进度数据
type ProgressData struct {
	CurrentStep int    `json:"current_step"`
	TotalSteps  int    `json:"total_steps"`
	Percentage  int    `json:"percentage"`
	StepName    string `json:"step_name"`
}

// DebugSummary 调试汇总
type DebugSummary struct {
	SessionID     string        `json:"session_id"`
	TotalSteps    int           `json:"total_steps"`
	SuccessSteps  int           `json:"success_steps"`
	FailedSteps   int           `json:"failed_steps"`
	TotalDuration int64         `json:"total_duration_ms"`
	Status        string        `json:"status"` // success, failed, timeout, stopped
	StepResults   []*StepResult `json:"step_results"`
	StartTime     time.Time     `json:"start_time"`
	EndTime       time.Time     `json:"end_time"`
}

// ErrorData 错误数据
type ErrorData struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
}

// Hub WebSocket 连接管理中心
type Hub struct {
	// 按会话ID分组的连接
	connections map[string]map[*websocket.Conn]bool
	mu          sync.RWMutex

	// 心跳配置
	pingInterval time.Duration
	pongTimeout  time.Duration
}

// NewHub 创建 Hub 实例
func NewHub() *Hub {
	return &Hub{
		connections:  make(map[string]map[*websocket.Conn]bool),
		pingInterval: 30 * time.Second,
		pongTimeout:  10 * time.Second,
	}
}

// Register 注册连接
func (h *Hub) Register(sessionID string, conn *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.connections[sessionID] == nil {
		h.connections[sessionID] = make(map[*websocket.Conn]bool)
	}
	h.connections[sessionID][conn] = true
}

// Unregister 注销连接
func (h *Hub) Unregister(sessionID string, conn *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if conns, ok := h.connections[sessionID]; ok {
		delete(conns, conn)
		if len(conns) == 0 {
			delete(h.connections, sessionID)
		}
	}
}

// GetConnectionCount 获取会话连接数
func (h *Hub) GetConnectionCount(sessionID string) int {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if conns, ok := h.connections[sessionID]; ok {
		return len(conns)
	}
	return 0
}

// HasConnections 检查会话是否有连接
func (h *Hub) HasConnections(sessionID string) bool {
	return h.GetConnectionCount(sessionID) > 0
}

// Broadcast 向指定会话广播消息
func (h *Hub) Broadcast(sessionID string, msg *Message) error {
	h.mu.RLock()
	conns, ok := h.connections[sessionID]
	if !ok || len(conns) == 0 {
		h.mu.RUnlock()
		return nil // 没有连接，静默返回
	}

	// 复制连接列表避免长时间持有锁
	connList := make([]*websocket.Conn, 0, len(conns))
	for conn := range conns {
		connList = append(connList, conn)
	}
	h.mu.RUnlock()

	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	// 向所有连接发送消息
	for _, conn := range connList {
		if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
			// 发送失败，注销连接
			h.Unregister(sessionID, conn)
		}
	}

	return nil
}

// BroadcastStepStarted 广播步骤开始事件
func (h *Hub) BroadcastStepStarted(sessionID string, stepID, stepName string) {
	msg := &Message{
		Type:      MsgTypeStepStarted,
		SessionID: sessionID,
		Timestamp: time.Now(),
		Data: &StepStartedData{
			StepID:   stepID,
			StepName: stepName,
		},
	}
	h.Broadcast(sessionID, msg)
}

// BroadcastStepCompleted 广播步骤完成事件
func (h *Hub) BroadcastStepCompleted(sessionID string, result *StepResult) {
	msg := &Message{
		Type:      MsgTypeStepCompleted,
		SessionID: sessionID,
		Timestamp: time.Now(),
		Data:      result,
	}
	h.Broadcast(sessionID, msg)
}

// BroadcastStepFailed 广播步骤失败事件
func (h *Hub) BroadcastStepFailed(sessionID string, stepID, stepName, errMsg string) {
	msg := &Message{
		Type:      MsgTypeStepFailed,
		SessionID: sessionID,
		Timestamp: time.Now(),
		Data: &StepResult{
			StepID:   stepID,
			StepName: stepName,
			Status:   "failed",
			Error:    errMsg,
		},
	}
	h.Broadcast(sessionID, msg)
}

// BroadcastProgress 广播进度更新
func (h *Hub) BroadcastProgress(sessionID string, progress *ProgressData) {
	msg := &Message{
		Type:      MsgTypeProgress,
		SessionID: sessionID,
		Timestamp: time.Now(),
		Data:      progress,
	}
	h.Broadcast(sessionID, msg)
}

// BroadcastDebugComplete 广播调试完成
func (h *Hub) BroadcastDebugComplete(sessionID string, summary *DebugSummary) {
	msg := &Message{
		Type:      MsgTypeDebugComplete,
		SessionID: sessionID,
		Timestamp: time.Now(),
		Data:      summary,
	}
	h.Broadcast(sessionID, msg)
}

// BroadcastError 广播错误消息
func (h *Hub) BroadcastError(sessionID string, code, message, details string) {
	msg := &Message{
		Type:      MsgTypeError,
		SessionID: sessionID,
		Timestamp: time.Now(),
		Data: &ErrorData{
			Code:    code,
			Message: message,
			Details: details,
		},
	}
	h.Broadcast(sessionID, msg)
}

// CleanupSession 清理会话的所有连接
func (h *Hub) CleanupSession(sessionID string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if conns, ok := h.connections[sessionID]; ok {
		for conn := range conns {
			conn.Close()
		}
		delete(h.connections, sessionID)
	}
}
