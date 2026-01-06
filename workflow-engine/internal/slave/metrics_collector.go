package slave

import (
	"sort"
	"sync"
	"time"

	"yqhp/workflow-engine/pkg/types"
)

// MetricsCollector 收集和聚合执行指标。
// Requirements: 9.1, 9.5
type MetricsCollector struct {
	stepMetrics map[string]*stepMetricsData
	startTime   time.Time
	mu          sync.RWMutex
}

// stepMetricsData 保存步骤的原始指标数据。
type stepMetricsData struct {
	count         int64
	successCount  int64
	failureCount  int64
	durations     []time.Duration
	customMetrics map[string][]float64
}

// NewMetricsCollector 创建一个新的指标收集器。
func NewMetricsCollector() *MetricsCollector {
	return &MetricsCollector{
		stepMetrics: make(map[string]*stepMetricsData),
		startTime:   time.Now(),
	}
}

// RecordStep 记录步骤执行的指标。
// Requirements: 9.1
func (c *MetricsCollector) RecordStep(stepID string, result *types.StepResult) {
	if result == nil {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	data, exists := c.stepMetrics[stepID]
	if !exists {
		data = &stepMetricsData{
			durations:     make([]time.Duration, 0, 100),
			customMetrics: make(map[string][]float64),
		}
		c.stepMetrics[stepID] = data
	}

	data.count++
	data.durations = append(data.durations, result.Duration)

	switch result.Status {
	case types.ResultStatusSuccess:
		data.successCount++
	case types.ResultStatusFailed, types.ResultStatusTimeout:
		data.failureCount++
	}

	// 记录自定义指标
	for k, v := range result.Metrics {
		if data.customMetrics[k] == nil {
			data.customMetrics[k] = make([]float64, 0, 100)
		}
		data.customMetrics[k] = append(data.customMetrics[k], v)
	}
}

// GetMetrics 返回聚合后的指标。
// Requirements: 9.1, 9.5
func (c *MetricsCollector) GetMetrics() *types.Metrics {
	c.mu.RLock()
	defer c.mu.RUnlock()

	metrics := &types.Metrics{
		Timestamp:   time.Now(),
		StepMetrics: make(map[string]*types.StepMetrics),
	}

	for stepID, data := range c.stepMetrics {
		stepMetrics := &types.StepMetrics{
			StepID:        stepID,
			Count:         data.count,
			SuccessCount:  data.successCount,
			FailureCount:  data.failureCount,
			Duration:      c.calculateDurationMetrics(data.durations),
			CustomMetrics: c.aggregateCustomMetrics(data.customMetrics),
		}
		metrics.StepMetrics[stepID] = stepMetrics
	}

	return metrics
}

// calculateDurationMetrics 计算耗时统计数据，包括百分位数。
// Requirements: 9.5
func (c *MetricsCollector) calculateDurationMetrics(durations []time.Duration) *types.DurationMetrics {
	if len(durations) == 0 {
		return &types.DurationMetrics{}
	}

	// 对耗时进行排序以计算百分位数
	sorted := make([]time.Duration, len(durations))
	copy(sorted, durations)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i] < sorted[j]
	})

	// 计算统计数据
	var sum time.Duration
	for _, d := range sorted {
		sum += d
	}

	return &types.DurationMetrics{
		Min: sorted[0],
		Max: sorted[len(sorted)-1],
		Avg: time.Duration(int64(sum) / int64(len(sorted))),
		P50: c.percentile(sorted, 50),
		P90: c.percentile(sorted, 90),
		P95: c.percentile(sorted, 95),
		P99: c.percentile(sorted, 99),
	}
}

// percentile 计算已排序耗时数据的第 p 百分位数。
// Requirements: 9.5
func (c *MetricsCollector) percentile(sorted []time.Duration, p int) time.Duration {
	if len(sorted) == 0 {
		return 0
	}

	if p <= 0 {
		return sorted[0]
	}
	if p >= 100 {
		return sorted[len(sorted)-1]
	}

	// 使用最近秩方法
	index := (p * len(sorted)) / 100
	if index >= len(sorted) {
		index = len(sorted) - 1
	}

	return sorted[index]
}

// aggregateCustomMetrics 聚合自定义指标。
func (c *MetricsCollector) aggregateCustomMetrics(customMetrics map[string][]float64) map[string]float64 {
	result := make(map[string]float64)

	for k, values := range customMetrics {
		if len(values) == 0 {
			continue
		}

		// 计算平均值
		var sum float64
		for _, v := range values {
			sum += v
		}
		result[k] = sum / float64(len(values))
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
		Duration:      c.calculateDurationMetrics(data.durations),
		CustomMetrics: c.aggregateCustomMetrics(data.customMetrics),
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

// GetDurationSamples 返回指定步骤的原始耗时样本。
// 适用于基于属性的测试。
func (c *MetricsCollector) GetDurationSamples(stepID string) []time.Duration {
	c.mu.RLock()
	defer c.mu.RUnlock()

	data, exists := c.stepMetrics[stepID]
	if !exists {
		return nil
	}

	result := make([]time.Duration, len(data.durations))
	copy(result, data.durations)
	return result
}
