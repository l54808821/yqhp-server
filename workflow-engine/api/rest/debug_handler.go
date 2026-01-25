// Package rest provides the REST API server for the workflow execution engine.
package rest

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"yqhp/workflow-engine/internal/executor"
	pkgExecutor "yqhp/workflow-engine/pkg/executor"
	"yqhp/workflow-engine/pkg/script"
	"yqhp/workflow-engine/pkg/types"

	"github.com/gofiber/fiber/v2"
)

// DebugStepRequest 单步调试请求
type DebugStepRequest struct {
	// 节点配置
	NodeConfig *DebugNodeConfig `json:"nodeConfig"`
	// 环境 ID
	EnvID int64 `json:"envId,omitempty"`
	// 变量上下文
	Variables map[string]interface{} `json:"variables,omitempty"`
	// 环境变量（从业务项目传入）
	EnvVars map[string]interface{} `json:"envVars,omitempty"`
	// 会话 ID（用于变量持久化）
	SessionID string `json:"sessionId,omitempty"`
}

// DebugNodeConfig 调试节点配置
type DebugNodeConfig struct {
	ID             string                 `json:"id"`
	Type           string                 `json:"type"`
	Name           string                 `json:"name"`
	Config         map[string]interface{} `json:"config"` // 通用配置，支持 http/script 等多种节点
	PreProcessors  []KeywordConfig        `json:"preProcessors,omitempty"`
	PostProcessors []KeywordConfig        `json:"postProcessors,omitempty"`
}

// DebugHTTPConfig HTTP 配置
type DebugHTTPConfig struct {
	Method     string            `json:"method"`
	URL        string            `json:"url"`
	DomainCode string            `json:"domainCode,omitempty"`
	Params     []ParamItem       `json:"params,omitempty"`
	Headers    []ParamItem       `json:"headers,omitempty"`
	Cookies    []ParamItem       `json:"cookies,omitempty"`
	Body       *DebugBodyConfig  `json:"body,omitempty"`
	Auth       *DebugAuthConfig  `json:"auth,omitempty"`
	Settings   *HTTPSettingsConf `json:"settings,omitempty"`
}

// ParamItem 参数项
type ParamItem struct {
	ID          string `json:"id"`
	Enabled     bool   `json:"enabled"`
	Key         string `json:"key"`
	Value       string `json:"value"`
	Type        string `json:"type,omitempty"`
	Description string `json:"description,omitempty"`
}

// DebugBodyConfig 请求体配置
type DebugBodyConfig struct {
	Type       string      `json:"type"`
	FormData   []ParamItem `json:"formData,omitempty"`
	URLEncoded []ParamItem `json:"urlencoded,omitempty"`
	Raw        string      `json:"raw,omitempty"`
}

// DebugAuthConfig 认证配置
type DebugAuthConfig struct {
	Type   string               `json:"type"`
	Basic  *DebugBasicAuth      `json:"basic,omitempty"`
	Bearer *DebugBearerAuth     `json:"bearer,omitempty"`
	APIKey *DebugAPIKeyAuthConf `json:"apikey,omitempty"`
}

// DebugBasicAuth Basic 认证
type DebugBasicAuth struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// DebugBearerAuth Bearer Token 认证
type DebugBearerAuth struct {
	Token string `json:"token"`
}

// DebugAPIKeyAuthConf API Key 认证
type DebugAPIKeyAuthConf struct {
	Key   string `json:"key"`
	Value string `json:"value"`
	AddTo string `json:"addTo"`
}

// HTTPSettingsConf HTTP 设置
type HTTPSettingsConf struct {
	ConnectTimeout  int  `json:"connectTimeout,omitempty"`
	ReadTimeout     int  `json:"readTimeout,omitempty"`
	FollowRedirects bool `json:"followRedirects"`
	MaxRedirects    int  `json:"maxRedirects,omitempty"`
	VerifySSL       bool `json:"verifySsl"`
	SaveCookies     bool `json:"saveCookies"`
}

// KeywordConfig 关键字配置
type KeywordConfig struct {
	ID      string                 `json:"id"`
	Type    string                 `json:"type"`
	Enabled bool                   `json:"enabled"`
	Name    string                 `json:"name,omitempty"`
	Config  map[string]interface{} `json:"config"`
}

