package executor

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"yqhp/workflow-engine/pkg/types"

	"github.com/valyala/fasthttp"
)

const (
	// FastHTTPExecutorType 是 FastHTTP 执行器的类型标识符。
	FastHTTPExecutorType = "http"

	// FastHTTP 请求的默认超时时间。
	defaultFastHTTPTimeout = 30 * time.Second
)

var (
	// 全局共享的 FastHTTP 客户端，多 VU 共享连接池
	globalFastHTTPClient     *fasthttp.Client
	globalFastHTTPClientOnce sync.Once
)

// FastHTTPExecutor 使用 fasthttp 执行 HTTP 请求步骤，性能更优。
type FastHTTPExecutor struct {
	*HTTPBaseExecutor
	client *fasthttp.Client
}

// NewFastHTTPExecutor 创建一个新的 FastHTTP 执行器。
func NewFastHTTPExecutor() *FastHTTPExecutor {
	return &FastHTTPExecutor{
		HTTPBaseExecutor: NewHTTPBaseExecutor(FastHTTPExecutorType, nil),
	}
}

// NewFastHTTPExecutorWithConfig 使用全局配置创建一个新的 FastHTTP 执行器。
func NewFastHTTPExecutorWithConfig(globalConfig *HTTPGlobalConfig) *FastHTTPExecutor {
	return &FastHTTPExecutor{
		HTTPBaseExecutor: NewHTTPBaseExecutor(FastHTTPExecutorType, globalConfig),
	}
}

// Init 使用配置初始化 FastHTTP 执行器。
func (e *FastHTTPExecutor) Init(ctx context.Context, config map[string]any) error {
	if err := e.BaseExecutor.Init(ctx, config); err != nil {
		return err
	}

	if httpConfig, ok := config["http"].(map[string]any); ok {
		e.ParseGlobalConfig(httpConfig)
	}

	return e.buildClient()
}

// buildClient 构建 FastHTTP 客户端（全局共享，仅初始化一次）
func (e *FastHTTPExecutor) buildClient() error {
	var initErr error
	globalFastHTTPClientOnce.Do(func() {
		tlsConfig, err := e.GlobalConfig.SSL.BuildTLSConfig()
		if err != nil {
			initErr = fmt.Errorf("构建 TLS 配置失败: %w", err)
			return
		}

		globalFastHTTPClient = &fasthttp.Client{
			MaxConnsPerHost:        1000,
			MaxIdleConnDuration:    90 * time.Second,
			ReadTimeout:            e.GlobalConfig.Timeout.Read,
			WriteTimeout:           e.GlobalConfig.Timeout.Write,
			TLSConfig:              tlsConfig,
			DisablePathNormalizing: true,
		}
	})
	if initErr != nil {
		return initErr
	}
	e.client = globalFastHTTPClient
	return nil
}

// Execute 执行 HTTP 请求步骤。
func (e *FastHTTPExecutor) Execute(ctx context.Context, step *types.Step, execCtx *ExecutionContext) (*types.StepResult, error) {
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
	timeout := e.ResolveTimeout(step, mergedConfig, defaultFastHTTPTimeout)

	// 6. 获取请求和响应对象（使用对象池）
	req := fasthttp.AcquireRequest()
	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseRequest(req)
	defer fasthttp.ReleaseResponse(resp)

	// 7. 构建请求
	if err := e.buildRequest(req, config, mergedConfig); err != nil {
		output.Error = fmt.Sprintf("构建请求失败: %s", err.Error())
		result.Fail(err)
		return result, nil
	}

	// 8. 捕获请求信息用于调试（构建请求后立即记录，这样即使请求失败也能看到）
	output.ActualRequest = e.captureRequestInfo(req, config)

	// 9. 执行请求（统一使用 DoDeadline 确保超时始终生效）
	var execErr error
	deadline := time.Now().Add(timeout)
	if e.GlobalConfig.Redirect.GetFollow() {
		// 使用 DoRedirects + deadline：先设置请求超时，再执行重定向
		req.SetTimeout(timeout)
		execErr = e.client.DoRedirects(req, resp, e.GlobalConfig.Redirect.GetMaxRedirects())
	} else {
		execErr = e.client.DoDeadline(req, resp, deadline)
	}

	if execErr != nil {
		if execErr == fasthttp.ErrTimeout || time.Now().After(deadline) {
			output.Error = fmt.Sprintf("请求超时（超时时间: %s）", timeout.String())
			result.Timeout(execErr)
		} else {
			output.Error = fmt.Sprintf("HTTP 请求失败: %s", execErr.Error())
			result.Fail(NewExecutionError(step.ID, "HTTP 请求失败", execErr))
		}
		return result, nil
	}

	// 10. 填充响应数据
	e.fillResponseData(output, resp)

	// 11. 执行后置处理器
	e.ExecutePostProcessors(ctx, step, execCtx, procExecutor, output, result.StartTime)

	// 12. 收集日志和断言
	e.CollectLogsAndAssertions(execCtx, output)

	// 13. 添加指标
	result.AddMetric("http_status", float64(resp.StatusCode()))
	result.AddMetric("http_response_size", float64(len(resp.Body())))
	result.AddMetric("data_received", float64(len(resp.Body())))
	result.AddMetric("data_sent", float64(len(req.Body())))

	return result, nil
}

