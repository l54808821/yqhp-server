package handler

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"yqhp/common/response"
	"yqhp/gulu/internal/executor"
	"yqhp/gulu/internal/logic"
	pkgExecutor "yqhp/workflow-engine/pkg/executor"
	"yqhp/workflow-engine/pkg/script"
	"yqhp/workflow-engine/pkg/types"

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

// KeywordConfig 关键字配置（与前端一致）
type KeywordConfig struct {
	ID      string                 `json:"id"`
	Type    string                 `json:"type"`
	Enabled bool                   `json:"enabled"`
	Name    string                 `json:"name,omitempty"`
	Config  map[string]interface{} `json:"config"`
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

// DebugStepResponse 单步调试响应（统一使用驼峰命名）
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

	// 合并变量
	variables := make(map[string]interface{})
	envVars := make(map[string]interface{})

	// 1. 从会话中获取变量
	if req.SessionID != "" && h.sessionManager != nil {
		session, ok := h.sessionManager.GetSession(req.SessionID)
		if ok {
			for k, v := range session.GetVariables() {
				variables[k] = v
			}
		}
	}

	// 2. 获取环境变量
	if req.EnvID > 0 {
		varLogic := logic.NewVarLogic(c.UserContext())
		vars, err := varLogic.GetVarsByEnvID(req.EnvID)
		if err == nil {
			for _, v := range vars {
				if v.Value != nil {
					envVars[v.Key] = *v.Value
				}
			}
		}
	}

	// 3. 请求变量优先级最高
	for k, v := range req.Variables {
		variables[k] = v
	}

	result, err := h.executeDebugStep(c.Context(), req.NodeConfig, variables, envVars)
	if err != nil {
		return response.Error(c, "执行失败: "+err.Error())
	}

	return response.Success(c, result)
}

// executeDebugStep 执行单步调试
func (h *DebugStepHandler) executeDebugStep(ctx context.Context, nodeConfig *DebugNodeConfig, variables, envVars map[string]interface{}) (*DebugStepResponse, error) {
	result := &DebugStepResponse{
		Success:          true,
		AssertionResults: make([]types.AssertionResult, 0),
		ConsoleLogs:      make([]types.ConsoleLogEntry, 0),
	}

	// 复制变量，避免污染原始变量
	workingVars := make(map[string]interface{})
	for k, v := range variables {
		workingVars[k] = v
	}

	switch nodeConfig.Type {
	case "http":
		// 创建处理器执行器（使用 workflow-engine 的统一实现）
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
		httpResp, actualReq, err := h.executeHTTPRequest(ctx, httpConfig, workingVars)
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

	case "script":
		scriptResult, err := h.executeScript(ctx, nodeConfig, workingVars, envVars)
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

	default:
		result.Success = false
		result.Error = fmt.Sprintf("不支持的节点类型: %s", nodeConfig.Type)
	}

	return result, nil
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

// executeHTTPRequest 执行 HTTP 请求
func (h *DebugStepHandler) executeHTTPRequest(ctx context.Context, config *DebugHTTPConfig, variables map[string]interface{}) (*types.HTTPResponseData, *types.ActualRequest, error) {
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

	if config.Settings != nil {
		if config.Settings.ReadTimeout > 0 {
			client.Timeout = time.Duration(config.Settings.ReadTimeout) * time.Millisecond
		}
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

	// 构建统一的响应结构
	httpResp := &types.HTTPResponseData{
		StatusCode: resp.StatusCode,
		StatusText: resp.Status,
		Duration:   duration,
		Size:       int64(len(body)),
		Headers:    respHeaders,
		Cookies:    respCookies,
		Body:       string(body),
		BodyType:   types.DetectBodyType(string(body)),
	}

	actualReq := &types.ActualRequest{
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

// executeScript 执行脚本
func (h *DebugStepHandler) executeScript(ctx context.Context, nodeConfig *DebugNodeConfig, variables, envVars map[string]interface{}) (*DebugScriptResult, error) {
	startTime := time.Now()

	var scriptCode string
	var language string
	var timeout int

	if nodeConfig.Config != nil {
		if s, ok := nodeConfig.Config["script"].(string); ok {
			scriptCode = s
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

	result := &DebugScriptResult{
		Script:      scriptCode,
		Language:    language,
		ConsoleLogs: consoleLogs,
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
