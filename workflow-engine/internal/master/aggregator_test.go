package master

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"yqhp/workflow-engine/pkg/types"
)

func TestNewDefaultMetricsAggregator(t *testing.T) {
	aggregator := NewDefaultMetricsAggregator()
	assert.NotNil(t, aggregator)
}

func TestAggregateEmpty(t *testing.T) {
	aggregator := NewDefaultMetricsAggregator()
	ctx := context.Background()

	result, err := aggregator.Aggregate(ctx, "exec-1", []*types.Metrics{})
	require.NoError(t, err)
	assert.Equal(t, "exec-1", result.ExecutionID)
	assert.Empty(t, result.StepMetrics)
}

func TestAggregateSingleSource(t *testing.T) {
	aggregator := NewDefaultMetricsAggregator()
	ctx := context.Background()

	metrics := []*types.Metrics{
		{
			StepMetrics: map[string]*types.StepMetrics{
				"step-1": {
					StepID:       "step-1",
					Count:        100,
					SuccessCount: 95,
					FailureCount: 5,
					Duration: &types.DurationMetrics{
						Min: 10 * time.Millisecond,
						Max: 500 * time.Millisecond,
						Avg: 100 * time.Millisecond,
						P50: 80 * time.Millisecond,
						P90: 200 * time.Millisecond,
						P95: 300 * time.Millisecond,
						P99: 450 * time.Millisecond,
					},
				},
			},
		},
	}

	result, err := aggregator.Aggregate(ctx, "exec-1", metrics)
	require.NoError(t, err)

	assert.Equal(t, int64(100), result.TotalIterations)
	assert.Contains(t, result.StepMetrics, "step-1")

	stepMetrics := result.StepMetrics["step-1"]
	assert.Equal(t, int64(100), stepMetrics.Count)
	assert.Equal(t, int64(95), stepMetrics.SuccessCount)
	assert.Equal(t, int64(5), stepMetrics.FailureCount)
}

func TestAggregateMultipleSources(t *testing.T) {
	aggregator := NewDefaultMetricsAggregator()
	ctx := context.Background()

	metrics := []*types.Metrics{
		{
			StepMetrics: map[string]*types.StepMetrics{
				"step-1": {
					StepID:       "step-1",
					Count:        100,
					SuccessCount: 95,
					FailureCount: 5,
					Duration: &types.DurationMetrics{
						Min: 10 * time.Millisecond,
						Max: 500 * time.Millisecond,
						Avg: 100 * time.Millisecond,
						P50: 80 * time.Millisecond,
						P90: 200 * time.Millisecond,
						P95: 300 * time.Millisecond,
						P99: 450 * time.Millisecond,
					},
				},
			},
		},
		{
			StepMetrics: map[string]*types.StepMetrics{
				"step-1": {
					StepID:       "step-1",
					Count:        150,
					SuccessCount: 140,
					FailureCount: 10,
					Duration: &types.DurationMetrics{
						Min: 5 * time.Millisecond,
						Max: 600 * time.Millisecond,
						Avg: 120 * time.Millisecond,
						P50: 90 * time.Millisecond,
						P90: 250 * time.Millisecond,
						P95: 350 * time.Millisecond,
						P99: 500 * time.Millisecond,
					},
				},
			},
		},
	}

	result, err := aggregator.Aggregate(ctx, "exec-1", metrics)
	require.NoError(t, err)

	stepMetrics := result.StepMetrics["step-1"]
	assert.Equal(t, int64(250), stepMetrics.Count)
	assert.Equal(t, int64(235), stepMetrics.SuccessCount)
	assert.Equal(t, int64(15), stepMetrics.FailureCount)

	// Duration should be aggregated
	assert.NotNil(t, stepMetrics.Duration)
	assert.Equal(t, 5*time.Millisecond, stepMetrics.Duration.Min)
	assert.Equal(t, 600*time.Millisecond, stepMetrics.Duration.Max)
}

func TestAggregateMultipleSteps(t *testing.T) {
	aggregator := NewDefaultMetricsAggregator()
	ctx := context.Background()

	metrics := []*types.Metrics{
		{
			StepMetrics: map[string]*types.StepMetrics{
				"step-1": {StepID: "step-1", Count: 100, SuccessCount: 100},
				"step-2": {StepID: "step-2", Count: 50, SuccessCount: 45, FailureCount: 5},
			},
		},
	}

	result, err := aggregator.Aggregate(ctx, "exec-1", metrics)
	require.NoError(t, err)

	assert.Len(t, result.StepMetrics, 2)
	assert.Contains(t, result.StepMetrics, "step-1")
	assert.Contains(t, result.StepMetrics, "step-2")
	assert.Equal(t, int64(150), result.TotalIterations)
}

