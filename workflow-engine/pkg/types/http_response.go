package types

// HTTPResponseData 统一的 HTTP 响应数据结构
// 用于单步调试和流程执行的响应返回
type HTTPResponseData struct {
	// 响应状态
	StatusCode int    `json:"statusCode"`
	StatusText string `json:"statusText,omitempty"`
	Duration   int64  `json:"duration"` // 毫秒
	Size       int64  `json:"size"`     // 字节

	// 响应内容
	Headers  map[string]string `json:"headers,omitempty"`
	Cookies  map[string]string `json:"cookies,omitempty"`
	Body     string            `json:"body,omitempty"`
	BodyType string            `json:"bodyType,omitempty"` // json, xml, html, text

	// 控制台日志（统一收集处理器执行结果和脚本日志）
	ConsoleLogs []ConsoleLogEntry `json:"consoleLogs,omitempty"`

	// 断言结果
	Assertions []AssertionResult `json:"assertions,omitempty"`

	// 实际请求（调试用）
	ActualRequest *ActualRequest `json:"actualRequest,omitempty"`

	// 错误信息
	Error string `json:"error,omitempty"`
}

// ActualRequest 实际发送的请求
type ActualRequest struct {
	URL     string            `json:"url"`
	Method  string            `json:"method"`
	Headers map[string]string `json:"headers,omitempty"`
	Body    string            `json:"body,omitempty"`
}

// AssertionResult 断言结果
type AssertionResult struct {
	ID       string `json:"id,omitempty"`
	Name     string `json:"name"`
	Passed   bool   `json:"passed"`
	Message  string `json:"message,omitempty"`
	Expected string `json:"expected,omitempty"`
	Actual   string `json:"actual,omitempty"`
}

// ToMap 转换为 map（用于兼容现有的输出格式）
func (r *HTTPResponseData) ToMap() map[string]interface{} {
	result := map[string]interface{}{
		"status_code": r.StatusCode,
		"statusCode":  r.StatusCode,
		"status":      r.StatusText,
		"duration":    r.Duration,
		"size":        r.Size,
		"body":        r.Body,
		"body_raw":    r.Body,
		"headers":     r.Headers,
	}

	if r.Cookies != nil {
		result["cookies"] = r.Cookies
	}

	if r.ConsoleLogs != nil {
		result["console_logs"] = r.ConsoleLogs
	}

	if r.Assertions != nil {
		result["assertions"] = r.Assertions
	}

	if r.ActualRequest != nil {
		result["request"] = r.ActualRequest
	}

	return result
}

// DetectBodyType 检测响应体类型
func DetectBodyType(body string) string {
	if body == "" {
		return "text"
	}
	if len(body) > 0 && (body[0] == '{' || body[0] == '[') {
		return "json"
	}
	if len(body) > 5 && body[:5] == "<?xml" {
		return "xml"
	}
	if len(body) > 0 && body[0] == '<' {
		if len(body) > 15 && (contains(body[:min(100, len(body))], "<!DOCTYPE html") || contains(body[:min(100, len(body))], "<html")) {
			return "html"
		}
		return "xml"
	}
	return "text"
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
