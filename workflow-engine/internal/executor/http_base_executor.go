package executor

import (
	"context"
	"strings"
	"time"

	pkgExecutor "yqhp/workflow-engine/pkg/executor"
	"yqhp/workflow-engine/pkg/types"
)

// HTTPBaseExecutor 提供两个 HTTP 执行器（FastHTTP 和标准库）的公共逻辑。
// 包括配置解析、变量解析、前后置处理器执行、日志收集等。
type HTTPBaseExecutor struct {
	*BaseExecutor
	GlobalConfig *HTTPGlobalConfig
}

// NewHTTPBaseExecutor 创建一个新的 HTTP 基础执行器。
func NewHTTPBaseExecutor(execType string, globalConfig *HTTPGlobalConfig) *HTTPBaseExecutor {
	if globalConfig == nil {
		globalConfig = DefaultHTTPGlobalConfig()
	}
	return &HTTPBaseExecutor{
		BaseExecutor: NewBaseExecutor(execType),
		GlobalConfig: globalConfig,
	}
}

// SetGlobalConfig 设置全局 HTTP 配置。
func (b *HTTPBaseExecutor) SetGlobalConfig(config *HTTPGlobalConfig) {
	if config != nil {
		b.GlobalConfig = config
	}
}

// GetGlobalConfig 返回全局 HTTP 配置。
func (b *HTTPBaseExecutor) GetGlobalConfig() *HTTPGlobalConfig {
	return b.GlobalConfig
}

// ParseGlobalConfig 从 map 解析全局配置。
func (b *HTTPBaseExecutor) ParseGlobalConfig(config map[string]any) {
	if baseURL, ok := config["base_url"].(string); ok {
		b.GlobalConfig.BaseURL = baseURL
	}

	if headers, ok := config["headers"].(map[string]any); ok {
		for k, v := range headers {
			if s, ok := v.(string); ok {
				b.GlobalConfig.Headers[k] = s
			}
		}
	}

	if domains, ok := config["domains"].(map[string]any); ok {
		for k, v := range domains {
			if s, ok := v.(string); ok {
				b.GlobalConfig.Domains[k] = s
			}
		}
	}

	// 解析 SSL 配置
	if ssl, ok := config["ssl"].(map[string]any); ok {
		if verify, ok := ssl["verify"].(bool); ok {
			b.GlobalConfig.SSL.Verify = &verify
		}
		if cert, ok := ssl["cert"].(string); ok {
			b.GlobalConfig.SSL.CertPath = cert
		}
		if key, ok := ssl["key"].(string); ok {
			b.GlobalConfig.SSL.KeyPath = key
		}
		if ca, ok := ssl["ca"].(string); ok {
			b.GlobalConfig.SSL.CAPath = ca
		}
	}

	// 解析重定向配置
	if redirect, ok := config["redirect"].(map[string]any); ok {
		if follow, ok := redirect["follow"].(bool); ok {
			b.GlobalConfig.Redirect.Follow = &follow
		}
		if maxRedirects, ok := redirect["max_redirects"].(int); ok {
			b.GlobalConfig.Redirect.MaxRedirects = &maxRedirects
		}
	}

	// 解析超时配置
	if timeout, ok := config["timeout"].(map[string]any); ok {
		if connect, ok := timeout["connect"].(string); ok {
			if d, err := time.ParseDuration(connect); err == nil {
				b.GlobalConfig.Timeout.Connect = d
			}
		}
		if read, ok := timeout["read"].(string); ok {
			if d, err := time.ParseDuration(read); err == nil {
				b.GlobalConfig.Timeout.Read = d
			}
		}
		if write, ok := timeout["write"].(string); ok {
			if d, err := time.ParseDuration(write); err == nil {
				b.GlobalConfig.Timeout.Write = d
			}
		}
		if request, ok := timeout["request"].(string); ok {
			if d, err := time.ParseDuration(request); err == nil {
				b.GlobalConfig.Timeout.Request = d
			}
		}
	}
}

