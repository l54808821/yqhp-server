// Package rest provides the REST API server for the workflow execution engine.
package rest

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"time"

	"yqhp/workflow-engine/internal/executor"
	"yqhp/workflow-engine/internal/keyword"
	kwinit "yqhp/workflow-engine/internal/keyword/init"
	"yqhp/workflow-engine/pkg/types"

	"github.com/gofiber/fiber/v2"
)

// DebugStepRequest 单步调试请求
type DebugStepRequest struct {
	// 节点配置
	NodeConfig *DebugNodeConfig `json:"nodeConfig"`
	// 环境 ID
	EnvID int `json:"envId,omitempty"`
	// 变量上下文
	Variables map[string]any `json:"variables,omitempty"`
}

// DebugNodeConfig 调试节点配置
type DebugNodeConfig struct {
	ID             string           `json:"id"`
	Type           string           `json:"type"`
	Name           string           `json:"name"`
	Config         *DebugHTTPConfig `json:"config"`
	PreProcessors  []KeywordConfig  `json:"preProcessors,omitempty"`
	PostProcessors []KeywordConfig  `json:"postProcessors,omitempty"`
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
	ID      string         `json:"id"`
	Type    string         `json:"type"`
	Enabled bool           `json:"enabled"`
	Name    string         `json:"name,omitempty"`
	Config  map[string]any `json:"config"`
}

// DebugStepResponse 单步调试响应
type DebugStepResponse struct {
	Success bool `json:"success"`
	// HTTP 响应
	Response *DebugHTTPResponse `json:"response,omitempty"`
	// 前置处理器结果
	PreProcessorResults []KeywordResult `json:"preProcessorResults,omitempty"`
	// 后置处理器结果
	PostProcessorResults []KeywordResult `json:"postProcessorResults,omitempty"`
	// 断言结果
	AssertionResults []AssertionResult `json:"assertionResults,omitempty"`
	// 控制台日志
	ConsoleLogs []string `json:"consoleLogs,omitempty"`
	// 实际请求
	ActualRequest *ActualRequest `json:"actualRequest,omitempty"`
	// 错误信息
	Error string `json:"error,omitempty"`
}

// DebugHTTPResponse HTTP 响应
type DebugHTTPResponse struct {
	StatusCode int               `json:"statusCode"`
	StatusText string            `json:"statusText"`
	Duration   int64             `json:"duration"` // 毫秒
	Size       int               `json:"size"`     // 字节
	Headers    map[string]string `json:"headers"`
	Cookies    map[string]string `json:"cookies,omitempty"`
	Body       string            `json:"body"`
	BodyType   string            `json:"bodyType"`
}

// KeywordResult 关键字执行结果
type KeywordResult struct {
	KeywordID string         `json:"keywordId"`
	Type      string         `json:"type"`
	Name      string         `json:"name,omitempty"`
	Success   bool           `json:"success"`
	Message   string         `json:"message,omitempty"`
	Output    map[string]any `json:"output,omitempty"`
	Logs      []string       `json:"logs,omitempty"`
}

// AssertionResult 断言结果
type AssertionResult struct {
	Name    string `json:"name"`
	Passed  bool   `json:"passed"`
	Message string `json:"message,omitempty"`
}

