package client

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"yqhp/gulu/internal/sse"
	"yqhp/workflow-engine/pkg/types"
)

// SlaveClient 远程 Slave 客户端
type SlaveClient struct {
	httpClient    *http.Client
	sseHttpClient *http.Client // SSE 专用客户端（无超时）
	baseURL       string
	slaveID       string
}

// SlaveClientManager Slave 客户端管理器
type SlaveClientManager struct {
	clients map[string]*SlaveClient
	mu      sync.RWMutex
}

// NewSlaveClientManager 创建 Slave 客户端管理器
func NewSlaveClientManager() *SlaveClientManager {
	return &SlaveClientManager{
		clients: make(map[string]*SlaveClient),
	}
}

// GetClient 获取或创建 Slave 客户端
func (m *SlaveClientManager) GetClient(slaveID, address string) *SlaveClient {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := slaveID
	if client, ok := m.clients[key]; ok {
		// 更新地址（可能变化）
		client.baseURL = address
		return client
	}

	client := NewSlaveClient(slaveID, address)
	m.clients[key] = client
	return client
}

// RemoveClient 移除 Slave 客户端
func (m *SlaveClientManager) RemoveClient(slaveID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.clients, slaveID)
}

// NewSlaveClient 创建 Slave 客户端
func NewSlaveClient(slaveID, baseURL string) *SlaveClient {
	return &SlaveClient{
		slaveID: slaveID,
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		sseHttpClient: &http.Client{
			Timeout: 0, // SSE 无超时
		},
	}
}

// ============ 请求/响应结构 ============

// SlaveStreamExecuteRequest 流式执行请求
type SlaveStreamExecuteRequest struct {
	Workflow       *types.Workflow        `json:"workflow"`
	Variables      map[string]interface{} `json:"variables,omitempty"`
	SessionID      string                 `json:"session_id"`
	Timeout        int                    `json:"timeout,omitempty"`
	InteractionURL string                 `json:"interaction_url"` // Gulu 接收交互响应的 URL
}

// SlaveBlockingExecuteRequest 阻塞式执行请求
type SlaveBlockingExecuteRequest struct {
	Workflow  *types.Workflow        `json:"workflow"`
	Variables map[string]interface{} `json:"variables,omitempty"`
	SessionID string                 `json:"session_id"`
	Timeout   int                    `json:"timeout,omitempty"`
}

// SlaveBlockingExecuteResponse 阻塞式执行响应
type SlaveBlockingExecuteResponse struct {
	SessionID     string                   `json:"session_id"`
	Status        string                   `json:"status"`
	TotalSteps    int                      `json:"total_steps"`
	SuccessSteps  int                      `json:"success_steps"`
	FailedSteps   int                      `json:"failed_steps"`
	TotalDuration int64                    `json:"total_duration_ms"`
	StepResults   []map[string]interface{} `json:"step_results,omitempty"`
	Error         string                   `json:"error,omitempty"`
}

// SlaveStatusResponse Slave 状态响应
type SlaveStatusResponse struct {
	SlaveID      string   `json:"slave_id"`
	State        string   `json:"state"` // online, offline, busy, draining
	Capabilities []string `json:"capabilities"`
	Load         float64  `json:"load"`
	ActiveTasks  int      `json:"active_tasks"`
	MaxTasks     int      `json:"max_tasks"`
	Version      string   `json:"version"`
}

// InteractionSubmitRequest 交互提交请求
type InteractionSubmitRequest struct {
	SessionID string `json:"session_id"`
	StepID    string `json:"step_id"`
	Value     string `json:"value"`
	Skipped   bool   `json:"skipped"`
}

// SSEEventHandler SSE 事件处理器
type SSEEventHandler func(eventType string, data []byte) error

// ============ 流式执行 ============

// ExecuteStream 流式执行工作流
// Slave 返回 SSE 流，Gulu 作为 SSE 客户端接收事件并转发
func (c *SlaveClient) ExecuteStream(ctx context.Context, req *SlaveStreamExecuteRequest, handler SSEEventHandler) error {
	// 构建请求
	jsonData, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("序列化请求失败: %w", err)
	}

	url := c.baseURL + "/api/v1/execute/stream"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonData))
	if err != nil {
		return fmt.Errorf("创建请求失败: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")
	httpReq.Header.Set("X-Slave-ID", c.slaveID)
	httpReq.Header.Set("X-Session-ID", req.SessionID)

	// 发送请求
	resp, err := c.sseHttpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Slave 返回错误 (%d): %s", resp.StatusCode, string(body))
	}

	// 检查 Content-Type
	contentType := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "text/event-stream") {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Slave 返回非 SSE 响应: %s, body: %s", contentType, string(body))
	}

	// 读取 SSE 流
	return c.readSSEStream(ctx, resp.Body, handler)
}

