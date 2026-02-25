package slave

import (
	"sync"
	"time"

	"github.com/HdrHistogram/hdrhistogram-go"

	"yqhp/workflow-engine/pkg/metrics"
	"yqhp/workflow-engine/pkg/output"
	"yqhp/workflow-engine/pkg/types"
)

// MetricsCollector 收集和聚合执行指标。
// 使用 HDR Histogram 实现固定内存的高精度百分位数计算。
// Requirements: 9.1, 9.5
type MetricsCollector struct {
	stepMetrics map[string]*stepMetricsData
	startTime   time.Time
	mu          sync.RWMutex

	// 输出相关
	samplesChan chan metrics.SampleContainer
	emitter     *output.SampleEmitter
}

// stepMetricsData 保存步骤的聚合指标数据。
// 使用 HDR Histogram 实现高精度百分位数计算，固定内存占用。
type stepMetricsData struct {
	count        int64
	successCount int64
	failureCount int64

	// 实时聚合的耗时统计
	minDuration time.Duration
	maxDuration time.Duration
	sumDuration time.Duration

	// HDR Histogram 用于精确计算百分位数（固定内存，约 20KB）
	histogram *hdrhistogram.Histogram

	// 自定义指标（聚合值，不存原始数据）
	customMetrics   map[string]*customMetricData
	customMetricsMu sync.RWMutex

	mu sync.Mutex
}

// customMetricData 保存自定义指标的聚合数据
type customMetricData struct {
	count int64
	sum   float64
	min   float64
	max   float64
}

// newStepMetricsData 创建新的步骤指标数据
func newStepMetricsData() *stepMetricsData {
	return &stepMetricsData{
		// HDR Histogram 参数:
		// - 最小值: 1 微秒
		// - 最大值: 3600 秒 (1小时) = 3,600,000,000 微秒
		// - 精度: 3 位有效数字 (误差 < 0.1%)
		// 内存占用约 20KB
		histogram:     hdrhistogram.New(1, 3600000000, 3),
		customMetrics: make(map[string]*customMetricData),
	}
}

// NewMetricsCollector 创建一个新的指标收集器。
func NewMetricsCollector() *MetricsCollector {
	return &MetricsCollector{
		stepMetrics: make(map[string]*stepMetricsData),
		startTime:   time.Now(),
	}
}

// SetSamplesChannel 设置样本输出通道
func (c *MetricsCollector) SetSamplesChannel(ch chan metrics.SampleContainer, tags map[string]string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.samplesChan = ch
	if ch != nil {
		c.emitter = output.NewSampleEmitter(ch, tags)
	}
}

// GetSamplesChannel 获取样本输出通道
func (c *MetricsCollector) GetSamplesChannel() chan metrics.SampleContainer {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.samplesChan
}

