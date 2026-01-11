package metrics

import (
	"math"
	"sort"
	"sync"
	"time"
)

// 确保 time 包被使用
var _ = time.Now

// Sink 定义指标聚合器接口
type Sink interface {
	// Add 添加一个样本值
	Add(sample Sample)
	// Format 返回格式化的统计结果
	Format(duration float64) map[string]float64
	// IsEmpty 检查是否为空
	IsEmpty() bool
}

// NewSink 根据指标类型创建对应的 Sink
func NewSink(metricType MetricType) Sink {
	switch metricType {
	case Counter:
		return &CounterSink{}
	case Gauge:
		return &GaugeSink{}
	case Rate:
		return &RateSink{}
	case Trend:
		return &TrendSink{}
	default:
		return &CounterSink{}
	}
}

// CounterSink 计数器聚合器
type CounterSink struct {
	Value float64
	First time.Time
	mu    sync.Mutex
}

// Add 添加样本
func (c *CounterSink) Add(sample Sample) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Value += sample.Value
	if c.First.IsZero() {
		c.First = sample.Time
	}
}

// Format 返回统计结果
func (c *CounterSink) Format(duration float64) map[string]float64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	result := map[string]float64{
		"count": c.Value,
	}
	if duration > 0 {
		result["rate"] = c.Value / duration
	}
	return result
}

// IsEmpty 检查是否为空
func (c *CounterSink) IsEmpty() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.Value == 0
}

// GaugeSink 仪表盘聚合器
type GaugeSink struct {
	Value  float64
	Min    float64
	Max    float64
	Sum    float64
	Count  int64
	minSet bool
	mu     sync.Mutex
}

// Add 添加样本
func (g *GaugeSink) Add(sample Sample) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.Value = sample.Value
	g.Sum += sample.Value
	g.Count++
	if !g.minSet || sample.Value < g.Min {
		g.Min = sample.Value
		g.minSet = true
	}
	if sample.Value > g.Max {
		g.Max = sample.Value
	}
}

// Format 返回统计结果
func (g *GaugeSink) Format(duration float64) map[string]float64 {
	g.mu.Lock()
	defer g.mu.Unlock()
	result := map[string]float64{
		"value": g.Value,
		"min":   g.Min,
		"max":   g.Max,
	}
	if g.Count > 0 {
		result["avg"] = g.Sum / float64(g.Count)
	}
	return result
}

// IsEmpty 检查是否为空
func (g *GaugeSink) IsEmpty() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.Count == 0
}

// RateSink 比率聚合器
type RateSink struct {
	Trues int64
	Total int64
	mu    sync.Mutex
}

// Add 添加样本（value != 0 表示 true/成功）
func (r *RateSink) Add(sample Sample) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.Total++
	if sample.Value != 0 {
		r.Trues++
	}
}

// Format 返回统计结果
func (r *RateSink) Format(duration float64) map[string]float64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	result := map[string]float64{
		"passes": float64(r.Trues),
		"fails":  float64(r.Total - r.Trues),
	}
	if r.Total > 0 {
		result["rate"] = float64(r.Trues) / float64(r.Total)
	} else {
		result["rate"] = 0
	}
	return result
}

// IsEmpty 检查是否为空
func (r *RateSink) IsEmpty() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.Total == 0
}

// TrendSink 趋势聚合器，计算百分位数
type TrendSink struct {
	Values []float64
	Count  int64
	Sum    float64
	Min    float64
	Max    float64
	minSet bool
	mu     sync.Mutex
}

// Add 添加样本
func (t *TrendSink) Add(sample Sample) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.Values = append(t.Values, sample.Value)
	t.Count++
	t.Sum += sample.Value
	if !t.minSet || sample.Value < t.Min {
		t.Min = sample.Value
		t.minSet = true
	}
	if sample.Value > t.Max {
		t.Max = sample.Value
	}
}

// Format 返回统计结果
func (t *TrendSink) Format(duration float64) map[string]float64 {
	t.mu.Lock()
	defer t.mu.Unlock()

	result := map[string]float64{
		"count": float64(t.Count),
		"min":   t.Min,
		"max":   t.Max,
	}

	if t.Count > 0 {
		result["avg"] = t.Sum / float64(t.Count)
		result["med"] = t.percentile(50)
		result["p(90)"] = t.percentile(90)
		result["p(95)"] = t.percentile(95)
		result["p(99)"] = t.percentile(99)
	}

	return result
}

// percentile 计算百分位数（需要在持有锁的情况下调用）
func (t *TrendSink) percentile(p float64) float64 {
	if len(t.Values) == 0 {
		return 0
	}

	// 复制并排序
	sorted := make([]float64, len(t.Values))
	copy(sorted, t.Values)
	sort.Float64s(sorted)

	// 计算索引
	rank := (p / 100) * float64(len(sorted)-1)
	lower := int(math.Floor(rank))
	upper := int(math.Ceil(rank))

	if lower == upper {
		return sorted[lower]
	}

	// 线性插值
	weight := rank - float64(lower)
	return sorted[lower]*(1-weight) + sorted[upper]*weight
}

// IsEmpty 检查是否为空
func (t *TrendSink) IsEmpty() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.Count == 0
}

// Percentile 计算指定百分位数（公开方法，会加锁）
func (t *TrendSink) Percentile(p float64) float64 {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.percentile(p)
}