func TestAggregateCustomMetrics(t *testing.T) {
	aggregator := NewDefaultMetricsAggregator()
	ctx := context.Background()

	metrics := []*types.Metrics{
		{
			StepMetrics: map[string]*types.StepMetrics{
				"step-1": {
					StepID:        "step-1",
					Count:         100,
					CustomMetrics: map[string]float64{"bytes_sent": 1000, "bytes_received": 5000},
				},
			},
		},
		{
			StepMetrics: map[string]*types.StepMetrics{
				"step-1": {
					StepID:        "step-1",
					Count:         100,
					CustomMetrics: map[string]float64{"bytes_sent": 2000, "bytes_received": 8000},
				},
			},
		},
	}

	result, err := aggregator.Aggregate(ctx, "exec-1", metrics)
	require.NoError(t, err)

	stepMetrics := result.StepMetrics["step-1"]
	assert.Equal(t, float64(3000), stepMetrics.CustomMetrics["bytes_sent"])
	assert.Equal(t, float64(13000), stepMetrics.CustomMetrics["bytes_received"])
}

func TestEvaluateThresholds(t *testing.T) {
	aggregator := NewDefaultMetricsAggregator()
	ctx := context.Background()

	metrics := &types.AggregatedMetrics{
		ExecutionID:     "exec-1",
		TotalIterations: 1000,
		TotalVUs:        10,
		StepMetrics: map[string]*types.StepMetrics{
			"step-1": {
				StepID:       "step-1",
				Count:        1000,
				SuccessCount: 950,
				FailureCount: 50,
				Duration: &types.DurationMetrics{
					Avg: 100 * time.Millisecond,
					P95: 300 * time.Millisecond,
					P99: 450 * time.Millisecond,
				},
			},
		},
	}

	thresholds := []types.Threshold{
		{Metric: "total_iterations", Condition: "> 500"},
		{Metric: "step-1.count", Condition: ">= 1000"},
		{Metric: "step-1.failure_rate", Condition: "< 0.1"},
		{Metric: "step-1.duration.p95", Condition: "< 500"},
	}

	results, err := aggregator.EvaluateThresholds(ctx, metrics, thresholds)
	require.NoError(t, err)
	assert.Len(t, results, 4)

	// All should pass
	for _, result := range results {
		assert.True(t, result.Passed, "threshold %s should pass", result.Metric)
	}
}

func TestEvaluateThresholdsFailure(t *testing.T) {
	aggregator := NewDefaultMetricsAggregator()
	ctx := context.Background()

	metrics := &types.AggregatedMetrics{
		ExecutionID:     "exec-1",
		TotalIterations: 100,
		StepMetrics: map[string]*types.StepMetrics{
			"step-1": {
				StepID:       "step-1",
				Count:        100,
				SuccessCount: 80,
				FailureCount: 20,
			},
		},
	}

	thresholds := []types.Threshold{
		{Metric: "step-1.failure_rate", Condition: "< 0.1"}, // Should fail (0.2 > 0.1)
	}

	results, err := aggregator.EvaluateThresholds(ctx, metrics, thresholds)
	require.NoError(t, err)
	assert.Len(t, results, 1)
	assert.False(t, results[0].Passed)
	assert.Equal(t, 0.2, results[0].Value)
}

func TestEvaluateThresholdsNilMetrics(t *testing.T) {
	aggregator := NewDefaultMetricsAggregator()
	ctx := context.Background()

	_, err := aggregator.EvaluateThresholds(ctx, nil, []types.Threshold{})
	assert.Error(t, err)
}

func TestEvaluateCondition(t *testing.T) {
	aggregator := NewDefaultMetricsAggregator()

	tests := []struct {
		value     float64
		condition string
		expected  bool
	}{
		{100, "< 200", true},
		{100, "< 50", false},
		{100, "<= 100", true},
		{100, "<= 99", false},
		{100, "> 50", true},
		{100, "> 200", false},
		{100, ">= 100", true},
		{100, ">= 101", false},
		{100, "== 100", true},
		{100, "== 99", false},
		{100, "!= 99", true},
		{100, "!= 100", false},
	}

	for _, tt := range tests {
		result, err := aggregator.evaluateCondition(tt.value, tt.condition)
		require.NoError(t, err, "condition: %s", tt.condition)
		assert.Equal(t, tt.expected, result, "value=%f, condition=%s", tt.value, tt.condition)
	}
}

func TestEvaluateConditionInvalid(t *testing.T) {
	aggregator := NewDefaultMetricsAggregator()

	_, err := aggregator.evaluateCondition(100, "invalid")
	assert.Error(t, err)

	_, err = aggregator.evaluateCondition(100, "< abc")
	assert.Error(t, err)
}

