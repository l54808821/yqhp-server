package slave

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"yqhp/workflow-engine/pkg/types"
)

func TestMetricsCollector_RecordStep(t *testing.T) {
	collector := NewMetricsCollector()

	result := &types.StepResult{
		StepID:   "step-1",
		Status:   types.ResultStatusSuccess,
		Duration: 100 * time.Millisecond,
		Metrics:  map[string]float64{"custom": 1.5},
	}

	collector.RecordStep("step-1", "", result)

	metrics := collector.GetMetrics()
	assert.NotNil(t, metrics.StepMetrics["step-1"])
	assert.Equal(t, int64(1), metrics.StepMetrics["step-1"].Count)
	assert.Equal(t, int64(1), metrics.StepMetrics["step-1"].SuccessCount)
	assert.Equal(t, int64(0), metrics.StepMetrics["step-1"].FailureCount)
}

func TestMetricsCollector_RecordMultipleSteps(t *testing.T) {
	collector := NewMetricsCollector()

	// Record success
	collector.RecordStep("step-1", "", &types.StepResult{
		StepID:   "step-1",
		Status:   types.ResultStatusSuccess,
		Duration: 100 * time.Millisecond,
	})

	// Record failure
	collector.RecordStep("step-1", "", &types.StepResult{
		StepID:   "step-1",
		Status:   types.ResultStatusFailed,
		Duration: 200 * time.Millisecond,
	})

	// Record timeout
	collector.RecordStep("step-1", "", &types.StepResult{
		StepID:   "step-1",
		Status:   types.ResultStatusTimeout,
		Duration: 300 * time.Millisecond,
	})

	metrics := collector.GetMetrics()
	stepMetrics := metrics.StepMetrics["step-1"]

	assert.Equal(t, int64(3), stepMetrics.Count)
	assert.Equal(t, int64(1), stepMetrics.SuccessCount)
	assert.Equal(t, int64(2), stepMetrics.FailureCount)
}

func TestMetricsCollector_DurationMetrics(t *testing.T) {
	collector := NewMetricsCollector()

	durations := []time.Duration{
		10 * time.Millisecond,
		20 * time.Millisecond,
		30 * time.Millisecond,
		40 * time.Millisecond,
		50 * time.Millisecond,
	}

	for _, d := range durations {
		collector.RecordStep("step-1", "", &types.StepResult{
			StepID:   "step-1",
			Status:   types.ResultStatusSuccess,
			Duration: d,
		})
	}

	metrics := collector.GetMetrics()
	durationMetrics := metrics.StepMetrics["step-1"].Duration

	assert.Equal(t, 10*time.Millisecond, durationMetrics.Min)
	assert.Equal(t, 50*time.Millisecond, durationMetrics.Max)
	assert.Equal(t, 30*time.Millisecond, durationMetrics.Avg)
}

func TestMetricsCollector_Percentiles(t *testing.T) {
	collector := NewMetricsCollector()

	// Record 100 samples
	for i := 1; i <= 100; i++ {
		collector.RecordStep("step-1", "", &types.StepResult{
			StepID:   "step-1",
			Status:   types.ResultStatusSuccess,
			Duration: time.Duration(i) * time.Millisecond,
		})
	}

	metrics := collector.GetMetrics()
	durationMetrics := metrics.StepMetrics["step-1"].Duration

	// 使用直方图后，百分位数是近似值，允许更大的误差范围
	// P50 should be around 50ms (允许 ±10ms 误差)
	assert.True(t, durationMetrics.P50 >= 40*time.Millisecond && durationMetrics.P50 <= 60*time.Millisecond,
		"P50 should be around 50ms, got %v", durationMetrics.P50)

	// P90 should be around 90ms (允许 ±10ms 误差)
	assert.True(t, durationMetrics.P90 >= 80*time.Millisecond && durationMetrics.P90 <= 100*time.Millisecond,
		"P90 should be around 90ms, got %v", durationMetrics.P90)

	// P95 should be around 95ms (允许 ±10ms 误差)
	assert.True(t, durationMetrics.P95 >= 85*time.Millisecond && durationMetrics.P95 <= 105*time.Millisecond,
		"P95 should be around 95ms, got %v", durationMetrics.P95)

	// P99 should be around 99ms (允许 ±10ms 误差)
	assert.True(t, durationMetrics.P99 >= 89*time.Millisecond && durationMetrics.P99 <= 110*time.Millisecond,
		"P99 should be around 99ms, got %v", durationMetrics.P99)
}

func TestMetricsCollector_GetThroughput(t *testing.T) {
	collector := NewMetricsCollector()

	// Record some steps
	for i := 0; i < 10; i++ {
		collector.RecordStep("step-1", "", &types.StepResult{
			StepID:   "step-1",
			Status:   types.ResultStatusSuccess,
			Duration: 10 * time.Millisecond,
		})
	}

	throughput := collector.GetThroughput()
	assert.True(t, throughput > 0)
}

