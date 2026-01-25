package executor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	pkgExecutor "yqhp/workflow-engine/pkg/executor"
	"yqhp/workflow-engine/pkg/types"
)

const (
	// HTTPExecutorType 是标准库 HTTP 执行器的类型标识符。
	// 注意：默认的 "http" 类型由 FastHTTP 执行器提供，此执行器使用 "http-std" 类型。
	HTTPExecutorType = "http-std"

	// HTTP 请求的默认超时时间。
	defaultHTTPTimeout = 30 * time.Second
)

// HTTPExecutor 执行 HTTP 请求步骤。
type HTTPExecutor struct {
	*BaseExecutor
	client       *http.Client
	globalConfig *HTTPGlobalConfig // 全局配置
}

// NewHTTPExecutor 创建一个新的 HTTP 执行器。
func NewHTTPExecutor() *HTTPExecutor {
	return &HTTPExecutor{
		BaseExecutor: NewBaseExecutor(HTTPExecutorType),
		globalConfig: DefaultHTTPGlobalConfig(),
	}
}

// NewHTTPExecutorWithConfig 使用全局配置创建一个新的 HTTP 执行器。
func NewHTTPExecutorWithConfig(globalConfig *HTTPGlobalConfig) *HTTPExecutor {
	if globalConfig == nil {
		globalConfig = DefaultHTTPGlobalConfig()
	}
	return &HTTPExecutor{
		BaseExecutor: NewBaseExecutor(HTTPExecutorType),
		globalConfig: globalConfig,
	}
}

// SetGlobalConfig 设置全局 HTTP 配置。
func (e *HTTPExecutor) SetGlobalConfig(config *HTTPGlobalConfig) {
	if config != nil {
		e.globalConfig = config
	}
}

// GetGlobalConfig 返回全局 HTTP 配置。
func (e *HTTPExecutor) GetGlobalConfig() *HTTPGlobalConfig {
	return e.globalConfig
}

// Init 使用配置初始化 HTTP 执行器。
func (e *HTTPExecutor) Init(ctx context.Context, config map[string]any) error {
	if err := e.BaseExecutor.Init(ctx, config); err != nil {
		return err
	}

	// 解析全局配置
	if httpConfig, ok := config["http"].(map[string]any); ok {
		e.parseGlobalConfig(httpConfig)
	}

	// 构建 HTTP 客户端
	if err := e.buildClient(); err != nil {
		return err
	}

	return nil
}

// parseGlobalConfig 解析全局配置
func (e *HTTPExecutor) parseGlobalConfig(config map[string]any) {
	if baseURL, ok := config["base_url"].(string); ok {
		e.globalConfig.BaseURL = baseURL
	}

	if headers, ok := config["headers"].(map[string]any); ok {
		for k, v := range headers {
			if s, ok := v.(string); ok {
				e.globalConfig.Headers[k] = s
			}
		}
	}

	if domains, ok := config["domains"].(map[string]any); ok {
		for k, v := range domains {
			if s, ok := v.(string); ok {
				e.globalConfig.Domains[k] = s
			}
		}
	}

	// 解析 SSL 配置
	if ssl, ok := config["ssl"].(map[string]any); ok {
		if verify, ok := ssl["verify"].(bool); ok {
			e.globalConfig.SSL.Verify = &verify
		}
		if cert, ok := ssl["cert"].(string); ok {
			e.globalConfig.SSL.CertPath = cert
		}
		if key, ok := ssl["key"].(string); ok {
			e.globalConfig.SSL.KeyPath = key
		}
		if ca, ok := ssl["ca"].(string); ok {
			e.globalConfig.SSL.CAPath = ca
		}
	}

	// 解析重定向配置
	if redirect, ok := config["redirect"].(map[string]any); ok {
		if follow, ok := redirect["follow"].(bool); ok {
			e.globalConfig.Redirect.Follow = &follow
		}
		if maxRedirects, ok := redirect["max_redirects"].(int); ok {
			e.globalConfig.Redirect.MaxRedirects = &maxRedirects
		}
	}

	// 解析超时配置
	if timeout, ok := config["timeout"].(map[string]any); ok {
		if connect, ok := timeout["connect"].(string); ok {
			if d, err := time.ParseDuration(connect); err == nil {
				e.globalConfig.Timeout.Connect = d
			}
		}
		if read, ok := timeout["read"].(string); ok {
			if d, err := time.ParseDuration(read); err == nil {
				e.globalConfig.Timeout.Read = d
			}
		}
		if write, ok := timeout["write"].(string); ok {
			if d, err := time.ParseDuration(write); err == nil {
				e.globalConfig.Timeout.Write = d
			}
		}
		if request, ok := timeout["request"].(string); ok {
			if d, err := time.ParseDuration(request); err == nil {
				e.globalConfig.Timeout.Request = d
			}
		}
	}
}

