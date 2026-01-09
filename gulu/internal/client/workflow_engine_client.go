package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"yqhp/gulu/internal/svc"
	"yqhp/gulu/internal/workflow"
	"yqhp/workflow-engine/pkg/types"
)

// WorkflowEngineClient Workflow Engine 服务 API 客户端
type WorkflowEngineClient struct {
	baseURL    string
	httpClient *http.Client
	embedded   bool
}

// NewWorkflowEngineClient 创建 Workflow Engine 客户端
func NewWorkflowEngineClient() *WorkflowEngineClient {
	cfg := svc.Ctx.Config.WorkflowEngine
	return &WorkflowEngineClient{
		baseURL:  cfg.ExternalURL,
		embedded: cfg.Embedded,
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

// doRequest 执行 HTTP 请求（外部模式使用）
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
	// 内置模式：直接调用内置 Master
	if c.embedded {
		return c.getExecutorListEmbedded()
	}

	// 外部模式：通过 HTTP 调用
	return c.getExecutorListExternal()
}

// getExecutorListEmbedded 从内置引擎获取执行机列表
func (c *WorkflowEngineClient) getExecutorListEmbedded() ([]ExecutorStatus, error) {
	engine := workflow.GetEngine()
	if engine == nil {
		return nil, fmt.Errorf("工作流引擎未初始化")
	}

	slaves, err := engine.GetSlaves(context.Background())
	if err != nil {
		return nil, err
	}

	result := make([]ExecutorStatus, 0, len(slaves))
	for _, slave := range slaves {
		result = append(result, ExecutorStatus{
			SlaveID:      slave.ID,
			Address:      slave.Address,
			Capabilities: slave.Capabilities,
			State:        "online", // 内置模式下，能获取到的都是在线的
		})
	}

	return result, nil
}

// getExecutorListExternal 从外部 Master 获取执行机列表
func (c *WorkflowEngineClient) getExecutorListExternal() ([]ExecutorStatus, error) {
	body, err := c.doRequest("GET", "/api/v1/slaves")
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
	// 内置模式：直接调用内置 Master
	if c.embedded {
		return c.getExecutorStatusEmbedded(slaveID)
	}

	// 外部模式：通过 HTTP 调用
	return c.getExecutorStatusExternal(slaveID)
}

// getExecutorStatusEmbedded 从内置引擎获取单个执行机状态
func (c *WorkflowEngineClient) getExecutorStatusEmbedded(slaveID string) (*ExecutorStatus, error) {
	engine := workflow.GetEngine()
	if engine == nil {
		return nil, fmt.Errorf("工作流引擎未初始化")
	}

	slaves, err := engine.GetSlaves(context.Background())
	if err != nil {
		return nil, err
	}

	for _, slave := range slaves {
		if slave.ID == slaveID {
			return &ExecutorStatus{
				SlaveID:      slave.ID,
				Address:      slave.Address,
				Capabilities: slave.Capabilities,
				State:        "online",
			}, nil
		}
	}

	return nil, fmt.Errorf("执行机不存在: %s", slaveID)
}

// getExecutorStatusExternal 从外部 Master 获取单个执行机状态
func (c *WorkflowEngineClient) getExecutorStatusExternal(slaveID string) (*ExecutorStatus, error) {
	body, err := c.doRequest("GET", fmt.Sprintf("/api/v1/slaves/%s", slaveID))
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

// SubmitWorkflow 提交工作流执行
func (c *WorkflowEngineClient) SubmitWorkflow(wf *types.Workflow) (string, error) {
	if c.embedded {
		engine := workflow.GetEngine()
		if engine == nil {
			return "", fmt.Errorf("工作流引擎未初始化")
		}
		return engine.SubmitWorkflow(context.Background(), wf)
	}

	// 外部模式：TODO 实现 HTTP 调用
	return "", fmt.Errorf("外部模式暂未实现")
}

// GetExecutionStatus 获取执行状态
func (c *WorkflowEngineClient) GetExecutionStatus(executionID string) (*types.ExecutionState, error) {
	if c.embedded {
		engine := workflow.GetEngine()
		if engine == nil {
			return nil, fmt.Errorf("工作流引擎未初始化")
		}
		return engine.GetExecutionStatus(context.Background(), executionID)
	}

	// 外部模式：TODO 实现 HTTP 调用
	return nil, fmt.Errorf("外部模式暂未实现")
}

// StopExecution 停止执行
func (c *WorkflowEngineClient) StopExecution(executionID string) error {
	if c.embedded {
		engine := workflow.GetEngine()
		if engine == nil {
			return fmt.Errorf("工作流引擎未初始化")
		}
		return engine.StopExecution(context.Background(), executionID)
	}

	// 外部模式：TODO 实现 HTTP 调用
	return fmt.Errorf("外部模式暂未实现")
}
