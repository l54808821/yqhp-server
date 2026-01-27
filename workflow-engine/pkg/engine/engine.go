// Package engine 提供工作流引擎的公共 API
package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"yqhp/workflow-engine/internal/executor"
	"yqhp/workflow-engine/internal/master"
	"yqhp/workflow-engine/pkg/script"
	"yqhp/workflow-engine/pkg/types"
)

// Config 引擎配置
type Config struct {
	// HTTPAddress HTTP 服务地址
	HTTPAddress string
	// Standalone 独立模式（无需 Slave 即可执行）
	Standalone bool
	// MaxExecutions 最大并发执行数
	MaxExecutions int
	// HeartbeatTimeout 心跳超时
	HeartbeatTimeout time.Duration
}

// DefaultConfig 返回默认配置
func DefaultConfig() *Config {
	return &Config{
		HTTPAddress:      ":8080",
		Standalone:       true,
		MaxExecutions:    100,
		HeartbeatTimeout: 30 * time.Second,
	}
}

// Engine 工作流引擎
type Engine struct {
	config   *Config
	master   *master.WorkflowMaster
	registry master.SlaveRegistry
	started  bool
	mu       sync.RWMutex
}

// New 创建新的工作流引擎
func New(cfg *Config) *Engine {
	if cfg == nil {
		cfg = DefaultConfig()
	}
	return &Engine{
		config: cfg,
	}
}

// Start 启动引擎
func (e *Engine) Start() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.started {
		return nil
	}

	// 创建 Master 配置
	masterCfg := &master.Config{
		Address:                 e.config.HTTPAddress,
		HeartbeatTimeout:        e.config.HeartbeatTimeout,
		HealthCheckInterval:     10 * time.Second,
		StandaloneMode:          e.config.Standalone,
		MaxConcurrentExecutions: e.config.MaxExecutions,
	}

	// 创建注册中心、调度器和聚合器
	e.registry = master.NewInMemorySlaveRegistry()
	scheduler := master.NewWorkflowScheduler(e.registry)
	aggregator := master.NewDefaultMetricsAggregator()

	// 创建并启动 Master
	e.master = master.NewWorkflowMaster(masterCfg, e.registry, scheduler, aggregator)

	ctx := context.Background()
	if err := e.master.Start(ctx); err != nil {
		return fmt.Errorf("启动 Master 失败: %w", err)
	}

	e.started = true
	return nil
}

// Stop 停止引擎
func (e *Engine) Stop() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if !e.started {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 停止 Master
	if e.master != nil {
		if err := e.master.Stop(ctx); err != nil {
			return fmt.Errorf("停止 Master 失败: %w", err)
		}
	}

	e.started = false
	return nil
}

// IsRunning 是否正在运行
func (e *Engine) IsRunning() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.started
}

// GetSlaves 获取所有 Slave
func (e *Engine) GetSlaves(ctx context.Context) ([]*types.SlaveInfo, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if e.master == nil {
		return nil, fmt.Errorf("引擎未启动")
	}

	return e.master.GetSlaves(ctx)
}

// SubmitWorkflow 提交工作流执行
func (e *Engine) SubmitWorkflow(ctx context.Context, workflow *types.Workflow) (string, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if e.master == nil {
		return "", fmt.Errorf("引擎未启动")
	}

	return e.master.SubmitWorkflow(ctx, workflow)
}

// GetExecutionStatus 获取执行状态
func (e *Engine) GetExecutionStatus(ctx context.Context, executionID string) (*types.ExecutionState, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if e.master == nil {
		return nil, fmt.Errorf("引擎未启动")
	}

	return e.master.GetExecutionStatus(ctx, executionID)
}

// StopExecution 停止执行
func (e *Engine) StopExecution(ctx context.Context, executionID string) error {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if e.master == nil {
		return fmt.Errorf("引擎未启动")
	}

	return e.master.StopExecution(ctx, executionID)
}