// buildClient 构建 HTTP 客户端
func (e *HTTPExecutor) buildClient() error {
	transport, err := e.globalConfig.BuildTransport()
	if err != nil {
		return fmt.Errorf("构建 HTTP 传输层失败: %w", err)
	}

	e.client = &http.Client{
		Timeout:       e.globalConfig.Timeout.Request,
		Transport:     transport,
		CheckRedirect: e.globalConfig.BuildCheckRedirect(),
	}

	return nil
}

// Execute 执行 HTTP 请求步骤。
func (e *HTTPExecutor) Execute(ctx context.Context, step *types.Step, execCtx *ExecutionContext) (*types.StepResult, error) {
	startTime := time.Now()

	// 确保客户端已初始化
	if e.client == nil {
		if err := e.buildClient(); err != nil {
			return CreateFailedResult(step.ID, startTime, err), nil
		}
	}

	// 创建处理器执行器
	variables := make(map[string]interface{})
	envVars := make(map[string]interface{})
	if execCtx != nil && execCtx.Variables != nil {
		for k, v := range execCtx.Variables {
			variables[k] = v
		}
	}
	procExecutor := pkgExecutor.NewProcessorExecutor(variables, envVars)

	// 收集所有控制台日志
	allConsoleLogs := make([]types.ConsoleLogEntry, 0)

	// 1. 执行前置处理器
	if len(step.PreProcessors) > 0 {
		preLogs := procExecutor.ExecuteProcessors(ctx, step.PreProcessors, "pre")
		allConsoleLogs = append(allConsoleLogs, preLogs...)

		// 更新执行上下文中的变量
		if execCtx != nil && execCtx.Variables != nil {
			for k, v := range procExecutor.GetVariables() {
				execCtx.Variables[k] = v
			}
		}
	}

	// 解析步骤配置
	config, err := e.parseConfig(step.Config)
	if err != nil {
		return CreateFailedResult(step.ID, startTime, err), nil
	}

	// 合并步骤级配置
	stepConfig := e.mergeStepConfig(config)

	// 解析配置中的变量
	config = e.resolveVariables(config, execCtx)

	// 解析 URL（支持多域名）
	config.URL = e.globalConfig.ResolveURL(config.URL, config.Domain)

	// 保存请求体字符串（用于调试输出）
	var reqBodyStr string
	if config.Body != nil {
		switch body := config.Body.(type) {
		case string:
			reqBodyStr = body
		case []byte:
			reqBodyStr = string(body)
		default:
			if jsonBody, err := json.Marshal(body); err == nil {
				reqBodyStr = string(jsonBody)
			}
		}
	}

	// 创建 HTTP 请求
	req, err := e.createRequest(ctx, config, stepConfig)
	if err != nil {
		return CreateFailedResult(step.ID, startTime, err), nil
	}

	// 如果指定了步骤超时则应用
	timeout := step.Timeout
	if timeout <= 0 {
		timeout = stepConfig.Timeout.Request
		if timeout <= 0 {
			timeout = defaultHTTPTimeout
		}
	}

	// 带超时执行
	var resp *http.Response
	err = ExecuteWithTimeout(ctx, timeout, func(ctx context.Context) error {
		req = req.WithContext(ctx)
		var execErr error
		resp, execErr = e.client.Do(req)
		return execErr
	})

	if err != nil {
		if err == context.DeadlineExceeded {
			return CreateTimeoutResult(step.ID, startTime, timeout), nil
		}
		return CreateFailedResult(step.ID, startTime, NewExecutionError(step.ID, "HTTP 请求失败", err)), nil
	}
	defer resp.Body.Close()

	// 读取响应体
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return CreateFailedResult(step.ID, startTime, NewExecutionError(step.ID, "读取响应体失败", err)), nil
	}

	// 构建响应输出（包含请求信息，用于调试）
	output := e.buildOutputWithRequest(req, resp, body, reqBodyStr)

	// 2. 执行后置处理器
	if len(step.PostProcessors) > 0 {
		// 将响应数据转换为 map 供处理器使用
		respHeaders := make(map[string]interface{})
		for k, v := range output.Headers {
			if len(v) > 0 {
				respHeaders[k] = v[0]
			}
		}

		procExecutor.SetResponse(map[string]interface{}{
			"status_code": output.StatusCode,
			"status":      output.Status,
			"body":        output.Body,
			"body_raw":    output.BodyRaw,
			"headers":     respHeaders,
			"duration":    time.Since(startTime).Milliseconds(),
		})

		postLogs := procExecutor.ExecuteProcessors(ctx, step.PostProcessors, "post")
		allConsoleLogs = append(allConsoleLogs, postLogs...)

		// 更新执行上下文中的变量
		if execCtx != nil && execCtx.Variables != nil {
			for k, v := range procExecutor.GetVariables() {
				execCtx.Variables[k] = v
			}
		}
	}

	// 添加控制台日志到输出
	if len(allConsoleLogs) > 0 {
		output.ConsoleLogs = allConsoleLogs
	}

	// 创建结果
	result := CreateSuccessResult(step.ID, startTime, output)

	// 添加 HTTP 特定指标
	result.Metrics["http_status"] = float64(resp.StatusCode)
	result.Metrics["http_response_size"] = float64(len(body))

	return result, nil
}