// RecordStep 记录步骤执行的指标。
// Requirements: 9.1
func (c *MetricsCollector) RecordStep(stepID, stepName string, result *types.StepResult) {
	if result == nil {
		return
	}

	c.mu.Lock()
	data, exists := c.stepMetrics[stepID]
	if !exists {
		data = newStepMetricsData()
		c.stepMetrics[stepID] = data
	}
	c.mu.Unlock()

	// 使用步骤级别的锁，减少锁竞争
	data.mu.Lock()
	defer data.mu.Unlock()

	// 更新计数
	data.count++

	switch result.Status {
	case types.ResultStatusSuccess:
		data.successCount++
	case types.ResultStatusFailed, types.ResultStatusTimeout:
		data.failureCount++
	}

	// 更新耗时统计
	duration := result.Duration
	durationMicros := duration.Microseconds()

	data.sumDuration += duration
	if data.minDuration == 0 || duration < data.minDuration {
		data.minDuration = duration
	}
	if duration > data.maxDuration {
		data.maxDuration = duration
	}

	// 记录到 HDR Histogram
	// 注意: HDR Histogram 只接受正整数，最小值为 1
	if durationMicros < 1 {
		durationMicros = 1
	}
	data.histogram.RecordValue(durationMicros)

	// 记录自定义指标
	for k, v := range result.Metrics {
		data.customMetricsMu.Lock()
		cm, exists := data.customMetrics[k]
		if !exists {
			cm = &customMetricData{
				min: v,
				max: v,
			}
			data.customMetrics[k] = cm
		}
		cm.count++
		cm.sum += v
		if v < cm.min {
			cm.min = v
		}
		if v > cm.max {
			cm.max = v
		}
		data.customMetricsMu.Unlock()
	}

	// 发送样本到输出通道
	// 指标名包含 step_id 后缀，让 MetricsEngine 按步骤分别聚合
	if c.emitter != nil {
		tags := map[string]string{
			"step_id":   stepID,
			"step_name": stepName,
		}

		c.emitter.EmitCounter("step_reqs_"+stepID, 1, tags)
		c.emitter.EmitTrend("step_duration_"+stepID, float64(duration.Milliseconds()), tags)

		success := result.Status == types.ResultStatusSuccess
		c.emitter.EmitRate("step_failed_"+stepID, success, tags)

		// 全局聚合指标（用于 time-series 计算）
		c.emitter.EmitTrend("step_duration", float64(duration.Milliseconds()), tags)
		c.emitter.EmitRate("step_failed", success, tags)

		for k, v := range result.Metrics {
			if k == "data_sent" || k == "data_received" {
				c.emitter.EmitCounter(k, v, tags)
			} else {
				c.emitter.EmitGauge("custom_"+k, v, tags)
			}
		}
	}
}

// GetMetrics 返回聚合后的指标。
// Requirements: 9.1, 9.5
func (c *MetricsCollector) GetMetrics() *types.Metrics {
	c.mu.RLock()
	defer c.mu.RUnlock()

	m := &types.Metrics{
		Timestamp:   time.Now(),
		StepMetrics: make(map[string]*types.StepMetrics),
	}

	for stepID, data := range c.stepMetrics {
		stepMetrics := &types.StepMetrics{
			StepID:        stepID,
			Count:         data.count,
			SuccessCount:  data.successCount,
			FailureCount:  data.failureCount,
			Duration:      c.calculateDurationMetrics(data),
			CustomMetrics: c.getCustomMetrics(data),
		}
		m.StepMetrics[stepID] = stepMetrics
	}

	return m
}

// calculateDurationMetrics 从 HDR Histogram 计算耗时统计。
// Requirements: 9.5
func (c *MetricsCollector) calculateDurationMetrics(data *stepMetricsData) *types.DurationMetrics {
	data.mu.Lock()
	defer data.mu.Unlock()

	if data.count == 0 {
		return &types.DurationMetrics{}
	}

	avgDuration := time.Duration(int64(data.sumDuration) / data.count)

	// 从 HDR Histogram 获取百分位数（微秒）
	p50 := time.Duration(data.histogram.ValueAtQuantile(50)) * time.Microsecond
	p90 := time.Duration(data.histogram.ValueAtQuantile(90)) * time.Microsecond
	p95 := time.Duration(data.histogram.ValueAtQuantile(95)) * time.Microsecond
	p99 := time.Duration(data.histogram.ValueAtQuantile(99)) * time.Microsecond

	return &types.DurationMetrics{
		Min: data.minDuration,
		Max: data.maxDuration,
		Avg: avgDuration,
		P50: p50,
		P90: p90,
		P95: p95,
		P99: p99,
	}
}

// getCustomMetrics 获取自定义指标的聚合值
func (c *MetricsCollector) getCustomMetrics(data *stepMetricsData) map[string]float64 {
	data.customMetricsMu.RLock()
	defer data.customMetricsMu.RUnlock()

	result := make(map[string]float64)
	for k, cm := range data.customMetrics {
		if cm.count > 0 {
			result[k] = cm.sum / float64(cm.count) // 返回平均值
		}
	}
	return result
}

