package client

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"yqhp/gulu/internal/svc"
)

// WorkflowEngineClient Workflow Engine 服务 API 客户端
type WorkflowEngineClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewWorkflowEngineClient 创建 Workflow Engine 客户端
func NewWorkflowEngineClient() *WorkflowEngineClient {
	return &WorkflowEngineClient{
		baseURL: svc.Ctx.Config.Gulu.WorkflowEngineURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// ExecutorStatus 执行机运行时状态（从 workflow-engine 获取）
type ExecutorStatus struct {
	SlaveID      string    `json:"slave_id"`
	Address      string    `json:"address"`
	Capabilities []string  `json:"capabilities"` // http_executor, script_executor 等
	State        string    `json:"state"`        // online, offline, busy, draining
	Load         float64   `json:"load"`
	ActiveTasks  int       `json:"active_tasks"`
	CurrentVUs   int       `json:"current_vus"`
	LastSeen     time.Time `json:"last_seen"`
}

// doRequest 执行 HTTP 请求
func (c *WorkflowEngineClient) doRequest(method, path string) ([]byte, error) {
	url := c.baseURL + path
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("API 错误: %s", string(body))
	}

	return body, nil
}

// GetExecutorList 获取执行机列表
func (c *WorkflowEngineClient) GetExecutorList() ([]ExecutorStatus, error) {
	body, err := c.doRequest("GET", "/api/slaves")
	if err != nil {
		return nil, err
	}

	var result struct {
		Code    int              `json:"code"`
		Message string           `json:"message"`
		Data    []ExecutorStatus `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		// 尝试直接解析为数组
		var executors []ExecutorStatus
		if err2 := json.Unmarshal(body, &executors); err2 != nil {
			return nil, fmt.Errorf("解析执行机列表失败: %w", err)
		}
		return executors, nil
	}

	return result.Data, nil
}

// GetExecutorStatus 获取单个执行机状态
func (c *WorkflowEngineClient) GetExecutorStatus(slaveID string) (*ExecutorStatus, error) {
	body, err := c.doRequest("GET", fmt.Sprintf("/api/slaves/%s", slaveID))
	if err != nil {
		return nil, err
	}

	var result struct {
		Code    int            `json:"code"`
		Message string         `json:"message"`
		Data    ExecutorStatus `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		// 尝试直接解析
		var executor ExecutorStatus
		if err2 := json.Unmarshal(body, &executor); err2 != nil {
			return nil, fmt.Errorf("解析执行机状态失败: %w", err)
		}
		return &executor, nil
	}

	return &result.Data, nil
}
