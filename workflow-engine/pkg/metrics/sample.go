package metrics

import (
	"sync"
	"time"
)

// MetricType 定义指标类型
type MetricType string

const (
	// Counter 计数器类型，只增不减
	Counter MetricType = "counter"
	// Gauge 仪表盘类型，可增可减
	Gauge MetricType = "gauge"
	// Rate 比率类型，计算成功/失败比率
	Rate MetricType = "rate"
	// Trend 趋势类型，计算百分位数等统计值
	Trend MetricType = "trend"
)

// Metric 定义一个指标
type Metric struct {
	Name        string            `json:"name"`
	Type        MetricType        `json:"type"`
	Description string            `json:"description,omitempty"`
	Contains    ValueType         `json:"contains,omitempty"`
	Thresholds  []string          `json:"thresholds,omitempty"`
	Tags        map[string]string `json:"tags,omitempty"`
	Sink        Sink              `json:"-"`
}

// ValueType 定义值的类型
type ValueType string

const (
	// Default 默认值类型
	Default ValueType = "default"
	// Time 时间类型（毫秒）
	Time ValueType = "time"
	// Data 数据量类型（字节）
	Data ValueType = "data"
)

// Sample 表示单个指标样本
type Sample struct {
	Metric   *Metric           `json:"metric"`
	Time     time.Time         `json:"time"`
	Value    float64           `json:"value"`
	Tags     map[string]string `json:"tags,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// SampleContainer 是可以返回多个样本的接口
type SampleContainer interface {
	GetSamples() []Sample
}

// Samples 是 Sample 切片，实现 SampleContainer 接口
type Samples []Sample

// GetSamples 返回样本切片
func (s Samples) GetSamples() []Sample {
	return s
}

// ConnectedSamples 表示一组相关的样本（如同一个请求的多个指标）
type ConnectedSamples struct {
	Samples []Sample
	Tags    map[string]string
	Time    time.Time
}

// GetSamples 返回样本切片
func (cs ConnectedSamples) GetSamples() []Sample {
	return cs.Samples
}

// Registry 管理所有已注册的指标
type Registry struct {
	metrics map[string]*Metric
	mu      sync.RWMutex
}

// NewRegistry 创建新的指标注册表
func NewRegistry() *Registry {
	return &Registry{
		metrics: make(map[string]*Metric),
	}
}

// NewMetric 创建并注册新指标
func (r *Registry) NewMetric(name string, metricType MetricType, contains ValueType) *Metric {
	r.mu.Lock()
	defer r.mu.Unlock()

	if m, ok := r.metrics[name]; ok {
		return m
	}

	m := &Metric{
		Name:     name,
		Type:     metricType,
		Contains: contains,
		Tags:     make(map[string]string),
		Sink:     NewSink(metricType),
	}
	r.metrics[name] = m
	return m
}

// Get 获取已注册的指标
func (r *Registry) Get(name string) *Metric {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.metrics[name]
}

// All 返回所有已注册的指标
func (r *Registry) All() map[string]*Metric {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make(map[string]*Metric, len(r.metrics))
	for k, v := range r.metrics {
		result[k] = v
	}
	return result
}