// mergeStepConfig 合并步骤级配置
func (e *HTTPExecutor) mergeStepConfig(config *HTTPConfig) *HTTPGlobalConfig {
	stepConfig := &HTTPGlobalConfig{
		Headers: make(map[string]string),
		Domains: make(map[string]string),
	}

	// 从步骤配置中提取 SSL 配置
	if config.SSL != nil {
		stepConfig.SSL = *config.SSL
	}

	// 从步骤配置中提取重定向配置
	if config.Redirect != nil {
		stepConfig.Redirect = *config.Redirect
	}

	// 从步骤配置中提取超时配置
	if config.Timeout != nil {
		stepConfig.Timeout = *config.Timeout
	}

	// 合并：全局配置 < 步骤配置
	return e.globalConfig.Merge(stepConfig)
}

// Cleanup 释放 HTTP 执行器持有的资源。
func (e *HTTPExecutor) Cleanup(ctx context.Context) error {
	if e.client != nil {
		e.client.CloseIdleConnections()
	}
	return nil
}

// HTTPConfig 表示 HTTP 步骤的配置。
type HTTPConfig struct {
	Method   string            `json:"method"`
	URL      string            `json:"url"`
	Domain   string            `json:"domain,omitempty"` // 域名标识，用于多域名配置
	Headers  map[string]string `json:"headers"`
	Body     any               `json:"body"`
	Params   map[string]string `json:"params"`
	SSL      *SSLConfig        `json:"ssl,omitempty"`      // 步骤级 SSL 配置
	Redirect *RedirectConfig   `json:"redirect,omitempty"` // 步骤级重定向配置
	Timeout  *TimeoutConfig    `json:"timeout,omitempty"`  // 步骤级超时配置
}