// DebugStepResponse 单步调试响应（统一使用 types 包中的类型）
type DebugStepResponse struct {
	Success bool `json:"success"`
	// HTTP 响应（使用统一类型）
	Response *types.HTTPResponseData `json:"response,omitempty"`
	// 脚本执行结果
	ScriptResult *DebugScriptResult `json:"scriptResult,omitempty"`
	// 断言结果（使用统一类型）
	AssertionResults []types.AssertionResult `json:"assertionResults,omitempty"`
	// 控制台日志（统一格式）
	ConsoleLogs []types.ConsoleLogEntry `json:"consoleLogs,omitempty"`
	// 实际请求（使用统一类型）
	ActualRequest *types.ActualRequest `json:"actualRequest,omitempty"`
	// 错误信息
	Error string `json:"error,omitempty"`
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

// KeywordResult 关键字执行结果
type KeywordResult struct {
	KeywordID string                 `json:"keywordId"`
	Type      string                 `json:"type"`
	Name      string                 `json:"name,omitempty"`
	Success   bool                   `json:"success"`
	Message   string                 `json:"message,omitempty"`
	Output    map[string]interface{} `json:"output,omitempty"`
	Logs      []string               `json:"logs,omitempty"`
}

// setupDebugRoutes 设置调试路由
func (s *Server) setupDebugRoutes() {
	api := s.app.Group("/api/v1")
	api.Post("/debug/step", s.debugStep)
}

// debugStep 处理单步调试请求
// POST /api/v1/debug/step
func (s *Server) debugStep(c *fiber.Ctx) error {
	var req DebugStepRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{
			Error:   "invalid_request",
			Message: "Failed to parse request body: " + err.Error(),
		})
	}

	if req.NodeConfig == nil {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{
			Error:   "invalid_request",
			Message: "Node config is required",
		})
	}

	// 支持 http 和 script 节点类型
	if req.NodeConfig.Type != "http" && req.NodeConfig.Type != "script" {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{
			Error:   "invalid_request",
			Message: "Only HTTP and script node types are supported for debug",
		})
	}

	// 执行调试
	result, err := s.executeDebugStep(c.Context(), &req)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(DebugStepResponse{
			Success: false,
			Error:   err.Error(),
		})
	}

	return c.JSON(result)
}

// executeDebugStep 执行单步调试（统一处理 http 和 script 节点）
func (s *Server) executeDebugStep(ctx context.Context, req *DebugStepRequest) (*DebugStepResponse, error) {
	result := &DebugStepResponse{
		Success:          true,
		AssertionResults: make([]types.AssertionResult, 0),
		ConsoleLogs:      make([]types.ConsoleLogEntry, 0),
	}

	// 复制变量，避免污染原始变量
	workingVars := make(map[string]interface{})
	for k, v := range req.Variables {
		workingVars[k] = v
	}

	// 环境变量
	envVars := make(map[string]interface{})
	for k, v := range req.EnvVars {
		envVars[k] = v
	}

	switch req.NodeConfig.Type {
	case "http":
		return s.executeHTTPDebugStep(ctx, req.NodeConfig, workingVars, envVars, result)
	case "script":
		return s.executeScriptDebugStep(ctx, req.NodeConfig, workingVars, envVars, result)
	default:
		result.Success = false
		result.Error = fmt.Sprintf("不支持的节点类型: %s", req.NodeConfig.Type)
		return result, nil
	}
}

// executeHTTPDebugStep 执行 HTTP 节点单步调试
func (s *Server) executeHTTPDebugStep(ctx context.Context, nodeConfig *DebugNodeConfig, workingVars, envVars map[string]interface{}, result *DebugStepResponse) (*DebugStepResponse, error) {
	// 创建处理器执行器（使用统一实现）
	procExecutor := pkgExecutor.NewProcessorExecutor(workingVars, envVars)

	// 1. 执行前置处理器
	if len(nodeConfig.PreProcessors) > 0 {
		processors := convertToProcessors(nodeConfig.PreProcessors)
		preLogs := procExecutor.ExecuteProcessors(ctx, processors, "pre")
		result.ConsoleLogs = append(result.ConsoleLogs, preLogs...)
	}

	// 2. 解析 HTTP 配置
	httpConfig, err := parseHTTPConfig(nodeConfig.Config)
	if err != nil {
		result.Success = false
		result.Error = "解析 HTTP 配置失败: " + err.Error()
		return result, nil
	}

	// 3. 执行 HTTP 请求（使用更新后的变量）
	workingVars = procExecutor.GetVariables()
	httpResp, actualReq, err := s.executeHTTPRequestUnified(ctx, httpConfig, workingVars)
	if err != nil {
		result.Success = false
		result.Error = "HTTP 请求执行失败: " + err.Error()
		return result, nil
	}
	result.Response = httpResp
	result.ActualRequest = actualReq

	// 4. 执行后置处理器（传递响应数据）
	if len(nodeConfig.PostProcessors) > 0 {
		// 设置响应数据到处理器执行器
		procExecutor.SetResponse(httpResp.ToMap())

		processors := convertToProcessors(nodeConfig.PostProcessors)
		postLogs := procExecutor.ExecuteProcessors(ctx, processors, "post")
		result.ConsoleLogs = append(result.ConsoleLogs, postLogs...)

		// 提取断言结果
		for _, entry := range postLogs {
			if entry.Type == types.LogTypeProcessor && entry.Processor != nil && entry.Processor.Type == "assertion" {
				result.AssertionResults = append(result.AssertionResults, types.AssertionResult{
					Name:    entry.Processor.Name,
					Passed:  entry.Processor.Success,
					Message: entry.Processor.Message,
				})
			}
		}
	}

	return result, nil
}

