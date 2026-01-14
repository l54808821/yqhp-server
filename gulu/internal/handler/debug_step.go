package handler

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"yqhp/common/response"
	"yqhp/gulu/internal/executor"
	"yqhp/gulu/internal/logic"
	"yqhp/workflow-engine/pkg/script"

	"github.com/gofiber/fiber/v2"
)

// DebugStepHandler 单步调试处理器
type DebugStepHandler struct {
	sessionManager *executor.SessionManager
}

// NewDebugStepHandler 创建单步调试处理器
func NewDebugStepHandler(sessionMgr *executor.SessionManager) *DebugStepHandler {
	return &DebugStepHandler{
		sessionManager: sessionMgr,
	}
}

// DebugStepRequest 单步调试请求
type DebugStepRequest struct {
	NodeConfig *DebugNodeConfig       `json:"nodeConfig"`
	EnvID      int64                  `json:"envId,omitempty"`
	Variables  map[string]interface{} `json:"variables,omitempty"`
	SessionID  string                 `json:"sessionId,omitempty"` // 调试会话 ID，用于获取会话变量
}

// DebugNodeConfig 调试节点配置
type DebugNodeConfig struct {
	ID             string                 `json:"id"`
	Type           string                 `json:"type"`
	Name           string                 `json:"name"`
	Config         map[string]interface{} `json:"config"` // 通用配置，根据 Type 解析
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

// DebugStepResponse 单步调试响应
type DebugStepResponse struct {
	Success              bool               `json:"success"`
	Response             *DebugHTTPResp     `json:"response,omitempty"`
	ScriptResult         *DebugScriptResult `json:"scriptResult,omitempty"`
	PreProcessorResults  []KeywordResult    `json:"preProcessorResults,omitempty"`
	PostProcessorResults []KeywordResult    `json:"postProcessorResults,omitempty"`
	AssertionResults     []AssertionResult  `json:"assertionResults,omitempty"`
	ConsoleLogs          []string           `json:"consoleLogs,omitempty"`
	ActualRequest        *ActualRequest     `json:"actualRequest,omitempty"`
	Error                string             `json:"error,omitempty"`
}

// DebugScriptResult 脚本执行结果
type DebugScriptResult struct {
	Script      string                 `json:"script"`          // 执行的脚本内容
	Language    string                 `json:"language"`        // 脚本语言
	Result      interface{}            `json:"result"`          // 脚本返回值
	ConsoleLogs []string               `json:"console_logs"`    // 控制台日志
	Error       string                 `json:"error,omitempty"` // 错误信息
	Variables   map[string]interface{} `json:"variables"`       // 修改的变量
	DurationMs  int64                  `json:"duration_ms"`     // 执行耗时（毫秒）
}

// DebugHTTPResp HTTP 响应
type DebugHTTPResp struct {
	StatusCode int               `json:"statusCode"`
	StatusText string            `json:"statusText"`
	Duration   int64             `json:"duration"`
	Size       int               `json:"size"`
	Headers    map[string]string `json:"headers"`
	Cookies    map[string]string `json:"cookies,omitempty"`
	Body       string            `json:"body"`
	BodyType   string            `json:"bodyType"`
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

// DebugStep 单步调试 HTTP 节点
// POST /api/debug/step
func (h *DebugStepHandler) DebugStep(c *fiber.Ctx) error {
	var req DebugStepRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败: "+err.Error())
	}

	if req.NodeConfig == nil {
		return response.Error(c, "节点配置不能为空")
	}

	if req.NodeConfig.Type != "http" && req.NodeConfig.Type != "script" {
		return response.Error(c, "目前只支持 HTTP 和脚本节点的单步调试")
	}

	// 合并变量：会话变量 + 请求变量
	variables := make(map[string]interface{})

	// 1. 如果有 session_id，从会话中获取变量
	if req.SessionID != "" && h.sessionManager != nil {
		session, ok := h.sessionManager.GetSession(req.SessionID)
		if ok {
			// 从会话中获取变量上下文
			sessionVars := session.GetVariables()
			for k, v := range sessionVars {
				variables[k] = v
			}
			fmt.Printf("[DEBUG] 从会话 %s 获取到 %d 个变量\n", req.SessionID, len(sessionVars))
		}
	}

	// 2. 如果有环境 ID，获取环境变量
	if req.EnvID > 0 {
		varLogic := logic.NewVarLogic(c.UserContext())
		envVars, err := varLogic.GetVarsByEnvID(req.EnvID)
		if err == nil {
			for _, v := range envVars {
				if v.Value != nil {
					variables[v.Key] = *v.Value
				}
			}
			fmt.Printf("[DEBUG] 从环境 %d 获取到 %d 个变量\n", req.EnvID, len(envVars))
		}
	}

	// 3. 请求中的变量优先级最高
	for k, v := range req.Variables {
		variables[k] = v
	}

	// 执行单步调试
	result, err := h.executeDebugStep(c.Context(), req.NodeConfig, variables)
	if err != nil {
		return response.Error(c, "执行失败: "+err.Error())
	}

	return response.Success(c, result)
}

// executeDebugStep 执行单步调试
func (h *DebugStepHandler) executeDebugStep(ctx context.Context, nodeConfig *DebugNodeConfig, variables map[string]interface{}) (*DebugStepResponse, error) {
	result := &DebugStepResponse{
		Success:              true,
		PreProcessorResults:  make([]KeywordResult, 0),
		PostProcessorResults: make([]KeywordResult, 0),
		AssertionResults:     make([]AssertionResult, 0),
		ConsoleLogs:          make([]string, 0),
	}

	switch nodeConfig.Type {
	case "http":
		// 解析 HTTP 配置
		httpConfig, err := parseHTTPConfig(nodeConfig.Config)
		if err != nil {
			result.Success = false
			result.Error = "解析 HTTP 配置失败: " + err.Error()
			return result, nil
		}
		// 执行 HTTP 请求
		httpResult, actualReq, err := h.executeHTTPRequest(ctx, httpConfig, variables)
		if err != nil {
			result.Success = false
			result.Error = "HTTP 请求执行失败: " + err.Error()
			return result, nil
		}
		result.Response = httpResult
		result.ActualRequest = actualReq

	case "script":
		// 执行脚本
		scriptResult, err := h.executeScript(ctx, nodeConfig, variables)
		if err != nil {
			result.Success = false
			result.Error = "脚本执行失败: " + err.Error()
			return result, nil
		}
		result.ScriptResult = scriptResult
		result.ConsoleLogs = scriptResult.ConsoleLogs
		if scriptResult.Error != "" {
			result.Success = false
			result.Error = scriptResult.Error
		}

	default:
		result.Success = false
		result.Error = fmt.Sprintf("不支持的节点类型: %s", nodeConfig.Type)
	}

	return result, nil
}

// executeHTTPRequest 执行 HTTP 请求
func (h *DebugStepHandler) executeHTTPRequest(ctx context.Context, config *DebugHTTPConfig, variables map[string]interface{}) (*DebugHTTPResp, *ActualRequest, error) {
	// 替换变量
	requestURL := replaceVariables(config.URL, variables)

	// 构建查询参数
	if len(config.Params) > 0 {
		params := url.Values{}
		for _, p := range config.Params {
			if p.Enabled && p.Key != "" {
				params.Add(p.Key, replaceVariables(p.Value, variables))
			}
		}
		if len(params) > 0 {
			if strings.Contains(requestURL, "?") {
				requestURL += "&" + params.Encode()
			} else {
				requestURL += "?" + params.Encode()
			}
		}
	}

	// 构建请求体
	var bodyReader io.Reader
	var bodyStr string
	contentType := ""

	if config.Body != nil {
		switch config.Body.Type {
		case "json":
			bodyStr = replaceVariables(config.Body.Raw, variables)
			bodyReader = strings.NewReader(bodyStr)
			contentType = "application/json"
		case "xml":
			bodyStr = replaceVariables(config.Body.Raw, variables)
			bodyReader = strings.NewReader(bodyStr)
			contentType = "application/xml"
		case "text":
			bodyStr = replaceVariables(config.Body.Raw, variables)
			bodyReader = strings.NewReader(bodyStr)
			contentType = "text/plain"
		case "form-data":
			formData := url.Values{}
			for _, item := range config.Body.FormData {
				if item.Enabled && item.Key != "" {
					formData.Add(item.Key, replaceVariables(item.Value, variables))
				}
			}
			bodyStr = formData.Encode()
			bodyReader = strings.NewReader(bodyStr)
			contentType = "multipart/form-data"
		case "x-www-form-urlencoded":
			formData := url.Values{}
			for _, item := range config.Body.URLEncoded {
				if item.Enabled && item.Key != "" {
					formData.Add(item.Key, replaceVariables(item.Value, variables))
				}
			}
			bodyStr = formData.Encode()
			bodyReader = strings.NewReader(bodyStr)
			contentType = "application/x-www-form-urlencoded"
		}
	}

	// 创建请求
	req, err := http.NewRequestWithContext(ctx, config.Method, requestURL, bodyReader)
	if err != nil {
		return nil, nil, fmt.Errorf("创建请求失败: %w", err)
	}

	// 设置请求头
	actualHeaders := make(map[string]string)
	for _, h := range config.Headers {
		if h.Enabled && h.Key != "" {
			value := replaceVariables(h.Value, variables)
			req.Header.Set(h.Key, value)
			actualHeaders[h.Key] = value
		}
	}

	// 设置 Content-Type
	if contentType != "" && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", contentType)
		actualHeaders["Content-Type"] = contentType
	}

	// 设置认证
	if config.Auth != nil {
		switch config.Auth.Type {
		case "basic":
			if config.Auth.Basic != nil {
				auth := config.Auth.Basic.Username + ":" + config.Auth.Basic.Password
				encoded := base64.StdEncoding.EncodeToString([]byte(auth))
				req.Header.Set("Authorization", "Basic "+encoded)
				actualHeaders["Authorization"] = "Basic " + encoded
			}
		case "bearer":
			if config.Auth.Bearer != nil {
				token := replaceVariables(config.Auth.Bearer.Token, variables)
				req.Header.Set("Authorization", "Bearer "+token)
				actualHeaders["Authorization"] = "Bearer " + token
			}
		case "apikey":
			if config.Auth.APIKey != nil {
				key := config.Auth.APIKey.Key
				value := replaceVariables(config.Auth.APIKey.Value, variables)
				if config.Auth.APIKey.AddTo == "header" {
					req.Header.Set(key, value)
					actualHeaders[key] = value
				}
			}
		}
	}

	// 设置 Cookies
	for _, cookie := range config.Cookies {
		if cookie.Enabled && cookie.Key != "" {
			req.AddCookie(&http.Cookie{
				Name:  cookie.Key,
				Value: replaceVariables(cookie.Value, variables),
			})
		}
	}

	// 创建 HTTP 客户端
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	// 设置超时
	if config.Settings != nil {
		if config.Settings.ReadTimeout > 0 {
			client.Timeout = time.Duration(config.Settings.ReadTimeout) * time.Millisecond
		}
		// 设置重定向
		if !config.Settings.FollowRedirects {
			client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			}
		}
	}

	// 执行请求
	startTime := time.Now()
	resp, err := client.Do(req)
	duration := time.Since(startTime).Milliseconds()

	if err != nil {
		return nil, nil, fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	// 读取响应体
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("读取响应失败: %w", err)
	}

	// 解析响应头
	respHeaders := make(map[string]string)
	for k, v := range resp.Header {
		if len(v) > 0 {
			respHeaders[k] = v[0]
		}
	}

	// 解析 Cookies
	respCookies := make(map[string]string)
	for _, cookie := range resp.Cookies() {
		respCookies[cookie.Name] = cookie.Value
	}

	// 构建响应
	httpResp := &DebugHTTPResp{
		StatusCode: resp.StatusCode,
		StatusText: resp.Status,
		Duration:   duration,
		Size:       len(body),
		Headers:    respHeaders,
		Cookies:    respCookies,
		Body:       string(body),
		BodyType:   detectBodyType(string(body)),
	}

	// 构建实际请求信息
	actualReq := &ActualRequest{
		URL:     requestURL,
		Method:  config.Method,
		Headers: actualHeaders,
		Body:    bodyStr,
	}

	return httpResp, actualReq, nil
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