// GetMetrics 获取执行指标
func (e *Engine) GetMetrics(ctx context.Context, executionID string) (*types.AggregatedMetrics, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if e.master == nil {
		return nil, fmt.Errorf("引擎未启动")
	}

	return e.master.GetMetrics(ctx, executionID)
}

// ListExecutions 列出所有执行
func (e *Engine) ListExecutions(ctx context.Context) ([]*types.ExecutionState, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if e.master == nil {
		return nil, fmt.Errorf("引擎未启动")
	}

	return e.master.ListExecutions(ctx)
}

// DebugStep 单步调试
func (e *Engine) DebugStep(ctx context.Context, req *types.DebugStepRequest) (*types.DebugStepResponse, error) {
	result := &types.DebugStepResponse{
		Success:          true,
		AssertionResults: make([]types.AssertionResult, 0),
		ConsoleLogs:      make([]types.ConsoleLogEntry, 0),
	}

	// 复制变量
	workingVars := make(map[string]interface{})
	for k, v := range req.Variables {
		workingVars[k] = v
	}

	envVars := make(map[string]interface{})
	for k, v := range req.EnvVars {
		envVars[k] = v
	}

	switch req.NodeConfig.Type {
	case "http":
		return e.executeHTTPDebugStep(ctx, req.NodeConfig, workingVars, envVars, result)
	case "script":
		return e.executeScriptDebugStep(ctx, req.NodeConfig, workingVars, envVars, result)
	default:
		result.Success = false
		result.Error = fmt.Sprintf("不支持的节点类型: %s", req.NodeConfig.Type)
		return result, nil
	}
}

// executeHTTPDebugStep 执行 HTTP 节点单步调试
// 重构：直接复用 HTTPExecutor，确保与流程执行逻辑一致
func (e *Engine) executeHTTPDebugStep(ctx context.Context, nodeConfig *types.DebugNodeConfig, workingVars, envVars map[string]interface{}, result *types.DebugStepResponse) (*types.DebugStepResponse, error) {
	// 1. 将 DebugNodeConfig 转换为标准 Step
	step, err := convertDebugNodeConfigToStep(nodeConfig, workingVars)
	if err != nil {
		result.Success = false
		result.Error = "转换节点配置失败: " + err.Error()
		return result, nil
	}

	// 2. 创建 ExecutionContext
	execCtx := executor.NewExecutionContext()
	// 设置临时变量
	for k, v := range workingVars {
		execCtx.SetVariable(k, v)
	}
	// 设置环境变量并标记
	for k, v := range envVars {
		execCtx.SetVariable(k, v)
		execCtx.MarkAsEnvVar(k)
	}

	// 3. 创建并调用 HTTPExecutor（复用流程执行的逻辑）
	httpExecutor := executor.NewFastHTTPExecutor()
	if err := httpExecutor.Init(ctx, nil); err != nil {
		result.Success = false
		result.Error = "初始化 HTTP 执行器失败: " + err.Error()
		return result, nil
	}
	defer httpExecutor.Cleanup(ctx)

	stepResult, execErr := httpExecutor.Execute(ctx, step, execCtx)

	// 4. 转换结果
	if stepResult != nil && stepResult.Output != nil {
		if httpResp, ok := stepResult.Output.(*types.HTTPResponseData); ok {
			result.Response = httpResp
			result.ActualRequest = httpResp.ActualRequest
			result.ConsoleLogs = httpResp.ConsoleLogs

			// 提取断言结果
			for _, entry := range httpResp.ConsoleLogs {
				if entry.Type == types.LogTypeProcessor && entry.Processor != nil && entry.Processor.Type == "assertion" {
					result.AssertionResults = append(result.AssertionResults, types.AssertionResult{
						Name:    entry.Processor.Name,
						Passed:  entry.Processor.Success,
						Message: entry.Processor.Message,
					})
				}
			}
		}
	}

	// 5. 处理错误
	if execErr != nil {
		result.Success = false
		result.Error = execErr.Error()
	} else if stepResult != nil && stepResult.Status != types.ResultStatusSuccess {
		result.Success = false
		if stepResult.Error != nil {
			result.Error = stepResult.Error.Error()
		}
	}

	return result, nil
}

