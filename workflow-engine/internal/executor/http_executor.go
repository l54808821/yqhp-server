package executor

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"yqhp/workflow-engine/pkg/types"
)

const (
	// HTTPExecutorType 是标准库 HTTP 执行器的类型标识符。
	// 注意：默认的 "http" 类型由 FastHTTP 执行器提供，此执行器使用 "http-std" 类型。
	HTTPExecutorType = "http-std"

	// HTTP 请求的默认超时时间。
	defaultHTTPTimeout = 30 * time.Second
)

// HTTPExecutor 使用标准库 net/http 执行 HTTP 请求步骤。
type HTTPExecutor struct {
	*HTTPBaseExecutor
	client *http.Client
}

// NewHTTPExecutor 创建一个新的 HTTP 执行器。
func NewHTTPExecutor() *HTTPExecutor {
	return &HTTPExecutor{
		HTTPBaseExecutor: NewHTTPBaseExecutor(HTTPExecutorType, nil),
	}
}

// NewHTTPExecutorWithConfig 使用全局配置创建一个新的 HTTP 执行器。
func NewHTTPExecutorWithConfig(globalConfig *HTTPGlobalConfig) *HTTPExecutor {
	return &HTTPExecutor{
		HTTPBaseExecutor: NewHTTPBaseExecutor(HTTPExecutorType, globalConfig),
	}
}

// Init 使用配置初始化 HTTP 执行器。
func (e *HTTPExecutor) Init(ctx context.Context, config map[string]any) error {
	if err := e.BaseExecutor.Init(ctx, config); err != nil {
		return err
	}

	if httpConfig, ok := config["http"].(map[string]any); ok {
		e.ParseGlobalConfig(httpConfig)
	}

	return e.buildClient()
}

// buildClient 构建 HTTP 客户端
func (e *HTTPExecutor) buildClient() error {
	transport, err := e.GlobalConfig.BuildTransport()
	if err != nil {
		return fmt.Errorf("构建 HTTP 传输层失败: %w", err)
	}

	e.client = &http.Client{
		Timeout:       e.GlobalConfig.Timeout.Request,
		Transport:     transport,
		CheckRedirect: e.GlobalConfig.BuildCheckRedirect(),
	}

	return nil
}

// Execute 执行 HTTP 请求步骤。
func (e *HTTPExecutor) Execute(ctx context.Context, step *types.Step, execCtx *ExecutionContext) (*types.StepResult, error) {
	// 1. 创建结果和输出，开头就构建好，过程中逐步填充
	result := types.NewStepResult(step.ID)
	output := &types.HTTPResponseData{}
	result.Output = output
	defer func() {
		result.Finish()
		// 自动填充 output 的收尾字段
		output.Duration = result.Duration.Milliseconds()
		if output.Error != "" {
			// 失败时设置 StatusText 和 Body，让前端面板能展示错误信息
			if output.StatusText == "" {
				output.StatusText = "Error"
			}
			if output.Body == "" {
				output.Body = output.Error
			}
			output.BodyType = "text"
		}
	}()

	// 2. 确保客户端已初始化
	if e.client == nil {
		if err := e.buildClient(); err != nil {
			output.Error = fmt.Sprintf("初始化 HTTP 客户端失败: %s", err.Error())
			result.Fail(err)
			return result, nil
		}
	}

	// 3. 执行前置处理器
	procExecutor := e.ExecutePreProcessors(ctx, step, execCtx)

	// 4. 准备配置（解析 → 合并 → 变量替换 → URL 解析 → 域名头合并）
	config, mergedConfig, err := e.PrepareConfig(step, execCtx)
	if err != nil {
		output.Error = fmt.Sprintf("解析步骤配置失败: %s", err.Error())
		result.Fail(err)
		return result, nil
	}

	// 5. 解析超时
	timeout := e.ResolveTimeout(step, mergedConfig, defaultHTTPTimeout)

	// 6. 保存请求体字符串（用于调试输出）
	reqBodyStr := e.extractBodyString(config)

	// 7. 创建 HTTP 请求
	req, err := e.createRequest(ctx, config, mergedConfig)
	if err != nil {
		output.Error = fmt.Sprintf("创建请求失败: %s", err.Error())
		result.Fail(err)
		return result, nil
	}

	// 8. 捕获请求信息用于调试（构建请求后立即记录）
	output.ActualRequest = e.captureRequestInfo(req, reqBodyStr)

	// 9. 带超时执行
	var resp *http.Response
	execErr := ExecuteWithTimeout(ctx, timeout, func(ctx context.Context) error {
		req = req.WithContext(ctx)
		var doErr error
		resp, doErr = e.client.Do(req)
		return doErr
	})

	if execErr != nil {
		if execErr == context.DeadlineExceeded {
			output.Error = fmt.Sprintf("请求超时（超时时间: %s）", timeout.String())
			result.Timeout(execErr)
		} else {
			output.Error = fmt.Sprintf("HTTP 请求失败: %s", execErr.Error())
			result.Fail(NewExecutionError(step.ID, "HTTP 请求失败", execErr))
		}
		return result, nil
	}
	defer resp.Body.Close()

	// 10. 读取并填充响应数据
	if err := e.fillResponseData(output, resp); err != nil {
		output.Error = fmt.Sprintf("读取响应体失败: %s", err.Error())
		result.Fail(NewExecutionError(step.ID, "读取响应体失败", err))
		return result, nil
	}

	// 11. 执行后置处理器
	e.ExecutePostProcessors(ctx, step, execCtx, procExecutor, output, result.StartTime)

	// 12. 收集日志和断言
	e.CollectLogsAndAssertions(execCtx, output)

	// 13. 添加指标
	result.AddMetric("http_status", float64(resp.StatusCode))
	result.AddMetric("http_response_size", float64(output.Size))

	return result, nil
}

