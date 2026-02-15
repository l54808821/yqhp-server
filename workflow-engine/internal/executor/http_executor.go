package executor

import (
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

// resolveURLWithBase 使用指定的 base URL 解析相对路径。
// 如果 url 已经是完整 URL（以 http:// 或 https:// 开头），直接返回。
// 否则拼接 baseURL + url。
func resolveURLWithBase(url string, baseURL string) string {
	// 如果 URL 已经是完整的，直接返回
	if len(url) > 0 && (url[0] == 'h' || url[0] == 'H') {
		if len(url) > 7 && (url[:7] == "http://" || url[:8] == "https://") {
			return url
		}
	}

	if baseURL == "" {
		return url
	}

	// 确保 baseURL 不以 / 结尾，url 以 / 开头
	baseURL = trimRight(baseURL, "/")
	if len(url) == 0 || url[0] != '/' {
		url = "/" + url
	}

	return baseURL + url
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

	// 1. 执行前置处理器
	if len(step.PreProcessors) > 0 {
		preLogs := procExecutor.ExecuteProcessors(ctx, step.PreProcessors, "pre")
		// 使用统一的日志接口
		execCtx.AppendLogs(preLogs)
		// 追踪变量变更
		e.trackVariableChanges(execCtx, preLogs)

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

	// 解析 URL：优先使用内联域名配置（由 gulu handler 注入），fallback 到全局配置
	if config.DomainBaseURL != "" {
		config.URL = resolveURLWithBase(config.URL, config.DomainBaseURL)
	} else {
		config.URL = e.globalConfig.ResolveURL(config.URL, config.Domain)
	}

	// 合并域名级请求头
	if len(config.DomainHeaders) > 0 {
		for k, v := range config.DomainHeaders {
			if _, exists := config.Headers[k]; !exists {
				config.Headers[k] = v
			}
		}
	}

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
			respHeaders[k] = v
		}

		procExecutor.SetResponse(map[string]interface{}{
			"statusCode": output.StatusCode,
			"statusText": output.StatusText,
			"body":       output.Body,
			"headers":    respHeaders,
			"duration":   time.Since(startTime).Milliseconds(),
		})

		postLogs := procExecutor.ExecuteProcessors(ctx, step.PostProcessors, "post")
		// 使用统一的日志接口
		execCtx.AppendLogs(postLogs)
		// 追踪变量变更
		e.trackVariableChanges(execCtx, postLogs)

		// 更新执行上下文中的变量
		if execCtx != nil && execCtx.Variables != nil {
			for k, v := range procExecutor.GetVariables() {
				execCtx.Variables[k] = v
			}
		}
	}

	// 创建变量快照（在 FlushLogs 之前，因为 FlushLogs 会清空日志）
	// 使用处理器执行器获取最新的变量状态
	execCtx.CreateVariableSnapshotWithEnvVars(nil)

	// 从执行上下文获取所有日志，并提取断言结果
	allConsoleLogs := execCtx.FlushLogs()
	if len(allConsoleLogs) > 0 {
		output.ConsoleLogs = allConsoleLogs

		// 从处理器日志中提取断言结果
		for _, entry := range allConsoleLogs {
			if entry.Type == types.LogTypeProcessor && entry.Processor != nil && entry.Processor.Type == "assertion" {
				output.Assertions = append(output.Assertions, types.AssertionResult{
					ID:      entry.Processor.ID,
					Name:    entry.Processor.Name,
					Passed:  entry.Processor.Success,
					Message: entry.Processor.Message,
				})
			}
		}
	}

	// 设置耗时
	output.Duration = time.Since(startTime).Milliseconds()

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
	Method         string            `json:"method"`
	URL            string            `json:"url"`
	Domain         string            `json:"domain,omitempty"`          // 域名标识，用于多域名配置
	DomainBaseURL  string            `json:"domain_base_url,omitempty"` // 域名 base URL（由 gulu handler 注入）
	DomainHeaders  map[string]string `json:"domain_headers,omitempty"`  // 域名级请求头（由 gulu handler 注入）
	Headers        map[string]string `json:"headers"`
	Body           any               `json:"body"`        // 保留原始 body（兼容旧格式）
	BodyConfig     *BodyConfig       `json:"-"`           // 解析后的 body 配置
	Params         map[string]string `json:"params"`
	SSL            *SSLConfig        `json:"ssl,omitempty"`      // 步骤级 SSL 配置
	Redirect       *RedirectConfig   `json:"redirect,omitempty"` // 步骤级重定向配置
	Timeout        *TimeoutConfig    `json:"timeout,omitempty"`  // 步骤级超时配置
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

	// 解析域名标识
	if domain, ok := config["domain"].(string); ok {
		httpConfig.Domain = domain
	}

	// 解析内联域名配置（由 gulu handler 从环境配置注入）
	if domainBaseURL, ok := config["domain_base_url"].(string); ok {
		httpConfig.DomainBaseURL = domainBaseURL
	}
	if domainHeaders, ok := config["domain_headers"].(map[string]interface{}); ok {
		httpConfig.DomainHeaders = make(map[string]string, len(domainHeaders))
		for k, v := range domainHeaders {
			if s, ok := v.(string); ok {
				httpConfig.DomainHeaders[k] = s
			}
		}
	} else if domainHeaders, ok := config["domain_headers"].(map[string]string); ok {
		httpConfig.DomainHeaders = domainHeaders
	}

	if url, ok := config["url"].(string); ok {
		url = strings.TrimSpace(url)
		if url == "" && httpConfig.Domain == "" {
			return nil, NewConfigError("HTTP 请求地址不能为空", nil)
		}
		httpConfig.URL = url
	} else if httpConfig.Domain == "" {
		return nil, NewConfigError("HTTP 步骤需要 'url' 配置", nil)
	}

	// 解析 headers（支持 map 格式和数组格式）
	if headersRaw, exists := config["headers"]; exists {
		httpConfig.Headers = ParseKeyValueConfig(headersRaw)
	}

	// 解析 params（支持 map 格式和数组格式）
	if paramsRaw, exists := config["params"]; exists {
		httpConfig.Params = ParseKeyValueConfig(paramsRaw)
	}

	// 解析 body 配置
	if bodyRaw, exists := config["body"]; exists {
		httpConfig.BodyConfig = ParseBodyConfig(bodyRaw)
	}

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
// 使用优化后的 VariableResolver，通过正则表达式一次性提取所有变量引用。
func (e *HTTPExecutor) resolveVariables(config *HTTPConfig, execCtx *ExecutionContext) *HTTPConfig {
	if execCtx == nil {
		return config
	}

	evalCtx := execCtx.ToEvaluationContext()
	resolver := GetVariableResolver()

	// 解析 URL
	config.URL = resolver.ResolveString(config.URL, evalCtx)

	// 解析 headers
	for k, v := range config.Headers {
		config.Headers[k] = resolver.ResolveString(v, evalCtx)
	}

	// 解析 params
	for k, v := range config.Params {
		config.Params[k] = resolver.ResolveString(v, evalCtx)
	}

	// 解析 body 中的变量
	if config.BodyConfig != nil {
		// 解析 raw 内容
		if config.BodyConfig.Raw != "" {
			config.BodyConfig.Raw = resolver.ResolveString(config.BodyConfig.Raw, evalCtx)
		}
		// 解析 formData
		for k, v := range config.BodyConfig.FormData {
			config.BodyConfig.FormData[k] = resolver.ResolveString(v, evalCtx)
		}
		// 解析 urlencoded
		for k, v := range config.BodyConfig.URLEncoded {
			config.BodyConfig.URLEncoded[k] = resolver.ResolveString(v, evalCtx)
		}
	}

	return config
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

	// 准备请求体和 Content-Type
	var bodyReader io.Reader
	var contentType string
	if config.BodyConfig != nil {
		switch config.BodyConfig.Type {
		case "form-data":
			if len(config.BodyConfig.FormData) > 0 {
				var parts []string
				for k, v := range config.BodyConfig.FormData {
					parts = append(parts, fmt.Sprintf("%s=%s", k, v))
				}
				bodyReader = strings.NewReader(strings.Join(parts, "&"))
				contentType = "multipart/form-data"
			}
		case "x-www-form-urlencoded":
			if len(config.BodyConfig.URLEncoded) > 0 {
				var parts []string
				for k, v := range config.BodyConfig.URLEncoded {
					parts = append(parts, fmt.Sprintf("%s=%s", k, v))
				}
				bodyReader = strings.NewReader(strings.Join(parts, "&"))
				contentType = "application/x-www-form-urlencoded"
			}
		case "json":
			if config.BodyConfig.Raw != "" {
				bodyReader = strings.NewReader(config.BodyConfig.Raw)
				contentType = "application/json"
			}
		case "xml":
			if config.BodyConfig.Raw != "" {
				bodyReader = strings.NewReader(config.BodyConfig.Raw)
				contentType = "application/xml"
			}
		case "text":
			if config.BodyConfig.Raw != "" {
				bodyReader = strings.NewReader(config.BodyConfig.Raw)
				contentType = "text/plain"
			}
		case "graphql":
			if config.BodyConfig.Raw != "" {
				bodyReader = strings.NewReader(config.BodyConfig.Raw)
				contentType = "application/json"
			}
		default:
			if config.BodyConfig.Raw != "" {
				bodyReader = strings.NewReader(config.BodyConfig.Raw)
				contentType = "application/json"
			}
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

	// 如果未指定 Content-Type 且有 body，则设置默认值
	if contentType != "" && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", contentType)
	}

	return req, nil
}

// buildOutput 构建响应输出（使用统一的 HTTPResponseData 结构）
func (e *HTTPExecutor) buildOutput(resp *http.Response, body []byte) *types.HTTPResponseData {
	bodyStr := string(body)

	// 将 headers 从 map[string][]string 转换为 map[string]string
	headers := make(map[string]string)
	for k, v := range resp.Header {
		if len(v) > 0 {
			headers[k] = v[0]
		}
	}

	output := &types.HTTPResponseData{
		StatusText: resp.Status,
		StatusCode: resp.StatusCode,
		Headers:    headers,
		Body:       bodyStr,
		BodyType:   types.DetectBodyType(bodyStr),
		Size:       int64(len(body)),
	}

	return output
}

// buildOutputWithRequest 构建包含请求信息的响应输出（用于调试）
func (e *HTTPExecutor) buildOutputWithRequest(req *http.Request, resp *http.Response, body []byte, reqBody string) *types.HTTPResponseData {
	output := e.buildOutput(resp, body)

	// 添加请求信息
	reqHeaders := make(map[string]string)
	for k, v := range req.Header {
		if len(v) > 0 {
			reqHeaders[k] = v[0]
		}
	}

	output.ActualRequest = &types.ActualRequest{
		Method:  req.Method,
		URL:     req.URL.String(),
		Headers: reqHeaders,
		Body:    reqBody,
	}

	return output
}

// trackVariableChanges 从处理器日志中追踪变量变更
func (e *HTTPExecutor) trackVariableChanges(execCtx *ExecutionContext, logs []types.ConsoleLogEntry) {
	if execCtx == nil {
		return
	}

	for _, entry := range logs {
		if entry.Type != types.LogTypeProcessor || entry.Processor == nil {
			continue
		}

		output := entry.Processor.Output
		if output == nil {
			continue
		}

		// 处理 set_variable 和 extract_param 的变量变更
		if entry.Processor.Type == "set_variable" || entry.Processor.Type == "extract_param" {
			varName, _ := output["variableName"].(string)
			if varName == "" {
				continue
			}
			scope, _ := output["scope"].(string)
			if scope == "" {
				scope = "temp"
			}
			source, _ := output["source"].(string)
			if source == "" {
				source = entry.Processor.Type
			}

			// 记录变量变更（变量值已经通过 procExecutor.GetVariables() 更新）
			execCtx.AppendLog(types.NewVariableChangeEntry(types.VariableChangeInfo{
				Name:     varName,
				OldValue: output["oldValue"],
				NewValue: output["value"],
				Scope:    scope,
				Source:   source,
			}))

			// 标记环境变量
			if scope == "env" {
				execCtx.MarkAsEnvVar(varName)
			}
		}

		// 处理 js_script 的变量变更
		if entry.Processor.Type == "js_script" {
			if varChanges, ok := output["varChanges"].([]map[string]any); ok {
				for _, change := range varChanges {
					name, _ := change["name"].(string)
					if name == "" {
						continue
					}
					scope, _ := change["scope"].(string)
					source, _ := change["source"].(string)

					execCtx.AppendLog(types.NewVariableChangeEntry(types.VariableChangeInfo{
						Name:     name,
						OldValue: change["oldValue"],
						NewValue: change["newValue"],
						Scope:    scope,
						Source:   source,
					}))

					if scope == "env" {
						execCtx.MarkAsEnvVar(name)
					}
				}
			}
		}
	}
}

// init 在默认注册表中注册标准库 HTTP 执行器。
// 注意：此执行器注册为 "http-std" 类型，默认的 "http" 类型由 FastHTTP 执行器提供。
// 如需使用标准库实现，可在工作流中指定 type: http-std
func init() {
	MustRegister(NewHTTPExecutor())
}