// captureRequestInfo 捕获请求信息用于调试。
func (e *FastHTTPExecutor) captureRequestInfo(req *fasthttp.Request, config *HTTPConfig) *types.ActualRequest {
	reqInfo := &types.ActualRequest{
		Method:  config.Method,
		URL:     string(req.URI().FullURI()),
		Headers: make(map[string]string),
	}
	req.Header.VisitAll(func(key, value []byte) {
		reqInfo.Headers[string(key)] = string(value)
	})
	if len(req.Body()) > 0 {
		reqInfo.Body = string(req.Body())
	}
	return reqInfo
}

// fillResponseData 从 fasthttp.Response 填充响应数据到 output。
func (e *FastHTTPExecutor) fillResponseData(output *types.HTTPResponseData, resp *fasthttp.Response) {
	// 复制响应体（resp.Body() 返回的是内部缓冲区的引用）
	bodyBytes := make([]byte, len(resp.Body()))
	copy(bodyBytes, resp.Body())
	bodyStr := string(bodyBytes)

	headers := make(map[string]string)
	resp.Header.VisitAll(func(key, value []byte) {
		k := string(key)
		if _, exists := headers[k]; !exists {
			headers[k] = string(value)
		}
	})

	output.StatusText = fmt.Sprintf("%d %s", resp.StatusCode(), fasthttp.StatusMessage(resp.StatusCode()))
	output.StatusCode = resp.StatusCode()
	output.Headers = headers
	output.Body = bodyStr
	output.BodyType = types.DetectBodyType(bodyStr)
	output.Size = int64(len(bodyBytes))
}

// buildRequest 构建 FastHTTP 请求。
func (e *FastHTTPExecutor) buildRequest(req *fasthttp.Request, config *HTTPConfig, mergedConfig *HTTPGlobalConfig) error {
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

	req.Header.SetMethod(config.Method)
	req.SetRequestURI(requestURL)

	// 先设置全局 headers，再设置步骤级 headers（覆盖全局）
	for k, v := range mergedConfig.Headers {
		req.Header.Set(k, v)
	}
	for k, v := range config.Headers {
		req.Header.Set(k, v)
	}

	// 设置请求体
	if config.BodyConfig != nil {
		e.setRequestBody(req, config.BodyConfig)
	}

	return nil
}

// setRequestBody 设置请求体。
func (e *FastHTTPExecutor) setRequestBody(req *fasthttp.Request, body *BodyConfig) {
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

	if content != "" {
		req.SetBodyString(content)
		if len(req.Header.ContentType()) == 0 {
			req.Header.SetContentType(contentType)
		}
	}
}

// ConfigureTLS 配置 TLS（用于 HTTPS 请求）
func (e *FastHTTPExecutor) ConfigureTLS(tlsConfig *tls.Config) {
	if e.client != nil {
		e.client.TLSConfig = tlsConfig
	}
}

// Cleanup 释放 FastHTTP 执行器持有的资源。
func (e *FastHTTPExecutor) Cleanup(ctx context.Context) error {
	return nil
}

// encodeKeyValues 将 key-value map 编码为 URL 编码字符串。
func encodeKeyValues(kv map[string]string) string {
	parts := make([]string, 0, len(kv))
	for k, v := range kv {
		parts = append(parts, url.QueryEscape(k)+"="+url.QueryEscape(v))
	}
	return strings.Join(parts, "&")
}

// init 在默认注册表中注册 FastHTTP 执行器（替代原有的 HTTP 执行器）。
func init() {
	MustRegister(NewFastHTTPExecutor())
}