// ParseConfig 将步骤配置解析为 HTTPConfig。
func (b *HTTPBaseExecutor) ParseConfig(config map[string]any) (*HTTPConfig, error) {
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

// MergeStepConfig 合并步骤级配置到全局配置。
func (b *HTTPBaseExecutor) MergeStepConfig(config *HTTPConfig) *HTTPGlobalConfig {
	stepConfig := &HTTPGlobalConfig{
		Headers: make(map[string]string),
		Domains: make(map[string]string),
	}

	if config.SSL != nil {
		stepConfig.SSL = *config.SSL
	}
	if config.Redirect != nil {
		stepConfig.Redirect = *config.Redirect
	}
	if config.Timeout != nil {
		stepConfig.Timeout = *config.Timeout
	}

	return b.GlobalConfig.Merge(stepConfig)
}

// ResolveVariables 解析配置中的变量引用。
func (b *HTTPBaseExecutor) ResolveVariables(config *HTTPConfig, execCtx *ExecutionContext) *HTTPConfig {
	if execCtx == nil {
		return config
	}

	evalCtx := execCtx.ToEvaluationContext()
	resolver := GetVariableResolver()

	config.URL = resolver.ResolveString(config.URL, evalCtx)

	for k, v := range config.Headers {
		config.Headers[k] = resolver.ResolveString(v, evalCtx)
	}

	for k, v := range config.Params {
		config.Params[k] = resolver.ResolveString(v, evalCtx)
	}

	if config.BodyConfig != nil {
		if config.BodyConfig.Raw != "" {
			config.BodyConfig.Raw = resolver.ResolveString(config.BodyConfig.Raw, evalCtx)
		}
		for k, v := range config.BodyConfig.FormData {
			config.BodyConfig.FormData[k] = resolver.ResolveString(v, evalCtx)
		}
		for k, v := range config.BodyConfig.URLEncoded {
			config.BodyConfig.URLEncoded[k] = resolver.ResolveString(v, evalCtx)
		}
	}

	return config
}

// ResolveURL 解析 URL，处理域名配置和 base URL。
func (b *HTTPBaseExecutor) ResolveURL(config *HTTPConfig) string {
	if config.DomainBaseURL != "" {
		return resolveURLWithBase(config.URL, config.DomainBaseURL)
	}
	return b.GlobalConfig.ResolveURL(config.URL, config.Domain)
}

// MergeDomainHeaders 合并域名级请求头到配置中。
func (b *HTTPBaseExecutor) MergeDomainHeaders(config *HTTPConfig) {
	if len(config.DomainHeaders) > 0 {
		for k, v := range config.DomainHeaders {
			if _, exists := config.Headers[k]; !exists {
				config.Headers[k] = v
			}
		}
	}
}

// ResolveTimeout 解析超时时间，优先级：步骤配置 > 全局配置 > 默认值。
func (b *HTTPBaseExecutor) ResolveTimeout(step *types.Step, mergedConfig *HTTPGlobalConfig, defaultTimeout time.Duration) time.Duration {
	timeout := step.Timeout
	if timeout <= 0 {
		timeout = mergedConfig.Timeout.Request
		if timeout <= 0 {
			timeout = defaultTimeout
		}
	}
	return timeout
}

// PrepareConfig 执行完整的配置准备流程：解析配置 → 合并 → 解析变量 → 解析 URL → 合并域名头。
// 返回解析后的 HTTPConfig 和合并后的全局配置。
func (b *HTTPBaseExecutor) PrepareConfig(step *types.Step, execCtx *ExecutionContext) (*HTTPConfig, *HTTPGlobalConfig, error) {
	config, err := b.ParseConfig(step.Config)
	if err != nil {
		return nil, nil, err
	}

	mergedConfig := b.MergeStepConfig(config)
	config = b.ResolveVariables(config, execCtx)
	config.URL = b.ResolveURL(config)
	b.MergeDomainHeaders(config)

	return config, mergedConfig, nil
}

// ExecutePreProcessors 执行前置处理器，返回处理器执行器（后续后置处理器需要用到）。
func (b *HTTPBaseExecutor) ExecutePreProcessors(ctx context.Context, step *types.Step, execCtx *ExecutionContext) *pkgExecutor.ProcessorExecutor {
	variables := make(map[string]interface{})
	envVars := make(map[string]interface{})
	if execCtx != nil && execCtx.Variables != nil {
		for k, v := range execCtx.Variables {
			variables[k] = v
		}
	}
	procExecutor := pkgExecutor.NewProcessorExecutor(variables, envVars)

	if len(step.PreProcessors) > 0 {
		preLogs := procExecutor.ExecuteProcessors(ctx, step.PreProcessors, "pre")
		execCtx.AppendLogs(preLogs)
		b.trackVariableChanges(execCtx, preLogs)

		if execCtx != nil && execCtx.Variables != nil {
			for k, v := range procExecutor.GetVariables() {
				execCtx.Variables[k] = v
			}
		}
	}

	return procExecutor
}

// ExecutePostProcessors 执行后置处理器。
func (b *HTTPBaseExecutor) ExecutePostProcessors(ctx context.Context, step *types.Step, execCtx *ExecutionContext, procExecutor *pkgExecutor.ProcessorExecutor, output *types.HTTPResponseData, startTime time.Time) {
	if len(step.PostProcessors) == 0 {
		return
	}

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
	execCtx.AppendLogs(postLogs)
	b.trackVariableChanges(execCtx, postLogs)

	if execCtx != nil && execCtx.Variables != nil {
		for k, v := range procExecutor.GetVariables() {
			execCtx.Variables[k] = v
		}
	}
}

// CollectLogsAndAssertions 收集日志和断言结果到 output 中。
func (b *HTTPBaseExecutor) CollectLogsAndAssertions(execCtx *ExecutionContext, output *types.HTTPResponseData) {
	execCtx.CreateVariableSnapshotWithEnvVars(nil)

	allConsoleLogs := execCtx.FlushLogs()
	if len(allConsoleLogs) > 0 {
		output.ConsoleLogs = allConsoleLogs

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
}

// trackVariableChanges 从处理器日志中追踪变量变更。
// 注意：实际逻辑委托给包级别的 trackVariableChanges 函数，以便其他执行器复用。
func (b *HTTPBaseExecutor) trackVariableChanges(execCtx *ExecutionContext, logs []types.ConsoleLogEntry) {
	trackVariableChangesShared(execCtx, logs)
}

// trackVariableChangesShared 从处理器日志中追踪变量变更（包级别函数，供所有执行器使用）。
func trackVariableChangesShared(execCtx *ExecutionContext, logs []types.ConsoleLogEntry) {
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

			execCtx.AppendLog(types.NewVariableChangeEntry(types.VariableChangeInfo{
				Name:     varName,
				OldValue: output["oldValue"],
				NewValue: output["value"],
				Scope:    scope,
				Source:   source,
			}))

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
