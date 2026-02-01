package executor

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	pkgExecutor "yqhp/workflow-engine/pkg/executor"
	"yqhp/workflow-engine/pkg/types"

	"github.com/valyala/fasthttp"
)

const (
	// FastHTTPExecutorType 是 FastHTTP 执行器的类型标识符。
	FastHTTPExecutorType = "http"

	// FastHTTP 请求的默认超时时间。
	defaultFastHTTPTimeout = 30 * time.Second
)

// FastHTTPExecutor 使用 fasthttp 执行 HTTP 请求步骤，性能更优。
type FastHTTPExecutor struct {
	*BaseExecutor
	client       *fasthttp.Client
	globalConfig *HTTPGlobalConfig
}

// NewFastHTTPExecutor 创建一个新的 FastHTTP 执行器。
func NewFastHTTPExecutor() *FastHTTPExecutor {
	return &FastHTTPExecutor{
		BaseExecutor: NewBaseExecutor(FastHTTPExecutorType),
		globalConfig: DefaultHTTPGlobalConfig(),
	}
}

// NewFastHTTPExecutorWithConfig 使用全局配置创建一个新的 FastHTTP 执行器。
func NewFastHTTPExecutorWithConfig(globalConfig *HTTPGlobalConfig) *FastHTTPExecutor {
	if globalConfig == nil {
		globalConfig = DefaultHTTPGlobalConfig()
	}
	return &FastHTTPExecutor{
		BaseExecutor: NewBaseExecutor(FastHTTPExecutorType),
		globalConfig: globalConfig,
	}
}

// SetGlobalConfig 设置全局 HTTP 配置。
func (e *FastHTTPExecutor) SetGlobalConfig(config *HTTPGlobalConfig) {
	if config != nil {
		e.globalConfig = config
	}
}

// GetGlobalConfig 返回全局 HTTP 配置。
func (e *FastHTTPExecutor) GetGlobalConfig() *HTTPGlobalConfig {
	return e.globalConfig
}

// Init 使用配置初始化 FastHTTP 执行器。
func (e *FastHTTPExecutor) Init(ctx context.Context, config map[string]any) error {
	if err := e.BaseExecutor.Init(ctx, config); err != nil {
		return err
	}

	// 解析全局配置
	if httpConfig, ok := config["http"].(map[string]any); ok {
		e.parseGlobalConfig(httpConfig)
	}

	// 构建 FastHTTP 客户端
	if err := e.buildClient(); err != nil {
		return err
	}

	return nil
}