// executeScriptDebugStep 执行 Script 节点单步调试
func (s *Server) executeScriptDebugStep(ctx context.Context, nodeConfig *DebugNodeConfig, workingVars, envVars map[string]interface{}, result *DebugStepResponse) (*DebugStepResponse, error) {
	scriptResult, err := s.executeScript(ctx, nodeConfig, workingVars, envVars)
	if err != nil {
		result.Success = false
		result.Error = "脚本执行失败: " + err.Error()
		return result, nil
	}
	result.ScriptResult = scriptResult
	// 脚本日志已经是 ConsoleLogEntry 格式，直接追加
	result.ConsoleLogs = append(result.ConsoleLogs, scriptResult.ConsoleLogs...)
	if scriptResult.Error != "" {
		result.Success = false
		result.Error = scriptResult.Error
	}
	return result, nil
}

// executeScript 执行脚本
func (s *Server) executeScript(ctx context.Context, nodeConfig *DebugNodeConfig, variables, envVars map[string]interface{}) (*DebugScriptResult, error) {
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
		return &DebugScriptResult{
			Error:      "脚本内容为空",
			DurationMs: time.Since(startTime).Milliseconds(),
		}, nil
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

	for k, v := range variables {
		rtConfig.Variables[k] = v
	}
	for k, v := range envVars {
		rtConfig.EnvVars[k] = v
	}

	runtime := script.NewJSRuntime(rtConfig)
	execResult, err := runtime.Execute(scriptCode, time.Duration(timeout)*time.Second)

	// 将字符串日志转换为 ConsoleLogEntry 格式
	consoleLogs := make([]types.ConsoleLogEntry, 0, len(execResult.ConsoleLogs))
	for _, log := range execResult.ConsoleLogs {
		consoleLogs = append(consoleLogs, types.NewLogEntry(log))
	}

	scriptResult := &DebugScriptResult{
		Script:      scriptCode,
		Language:    language,
		ConsoleLogs: consoleLogs,
		Variables:   execResult.Variables,
		DurationMs:  time.Since(startTime).Milliseconds(),
	}

	if err != nil {
		scriptResult.Error = err.Error()
		return scriptResult, nil
	}

	scriptResult.Result = execResult.Value
	return scriptResult, nil
}

