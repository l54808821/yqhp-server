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

	"yqhp/workflow-engine/pkg/types"
)

const (
	// HTTPExecutorType is the type identifier for HTTP executor.
	HTTPExecutorType = "http"

	// Default timeout for HTTP requests.
	defaultHTTPTimeout = 30 * time.Second
)

// HTTPExecutor executes HTTP request steps.
type HTTPExecutor struct {
	*BaseExecutor
	client       *http.Client
	globalConfig *HTTPGlobalConfig // 全局配置
}

// NewHTTPExecutor creates a new HTTP executor.
func NewHTTPExecutor() *HTTPExecutor {
	return &HTTPExecutor{
		BaseExecutor: NewBaseExecutor(HTTPExecutorType),
		globalConfig: DefaultHTTPGlobalConfig(),
	}
}

// NewHTTPExecutorWithConfig creates a new HTTP executor with global config.
func NewHTTPExecutorWithConfig(globalConfig *HTTPGlobalConfig) *HTTPExecutor {
	if globalConfig == nil {
		globalConfig = DefaultHTTPGlobalConfig()
	}
	return &HTTPExecutor{
		BaseExecutor: NewBaseExecutor(HTTPExecutorType),
		globalConfig: globalConfig,
	}
}

// SetGlobalConfig sets the global HTTP configuration.
func (e *HTTPExecutor) SetGlobalConfig(config *HTTPGlobalConfig) {
	if config != nil {
		e.globalConfig = config
	}
}

// GetGlobalConfig returns the global HTTP configuration.
func (e *HTTPExecutor) GetGlobalConfig() *HTTPGlobalConfig {
	return e.globalConfig
}

// Init initializes the HTTP executor with configuration.
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
		return fmt.Errorf("failed to build HTTP transport: %w", err)
	}

	e.client = &http.Client{
		Timeout:       e.globalConfig.Timeout.Request,
		Transport:     transport,
		CheckRedirect: e.globalConfig.BuildCheckRedirect(),
	}

	return nil
}

// Execute executes an HTTP request step.
func (e *HTTPExecutor) Execute(ctx context.Context, step *types.Step, execCtx *ExecutionContext) (*types.StepResult, error) {
	startTime := time.Now()

	// 确保客户端已初始化
	if e.client == nil {
		if err := e.buildClient(); err != nil {
			return CreateFailedResult(step.ID, startTime, err), nil
		}
	}

	// Parse step configuration
	config, err := e.parseConfig(step.Config)
	if err != nil {
		return CreateFailedResult(step.ID, startTime, err), nil
	}

	// 合并步骤级配置
	stepConfig := e.mergeStepConfig(config)

	// Resolve variables in config
	config = e.resolveVariables(config, execCtx)

	// 解析 URL（支持多域名）
	config.URL = e.globalConfig.ResolveURL(config.URL, config.Domain)

	// Create HTTP request
	req, err := e.createRequest(ctx, config, stepConfig)
	if err != nil {
		return CreateFailedResult(step.ID, startTime, err), nil
	}

	// Apply step timeout if specified
	timeout := step.Timeout
	if timeout <= 0 {
		timeout = stepConfig.Timeout.Request
		if timeout <= 0 {
			timeout = defaultHTTPTimeout
		}
	}

	// Execute with timeout
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
		return CreateFailedResult(step.ID, startTime, NewExecutionError(step.ID, "HTTP request failed", err)), nil
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return CreateFailedResult(step.ID, startTime, NewExecutionError(step.ID, "failed to read response body", err)), nil
	}

	// Build response output
	output := e.buildOutput(resp, body)

	// Create result
	result := CreateSuccessResult(step.ID, startTime, output)

	// Add HTTP-specific metrics
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

// Cleanup releases resources held by the HTTP executor.
func (e *HTTPExecutor) Cleanup(ctx context.Context) error {
	if e.client != nil {
		e.client.CloseIdleConnections()
	}
	return nil
}

// HTTPConfig represents the configuration for an HTTP step.
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

// parseConfig parses the step configuration into HTTPConfig.
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
		httpConfig.URL = url
	} else {
		return nil, NewConfigError("HTTP step requires 'url' configuration", nil)
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

// resolveVariables resolves variable references in the config.
func (e *HTTPExecutor) resolveVariables(config *HTTPConfig, execCtx *ExecutionContext) *HTTPConfig {
	if execCtx == nil {
		return config
	}

	evalCtx := execCtx.ToEvaluationContext()

	// Resolve URL
	config.URL = resolveString(config.URL, evalCtx)

	// Resolve headers
	for k, v := range config.Headers {
		config.Headers[k] = resolveString(v, evalCtx)
	}

	// Resolve params
	for k, v := range config.Params {
		config.Params[k] = resolveString(v, evalCtx)
	}

	// Resolve body if it's a string
	if bodyStr, ok := config.Body.(string); ok {
		config.Body = resolveString(bodyStr, evalCtx)
	}

	return config
}

// resolveString resolves variable references in a string.
func resolveString(s string, ctx map[string]any) string {
	result := s
	for key, value := range ctx {
		placeholder := fmt.Sprintf("${%s}", key)
		if strings.Contains(result, placeholder) {
			result = strings.ReplaceAll(result, placeholder, fmt.Sprintf("%v", value))
		}

		// Also handle nested access like ${login.token}
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

// createRequest creates an HTTP request from the config.
func (e *HTTPExecutor) createRequest(ctx context.Context, config *HTTPConfig, mergedConfig *HTTPGlobalConfig) (*http.Request, error) {
	// Build URL with query params
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

	// Prepare body
	var bodyReader io.Reader
	if config.Body != nil {
		switch body := config.Body.(type) {
		case string:
			bodyReader = strings.NewReader(body)
		case []byte:
			bodyReader = bytes.NewReader(body)
		default:
			// Marshal as JSON
			jsonBody, err := json.Marshal(body)
			if err != nil {
				return nil, NewConfigError("failed to marshal request body", err)
			}
			bodyReader = bytes.NewReader(jsonBody)
		}
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, config.Method, url, bodyReader)
	if err != nil {
		return nil, NewConfigError("failed to create HTTP request", err)
	}

	// 先设置全局 headers
	for k, v := range mergedConfig.Headers {
		req.Header.Set(k, v)
	}

	// 再设置步骤级 headers（覆盖全局）
	for k, v := range config.Headers {
		req.Header.Set(k, v)
	}

	// Set default Content-Type for JSON body if not specified
	if config.Body != nil && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

	return req, nil
}

// HTTPResponse represents the output of an HTTP step.
type HTTPResponse struct {
	Status     string              `json:"status"`
	StatusCode int                 `json:"status_code"`
	Headers    map[string][]string `json:"headers"`
	Body       any                 `json:"body"`
	BodyRaw    string              `json:"body_raw"`
}

// buildOutput builds the response output.
func (e *HTTPExecutor) buildOutput(resp *http.Response, body []byte) *HTTPResponse {
	output := &HTTPResponse{
		Status:     resp.Status,
		StatusCode: resp.StatusCode,
		Headers:    resp.Header,
		BodyRaw:    string(body),
	}

	// Try to parse body as JSON
	var jsonBody any
	if err := json.Unmarshal(body, &jsonBody); err == nil {
		output.Body = jsonBody
	} else {
		output.Body = string(body)
	}

	return output
}

// init registers the HTTP executor with the default registry.
func init() {
	MustRegister(NewHTTPExecutor())
}