// ActualRequest 实际请求
type ActualRequest struct {
	URL     string            `json:"url"`
	Method  string            `json:"method"`
	Headers map[string]string `json:"headers"`
	Body    string            `json:"body,omitempty"`
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

	if req.NodeConfig.Type != "http" {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{
			Error:   "invalid_request",
			Message: "Only HTTP node type is supported for debug",
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

// executeDebugStep 执行单步调试
func (s *Server) executeDebugStep(ctx context.Context, req *DebugStepRequest) (*DebugStepResponse, error) {
	result := &DebugStepResponse{
		Success:              true,
		PreProcessorResults:  make([]KeywordResult, 0),
		PostProcessorResults: make([]KeywordResult, 0),
		AssertionResults:     make([]AssertionResult, 0),
		ConsoleLogs:          make([]string, 0),
	}

	// 初始化关键字注册表
	registry := keyword.NewRegistry()
	kwinit.RegisterAllKeywords(registry)

	// 创建执行上下文
	execCtx := keyword.NewExecutionContextWithVars(req.Variables)

	// 创建脚本执行器
	scriptExecutor := keyword.NewScriptExecutor(registry)

	// 1. 执行前置处理器
	if len(req.NodeConfig.PreProcessors) > 0 {
		preActions := convertToActions(req.NodeConfig.PreProcessors)
		preRecords, err := scriptExecutor.ExecuteScripts(ctx, execCtx, preActions, "pre")
		for _, record := range preRecords {
			kwResult := KeywordResult{
				KeywordID: record.Keyword,
				Type:      record.Keyword,
				Success:   record.Success,
			}
			if record.Error != nil {
				kwResult.Message = record.Error.Error()
			}
			result.PreProcessorResults = append(result.PreProcessorResults, kwResult)
		}
		if err != nil {
			result.Success = false
			result.Error = "前置处理器执行失败: " + err.Error()
			return result, nil
		}
	}

	// 2. 执行 HTTP 请求
	httpResult, actualReq, err := s.executeHTTPRequest(ctx, req.NodeConfig.Config, execCtx)
	if err != nil {
		result.Success = false
		result.Error = "HTTP 请求执行失败: " + err.Error()
		return result, nil
	}
	result.Response = httpResult
	result.ActualRequest = actualReq

	// 设置响应到执行上下文
	execCtx.SetResponse(&keyword.ResponseData{
		Status:   httpResult.StatusCode,
		Headers:  httpResult.Headers,
		Body:     httpResult.Body,
		Duration: httpResult.Duration,
	})

	// 3. 执行后置处理器
	if len(req.NodeConfig.PostProcessors) > 0 {
		postActions := convertToActions(req.NodeConfig.PostProcessors)
		postRecords, err := scriptExecutor.ExecuteScripts(ctx, execCtx, postActions, "post")
		for _, record := range postRecords {
			kwResult := KeywordResult{
				KeywordID: record.Keyword,
				Type:      record.Keyword,
				Success:   record.Success,
			}
			if record.Error != nil {
				kwResult.Message = record.Error.Error()
			}
			result.PostProcessorResults = append(result.PostProcessorResults, kwResult)

			// 收集断言结果
			if record.Keyword == "assertion" || record.Keyword == "equals" ||
				record.Keyword == "contains" || record.Keyword == "greater_than" {
				result.AssertionResults = append(result.AssertionResults, AssertionResult{
					Name:    record.Keyword,
					Passed:  record.Success,
					Message: kwResult.Message,
				})
			}
		}
		if err != nil {
			// 后置处理器失败不影响整体成功状态，但记录错误
			result.ConsoleLogs = append(result.ConsoleLogs, "后置处理器执行失败: "+err.Error())
		}
	}

	// 收集控制台日志
	if logs, ok := execCtx.GetMetadata("console_logs"); ok {
		if logSlice, ok := logs.([]string); ok {
			result.ConsoleLogs = append(result.ConsoleLogs, logSlice...)
		}
	}

	// 收集测试结果
	if testResults, ok := execCtx.GetMetadata("test_results"); ok {
		if tests, ok := testResults.([]map[string]any); ok {
			for _, test := range tests {
				name, _ := test["name"].(string)
				passed, _ := test["passed"].(bool)
				errMsg, _ := test["error"].(string)
				result.AssertionResults = append(result.AssertionResults, AssertionResult{
					Name:    name,
					Passed:  passed,
					Message: errMsg,
				})
			}
		}
	}

	return result, nil
}

// executeHTTPRequest 执行 HTTP 请求
func (s *Server) executeHTTPRequest(ctx context.Context, config *DebugHTTPConfig, execCtx *keyword.ExecutionContext) (*DebugHTTPResponse, *ActualRequest, error) {
	// 转换配置为 executor 格式
	stepConfig := convertToStepConfig(config)

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
	for k, v := range execCtx.GetVariables() {
		httpExecCtx.SetVariable(k, v)
	}

	// 执行请求
	startTime := time.Now()
	result, err := httpExecutor.Execute(ctx, step, httpExecCtx)
	if err != nil {
		return nil, nil, err
	}

	// 解析响应
	response := &DebugHTTPResponse{
		Duration: time.Since(startTime).Milliseconds(),
		Headers:  make(map[string]string),
		Cookies:  make(map[string]string),
	}

	actualReq := &ActualRequest{
		Headers: make(map[string]string),
	}

	if result.Output != nil {
		// 解析输出
		if output, ok := result.Output.(*executor.HTTPResponse); ok {
			response.StatusCode = output.StatusCode
			response.StatusText = output.Status
			response.Size = len(output.BodyRaw)
			response.Body = output.BodyRaw
			response.BodyType = detectBodyType(output.BodyRaw)

			// 解析响应头
			for k, v := range output.Headers {
				if len(v) > 0 {
					response.Headers[k] = v[0]
				}
			}

			// 解析实际请求
			if output.Request != nil {
				actualReq.URL = output.Request.URL
				actualReq.Method = output.Request.Method
				actualReq.Headers = output.Request.Headers
				actualReq.Body = output.Request.Body
			}
		} else if outputMap, ok := result.Output.(map[string]any); ok {
			// 尝试从 map 解析
			if sc, ok := outputMap["status_code"].(int); ok {
				response.StatusCode = sc
			} else if sc, ok := outputMap["status_code"].(float64); ok {
				response.StatusCode = int(sc)
			}
			if status, ok := outputMap["status"].(string); ok {
				response.StatusText = status
			}
			if body, ok := outputMap["body_raw"].(string); ok {
				response.Body = body
				response.Size = len(body)
				response.BodyType = detectBodyType(body)
			}
			if headers, ok := outputMap["headers"].(map[string][]string); ok {
				for k, v := range headers {
					if len(v) > 0 {
						response.Headers[k] = v[0]
					}
				}
			}
			// 解析请求信息
			if reqInfo, ok := outputMap["request"].(map[string]any); ok {
				if url, ok := reqInfo["url"].(string); ok {
					actualReq.URL = url
				}
				if method, ok := reqInfo["method"].(string); ok {
					actualReq.Method = method
				}
				if headers, ok := reqInfo["headers"].(map[string]string); ok {
					actualReq.Headers = headers
				}
				if body, ok := reqInfo["body"].(string); ok {
					actualReq.Body = body
				}
			}
		}
	}

	return response, actualReq, nil
}

// convertToStepConfig 转换配置为步骤配置
func convertToStepConfig(config *DebugHTTPConfig) map[string]any {
	stepConfig := map[string]any{
		"method": config.Method,
		"url":    config.URL,
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

// convertToActions 转换关键字配置为 Action
func convertToActions(keywords []KeywordConfig) []keyword.Action {
	actions := make([]keyword.Action, 0, len(keywords))
	for _, kw := range keywords {
		if !kw.Enabled {
			continue
		}
		actions = append(actions, keyword.Action{
			Keyword: kw.Type,
			Params:  kw.Config,
		})
	}
	return actions
}

// detectBodyType 检测响应体类型
func detectBodyType(body string) string {
	if len(body) == 0 {
		return "text"
	}

	// 尝试解析为 JSON
	var js json.RawMessage
	if json.Unmarshal([]byte(body), &js) == nil {
		return "json"
	}

	// 检查是否为 XML
	if len(body) > 0 && body[0] == '<' {
		return "xml"
	}

	// 检查是否为 HTML
	if len(body) > 5 && (body[:5] == "<html" || body[:5] == "<!DOC") {
		return "html"
	}

	return "text"
}

// basicAuthEncode 编码 Basic Auth
func basicAuthEncode(username, password string) string {
	auth := username + ":" + password
	return base64.StdEncoding.EncodeToString([]byte(auth))
}
