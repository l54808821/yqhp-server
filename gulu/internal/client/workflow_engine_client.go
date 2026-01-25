package client

import (
	"bytes"
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
func (c *WorkflowEngineClient) doRequest(method, path string, body interface{}) ([]byte, error) {
	url := c.baseURL + path

	var reqBody io.Reader
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("序列化请求体失败: %w", err)
		}
		reqBody = bytes.NewReader(jsonData)
	}

	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

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
	body, err := c.doRequest("GET", "/api/v1/slaves", nil)
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
	body, err := c.doRequest("GET", fmt.Sprintf("/api/v1/slaves/%s", slaveID), nil)
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

	// 外部模式：通过 HTTP 调用
	return c.submitWorkflowExternal(wf)
}

// submitWorkflowExternal 通过 HTTP 提交工作流
func (c *WorkflowEngineClient) submitWorkflowExternal(wf *types.Workflow) (string, error) {
	reqBody := struct {
		Workflow *types.Workflow `json:"workflow"`
	}{
		Workflow: wf,
	}

	body, err := c.doRequest("POST", "/api/v1/workflows/submit", reqBody)
	if err != nil {
		return "", fmt.Errorf("提交工作流失败: %w", err)
	}

	var result struct {
		ExecutionID string `json:"execution_id"`
		WorkflowID  string `json:"workflow_id"`
		Status      string `json:"status"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("解析响应失败: %w", err)
	}

	return result.ExecutionID, nil
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

	// 外部模式：通过 HTTP 调用
	return c.getExecutionStatusExternal(executionID)
}

// getExecutionStatusExternal 通过 HTTP 获取执行状态
func (c *WorkflowEngineClient) getExecutionStatusExternal(executionID string) (*types.ExecutionState, error) {
	body, err := c.doRequest("GET", fmt.Sprintf("/api/v1/executions/%s", executionID), nil)
	if err != nil {
		return nil, fmt.Errorf("获取执行状态失败: %w", err)
	}

	// 解析响应
	var resp struct {
		ID          string                             `json:"id"`
		WorkflowID  string                             `json:"workflow_id"`
		Status      string                             `json:"status"`
		Progress    float64                            `json:"progress"`
		StartTime   string                             `json:"start_time"`
		EndTime     string                             `json:"end_time,omitempty"`
		SlaveStates map[string]*SlaveExecutionResponse `json:"slave_states,omitempty"`
		Errors      []ExecutionErrorResponse           `json:"errors,omitempty"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("解析执行状态失败: %w", err)
	}

	// 转换为内部类型
	state := &types.ExecutionState{
		ID:         resp.ID,
		WorkflowID: resp.WorkflowID,
		Status:     types.ExecutionStatus(resp.Status),
		Progress:   resp.Progress,
	}

	// 解析时间
	if resp.StartTime != "" {
		if t, err := time.Parse(time.RFC3339, resp.StartTime); err == nil {
			state.StartTime = t
		}
	}
	if resp.EndTime != "" {
		if t, err := time.Parse(time.RFC3339, resp.EndTime); err == nil {
			state.EndTime = &t
		}
	}

	// 转换 SlaveStates
	if len(resp.SlaveStates) > 0 {
		state.SlaveStates = make(map[string]*types.SlaveExecutionState)
		for id, ss := range resp.SlaveStates {
			state.SlaveStates[id] = &types.SlaveExecutionState{
				SlaveID:        ss.SlaveID,
				Status:         types.ExecutionStatus(ss.Status),
				CompletedVUs:   ss.CompletedVUs,
				CompletedIters: ss.CompletedIters,
				Segment: types.ExecutionSegment{
					Start: ss.SegmentStart,
					End:   ss.SegmentEnd,
				},
			}
		}
	}

	// 转换 Errors
	if len(resp.Errors) > 0 {
		state.Errors = make([]types.ExecutionError, len(resp.Errors))
		for i, e := range resp.Errors {
			var timestamp time.Time
			if e.Timestamp != "" {
				timestamp, _ = time.Parse(time.RFC3339, e.Timestamp)
			}
			state.Errors[i] = types.ExecutionError{
				Code:      types.ErrorCode(e.Code),
				Message:   e.Message,
				StepID:    e.StepID,
				Timestamp: timestamp,
			}
		}
	}

	return state, nil
}

// SlaveExecutionResponse 用于解析 Slave 执行状态响应
type SlaveExecutionResponse struct {
	SlaveID        string  `json:"slave_id"`
	Status         string  `json:"status"`
	CompletedVUs   int     `json:"completed_vus"`
	CompletedIters int     `json:"completed_iters"`
	SegmentStart   float64 `json:"segment_start"`
	SegmentEnd     float64 `json:"segment_end"`
}

// ExecutionErrorResponse 用于解析执行错误响应
type ExecutionErrorResponse struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	StepID    string `json:"step_id,omitempty"`
	Timestamp string `json:"timestamp"`
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

	// 外部模式：通过 HTTP 调用
	return c.stopExecutionExternal(executionID)
}