func TestMetricsCollector_GetErrorRate(t *testing.T) {
	collector := NewMetricsCollector()

	// Record 8 successes and 2 failures
	for i := 0; i < 8; i++ {
		collector.RecordStep("step-1", "", &types.StepResult{
			StepID:   "step-1",
			Status:   types.ResultStatusSuccess,
			Duration: 10 * time.Millisecond,
		})
	}

	for i := 0; i < 2; i++ {
		collector.RecordStep("step-1", "", &types.StepResult{
			StepID:   "step-1",
			Status:   types.ResultStatusFailed,
			Duration: 10 * time.Millisecond,
		})
	}

	errorRate := collector.GetErrorRate()
	assert.InDelta(t, 0.2, errorRate, 0.001)
}

func TestMetricsCollector_Reset(t *testing.T) {
	collector := NewMetricsCollector()

	collector.RecordStep("step-1", "", &types.StepResult{
		StepID:   "step-1",
		Status:   types.ResultStatusSuccess,
		Duration: 10 * time.Millisecond,
	})

	collector.Reset()

	metrics := collector.GetMetrics()
	assert.Empty(t, metrics.StepMetrics)
}

func TestMetricsCollector_GetStepMetrics(t *testing.T) {
	collector := NewMetricsCollector()

	collector.RecordStep("step-1", "", &types.StepResult{
		StepID:   "step-1",
		Status:   types.ResultStatusSuccess,
		Duration: 10 * time.Millisecond,
	})

	stepMetrics := collector.GetStepMetrics("step-1")
	assert.NotNil(t, stepMetrics)
	assert.Equal(t, "step-1", stepMetrics.StepID)

	// Non-existent step
	stepMetrics = collector.GetStepMetrics("non-existent")
	assert.Nil(t, stepMetrics)
}

func TestMetricsCollector_CustomMetrics(t *testing.T) {
	collector := NewMetricsCollector()

	collector.RecordStep("step-1", "", &types.StepResult{
		StepID:   "step-1",
		Status:   types.ResultStatusSuccess,
		Duration: 10 * time.Millisecond,
		Metrics:  map[string]float64{"response_size": 1000},
	})

	collector.RecordStep("step-1", "", &types.StepResult{
		StepID:   "step-1",
		Status:   types.ResultStatusSuccess,
		Duration: 10 * time.Millisecond,
		Metrics:  map[string]float64{"response_size": 2000},
	})

	metrics := collector.GetMetrics()
	customMetrics := metrics.StepMetrics["step-1"].CustomMetrics

	// Average should be 1500
	assert.InDelta(t, 1500, customMetrics["response_size"], 0.001)
}

func TestMetricsCollector_RecordNil(t *testing.T) {
	collector := NewMetricsCollector()

	// Should not panic
	collector.RecordStep("step-1", "", nil)

	metrics := collector.GetMetrics()
	assert.Empty(t, metrics.StepMetrics)
}

func TestMetricsCollector_EmptyDurations(t *testing.T) {
	collector := NewMetricsCollector()

	// Get metrics without recording anything
	metrics := collector.GetMetrics()
	assert.Empty(t, metrics.StepMetrics)
}

func TestMetricsCollector_GetDurationSamples(t *testing.T) {
	collector := NewMetricsCollector()

	durations := []time.Duration{
		10 * time.Millisecond,
		20 * time.Millisecond,
		30 * time.Millisecond,
	}

	for _, d := range durations {
		collector.RecordStep("step-1", "", &types.StepResult{
			StepID:   "step-1",
			Status:   types.ResultStatusSuccess,
			Duration: d,
		})
	}

	samples := collector.GetDurationSamples("step-1")
	// 现在返回 8 个百分位数样本：min, p25, p50, p75, p90, p95, p99, max
	assert.Len(t, samples, 8)
	// 第一个是 min
	assert.Equal(t, 10*time.Millisecond, samples[0])
	// 最后一个是 max
	assert.Equal(t, 30*time.Millisecond, samples[7])

	// Non-existent step
	samples = collector.GetDurationSamples("non-existent")
	assert.Nil(t, samples)
}

func TestMetricsCollector_GetMemoryUsage(t *testing.T) {
	collector := NewMetricsCollector()

	// 记录多个步骤
	for i := 0; i < 10; i++ {
		collector.RecordStep("step-1", "", &types.StepResult{
			StepID:   "step-1",
			Status:   types.ResultStatusSuccess,
			Duration: time.Duration(i) * time.Millisecond,
		})
	}

	memUsage := collector.GetMemoryUsage()
	// HDR Histogram 每个步骤约 20KB
	assert.True(t, memUsage > 0)
	assert.True(t, memUsage < 50*1024) // 应该小于 50KB
}

func TestMetricsCollector_HighVolume(t *testing.T) {
	collector := NewMetricsCollector()

	// 模拟高并发场景：100万次请求
	// 使用 HDR Histogram 后，内存应该保持稳定
	for i := 0; i < 1000000; i++ {
		collector.RecordStep("step-1", "", &types.StepResult{
			StepID:   "step-1",
			Status:   types.ResultStatusSuccess,
			Duration: time.Duration(i%1000) * time.Millisecond,
		})
	}

	metrics := collector.GetMetrics()
	assert.Equal(t, int64(1000000), metrics.StepMetrics["step-1"].Count)

	// HDR Histogram 内存使用约 20KB，应该小于 50KB
	memUsage := collector.GetMemoryUsage()
	assert.True(t, memUsage < 50*1024)
}