func TestGenerateSummary(t *testing.T) {
	aggregator := NewDefaultMetricsAggregator()
	ctx := context.Background()

	metrics := &types.AggregatedMetrics{
		ExecutionID:     "exec-1",
		TotalVUs:        10,
		TotalIterations: 1000,
		Duration:        5 * time.Minute,
		StepMetrics: map[string]*types.StepMetrics{
			"step-1": {
				StepID:       "step-1",
				Count:        500,
				SuccessCount: 480,
				FailureCount: 20,
				Duration: &types.DurationMetrics{
					Avg: 100 * time.Millisecond,
					P95: 300 * time.Millisecond,
					P99: 450 * time.Millisecond,
				},
			},
			"step-2": {
				StepID:       "step-2",
				Count:        500,
				SuccessCount: 490,
				FailureCount: 10,
				Duration: &types.DurationMetrics{
					Avg: 50 * time.Millisecond,
					P95: 150 * time.Millisecond,
					P99: 200 * time.Millisecond,
				},
			},
		},
		Thresholds: []types.ThresholdResult{
			{Passed: true},
			{Passed: true},
			{Passed: false},
		},
	}

	summary, err := aggregator.GenerateSummary(ctx, metrics)
	require.NoError(t, err)

	assert.Equal(t, "exec-1", summary.ExecutionID)
	assert.Equal(t, 10, summary.TotalVUs)
	assert.Equal(t, int64(1000), summary.TotalIterations)
	assert.Equal(t, int64(1000), summary.TotalRequests)
	assert.InDelta(t, 0.97, summary.SuccessRate, 0.001)
	assert.InDelta(t, 0.03, summary.ErrorRate, 0.001)
	assert.Equal(t, 2, summary.ThresholdsPassed)
	assert.Equal(t, 1, summary.ThresholdsFailed)
}

func TestGenerateSummaryNilMetrics(t *testing.T) {
	aggregator := NewDefaultMetricsAggregator()
	ctx := context.Background()

	_, err := aggregator.GenerateSummary(ctx, nil)
	assert.Error(t, err)
}

func TestGenerateSummaryEmpty(t *testing.T) {
	aggregator := NewDefaultMetricsAggregator()
	ctx := context.Background()

	metrics := &types.AggregatedMetrics{
		ExecutionID: "exec-1",
		StepMetrics: map[string]*types.StepMetrics{},
	}

	summary, err := aggregator.GenerateSummary(ctx, metrics)
	require.NoError(t, err)

	assert.Equal(t, "exec-1", summary.ExecutionID)
	assert.Equal(t, int64(0), summary.TotalRequests)
	assert.Equal(t, float64(0), summary.SuccessRate)
}

func TestGetMetricValue(t *testing.T) {
	aggregator := NewDefaultMetricsAggregator()

	metrics := &types.AggregatedMetrics{
		ExecutionID:     "exec-1",
		TotalIterations: 1000,
		TotalVUs:        10,
		Duration:        5 * time.Minute,
		StepMetrics: map[string]*types.StepMetrics{
			"step-1": {
				StepID:       "step-1",
				Count:        500,
				SuccessCount: 480,
				FailureCount: 20,
				Duration: &types.DurationMetrics{
					Min: 10 * time.Millisecond,
					Max: 500 * time.Millisecond,
					Avg: 100 * time.Millisecond,
					P95: 300 * time.Millisecond,
				},
			},
		},
	}

	tests := []struct {
		metricName string
		expected   float64
	}{
		{"total_iterations", 1000},
		{"total_vus", 10},
		{"step-1.count", 500},
		{"step-1.success_count", 480},
		{"step-1.failure_count", 20},
		{"step-1.success_rate", 0.96},
		{"step-1.failure_rate", 0.04},
		{"step-1.duration.min", 10},
		{"step-1.duration.max", 500},
		{"step-1.duration.avg", 100},
		{"step-1.duration.p95", 300},
	}

	for _, tt := range tests {
		value, err := aggregator.getMetricValue(metrics, tt.metricName)
		require.NoError(t, err, "metric: %s", tt.metricName)
		assert.InDelta(t, tt.expected, value, 0.001, "metric: %s", tt.metricName)
	}
}

func TestGetMetricValueNotFound(t *testing.T) {
	aggregator := NewDefaultMetricsAggregator()

	metrics := &types.AggregatedMetrics{
		ExecutionID: "exec-1",
		StepMetrics: map[string]*types.StepMetrics{},
	}

	_, err := aggregator.getMetricValue(metrics, "non-existent.count")
	assert.Error(t, err)
}