// parseConfig 将步骤配置解析为 HTTPConfig。
func (e *HTTPExecutor) parseConfig(config map[string]any) (*HTTPConfig, error) {
	httpConfig := &HTTPConfig{
		Method:  "GET",
		Headers: make(map[string]string),
		Params:  make(map[string]string),
	}

	if method, ok := config["method"].(string); ok {
		httpConfig.Method = strings.ToUpper(method)
	}

	if url, ok := config["url"].(string); ok {
		url = strings.TrimSpace(url)
		if url == "" {
			return nil, NewConfigError("HTTP 请求地址不能为空", nil)
		}
		httpConfig.URL = url
	} else {
		return nil, NewConfigError("HTTP 步骤需要 'url' 配置", nil)
	}

	// 解析域名标识
	if domain, ok := config["domain"].(string); ok {
		httpConfig.Domain = domain
	}

	if headers, ok := config["headers"].(map[string]any); ok {
		for k, v := range headers {
			if s, ok := v.(string); ok {
				httpConfig.Headers[k] = s
			}
		}
	}

	if params, ok := config["params"].(map[string]any); ok {
		for k, v := range params {
			if s, ok := v.(string); ok {
				httpConfig.Params[k] = s
			}
		}
	}

	httpConfig.Body = config["body"]

	// 解析步骤级 SSL 配置
	if ssl, ok := config["ssl"].(map[string]any); ok {
		httpConfig.SSL = &SSLConfig{}
		if verify, ok := ssl["verify"].(bool); ok {
			httpConfig.SSL.Verify = &verify
		}
		if cert, ok := ssl["cert"].(string); ok {
			httpConfig.SSL.CertPath = cert
		}
		if key, ok := ssl["key"].(string); ok {
			httpConfig.SSL.KeyPath = key
		}
		if ca, ok := ssl["ca"].(string); ok {
			httpConfig.SSL.CAPath = ca
		}
	}

	// 解析步骤级重定向配置
	if redirect, ok := config["redirect"].(map[string]any); ok {
		httpConfig.Redirect = &RedirectConfig{}
		if follow, ok := redirect["follow"].(bool); ok {
			httpConfig.Redirect.Follow = &follow
		}
		if maxRedirects, ok := redirect["max_redirects"].(int); ok {
			httpConfig.Redirect.MaxRedirects = &maxRedirects
		}
	}

	// 解析步骤级超时配置
	if timeout, ok := config["timeout"].(map[string]any); ok {
		httpConfig.Timeout = &TimeoutConfig{}
		if connect, ok := timeout["connect"].(string); ok {
			if d, err := time.ParseDuration(connect); err == nil {
				httpConfig.Timeout.Connect = d
			}
		}
		if read, ok := timeout["read"].(string); ok {
			if d, err := time.ParseDuration(read); err == nil {
				httpConfig.Timeout.Read = d
			}
		}
		if write, ok := timeout["write"].(string); ok {
			if d, err := time.ParseDuration(write); err == nil {
				httpConfig.Timeout.Write = d
			}
		}
		if request, ok := timeout["request"].(string); ok {
			if d, err := time.ParseDuration(request); err == nil {
				httpConfig.Timeout.Request = d
			}
		}
	}

	return httpConfig, nil
}

// resolveVariables 解析配置中的变量引用。
func (e *HTTPExecutor) resolveVariables(config *HTTPConfig, execCtx *ExecutionContext) *HTTPConfig {
	if execCtx == nil {
		return config
	}

	evalCtx := execCtx.ToEvaluationContext()

	// 解析 URL
	config.URL = resolveString(config.URL, evalCtx)

	// 解析 headers
	for k, v := range config.Headers {
		config.Headers[k] = resolveString(v, evalCtx)
	}

	// 解析 params
	for k, v := range config.Params {
		config.Params[k] = resolveString(v, evalCtx)
	}

	// 如果 body 是字符串则解析
	if bodyStr, ok := config.Body.(string); ok {
		config.Body = resolveString(bodyStr, evalCtx)
	}

	return config
}

// resolveString 解析字符串中的变量引用。
func resolveString(s string, ctx map[string]any) string {
	result := s
	for key, value := range ctx {
		placeholder := fmt.Sprintf("${%s}", key)
		if strings.Contains(result, placeholder) {
			result = strings.ReplaceAll(result, placeholder, fmt.Sprintf("%v", value))
		}

		// 同时处理嵌套访问，如 ${login.token}
		if m, ok := value.(map[string]any); ok {
			for subKey, subValue := range m {
				nestedPlaceholder := fmt.Sprintf("${%s.%s}", key, subKey)
				if strings.Contains(result, nestedPlaceholder) {
					result = strings.ReplaceAll(result, nestedPlaceholder, fmt.Sprintf("%v", subValue))
				}
			}
		}
	}
	return result
}

