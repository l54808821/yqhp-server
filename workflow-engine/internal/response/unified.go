// Package response 提供工作流引擎 v2 的统一响应处理。
package response

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// UnifiedResponse 统一响应结构
type UnifiedResponse struct {
	// 响应状态
	Status ResponseStatus `json:"status"`

	// 状态码（HTTP 状态码或自定义状态码）
	StatusCode int `json:"status_code"`

	// 响应数据（自动解析 JSON）
	Data any `json:"data,omitempty"`

	// 原始响应体
	RawBody []byte `json:"raw_body,omitempty"`

	// 响应头
	Headers map[string]string `json:"headers,omitempty"`

	// 响应时间
	Duration time.Duration `json:"duration"`

	// 错误信息
	Error string `json:"error,omitempty"`

	// 元数据
	Metadata map[string]any `json:"metadata,omitempty"`
}

// ResponseStatus 响应状态
type ResponseStatus string

const (
	StatusSuccess ResponseStatus = "success"
	StatusError   ResponseStatus = "error"
	StatusTimeout ResponseStatus = "timeout"
)

// NewSuccessResponse 创建成功响应
func NewSuccessResponse(statusCode int, data any, duration time.Duration) *UnifiedResponse {
	return &UnifiedResponse{
		Status:     StatusSuccess,
		StatusCode: statusCode,
		Data:       data,
		Duration:   duration,
		Headers:    make(map[string]string),
		Metadata:   make(map[string]any),
	}
}

// NewErrorResponse 创建错误响应
func NewErrorResponse(statusCode int, err error, duration time.Duration) *UnifiedResponse {
	errMsg := ""
	if err != nil {
		errMsg = err.Error()
	}
	return &UnifiedResponse{
		Status:     StatusError,
		StatusCode: statusCode,
		Error:      errMsg,
		Duration:   duration,
		Headers:    make(map[string]string),
		Metadata:   make(map[string]any),
	}
}

// NewTimeoutResponse 创建超时响应
func NewTimeoutResponse(duration time.Duration) *UnifiedResponse {
	return &UnifiedResponse{
		Status:     StatusTimeout,
		StatusCode: 0,
		Error:      "request timeout",
		Duration:   duration,
		Headers:    make(map[string]string),
		Metadata:   make(map[string]any),
	}
}

// FromHTTPResponse 从 HTTP 响应创建统一响应
func FromHTTPResponse(resp *http.Response, body []byte, duration time.Duration) *UnifiedResponse {
	result := &UnifiedResponse{
		Status:     StatusSuccess,
		StatusCode: resp.StatusCode,
		RawBody:    body,
		Duration:   duration,
		Headers:    make(map[string]string),
		Metadata:   make(map[string]any),
	}

	// 复制响应头
	for key := range resp.Header {
		result.Headers[key] = resp.Header.Get(key)
	}

	// 自动解析 JSON
	result.Data = result.parseBody(body)

	// 判断是否为错误状态
	if resp.StatusCode >= 400 {
		result.Status = StatusError
	}

	return result
}

// parseBody 解析响应体
func (r *UnifiedResponse) parseBody(body []byte) any {
	if len(body) == 0 {
		return nil
	}

	// 尝试解析为 JSON
	var jsonData any
	if err := json.Unmarshal(body, &jsonData); err == nil {
		return jsonData
	}

	// 返回原始字符串
	return string(body)
}

// IsSuccess 判断是否成功
func (r *UnifiedResponse) IsSuccess() bool {
	return r.Status == StatusSuccess
}

// IsError 判断是否错误
func (r *UnifiedResponse) IsError() bool {
	return r.Status == StatusError
}

// IsTimeout 判断是否超时
func (r *UnifiedResponse) IsTimeout() bool {
	return r.Status == StatusTimeout
}

// GetString 获取字符串数据
func (r *UnifiedResponse) GetString() string {
	if r.Data == nil {
		return ""
	}
	if s, ok := r.Data.(string); ok {
		return s
	}
	data, _ := json.Marshal(r.Data)
	return string(data)
}

// GetJSON 获取 JSON 数据
func (r *UnifiedResponse) GetJSON() map[string]any {
	if r.Data == nil {
		return nil
	}
	if m, ok := r.Data.(map[string]any); ok {
		return m
	}
	return nil
}

// GetList 获取列表数据
func (r *UnifiedResponse) GetList() []any {
	if r.Data == nil {
		return nil
	}
	if list, ok := r.Data.([]any); ok {
		return list
	}
	return nil
}

// GetHeader 获取响应头
func (r *UnifiedResponse) GetHeader(key string) string {
	if r.Headers == nil {
		return ""
	}
	return r.Headers[key]
}

// SetMetadata 设置元数据
func (r *UnifiedResponse) SetMetadata(key string, value any) {
	if r.Metadata == nil {
		r.Metadata = make(map[string]any)
	}
	r.Metadata[key] = value
}

// GetMetadata 获取元数据
func (r *UnifiedResponse) GetMetadata(key string) any {
	if r.Metadata == nil {
		return nil
	}
	return r.Metadata[key]
}

// ToMap 转换为 map（用于变量存储）
func (r *UnifiedResponse) ToMap() map[string]any {
	return map[string]any{
		"status":      string(r.Status),
		"status_code": r.StatusCode,
		"data":        r.Data,
		"headers":     r.Headers,
		"duration":    r.Duration.Milliseconds(),
		"error":       r.Error,
		"metadata":    r.Metadata,
	}
}

// String 返回字符串表示
func (r *UnifiedResponse) String() string {
	return fmt.Sprintf("UnifiedResponse{status=%s, status_code=%d, duration=%v}",
		r.Status, r.StatusCode, r.Duration)
}