// GetThroughput 返回当前吞吐量（每秒请求数）。
func (c *MetricsCollector) GetThroughput() float64 {
	c.mu.RLock()
	defer c.mu.RUnlock()

	elapsed := time.Since(c.startTime).Seconds()
	if elapsed <= 0 {
		return 0
	}

	var totalCount int64
	for _, data := range c.stepMetrics {
		totalCount += data.count
	}

	return float64(totalCount) / elapsed
}

// GetStepMetrics 返回指定步骤的指标。
func (c *MetricsCollector) GetStepMetrics(stepID string) *types.StepMetrics {
	c.mu.RLock()
	defer c.mu.RUnlock()

	data, exists := c.stepMetrics[stepID]
	if !exists {
		return nil
	}

	return &types.StepMetrics{
		StepID:        stepID,
		Count:         data.count,
		SuccessCount:  data.successCount,
		FailureCount:  data.failureCount,
		Duration:      c.calculateDurationMetrics(data),
		CustomMetrics: c.getCustomMetrics(data),
	}
}

// Reset 重置所有已收集的指标。
func (c *MetricsCollector) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.stepMetrics = make(map[string]*stepMetricsData)
	c.startTime = time.Now()
}

// GetErrorRate 返回总体错误率。
func (c *MetricsCollector) GetErrorRate() float64 {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var totalCount, failureCount int64
	for _, data := range c.stepMetrics {
		totalCount += data.count
		failureCount += data.failureCount
	}

	if totalCount == 0 {
		return 0
	}

	return float64(failureCount) / float64(totalCount)
}

// GetDurationSamples 返回指定步骤的耗时百分位数样本。
// 返回关键百分位数作为样本，用于测试和调试。
func (c *MetricsCollector) GetDurationSamples(stepID string) []time.Duration {
	c.mu.RLock()
	defer c.mu.RUnlock()

	data, exists := c.stepMetrics[stepID]
	if !exists {
		return nil
	}

	data.mu.Lock()
	defer data.mu.Unlock()

	// 返回关键百分位数作为样本
	return []time.Duration{
		data.minDuration,
		time.Duration(data.histogram.ValueAtQuantile(25)) * time.Microsecond,
		time.Duration(data.histogram.ValueAtQuantile(50)) * time.Microsecond,
		time.Duration(data.histogram.ValueAtQuantile(75)) * time.Microsecond,
		time.Duration(data.histogram.ValueAtQuantile(90)) * time.Microsecond,
		time.Duration(data.histogram.ValueAtQuantile(95)) * time.Microsecond,
		time.Duration(data.histogram.ValueAtQuantile(99)) * time.Microsecond,
		data.maxDuration,
	}
}

// GetMemoryUsage 返回估算的内存使用量（字节）
func (c *MetricsCollector) GetMemoryUsage() int64 {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var total int64
	for _, data := range c.stepMetrics {
		// HDR Histogram 内存占用约 20KB (3 位精度)
		total += 20 * 1024
		// 自定义指标
		data.customMetricsMu.RLock()
		total += int64(len(data.customMetrics) * 40) // 每个指标约 40 字节
		data.customMetricsMu.RUnlock()
	}
	return total
}

// GetHistogramSnapshot 返回指定步骤的直方图快照（用于调试）
func (c *MetricsCollector) GetHistogramSnapshot(stepID string) *hdrhistogram.Snapshot {
	c.mu.RLock()
	defer c.mu.RUnlock()

	data, exists := c.stepMetrics[stepID]
	if !exists {
		return nil
	}

	data.mu.Lock()
	defer data.mu.Unlock()

	return data.histogram.Export()
}

// MergeHistogram 合并另一个直方图的数据（用于分布式聚合）
func (c *MetricsCollector) MergeHistogram(stepID string, snapshot *hdrhistogram.Snapshot) error {
	c.mu.Lock()
	data, exists := c.stepMetrics[stepID]
	if !exists {
		data = newStepMetricsData()
		c.stepMetrics[stepID] = data
	}
	c.mu.Unlock()

	data.mu.Lock()
	defer data.mu.Unlock()

	// 导入快照数据
	imported := hdrhistogram.Import(snapshot)
	data.histogram.Merge(imported)

	return nil
}