// createRequest 从配置创建 HTTP 请求。
func (e *HTTPExecutor) createRequest(ctx context.Context, config *HTTPConfig, mergedConfig *HTTPGlobalConfig) (*http.Request, error) {
	// 构建带查询参数的 URL
	url := config.URL
	if len(config.Params) > 0 {
		params := make([]string, 0, len(config.Params))
		for k, v := range config.Params {
			params = append(params, fmt.Sprintf("%s=%s", k, v))
		}
		if strings.Contains(url, "?") {
			url += "&" + strings.Join(params, "&")
		} else {
			url += "?" + strings.Join(params, "&")
		}
	}

	// 准备请求体
	var bodyReader io.Reader
	if config.Body != nil {
		switch body := config.Body.(type) {
		case string:
			bodyReader = strings.NewReader(body)
		case []byte:
			bodyReader = bytes.NewReader(body)
		default:
			// 序列化为 JSON
			jsonBody, err := json.Marshal(body)
			if err != nil {
				return nil, NewConfigError("序列化请求体失败", err)
			}
			bodyReader = bytes.NewReader(jsonBody)
		}
	}

	// 创建请求
	req, err := http.NewRequestWithContext(ctx, config.Method, url, bodyReader)
	if err != nil {
		return nil, NewConfigError("创建 HTTP 请求失败", err)
	}

	// 先设置全局 headers
	for k, v := range mergedConfig.Headers {
		req.Header.Set(k, v)
	}

	// 再设置步骤级 headers（覆盖全局）
	for k, v := range config.Headers {
		req.Header.Set(k, v)
	}

	// 如果未指定 Content-Type 且有 JSON body，则设置默认值
	if config.Body != nil && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

	return req, nil
}

// HTTPResponse 表示 HTTP 步骤的输出。
type HTTPResponse struct {
	// 请求信息（调试用）
	Request *HTTPRequestInfo `json:"request,omitempty"`
	// 响应信息
	Status     string              `json:"status"`
	StatusCode int                 `json:"status_code"`
	Headers    map[string][]string `json:"headers"`
	Body       any                 `json:"body"`
	BodyRaw    string              `json:"body_raw"`
	// 控制台日志（统一格式）
	ConsoleLogs []types.ConsoleLogEntry `json:"console_logs,omitempty"`
}

// HTTPRequestInfo 请求详情（用于调试）
type HTTPRequestInfo struct {
	Method  string            `json:"method"`
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers"`
	Body    string            `json:"body,omitempty"`
}

// buildOutput 构建响应输出。
func (e *HTTPExecutor) buildOutput(resp *http.Response, body []byte) *HTTPResponse {
	output := &HTTPResponse{
		Status:     resp.Status,
		StatusCode: resp.StatusCode,
		Headers:    resp.Header,
		BodyRaw:    string(body),
	}

	// 尝试将 body 解析为 JSON
	var jsonBody any
	if err := json.Unmarshal(body, &jsonBody); err == nil {
		output.Body = jsonBody
	} else {
		output.Body = string(body)
	}

	return output
}

// buildOutputWithRequest 构建包含请求信息的响应输出（用于调试）
func (e *HTTPExecutor) buildOutputWithRequest(req *http.Request, resp *http.Response, body []byte, reqBody string) *HTTPResponse {
	output := e.buildOutput(resp, body)

	// 添加请求信息
	headers := make(map[string]string)
	for k, v := range req.Header {
		if len(v) > 0 {
			headers[k] = v[0]
		}
	}

	output.Request = &HTTPRequestInfo{
		Method:  req.Method,
		URL:     req.URL.String(),
		Headers: headers,
		Body:    reqBody,
	}

	return output
}

// init 在默认注册表中注册标准库 HTTP 执行器。
// 注意：此执行器注册为 "http-std" 类型，默认的 "http" 类型由 FastHTTP 执行器提供。
// 如需使用标准库实现，可在工作流中指定 type: http-std
func init() {
	MustRegister(NewHTTPExecutor())
}