// parseHTTPConfig 从 map 解析 HTTP 配置
func parseHTTPConfig(config map[string]interface{}) (*DebugHTTPConfig, error) {
	if config == nil {
		return nil, fmt.Errorf("配置为空")
	}

	httpConfig := &DebugHTTPConfig{}

	// 解析基本字段
	if method, ok := config["method"].(string); ok {
		httpConfig.Method = method
	}
	if urlStr, ok := config["url"].(string); ok {
		httpConfig.URL = urlStr
	}
	if domainCode, ok := config["domainCode"].(string); ok {
		httpConfig.DomainCode = domainCode
	}

	// 解析 params
	if params, ok := config["params"].([]interface{}); ok {
		httpConfig.Params = parseParamItems(params)
	}

	// 解析 headers
	if headers, ok := config["headers"].([]interface{}); ok {
		httpConfig.Headers = parseParamItems(headers)
	}

	// 解析 cookies
	if cookies, ok := config["cookies"].([]interface{}); ok {
		httpConfig.Cookies = parseParamItems(cookies)
	}

	// 解析 body
	if body, ok := config["body"].(map[string]interface{}); ok {
		httpConfig.Body = parseBodyConfig(body)
	}

	// 解析 auth
	if auth, ok := config["auth"].(map[string]interface{}); ok {
		httpConfig.Auth = parseAuthConfig(auth)
	}

	// 解析 settings
	if settings, ok := config["settings"].(map[string]interface{}); ok {
		httpConfig.Settings = parseSettingsConfig(settings)
	}

	return httpConfig, nil
}