// convertDebugNodeConfigToStep 将 DebugNodeConfig 转换为标准 types.Step
func convertDebugNodeConfigToStep(nodeConfig *types.DebugNodeConfig, variables map[string]interface{}) (*types.Step, error) {
	// 解析并转换 HTTP 配置
	httpConfig, err := parseHTTPConfigFromMap(nodeConfig.Config)
	if err != nil {
		return nil, fmt.Errorf("解析 HTTP 配置失败: %w", err)
	}

	// 转换为 Step.Config 格式
	stepConfig := convertHTTPConfigToStepConfig(httpConfig, variables)

	step := &types.Step{
		ID:             nodeConfig.ID,
		Name:           nodeConfig.Name,
		Type:           "http", // 使用 fasthttp 执行器
		Config:         stepConfig,
		PreProcessors:  convertKeywordToProcessors(nodeConfig.PreProcessors),
		PostProcessors: convertKeywordToProcessors(nodeConfig.PostProcessors),
	}

	return step, nil
}

// createVariableSnapshotEntry 创建变量快照日志条目
func createVariableSnapshotEntry(tempVars, envVars map[string]interface{}) types.ConsoleLogEntry {
	tempVarsAny := make(map[string]any, len(tempVars))
	for k, v := range tempVars {
		tempVarsAny[k] = v
	}

	envVarsAny := make(map[string]any, len(envVars))
	for k, v := range envVars {
		envVarsAny[k] = v
	}

	return types.NewSnapshotEntry(types.VariableSnapshotInfo{
		EnvVars:  envVarsAny,
		TempVars: tempVarsAny,
	})
}

// executeScriptDebugStep 执行 Script 节点单步调试
func (e *Engine) executeScriptDebugStep(ctx context.Context, nodeConfig *types.DebugNodeConfig, workingVars, envVars map[string]interface{}, result *types.DebugStepResponse) (*types.DebugStepResponse, error) {
	startTime := time.Now()

	var scriptCode string
	var language string
	var timeout int

	if nodeConfig.Config != nil {
		if sc, ok := nodeConfig.Config["script"].(string); ok {
			scriptCode = sc
		}
		if lang, ok := nodeConfig.Config["language"].(string); ok {
			language = lang
		}
		if t, ok := nodeConfig.Config["timeout"].(float64); ok {
			timeout = int(t)
		}
	}

	if scriptCode == "" {
		result.ScriptResult = &types.ScriptResponseData{
			Error:      "脚本内容为空",
			DurationMs: time.Since(startTime).Milliseconds(),
		}
		result.Success = false
		result.Error = "脚本内容为空"
		return result, nil
	}

	if language == "" {
		language = "javascript"
	}
	if timeout <= 0 {
		timeout = 60
	}

	rtConfig := &script.JSRuntimeConfig{
		Variables: make(map[string]interface{}),
		EnvVars:   make(map[string]interface{}),
	}

	for k, v := range workingVars {
		rtConfig.Variables[k] = v
	}
	for k, v := range envVars {
		rtConfig.EnvVars[k] = v
	}

	runtime := script.NewJSRuntime(rtConfig)
	execResult, err := runtime.Execute(scriptCode, time.Duration(timeout)*time.Second)

	// 转换日志
	consoleLogs := make([]types.ConsoleLogEntry, 0, len(execResult.ConsoleLogs))
	for _, log := range execResult.ConsoleLogs {
		consoleLogs = append(consoleLogs, types.NewLogEntry(log))
	}

	scriptResult := &types.ScriptResponseData{
		Script:      scriptCode,
		Language:    language,
		ConsoleLogs: consoleLogs,
		Variables:   execResult.Variables,
		DurationMs:  time.Since(startTime).Milliseconds(),
	}

	if err != nil {
		scriptResult.Error = err.Error()
		result.Success = false
		result.Error = err.Error()
	} else {
		scriptResult.Result = execResult.Value
	}

	result.ScriptResult = scriptResult
	result.ConsoleLogs = append(result.ConsoleLogs, consoleLogs...)

	// 创建变量快照（合并脚本执行后的变量）
	finalVars := make(map[string]interface{})
	for k, v := range workingVars {
		finalVars[k] = v
	}
	for k, v := range execResult.Variables {
		finalVars[k] = v
	}
	snapshot := createVariableSnapshotEntry(finalVars, envVars)
	result.ConsoleLogs = append(result.ConsoleLogs, snapshot)

	return result, nil
}