// extractBodyString 从配置中提取请求体字符串（用于调试）。
func (e *HTTPExecutor) extractBodyString(config *HTTPConfig) string {
	if config.Body == nil {
		return ""
	}
	switch body := config.Body.(type) {
	case string:
		return body
	case []byte:
		return string(body)
	default:
		if jsonBody, err := json.Marshal(body); err == nil {
			return string(jsonBody)
		}
		return ""
	}
}

// captureRequestInfo 捕获请求信息用于调试。
func (e *HTTPExecutor) captureRequestInfo(req *http.Request, reqBody string) *types.ActualRequest {
	reqHeaders := make(map[string]string)
	for k, v := range req.Header {
		if len(v) > 0 {
			reqHeaders[k] = v[0]
		}
	}
	return &types.ActualRequest{
		Method:  req.Method,
		URL:     req.URL.String(),
		Headers: reqHeaders,
		Body:    reqBody,
	}
}

// fillResponseData 从 http.Response 填充响应数据到 output。
func (e *HTTPExecutor) fillResponseData(output *types.HTTPResponseData, resp *http.Response) error {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	bodyStr := string(body)
	headers := make(map[string]string)
	for k, v := range resp.Header {
		if len(v) > 0 {
			headers[k] = v[0]
		}
	}

	output.StatusText = resp.Status
	output.StatusCode = resp.StatusCode
	output.Headers = headers
	output.Body = bodyStr
	output.BodyType = types.DetectBodyType(bodyStr)
	output.Size = int64(len(body))

	return nil
}

// createRequest 从配置创建 HTTP 请求。
func (e *HTTPExecutor) createRequest(ctx context.Context, config *HTTPConfig, mergedConfig *HTTPGlobalConfig) (*http.Request, error) {
	// 构建带查询参数的 URL（使用 URL 编码防止特殊字符破坏请求）
	requestURL := config.URL
	if len(config.Params) > 0 {
		params := make([]string, 0, len(config.Params))
		for k, v := range config.Params {
			params = append(params, url.QueryEscape(k)+"="+url.QueryEscape(v))
		}
		if strings.Contains(requestURL, "?") {
			requestURL += "&" + strings.Join(params, "&")
		} else {
			requestURL += "?" + strings.Join(params, "&")
		}
	}

	// 准备请求体和 Content-Type
	var bodyReader io.Reader
	var contentType string
	if config.BodyConfig != nil {
		bodyReader, contentType = e.prepareBody(config.BodyConfig)
	}

	req, err := http.NewRequestWithContext(ctx, config.Method, requestURL, bodyReader)
	if err != nil {
		return nil, NewConfigError("创建 HTTP 请求失败", err)
	}

	// 先设置全局 headers，再设置步骤级 headers（覆盖全局）
	for k, v := range mergedConfig.Headers {
		req.Header.Set(k, v)
	}
	for k, v := range config.Headers {
		req.Header.Set(k, v)
	}

	if contentType != "" && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", contentType)
	}

	return req, nil
}

// prepareBody 准备请求体和 Content-Type。
func (e *HTTPExecutor) prepareBody(body *BodyConfig) (io.Reader, string) {
	var content string
	var contentType string

	switch body.Type {
	case "form-data":
		if len(body.FormData) > 0 {
			content = encodeKeyValues(body.FormData)
			contentType = "multipart/form-data"
		}
	case "x-www-form-urlencoded":
		if len(body.URLEncoded) > 0 {
			content = encodeKeyValues(body.URLEncoded)
			contentType = "application/x-www-form-urlencoded"
		}
	case "json":
		content = body.Raw
		contentType = "application/json"
	case "xml":
		content = body.Raw
		contentType = "application/xml"
	case "text":
		content = body.Raw
		contentType = "text/plain"
	case "graphql":
		content = body.Raw
		contentType = "application/json"
	default:
		content = body.Raw
		contentType = "application/json"
	}

	if content == "" {
		return nil, ""
	}
	return strings.NewReader(content), contentType
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
	Domain         string            `json:"domain,omitempty"`
	DomainBaseURL  string            `json:"domain_base_url,omitempty"`
	DomainHeaders  map[string]string `json:"domain_headers,omitempty"`
	Headers        map[string]string `json:"headers"`
	Body           any               `json:"body"`
	BodyConfig     *BodyConfig       `json:"-"`
	Params         map[string]string `json:"params"`
	SSL            *SSLConfig        `json:"ssl,omitempty"`
	Redirect       *RedirectConfig   `json:"redirect,omitempty"`
	Timeout        *TimeoutConfig    `json:"timeout,omitempty"`
}

// resolveURLWithBase 使用指定的 base URL 解析相对路径。
func resolveURLWithBase(url string, baseURL string) string {
	if len(url) > 0 && (url[0] == 'h' || url[0] == 'H') {
		if len(url) > 7 && (url[:7] == "http://" || url[:8] == "https://") {
			return url
		}
	}

	if baseURL == "" {
		return url
	}

	baseURL = trimRight(baseURL, "/")
	if len(url) == 0 || url[0] != '/' {
		url = "/" + url
	}

	return baseURL + url
}

// init 在默认注册表中注册标准库 HTTP 执行器。
func init() {
	MustRegister(NewHTTPExecutor())
}