// stopExecutionExternal 通过 HTTP 停止执行
func (c *WorkflowEngineClient) stopExecutionExternal(executionID string) error {
	_, err := c.doRequest("DELETE", fmt.Sprintf("/api/v1/executions/%s", executionID), nil)
	if err != nil {
		return fmt.Errorf("停止执行失败: %w", err)
	}
	return nil
}

// DebugStepRequest 单步调试请求
type DebugStepRequest struct {
	NodeConfig *DebugNodeConfig       `json:"nodeConfig"`
	EnvID      int64                  `json:"envId,omitempty"`
	Variables  map[string]interface{} `json:"variables,omitempty"`
	EnvVars    map[string]interface{} `json:"envVars,omitempty"`
	SessionID  string                 `json:"sessionId,omitempty"`
}

// DebugNodeConfig 调试节点配置
type DebugNodeConfig struct {
	ID             string                 `json:"id"`
	Type           string                 `json:"type"`
	Name           string                 `json:"name"`
	Config         map[string]interface{} `json:"config"`
	PreProcessors  []KeywordConfig        `json:"preProcessors,omitempty"`
	PostProcessors []KeywordConfig        `json:"postProcessors,omitempty"`
}

// KeywordConfig 关键字配置
type KeywordConfig struct {
	ID      string                 `json:"id"`
	Type    string                 `json:"type"`
	Enabled bool                   `json:"enabled"`
	Name    string                 `json:"name,omitempty"`
	Config  map[string]interface{} `json:"config"`
}

// DebugStepResponse 单步调试响应
type DebugStepResponse struct {
	Success          bool                     `json:"success"`
	Response         *types.HTTPResponseData  `json:"response,omitempty"`
	ScriptResult     *DebugScriptResult       `json:"scriptResult,omitempty"`
	AssertionResults []types.AssertionResult  `json:"assertionResults,omitempty"`
	ConsoleLogs      []types.ConsoleLogEntry  `json:"consoleLogs,omitempty"`
	ActualRequest    *types.ActualRequest     `json:"actualRequest,omitempty"`
	Error            string                   `json:"error,omitempty"`
}

// DebugScriptResult 脚本执行结果
type DebugScriptResult struct {
	Script      string                  `json:"script"`
	Language    string                  `json:"language"`
	Result      interface{}             `json:"result"`
	ConsoleLogs []types.ConsoleLogEntry `json:"consoleLogs"`
	Error       string                  `json:"error,omitempty"`
	Variables   map[string]interface{}  `json:"variables"`
	DurationMs  int64                   `json:"durationMs"`
}

// DebugStep 单步调试（调用 workflow-engine API）
func (c *WorkflowEngineClient) DebugStep(ctx context.Context, req *DebugStepRequest) (*DebugStepResponse, error) {
	if c.embedded {
		return c.debugStepEmbedded(ctx, req)
	}
	return c.debugStepExternal(req)
}

// debugStepEmbedded 内置模式单步调试
func (c *WorkflowEngineClient) debugStepEmbedded(ctx context.Context, req *DebugStepRequest) (*DebugStepResponse, error) {
	engine := workflow.GetEngine()
	if engine == nil {
		return nil, fmt.Errorf("工作流引擎未初始化")
	}

	// 调用内置引擎的 DebugStep 方法
	result, err := engine.DebugStep(ctx, &types.DebugStepRequest{
		NodeConfig: &types.DebugNodeConfig{
			ID:             req.NodeConfig.ID,
			Type:           req.NodeConfig.Type,
			Name:           req.NodeConfig.Name,
			Config:         req.NodeConfig.Config,
			PreProcessors:  convertKeywordConfigs(req.NodeConfig.PreProcessors),
			PostProcessors: convertKeywordConfigs(req.NodeConfig.PostProcessors),
		},
		EnvID:     req.EnvID,
		Variables: req.Variables,
		EnvVars:   req.EnvVars,
		SessionID: req.SessionID,
	})
	if err != nil {
		return nil, err
	}

	return convertDebugStepResponse(result), nil
}

// debugStepExternal 外部模式单步调试
func (c *WorkflowEngineClient) debugStepExternal(req *DebugStepRequest) (*DebugStepResponse, error) {
	body, err := c.doRequest("POST", "/api/v1/debug/step", req)
	if err != nil {
		return nil, fmt.Errorf("单步调试请求失败: %w", err)
	}

	var result DebugStepResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	return &result, nil
}

// convertKeywordConfigs 转换关键字配置
func convertKeywordConfigs(configs []KeywordConfig) []types.KeywordConfig {
	result := make([]types.KeywordConfig, len(configs))
	for i, c := range configs {
		result[i] = types.KeywordConfig{
			ID:      c.ID,
			Type:    c.Type,
			Enabled: c.Enabled,
			Name:    c.Name,
			Config:  c.Config,
		}
	}
	return result
}

