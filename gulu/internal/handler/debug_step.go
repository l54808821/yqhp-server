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
	ID      string                 `json:"id"`
	Type    string                 `json:"type"`
	Enabled bool                   `json:"enabled"`
	Name    string                 `json:"name,omitempty"`
	Config  map[string]interface{} `json:"config"`
}

// DebugStepResponse 单步调试响应
type DebugStepResponse struct {
	Success              bool              `json:"success"`
	Response             *DebugHTTPResp    `json:"response,omitempty"`
	PreProcessorResults  []KeywordResult   `json:"preProcessorResults,omitempty"`
	PostProcessorResults []KeywordResult   `json:"postProcessorResults,omitempty"`
	AssertionResults     []AssertionResult `json:"assertionResults,omitempty"`
	ConsoleLogs          []string          `json:"consoleLogs,omitempty"`
	ActualRequest        *ActualRequest    `json:"actualRequest,omitempty"`
	Error                string            `json:"error,omitempty"`
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

	if req.NodeConfig.Type != "http" {
		return response.Error(c, "目前只支持 HTTP 节点的单步调试")
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

	// 执行 HTTP 请求
	httpResult, actualReq, err := h.executeHTTPRequest(ctx, nodeConfig.Config, variables)
	if err != nil {
		result.Success = false
		result.Error = "HTTP 请求执行失败: " + err.Error()
		return result, nil
	}

	result.Response = httpResult
	result.ActualRequest = actualReq

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