// parseParamItems 解析参数列表
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

// parseBodyConfig 解析请求体配置
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

// parseAuthConfig 解析认证配置
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

// parseSettingsConfig 解析设置配置
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
	if len(body) > 5 && (strings.HasPrefix(body, "<html") || strings.HasPrefix(body, "<!DOC")) {
		return "html"
	}

	return "text"
}

// executeScript 执行脚本
func (h *DebugStepHandler) executeScript(ctx context.Context, nodeConfig *DebugNodeConfig, variables map[string]interface{}) (*DebugScriptResult, error) {
	startTime := time.Now()

	// 从 config 中获取脚本配置
	var scriptCode string
	var language string
	var timeout int

	if nodeConfig.Config != nil {
		if script, ok := nodeConfig.Config["script"].(string); ok {
			scriptCode = script
		}
		if lang, ok := nodeConfig.Config["language"].(string); ok {
			language = lang
		}
		if t, ok := nodeConfig.Config["timeout"].(float64); ok {
			timeout = int(t)
		} else if t, ok := nodeConfig.Config["timeout"].(int); ok {
			timeout = t
		}
	}

	if scriptCode == "" {
		return &DebugScriptResult{
			Error:      "脚本内容为空",
			DurationMs: time.Since(startTime).Milliseconds(),
		}, nil
	}

	// 默认语言为 JavaScript
	if language == "" {
		language = "javascript"
	}

	// 默认超时 60 秒
	if timeout <= 0 {
		timeout = 60
	}

	// 准备运行时配置
	rtConfig := &script.JSRuntimeConfig{
		Variables: make(map[string]interface{}),
		EnvVars:   make(map[string]interface{}),
	}

	// 注入变量
	for k, v := range variables {
		rtConfig.Variables[k] = v
		// 提取环境变量
		if len(k) > 4 && k[:4] == "env_" {
			rtConfig.EnvVars[k[4:]] = v
		}
	}

	// 创建 JS 运行时
	runtime := script.NewJSRuntime(rtConfig)

	// 执行脚本
	execResult, err := runtime.Execute(scriptCode, time.Duration(timeout)*time.Second)

	// 构建结果
	result := &DebugScriptResult{
		Script:      scriptCode,
		Language:    language,
		ConsoleLogs: execResult.ConsoleLogs,
		Variables:   execResult.Variables,
		DurationMs:  time.Since(startTime).Milliseconds(),
	}

	if err != nil {
		result.Error = err.Error()
		return result, nil
	}

	result.Result = execResult.Value

	return result, nil
}