// httpConfig HTTP 配置
type httpConfig struct {
	Method   string
	URL      string
	Params   []paramItem
	Headers  []paramItem
	Cookies  []paramItem
	Body     *bodyConfig
	Auth     *authConfig
	Settings *settingsConfig
}

type paramItem struct {
	Enabled bool
	Key     string
	Value   string
}

type bodyConfig struct {
	Type       string
	FormData   []paramItem
	URLEncoded []paramItem
	Raw        string
}

type authConfig struct {
	Type     string
	Username string
	Password string
	Token    string
	Key      string
	Value    string
	AddTo    string
}

type settingsConfig struct {
	ConnectTimeout  int
	ReadTimeout     int
	FollowRedirects bool
	MaxRedirects    int
	VerifySSL       bool
}

// parseHTTPConfigFromMap 从 map 解析 HTTP 配置
func parseHTTPConfigFromMap(config map[string]interface{}) (*httpConfig, error) {
	if config == nil {
		return nil, fmt.Errorf("配置为空")
	}

	httpCfg := &httpConfig{}

	if method, ok := config["method"].(string); ok {
		httpCfg.Method = method
	}
	if urlStr, ok := config["url"].(string); ok {
		httpCfg.URL = urlStr
	}

	if params, ok := config["params"].([]interface{}); ok {
		httpCfg.Params = parseParamItemsFromSlice(params)
	}
	if headers, ok := config["headers"].([]interface{}); ok {
		httpCfg.Headers = parseParamItemsFromSlice(headers)
	}
	if cookies, ok := config["cookies"].([]interface{}); ok {
		httpCfg.Cookies = parseParamItemsFromSlice(cookies)
	}
	if body, ok := config["body"].(map[string]interface{}); ok {
		httpCfg.Body = parseBodyConfigFromMap(body)
	}
	if auth, ok := config["auth"].(map[string]interface{}); ok {
		httpCfg.Auth = parseAuthConfigFromMap(auth)
	}
	if settings, ok := config["settings"].(map[string]interface{}); ok {
		httpCfg.Settings = parseSettingsConfigFromMap(settings)
	}

	return httpCfg, nil
}

func parseParamItemsFromSlice(items []interface{}) []paramItem {
	result := make([]paramItem, 0, len(items))
	for _, item := range items {
		if m, ok := item.(map[string]interface{}); ok {
			p := paramItem{}
			if enabled, ok := m["enabled"].(bool); ok {
				p.Enabled = enabled
			}
			if key, ok := m["key"].(string); ok {
				p.Key = key
			}
			if value, ok := m["value"].(string); ok {
				p.Value = value
			}
			result = append(result, p)
		}
	}
	return result
}

func parseBodyConfigFromMap(body map[string]interface{}) *bodyConfig {
	cfg := &bodyConfig{}
	if t, ok := body["type"].(string); ok {
		cfg.Type = t
	}
	if raw, ok := body["raw"].(string); ok {
		cfg.Raw = raw
	}
	if formData, ok := body["formData"].([]interface{}); ok {
		cfg.FormData = parseParamItemsFromSlice(formData)
	}
	if urlencoded, ok := body["urlencoded"].([]interface{}); ok {
		cfg.URLEncoded = parseParamItemsFromSlice(urlencoded)
	}
	return cfg
}