// convertToProcessors 将 KeywordConfig 转换为 types.Processor
func convertToProcessors(keywords []KeywordConfig) []types.Processor {
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

// parseHTTPConfig 从 map 解析 HTTP 配置
func parseHTTPConfig(config map[string]interface{}) (*DebugHTTPConfig, error) {
	if config == nil {
		return nil, fmt.Errorf("配置为空")
	}

	httpConfig := &DebugHTTPConfig{}

	if method, ok := config["method"].(string); ok {
		httpConfig.Method = method
	}
	if urlStr, ok := config["url"].(string); ok {
		httpConfig.URL = urlStr
	}
	if domainCode, ok := config["domainCode"].(string); ok {
		httpConfig.DomainCode = domainCode
	}

	if params, ok := config["params"].([]interface{}); ok {
		httpConfig.Params = parseParamItems(params)
	}
	if headers, ok := config["headers"].([]interface{}); ok {
		httpConfig.Headers = parseParamItems(headers)
	}
	if cookies, ok := config["cookies"].([]interface{}); ok {
		httpConfig.Cookies = parseParamItems(cookies)
	}
	if body, ok := config["body"].(map[string]interface{}); ok {
		httpConfig.Body = parseBodyConfig(body)
	}
	if auth, ok := config["auth"].(map[string]interface{}); ok {
		httpConfig.Auth = parseAuthConfig(auth)
	}
	if settings, ok := config["settings"].(map[string]interface{}); ok {
		httpConfig.Settings = parseSettingsConfig(settings)
	}

	return httpConfig, nil
}

func parseParamItems(items []interface{}) []ParamItem {
	result := make([]ParamItem, 0, len(items))
	for _, item := range items {
		if m, ok := item.(map[string]interface{}); ok {
			p := ParamItem{}
			if id, ok := m["id"].(string); ok {
				p.ID = id
			}
			if enabled, ok := m["enabled"].(bool); ok {
				p.Enabled = enabled
			}
			if key, ok := m["key"].(string); ok {
				p.Key = key
			}
			if value, ok := m["value"].(string); ok {
				p.Value = value
			}
			if t, ok := m["type"].(string); ok {
				p.Type = t
			}
			if desc, ok := m["description"].(string); ok {
				p.Description = desc
			}
			result = append(result, p)
		}
	}
	return result
}

func parseBodyConfig(body map[string]interface{}) *DebugBodyConfig {
	config := &DebugBodyConfig{}
	if t, ok := body["type"].(string); ok {
		config.Type = t
	}
	if raw, ok := body["raw"].(string); ok {
		config.Raw = raw
	}
	if formData, ok := body["formData"].([]interface{}); ok {
		config.FormData = parseParamItems(formData)
	}
	if urlencoded, ok := body["urlencoded"].([]interface{}); ok {
		config.URLEncoded = parseParamItems(urlencoded)
	}
	return config
}

func parseAuthConfig(auth map[string]interface{}) *DebugAuthConfig {
	config := &DebugAuthConfig{}
	if t, ok := auth["type"].(string); ok {
		config.Type = t
	}
	if basic, ok := auth["basic"].(map[string]interface{}); ok {
		config.Basic = &DebugBasicAuth{}
		if username, ok := basic["username"].(string); ok {
			config.Basic.Username = username
		}
		if password, ok := basic["password"].(string); ok {
			config.Basic.Password = password
		}
	}
	if bearer, ok := auth["bearer"].(map[string]interface{}); ok {
		config.Bearer = &DebugBearerAuth{}
		if token, ok := bearer["token"].(string); ok {
			config.Bearer.Token = token
		}
	}
	if apikey, ok := auth["apikey"].(map[string]interface{}); ok {
		config.APIKey = &DebugAPIKeyAuthConf{}
		if key, ok := apikey["key"].(string); ok {
			config.APIKey.Key = key
		}
		if value, ok := apikey["value"].(string); ok {
			config.APIKey.Value = value
		}
		if addTo, ok := apikey["addTo"].(string); ok {
			config.APIKey.AddTo = addTo
		}
	}
	return config
}

func parseSettingsConfig(settings map[string]interface{}) *HTTPSettingsConf {
	config := &HTTPSettingsConf{}
	if connectTimeout, ok := settings["connectTimeout"].(float64); ok {
		config.ConnectTimeout = int(connectTimeout)
	}
	if readTimeout, ok := settings["readTimeout"].(float64); ok {
		config.ReadTimeout = int(readTimeout)
	}
	if followRedirects, ok := settings["followRedirects"].(bool); ok {
		config.FollowRedirects = followRedirects
	}
	if maxRedirects, ok := settings["maxRedirects"].(float64); ok {
		config.MaxRedirects = int(maxRedirects)
	}
	if verifySsl, ok := settings["verifySsl"].(bool); ok {
		config.VerifySSL = verifySsl
	}
	if saveCookies, ok := settings["saveCookies"].(bool); ok {
		config.SaveCookies = saveCookies
	}
	return config
}

// executeHTTPRequestUnified 执行 HTTP 请求（使用统一类型返回）
func (s *Server) executeHTTPRequestUnified(ctx context.Context, config *DebugHTTPConfig, variables map[string]interface{}) (*types.HTTPResponseData, *types.ActualRequest, error) {
	// 转换配置为 executor 格式
	stepConfig := convertToStepConfig(config, variables)

	// 创建 HTTP 执行器
	httpExecutor := executor.NewHTTPExecutor()
	if err := httpExecutor.Init(ctx, nil); err != nil {
		return nil, nil, err
	}
	defer httpExecutor.Cleanup(ctx)

	// 创建步骤
	step := &types.Step{
		ID:     "debug-step",
		Type:   "http-std",
		Config: stepConfig,
	}

	// 设置超时
	if config.Settings != nil && config.Settings.ReadTimeout > 0 {
		step.Timeout = time.Duration(config.Settings.ReadTimeout) * time.Millisecond
	}

	// 创建执行上下文
	httpExecCtx := executor.NewExecutionContext()
	// 复制变量
	for k, v := range variables {
		httpExecCtx.SetVariable(k, v)
	}

	// 执行请求
	startTime := time.Now()
	result, err := httpExecutor.Execute(ctx, step, httpExecCtx)
	if err != nil {
		return nil, nil, err
	}

	// 解析响应（输出已经是统一的 HTTPResponseData 类型）
	if result.Output != nil {
		if output, ok := result.Output.(*types.HTTPResponseData); ok {
			// 设置耗时（如果还没有设置）
			if output.Duration == 0 {
				output.Duration = time.Since(startTime).Milliseconds()
			}
			return output, output.ActualRequest, nil
		}
	}

	// 如果输出不是预期类型，返回空响应
	return &types.HTTPResponseData{
		Duration: time.Since(startTime).Milliseconds(),
		Headers:  make(map[string]string),
	}, nil, nil
}

// convertToStepConfig 转换配置为步骤配置
func convertToStepConfig(config *DebugHTTPConfig, variables map[string]interface{}) map[string]interface{} {
	stepConfig := map[string]interface{}{
		"method": config.Method,
		"url":    replaceVariables(config.URL, variables),
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

	// 转换认证
	if config.Auth != nil {
		switch config.Auth.Type {
		case "basic":
			if config.Auth.Basic != nil {
				if headers, ok := stepConfig["headers"].(map[string]string); ok {
					// 添加 Basic Auth 头
					headers["Authorization"] = "Basic " + basicAuthEncode(config.Auth.Basic.Username, config.Auth.Basic.Password)
				}
			}
		case "bearer":
			if config.Auth.Bearer != nil {
				if headers, ok := stepConfig["headers"].(map[string]string); ok {
					headers["Authorization"] = "Bearer " + config.Auth.Bearer.Token
				} else {
					stepConfig["headers"] = map[string]string{
						"Authorization": "Bearer " + config.Auth.Bearer.Token,
					}
				}
			}
		case "apikey":
			if config.Auth.APIKey != nil {
				if config.Auth.APIKey.AddTo == "header" {
					if headers, ok := stepConfig["headers"].(map[string]string); ok {
						headers[config.Auth.APIKey.Key] = config.Auth.APIKey.Value
					} else {
						stepConfig["headers"] = map[string]string{
							config.Auth.APIKey.Key: config.Auth.APIKey.Value,
						}
					}
				} else {
					if params, ok := stepConfig["params"].(map[string]string); ok {
						params[config.Auth.APIKey.Key] = config.Auth.APIKey.Value
					} else {
						stepConfig["params"] = map[string]string{
							config.Auth.APIKey.Key: config.Auth.APIKey.Value,
						}
					}
				}
			}
		}
	}

	// 转换超时设置
	if config.Settings != nil {
		timeout := make(map[string]any)
		if config.Settings.ConnectTimeout > 0 {
			timeout["connect"] = time.Duration(config.Settings.ConnectTimeout) * time.Millisecond
		}
		if config.Settings.ReadTimeout > 0 {
			timeout["request"] = time.Duration(config.Settings.ReadTimeout) * time.Millisecond
		}
		if len(timeout) > 0 {
			stepConfig["timeout"] = timeout
		}

		// SSL 配置
		if !config.Settings.VerifySSL {
			stepConfig["ssl"] = map[string]any{
				"verify": false,
			}
		}

		// 重定向配置
		stepConfig["redirect"] = map[string]any{
			"follow":        config.Settings.FollowRedirects,
			"max_redirects": config.Settings.MaxRedirects,
		}
	}

	return stepConfig
}

// replaceVariables 替换变量
func replaceVariables(s string, variables map[string]interface{}) string {
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

// basicAuthEncode 编码 Basic Auth
func basicAuthEncode(username, password string) string {
	auth := username + ":" + password
	return base64.StdEncoding.EncodeToString([]byte(auth))
}
