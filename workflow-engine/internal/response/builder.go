package response

import (
	"time"
)

// Builder 响应构建器
type Builder struct {
	response *UnifiedResponse
}

// NewBuilder 创建响应构建器
func NewBuilder() *Builder {
	return &Builder{
		response: &UnifiedResponse{
			Status:   StatusSuccess,
			Headers:  make(map[string]string),
			Metadata: make(map[string]any),
		},
	}
}

// WithStatus 设置状态
func (b *Builder) WithStatus(status ResponseStatus) *Builder {
	b.response.Status = status
	return b
}

// WithStatusCode 设置状态码
func (b *Builder) WithStatusCode(code int) *Builder {
	b.response.StatusCode = code
	return b
}

// WithData 设置数据
func (b *Builder) WithData(data any) *Builder {
	b.response.Data = data
	return b
}

// WithRawBody 设置原始响应体
func (b *Builder) WithRawBody(body []byte) *Builder {
	b.response.RawBody = body
	return b
}

// WithHeaders 设置响应头
func (b *Builder) WithHeaders(headers map[string]string) *Builder {
	for k, v := range headers {
		b.response.Headers[k] = v
	}
	return b
}

// WithHeader 设置单个响应头
func (b *Builder) WithHeader(key, value string) *Builder {
	b.response.Headers[key] = value
	return b
}

// WithDuration 设置响应时间
func (b *Builder) WithDuration(duration time.Duration) *Builder {
	b.response.Duration = duration
	return b
}

// WithError 设置错误信息
func (b *Builder) WithError(err string) *Builder {
	b.response.Error = err
	b.response.Status = StatusError
	return b
}

// WithMetadata 设置元数据
func (b *Builder) WithMetadata(key string, value any) *Builder {
	b.response.Metadata[key] = value
	return b
}

// Build 构建响应
func (b *Builder) Build() *UnifiedResponse {
	return b.response
}

// Success 快速创建成功响应
func Success(data any) *UnifiedResponse {
	return NewBuilder().
		WithStatus(StatusSuccess).
		WithStatusCode(200).
		WithData(data).
		Build()
}

// Error 快速创建错误响应
func Error(code int, err string) *UnifiedResponse {
	return NewBuilder().
		WithStatus(StatusError).
		WithStatusCode(code).
		WithError(err).
		Build()
}

// Timeout 快速创建超时响应
func Timeout(duration time.Duration) *UnifiedResponse {
	return NewBuilder().
		WithStatus(StatusTimeout).
		WithDuration(duration).
		WithError("request timeout").
		Build()
}
