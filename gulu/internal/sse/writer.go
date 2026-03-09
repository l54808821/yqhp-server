package sse

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// EventType SSE 事件类型
type EventType string

const (
	// 流程控制事件
	EventStepStarted       EventType = "step_started"
	EventStepCompleted     EventType = "step_completed" // 统一：成功/失败/跳过
	EventWorkflowCompleted EventType = "workflow_completed"
	EventHeartbeat         EventType = "heartbeat"
	EventError             EventType = "error"

	// AI 内容事件
	EventAIChunk            EventType = "ai_chunk"
	EventAIThinking         EventType = "ai_thinking"
	EventAIToolCallStart    EventType = "ai_tool_call_start"
	EventAIToolCallComplete EventType = "ai_tool_call_complete"
	EventAIPlanUpdate       EventType = "ai_plan_update" // 合并原 started/step_update/completed/modified
	EventAIError            EventType = "ai_error"
	EventAIInteraction      EventType = "ai_interaction_required"
	EventAIArtifactStart    EventType = "ai_artifact_start"
	EventAIArtifactChunk    EventType = "ai_artifact_chunk"
	EventAIArtifactComplete EventType = "ai_artifact_complete"
	EventMessageComplete    EventType = "message_complete"
)

// Timestamp 自定义时间戳类型，格式化为 "2006-01-02 15:04:05.000"
type Timestamp time.Time

// MarshalJSON 自定义 JSON 序列化
func (t Timestamp) MarshalJSON() ([]byte, error) {
	formatted := time.Time(t).Format("2006-01-02 15:04:05.000")
	return []byte(`"` + formatted + `"`), nil
}

// Event SSE 事件结构
type Event struct {
	Type      EventType   `json:"type"`
	SessionID string      `json:"sessionId"`
	Timestamp Timestamp   `json:"timestamp"`
	Data      interface{} `json:"data"`
}

// Writer SSE 写入器
type Writer struct {
	w         io.Writer
	flusher   http.Flusher
	sessionID string
	mu        sync.Mutex
	closed    bool
}

// NewWriter 创建 SSE Writer
func NewWriter(w io.Writer, sessionID string) *Writer {
	sw := &Writer{
		w:         w,
		sessionID: sessionID,
	}
	// 尝试获取 Flusher 接口
	if f, ok := w.(http.Flusher); ok {
		sw.flusher = f
	}
	return sw
}

// WriteEvent 写入事件
func (sw *Writer) WriteEvent(event *Event) error {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	if sw.closed {
		return fmt.Errorf("writer is closed")
	}

	// 设置时间戳和会话ID
	if time.Time(event.Timestamp).IsZero() {
		event.Timestamp = Timestamp(time.Now())
	}
	if event.SessionID == "" {
		event.SessionID = sw.sessionID
	}

	// 序列化为 JSON（单行格式）
	// 使用 Encoder 并禁用 HTML 转义，保持 UTF-8 原样输出
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(event); err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	// Encode 会添加换行符，需要去掉
	jsonStr := strings.TrimSuffix(buf.String(), "\n")

	// 转义换行符确保 JSON 在单行
	jsonStr = escapeNewlines(jsonStr)

	// 写入 SSE 格式
	// event: <type>
	// data: <json>
	// (空行表示事件结束)
	_, err := fmt.Fprintf(sw.w, "event: %s\ndata: %s\n\n", event.Type, jsonStr)
	if err != nil {
		return fmt.Errorf("failed to write event: %w", err)
	}

	// 立即刷新
	if sw.flusher != nil {
		sw.flusher.Flush()
	}

	return nil
}

// WriteHeartbeat 写入心跳
func (sw *Writer) WriteHeartbeat() error {
	return sw.WriteEvent(&Event{
		Type: EventHeartbeat,
		Data: map[string]interface{}{
			"time": time.Now().Unix(),
		},
	})
}

// WriteError 写入错误事件
func (sw *Writer) WriteError(code, message, details string, recoverable bool) error {
	return sw.WriteEvent(&Event{
		Type: EventError,
		Data: &ErrorData{
			Code:        code,
			Message:     message,
			Details:     details,
			Recoverable: recoverable,
		},
	})
}

// WriteErrorCode 使用错误码写入错误事件
func (sw *Writer) WriteErrorCode(code ErrorCode, message string, details string) error {
	return sw.WriteEvent(&Event{
		Type: EventError,
		Data: NewError(code, message, details),
	})
}

// Close 关闭写入器
func (sw *Writer) Close() {
	sw.mu.Lock()
	defer sw.mu.Unlock()
	sw.closed = true
}

// IsClosed 检查是否已关闭
func (sw *Writer) IsClosed() bool {
	sw.mu.Lock()
	defer sw.mu.Unlock()
	return sw.closed
}

// SessionID 获取会话ID
func (sw *Writer) SessionID() string {
	return sw.sessionID
}

// escapeNewlines 转义 JSON 中的换行符
func escapeNewlines(s string) string {
	s = strings.ReplaceAll(s, "\n", "\\n")
	s = strings.ReplaceAll(s, "\r", "\\r")
	return s
}

// ============ 事件数据结构 ============

// StepStartedData 步骤开始数据
type StepStartedData struct {
	StepID    string `json:"stepId"`
	StepName  string `json:"stepName"`
	StepType  string `json:"stepType,omitempty"`
	ParentID  string `json:"parentId,omitempty"`
	Iteration int    `json:"iteration,omitempty"`
}

// StepCompletedData 步骤完成数据（统一：成功/失败/跳过）
type StepCompletedData struct {
	StepID     string      `json:"stepId"`
	StepName   string      `json:"stepName"`
	StepType   string      `json:"stepType,omitempty"`
	ParentID   string      `json:"parentId,omitempty"`
	Iteration  int         `json:"iteration,omitempty"`
	Status     string      `json:"status"` // success / failed / skipped
	DurationMs int64       `json:"durationMs"`
	Result     interface{} `json:"result,omitempty"`
	Error      string      `json:"error,omitempty"`
}

// WorkflowCompletedData 工作流完成数据
type WorkflowCompletedData struct {
	SessionID     string                 `json:"sessionId"`
	TotalSteps    int                    `json:"totalSteps"`
	SuccessSteps  int                    `json:"successSteps"`
	FailedSteps   int                    `json:"failedSteps"`
	TotalDuration int64                  `json:"totalDurationMs"`
	Status        string                 `json:"status"`
	Variables     map[string]interface{} `json:"variables,omitempty"`
	EnvVariables  map[string]interface{} `json:"envVariables,omitempty"`
}

// AIErrorData AI 错误数据
type AIErrorData struct {
	StepID  string `json:"stepId"`
	Error   string `json:"error"`
	Details string `json:"details,omitempty"`
}

// InteractionType 交互类型
type InteractionType string

const (
	InteractionTypeConfirm InteractionType = "confirm"
	InteractionTypeInput   InteractionType = "input"
	InteractionTypeSelect  InteractionType = "select"
)

// InteractionOption 交互选项
type InteractionOption struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

// AIInteractionData AI 交互数据
type AIInteractionData struct {
	StepID       string              `json:"stepId"`
	Type         InteractionType     `json:"type"`
	Prompt       string              `json:"prompt"`
	Options      []InteractionOption `json:"options,omitempty"`
	DefaultValue string              `json:"defaultValue,omitempty"`
	Timeout      int                 `json:"timeout"`
}

// ErrorData 错误数据
type ErrorData struct {
	Code        string `json:"code"`
	Message     string `json:"message"`
	Details     string `json:"details,omitempty"`
	Recoverable bool   `json:"recoverable"`
}