func parseAuthConfigFromMap(auth map[string]interface{}) *authConfig {
	cfg := &authConfig{}
	if t, ok := auth["type"].(string); ok {
		cfg.Type = t
	}
	if basic, ok := auth["basic"].(map[string]interface{}); ok {
		if username, ok := basic["username"].(string); ok {
			cfg.Username = username
		}
		if password, ok := basic["password"].(string); ok {
			cfg.Password = password
		}
	}
	if bearer, ok := auth["bearer"].(map[string]interface{}); ok {
		if token, ok := bearer["token"].(string); ok {
			cfg.Token = token
		}
	}
	if apikey, ok := auth["apikey"].(map[string]interface{}); ok {
		if key, ok := apikey["key"].(string); ok {
			cfg.Key = key
		}
		if value, ok := apikey["value"].(string); ok {
			cfg.Value = value
		}
		if addTo, ok := apikey["addTo"].(string); ok {
			cfg.AddTo = addTo
		}
	}
	return cfg
}

func parseSettingsConfigFromMap(settings map[string]interface{}) *settingsConfig {
	cfg := &settingsConfig{}
	if connectTimeout, ok := settings["connectTimeout"].(float64); ok {
		cfg.ConnectTimeout = int(connectTimeout)
	}
	if readTimeout, ok := settings["readTimeout"].(float64); ok {
		cfg.ReadTimeout = int(readTimeout)
	}
	if followRedirects, ok := settings["followRedirects"].(bool); ok {
		cfg.FollowRedirects = followRedirects
	}
	if maxRedirects, ok := settings["maxRedirects"].(float64); ok {
		cfg.MaxRedirects = int(maxRedirects)
	}
	if verifySsl, ok := settings["verifySsl"].(bool); ok {
		cfg.VerifySSL = verifySsl
	}
	return cfg
}

// convertKeywordToProcessors 转换关键字配置为处理器
func convertKeywordToProcessors(keywords []types.KeywordConfig) []types.Processor {
	processors := make([]types.Processor, len(keywords))
	for i, kw := range keywords {
		processors[i] = types.Processor{
			ID:      kw.ID,
			Type:    kw.Type,
			Enabled: kw.Enabled,
			Name:    kw.Name,
			Config:  kw.Config,
		}
	}
	return processors
}

// convertHTTPConfigToStepConfig 转换 HTTP 配置为步骤配置
func convertHTTPConfigToStepConfig(config *httpConfig, variables map[string]interface{}) map[string]interface{} {
	stepConfig := map[string]interface{}{
		"method": config.Method,
		"url":    replaceVariablesInString(config.URL, variables),
	}

	// 转换参数
	if len(config.Params) > 0 {
		params := make(map[string]string)
		for _, p := range config.Params {
			if p.Enabled {
				params[p.Key] = p.Value
			}
		}
		stepConfig["params"] = params
	}

	// 转换请求头
	if len(config.Headers) > 0 {
		headers := make(map[string]string)
		for _, h := range config.Headers {
			if h.Enabled {
				headers[h.Key] = h.Value
			}
		}
		stepConfig["headers"] = headers
	}

	// 转换请求体
	if config.Body != nil {
		switch config.Body.Type {
		case "json", "xml", "text":
			stepConfig["body"] = config.Body.Raw
		case "form-data", "x-www-form-urlencoded":
			formData := make(map[string]string)
			items := config.Body.FormData
			if config.Body.Type == "x-www-form-urlencoded" {
				items = config.Body.URLEncoded
			}
			for _, item := range items {
				if item.Enabled {
					formData[item.Key] = item.Value
				}
			}
			stepConfig["body"] = formData
		}
	}

	// 转换超时设置
	if config.Settings != nil {
		timeout := make(map[string]interface{})
		if config.Settings.ConnectTimeout > 0 {
			timeout["connect"] = time.Duration(config.Settings.ConnectTimeout) * time.Millisecond
		}
		if config.Settings.ReadTimeout > 0 {
			timeout["request"] = time.Duration(config.Settings.ReadTimeout) * time.Millisecond
		}
		if len(timeout) > 0 {
			stepConfig["timeout"] = timeout
		}

		if !config.Settings.VerifySSL {
			stepConfig["ssl"] = map[string]interface{}{
				"verify": false,
			}
		}

		stepConfig["redirect"] = map[string]interface{}{
			"follow":        config.Settings.FollowRedirects,
			"max_redirects": config.Settings.MaxRedirects,
		}
	}

	return stepConfig
}