// parseGlobalConfig 解析全局配置
func (e *FastHTTPExecutor) parseGlobalConfig(config map[string]any) {
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

// buildClient 构建 FastHTTP 客户端
func (e *FastHTTPExecutor) buildClient() error {
	// 构建 TLS 配置
	tlsConfig, err := e.globalConfig.SSL.BuildTLSConfig()
	if err != nil {
		return fmt.Errorf("构建 TLS 配置失败: %w", err)
	}

	e.client = &fasthttp.Client{
		// 连接池配置
		MaxConnsPerHost:     1000,
		MaxIdleConnDuration: 90 * time.Second,

		// 超时配置
		ReadTimeout:  e.globalConfig.Timeout.Read,
		WriteTimeout: e.globalConfig.Timeout.Write,

		// TLS 配置
		TLSConfig: tlsConfig,

		// 禁用路径规范化以提高性能
		DisablePathNormalizing: true,
	}

	return nil
}

// Execute 执行 HTTP 请求步骤。
func (e *FastHTTPExecutor) Execute(ctx context.Context, step *types.Step, execCtx *ExecutionContext) (*types.StepResult, error) {
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

	// 解析 URL（支持多域名）
	config.URL = e.globalConfig.ResolveURL(config.URL, config.Domain)

	// 如果指定了步骤超时则应用
	timeout := step.Timeout
	if timeout <= 0 {
		timeout = stepConfig.Timeout.Request
		if timeout <= 0 {
			timeout = defaultFastHTTPTimeout
		}
	}

	// 获取请求和响应对象（使用对象池）
	req := fasthttp.AcquireRequest()
	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseRequest(req)
	defer fasthttp.ReleaseResponse(resp)

	// 构建请求
	if err := e.buildRequest(req, config, stepConfig); err != nil {
		return CreateFailedResult(step.ID, startTime, err), nil
	}

	// 捕获请求信息用于调试
	reqInfo := &types.ActualRequest{
		Method:  config.Method,
		URL:     string(req.URI().FullURI()),
		Headers: make(map[string]string),
	}
	// 复制请求头
	req.Header.VisitAll(func(key, value []byte) {
		reqInfo.Headers[string(key)] = string(value)
	})
	// 复制请求体
	if len(req.Body()) > 0 {
		reqInfo.Body = string(req.Body())
	}

	// 执行请求
	var execErr error
	if e.globalConfig.Redirect.GetFollow() {
		// 跟随重定向
		execErr = e.client.DoRedirects(req, resp, e.globalConfig.Redirect.GetMaxRedirects())
	} else {
		execErr = e.client.DoTimeout(req, resp, timeout)
	}

	if execErr != nil {
		if execErr == fasthttp.ErrTimeout {
			return CreateTimeoutResult(step.ID, startTime, timeout), nil
		}
		return CreateFailedResult(step.ID, startTime, NewExecutionError(step.ID, "HTTP 请求失败", execErr)), nil
	}

	// 构建响应输出（包含请求信息）
	output := e.buildOutputWithRequest(resp, reqInfo)

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
	result.Metrics["http_status"] = float64(resp.StatusCode())
	result.Metrics["http_response_size"] = float64(len(resp.Body()))

	return result, nil
}

// mergeStepConfig 合并步骤级配置
func (e *FastHTTPExecutor) mergeStepConfig(config *HTTPConfig) *HTTPGlobalConfig {
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

// Cleanup 释放 FastHTTP 执行器持有的资源。
func (e *FastHTTPExecutor) Cleanup(ctx context.Context) error {
	// fasthttp.Client 不需要显式关闭
	return nil
}

// parseConfig 将步骤配置解析为 HTTPConfig。
func (e *FastHTTPExecutor) parseConfig(config map[string]any) (*HTTPConfig, error) {
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

	// 解析 headers（支持 map 格式和数组格式）
	if headersRaw, exists := config["headers"]; exists {
		httpConfig.Headers = ParseKeyValueConfig(headersRaw)
	}

	// 解析 params（支持 map 格式和数组格式）
	if paramsRaw, exists := config["params"]; exists {
		httpConfig.Params = ParseKeyValueConfig(paramsRaw)
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
// 使用优化后的 VariableResolver，通过正则表达式一次性提取所有变量引用。
func (e *FastHTTPExecutor) resolveVariables(config *HTTPConfig, execCtx *ExecutionContext) *HTTPConfig {
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

	// 如果 body 是字符串则解析
	if bodyStr, ok := config.Body.(string); ok {
		config.Body = resolver.ResolveString(bodyStr, evalCtx)
	}

	return config
}

// buildRequest 构建 FastHTTP 请求。
func (e *FastHTTPExecutor) buildRequest(req *fasthttp.Request, config *HTTPConfig, mergedConfig *HTTPGlobalConfig) error {
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

	// 设置请求方法和 URL
	req.Header.SetMethod(config.Method)
	req.SetRequestURI(url)

	// 先设置全局 headers
	for k, v := range mergedConfig.Headers {
		req.Header.Set(k, v)
	}

	// 再设置步骤级 headers（覆盖全局）
	for k, v := range config.Headers {
		req.Header.Set(k, v)
	}

	// 设置请求体
	if config.Body != nil {
		switch body := config.Body.(type) {
		case string:
			req.SetBodyString(body)
		case []byte:
			req.SetBody(body)
		default:
			// 序列化为 JSON
			jsonBody, err := json.Marshal(body)
			if err != nil {
				return NewConfigError("序列化请求体失败", err)
			}
			req.SetBody(jsonBody)
		}

		// 如果未指定 Content-Type 且有 body，则设置默认值
		if len(req.Header.ContentType()) == 0 {
			req.Header.SetContentType("application/json")
		}
	}

	return nil
}

// buildOutput 构建响应输出（使用统一的 HTTPResponseData 结构）
func (e *FastHTTPExecutor) buildOutput(resp *fasthttp.Response) *types.HTTPResponseData {
	return e.buildOutputWithRequest(resp, nil)
}

// buildOutputWithRequest 构建响应输出，包含请求信息。
func (e *FastHTTPExecutor) buildOutputWithRequest(resp *fasthttp.Response, reqInfo *types.ActualRequest) *types.HTTPResponseData {
	// 复制响应体（因为 resp.Body() 返回的是内部缓冲区的引用）
	bodyBytes := make([]byte, len(resp.Body()))
	copy(bodyBytes, resp.Body())
	bodyStr := string(bodyBytes)

	// 将 headers 转换为 map[string]string（取第一个值）
	headers := make(map[string]string)
	resp.Header.VisitAll(func(key, value []byte) {
		k := string(key)
		// 如果已存在则跳过（保留第一个值）
		if _, exists := headers[k]; !exists {
			headers[k] = string(value)
		}
	})

	output := &types.HTTPResponseData{
		StatusText:    fmt.Sprintf("%d %s", resp.StatusCode(), fasthttp.StatusMessage(resp.StatusCode())),
		StatusCode:    resp.StatusCode(),
		Headers:       headers,
		Body:          bodyStr,
		BodyType:      types.DetectBodyType(bodyStr),
		Size:          int64(len(bodyBytes)),
		ActualRequest: reqInfo,
	}

	return output
}

// ConfigureTLS 配置 TLS（用于 HTTPS 请求）
func (e *FastHTTPExecutor) ConfigureTLS(tlsConfig *tls.Config) {
	if e.client != nil {
		e.client.TLSConfig = tlsConfig
	}
}

// trackVariableChanges 从处理器日志中追踪变量变更
func (e *FastHTTPExecutor) trackVariableChanges(execCtx *ExecutionContext, logs []types.ConsoleLogEntry) {
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

			// 记录变量变更
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

// init 在默认注册表中注册 FastHTTP 执行器（替代原有的 HTTP 执行器）。
func init() {
	// 注册 FastHTTP 执行器作为默认的 HTTP 执行器
	MustRegister(NewFastHTTPExecutor())
}