// readSSEStream 读取 SSE 流
func (c *SlaveClient) readSSEStream(ctx context.Context, body io.Reader, handler SSEEventHandler) error {
	scanner := bufio.NewScanner(body)
	var eventType string
	var dataLines []string

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		line := scanner.Text()

		// 空行表示事件结束
		if line == "" {
			if eventType != "" && len(dataLines) > 0 {
				data := strings.Join(dataLines, "\n")
				// 反转义换行符
				data = strings.ReplaceAll(data, "\\n", "\n")
				data = strings.ReplaceAll(data, "\\r", "\r")

				if err := handler(eventType, []byte(data)); err != nil {
					return err
				}
			}
			eventType = ""
			dataLines = nil
			continue
		}

		// 解析 SSE 行
		if strings.HasPrefix(line, "event:") {
			eventType = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		} else if strings.HasPrefix(line, "data:") {
			dataLines = append(dataLines, strings.TrimSpace(strings.TrimPrefix(line, "data:")))
		}
		// 忽略 id: 和 retry: 行
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("读取 SSE 流失败: %w", err)
	}

	return nil
}

// ============ 阻塞式执行 ============

// ExecuteBlocking 阻塞式执行工作流
func (c *SlaveClient) ExecuteBlocking(ctx context.Context, req *SlaveBlockingExecuteRequest) (*SlaveBlockingExecuteResponse, error) {
	body, err := c.doRequest(ctx, "POST", "/api/v1/execute", req)
	if err != nil {
		return nil, fmt.Errorf("执行请求失败: %w", err)
	}

	var resp SlaveBlockingExecuteResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	return &resp, nil
}

// ============ 交互支持 ============

// SubmitInteraction 提交交互响应到 Slave
func (c *SlaveClient) SubmitInteraction(ctx context.Context, req *InteractionSubmitRequest) error {
	_, err := c.doRequest(ctx, "POST", fmt.Sprintf("/api/v1/executions/%s/interaction", req.SessionID), req)
	if err != nil {
		return fmt.Errorf("提交交互响应失败: %w", err)
	}
	return nil
}

// ============ 状态查询 ============

// Stop 停止执行
func (c *SlaveClient) Stop(ctx context.Context, sessionID string) error {
	_, err := c.doRequest(ctx, "DELETE", fmt.Sprintf("/api/v1/executions/%s", sessionID), nil)
	if err != nil {
		return fmt.Errorf("停止执行失败: %w", err)
	}
	return nil
}

// GetStatus 获取 Slave 状态
func (c *SlaveClient) GetStatus(ctx context.Context) (*SlaveStatusResponse, error) {
	body, err := c.doRequest(ctx, "GET", "/api/v1/status", nil)
	if err != nil {
		return nil, fmt.Errorf("获取状态失败: %w", err)
	}

	var resp SlaveStatusResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	return &resp, nil
}

// Ping 检查 Slave 是否可用
func (c *SlaveClient) Ping(ctx context.Context) error {
	_, err := c.doRequest(ctx, "GET", "/api/v1/ping", nil)
	return err
}

// SlaveID 获取 Slave ID
func (c *SlaveClient) SlaveID() string {
	return c.slaveID
}

// BaseURL 获取基础 URL
func (c *SlaveClient) BaseURL() string {
	return c.baseURL
}

// ============ 内部方法 ============

// doRequest 执行 HTTP 请求
func (c *SlaveClient) doRequest(ctx context.Context, method, path string, body interface{}) ([]byte, error) {
	url := c.baseURL + path

	var reqBody io.Reader
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("序列化请求体失败: %w", err)
		}
		reqBody = bytes.NewReader(jsonData)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Slave-ID", c.slaveID)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	if resp.StatusCode >= 400 {
		var errResp struct {
			Error   string `json:"error"`
			Message string `json:"message"`
		}
		if err := json.Unmarshal(respBody, &errResp); err == nil && errResp.Message != "" {
			return nil, fmt.Errorf("API 错误 (%d): %s", resp.StatusCode, errResp.Message)
		}
		return nil, fmt.Errorf("API 错误 (%d): %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

// ============ SSE 事件转发辅助 ============

// ForwardSSEEvent 将 Slave SSE 事件转发到 Gulu SSE Writer
func ForwardSSEEvent(writer *sse.Writer, eventType string, data []byte) error {
	// 解析事件数据
	var eventData map[string]interface{}
	if err := json.Unmarshal(data, &eventData); err != nil {
		return fmt.Errorf("解析事件数据失败: %w", err)
	}

	// 构建 SSE 事件
	event := &sse.Event{
		Type: sse.EventType(eventType),
		Data: eventData,
	}

	return writer.WriteEvent(event)
}