// convertDebugStepResponse 转换调试响应
func convertDebugStepResponse(resp *types.DebugStepResponse) *DebugStepResponse {
	result := &DebugStepResponse{
		Success:          resp.Success,
		Response:         resp.Response,
		AssertionResults: resp.AssertionResults,
		ConsoleLogs:      resp.ConsoleLogs,
		ActualRequest:    resp.ActualRequest,
		Error:            resp.Error,
	}

	if resp.ScriptResult != nil {
		result.ScriptResult = &DebugScriptResult{
			Script:      resp.ScriptResult.Script,
			Language:    resp.ScriptResult.Language,
			Result:      resp.ScriptResult.Result,
			ConsoleLogs: resp.ScriptResult.ConsoleLogs,
			Error:       resp.ScriptResult.Error,
			Variables:   resp.ScriptResult.Variables,
			DurationMs:  resp.ScriptResult.DurationMs,
		}
	}

	return result
}

// ExecuteWorkflowRequest 执行工作流请求
type ExecuteWorkflowRequest struct {
	Workflow      *types.Workflow        `json:"workflow,omitempty"`
	WorkflowJSON  string                 `json:"workflowJson,omitempty"`
	EnvID         int64                  `json:"envId,omitempty"`
	Variables     map[string]interface{} `json:"variables,omitempty"`
	EnvVars       map[string]interface{} `json:"envVars,omitempty"`
	Timeout       int                    `json:"timeout,omitempty"`
	Mode          string                 `json:"mode,omitempty"`
	ExecutorType  string                 `json:"executorType,omitempty"`
	SlaveID       string                 `json:"slaveId,omitempty"`
	SessionID     string                 `json:"sessionId,omitempty"`
	SelectedSteps []string               `json:"selectedSteps,omitempty"`
}

// ExecuteWorkflowResponse 执行工作流响应
type ExecuteWorkflowResponse struct {
	Success     bool                     `json:"success"`
	ExecutionID string                   `json:"executionId,omitempty"`
	SessionID   string                   `json:"sessionId,omitempty"`
	Summary     *types.ExecuteSummary  `json:"summary,omitempty"`
	Error       string                   `json:"error,omitempty"`
}

// ExecuteWorkflowBlocking 阻塞式执行工作流
func (c *WorkflowEngineClient) ExecuteWorkflowBlocking(ctx context.Context, req *ExecuteWorkflowRequest) (*ExecuteWorkflowResponse, error) {
	if c.embedded {
		return c.executeWorkflowBlockingEmbedded(ctx, req)
	}
	return c.executeWorkflowBlockingExternal(req)
}

// executeWorkflowBlockingEmbedded 内置模式阻塞式执行
func (c *WorkflowEngineClient) executeWorkflowBlockingEmbedded(ctx context.Context, req *ExecuteWorkflowRequest) (*ExecuteWorkflowResponse, error) {
	engine := workflow.GetEngine()
	if engine == nil {
		return nil, fmt.Errorf("工作流引擎未初始化")
	}

	// 调用内置引擎的 ExecuteWorkflow 方法
	result, err := engine.ExecuteWorkflowBlocking(ctx, &types.ExecuteWorkflowRequest{
		Workflow:      req.Workflow,
		WorkflowJSON:  req.WorkflowJSON,
		EnvID:         req.EnvID,
		Variables:     req.Variables,
		EnvVars:       req.EnvVars,
		Timeout:       req.Timeout,
		Mode:          req.Mode,
		ExecutorType:  req.ExecutorType,
		SlaveID:       req.SlaveID,
		SessionID:     req.SessionID,
		SelectedSteps: req.SelectedSteps,
	})
	if err != nil {
		return nil, err
	}

	return &ExecuteWorkflowResponse{
		Success:     result.Success,
		ExecutionID: result.ExecutionID,
		SessionID:   result.SessionID,
		Summary:     result.Summary,
		Error:       result.Error,
	}, nil
}

// executeWorkflowBlockingExternal 外部模式阻塞式执行
func (c *WorkflowEngineClient) executeWorkflowBlockingExternal(req *ExecuteWorkflowRequest) (*ExecuteWorkflowResponse, error) {
	body, err := c.doRequest("POST", "/api/v1/execute", req)
	if err != nil {
		return nil, fmt.Errorf("执行工作流请求失败: %w", err)
	}

	var result ExecuteWorkflowResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	return &result, nil
}

// ExecuteWorkflowStreamURL 获取流式执行工作流的 URL
func (c *WorkflowEngineClient) ExecuteWorkflowStreamURL() string {
	return c.baseURL + "/api/v1/execute/stream"
}

// IsEmbedded 是否为内置模式
func (c *WorkflowEngineClient) IsEmbedded() bool {
	return c.embedded
}

// GetBaseURL 获取基础 URL
func (c *WorkflowEngineClient) GetBaseURL() string {
	return c.baseURL
}