// replaceVariablesInString 替换字符串中的变量
func replaceVariablesInString(s string, variables map[string]interface{}) string {
	if variables == nil {
		return s
	}
	result := s
	for k, v := range variables {
		placeholder := "${" + k + "}"
		result = strings.ReplaceAll(result, placeholder, fmt.Sprintf("%v", v))
	}
	return result
}

// ExecuteWorkflowBlocking 阻塞式执行工作流
func (e *Engine) ExecuteWorkflowBlocking(ctx context.Context, req *types.ExecuteWorkflowRequest) (*types.ExecuteWorkflowResponse, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if e.master == nil {
		return nil, fmt.Errorf("引擎未启动")
	}

	// 解析工作流
	wf := req.Workflow
	if wf == nil && req.WorkflowJSON != "" {
		var parsedWf types.Workflow
		if err := json.Unmarshal([]byte(req.WorkflowJSON), &parsedWf); err != nil {
			return nil, fmt.Errorf("解析工作流 JSON 失败: %w", err)
		}
		wf = &parsedWf
	}

	if wf == nil {
		return nil, fmt.Errorf("工作流定义不能为空")
	}

	// 设置会话 ID
	if req.SessionID != "" {
		wf.ID = req.SessionID
	}

	// 合并变量
	if req.Variables != nil {
		if wf.Variables == nil {
			wf.Variables = make(map[string]interface{})
		}
		for k, v := range req.Variables {
			wf.Variables[k] = v
		}
	}

	// 设置超时
	timeout := 30 * time.Minute
	if req.Timeout > 0 {
		timeout = time.Duration(req.Timeout) * time.Second
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// 提交执行
	execID, err := e.master.SubmitWorkflow(ctx, wf)
	if err != nil {
		return &types.ExecuteWorkflowResponse{
			Success: false,
			Error:   "提交执行失败: " + err.Error(),
		}, nil
	}

	// 等待执行完成
	var execErr error
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	totalSteps := 0
	successSteps := 0
	failedSteps := 0

	for {
		select {
		case <-ctx.Done():
			execErr = ctx.Err()
			goto done
		case <-ticker.C:
			state, err := e.master.GetExecutionStatus(ctx, execID)
			if err != nil {
				continue
			}

			switch state.Status {
			case types.ExecutionStatusCompleted:
				goto done
			case types.ExecutionStatusFailed:
				if len(state.Errors) > 0 {
					execErr = fmt.Errorf(state.Errors[0].Message)
				} else {
					execErr = fmt.Errorf("执行失败")
				}
				goto done
			case types.ExecutionStatusAborted:
				execErr = fmt.Errorf("执行被中止")
				goto done
			}
		}
	}

done:
	status := "success"
	if execErr != nil {
		status = "failed"
	} else if failedSteps > 0 {
		status = "failed"
	}

	return &types.ExecuteWorkflowResponse{
		Success:     execErr == nil && failedSteps == 0,
		ExecutionID: execID,
		SessionID:   wf.ID,
		Summary: &types.ExecuteSummary{
			SessionID:     wf.ID,
			TotalSteps:    totalSteps,
			SuccessSteps:  successSteps,
			FailedSteps:   failedSteps,
			TotalDuration: 0, // 由调用方计算
			Status:        status,
		},
		Error: func() string {
			if execErr != nil {
				return execErr.Error()
			}
			return ""
		}(),
	}, nil
}
