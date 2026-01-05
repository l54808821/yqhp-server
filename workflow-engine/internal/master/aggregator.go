package master

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"yqhp/workflow-engine/pkg/types"
)

// DefaultMetricsAggregator implements the MetricsAggregator interface.
// Requirements: 5.4, 5.6, 6.6
type DefaultMetricsAggregator struct{}

// NewDefaultMetricsAggregator creates a new metrics aggregator.
func NewDefaultMetricsAggregator() *DefaultMetricsAggregator {
	return &DefaultMetricsAggregator{}
}

// Aggregate aggregates metrics from multiple slaves.
// Requirements: 5.4, 9.1, 9.2
func (a *DefaultMetricsAggregator) Aggregate(ctx context.Context, executionID string, slaveMetrics []*types.Metrics) (*types.AggregatedMetrics, error) {
	if len(slaveMetrics) == 0 {
		return &types.AggregatedMetrics{
			ExecutionID: executionID,
			StepMetrics: make(map[string]*types.StepMetrics),
		}, nil
	}

	aggregated := &types.AggregatedMetrics{
		ExecutionID: executionID,
		StepMetrics: make(map[string]*types.StepMetrics),
	}

	// Collect all step metrics by step ID
	stepMetricsMap := make(map[string][]*types.StepMetrics)

	for _, metrics := range slaveMetrics {
		if metrics == nil || metrics.StepMetrics == nil {
			continue
		}
		for stepID, stepMetrics := range metrics.StepMetrics {
			stepMetricsMap[stepID] = append(stepMetricsMap[stepID], stepMetrics)
		}
	}

	// Aggregate each step's metrics
	for stepID, metricsSlice := range stepMetricsMap {
		aggregated.StepMetrics[stepID] = a.aggregateStepMetrics(stepID, metricsSlice)
	}

	// Calculate totals
	for _, stepMetrics := range aggregated.StepMetrics {
		aggregated.TotalIterations += stepMetrics.Count
	}

	return aggregated, nil
}

// aggregateStepMetrics aggregates metrics for a single step from multiple sources.
func (a *DefaultMetricsAggregator) aggregateStepMetrics(stepID string, metricsSlice []*types.StepMetrics) *types.StepMetrics {
	if len(metricsSlice) == 0 {
		return &types.StepMetrics{StepID: stepID}
	}

	aggregated := &types.StepMetrics{
		StepID:        stepID,
		CustomMetrics: make(map[string]float64),
	}

	// Aggregate counts
	for _, m := range metricsSlice {
		aggregated.Count += m.Count
		aggregated.SuccessCount += m.SuccessCount
		aggregated.FailureCount += m.FailureCount
	}

	// Aggregate duration metrics
	aggregated.Duration = a.aggregateDurationMetrics(metricsSlice)

	// Aggregate custom metrics (sum)
	for _, m := range metricsSlice {
		if m.CustomMetrics == nil {
			continue
		}
		for key, value := range m.CustomMetrics {
			aggregated.CustomMetrics[key] += value
		}
	}

	return aggregated
}

// aggregateDurationMetrics aggregates duration metrics from multiple sources.
func (a *DefaultMetricsAggregator) aggregateDurationMetrics(metricsSlice []*types.StepMetrics) *types.DurationMetrics {
	if len(metricsSlice) == 0 {
		return nil
	}

	// Collect all duration values for percentile calculation
	var allDurations []time.Duration
	var totalDuration time.Duration
	var totalCount int64
	var minDuration, maxDuration time.Duration
	first := true

	for _, m := range metricsSlice {
		if m.Duration == nil {
			continue
		}

		// Track min/max
		if first || m.Duration.Min < minDuration {
			minDuration = m.Duration.Min
		}
		if first || m.Duration.Max > maxDuration {
			maxDuration = m.Duration.Max
		}
		first = false

		// Accumulate for weighted average
		totalDuration += m.Duration.Avg * time.Duration(m.Count)
		totalCount += m.Count

		// For percentiles, we need to estimate from the individual percentiles
		// This is an approximation since we don't have raw data
		allDurations = append(allDurations, m.Duration.P50, m.Duration.P90, m.Duration.P95, m.Duration.P99)
	}

	if totalCount == 0 {
		return nil
	}

	// Calculate weighted average
	avgDuration := totalDuration / time.Duration(totalCount)

	// Sort durations for percentile estimation
	sort.Slice(allDurations, func(i, j int) bool {
		return allDurations[i] < allDurations[j]
	})

	return &types.DurationMetrics{
		Min: minDuration,
		Max: maxDuration,
		Avg: avgDuration,
		P50: a.estimatePercentile(allDurations, 50),
		P90: a.estimatePercentile(allDurations, 90),
		P95: a.estimatePercentile(allDurations, 95),
		P99: a.estimatePercentile(allDurations, 99),
	}
}

// estimatePercentile estimates a percentile from a sorted slice.
func (a *DefaultMetricsAggregator) estimatePercentile(sorted []time.Duration, percentile int) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	index := (percentile * len(sorted)) / 100
	if index >= len(sorted) {
		index = len(sorted) - 1
	}
	return sorted[index]
}

// EvaluateThresholds evaluates thresholds against aggregated metrics.
// Requirements: 6.6
func (a *DefaultMetricsAggregator) EvaluateThresholds(ctx context.Context, metrics *types.AggregatedMetrics, thresholds []types.Threshold) ([]types.ThresholdResult, error) {
	if metrics == nil {
		return nil, fmt.Errorf("metrics cannot be nil")
	}

	results := make([]types.ThresholdResult, len(thresholds))

	for i, threshold := range thresholds {
		result := types.ThresholdResult{
			Metric:    threshold.Metric,
			Condition: threshold.Condition,
		}

		// Get the metric value
		value, err := a.getMetricValue(metrics, threshold.Metric)
		if err != nil {
			result.Passed = false
			results[i] = result
			continue
		}

		result.Value = value

		// Evaluate the condition
		passed, err := a.evaluateCondition(value, threshold.Condition)
		if err != nil {
			result.Passed = false
		} else {
			result.Passed = passed
		}

		results[i] = result
	}

	return results, nil
}

// getMetricValue extracts a metric value from aggregated metrics.
func (a *DefaultMetricsAggregator) getMetricValue(metrics *types.AggregatedMetrics, metricName string) (float64, error) {
	// Parse metric name (e.g., "http_req_duration", "step_1.duration.p95")
	parts := strings.Split(metricName, ".")

	// Handle global metrics
	switch metricName {
	case "total_iterations":
		return float64(metrics.TotalIterations), nil
	case "total_vus":
		return float64(metrics.TotalVUs), nil
	case "duration":
		return float64(metrics.Duration.Milliseconds()), nil
	}

	// Handle step-specific metrics
	if len(parts) >= 2 {
		stepID := parts[0]
		stepMetrics, ok := metrics.StepMetrics[stepID]
		if !ok {
			return 0, fmt.Errorf("step not found: %s", stepID)
		}

		return a.getStepMetricValue(stepMetrics, parts[1:])
	}

	// Handle aggregated step metrics (e.g., "http_req_duration" -> aggregate all HTTP steps)
	return a.getAggregatedStepMetricValue(metrics, metricName)
}

// getStepMetricValue extracts a metric value from step metrics.
func (a *DefaultMetricsAggregator) getStepMetricValue(stepMetrics *types.StepMetrics, parts []string) (float64, error) {
	if len(parts) == 0 {
		return 0, fmt.Errorf("invalid metric path")
	}

	switch parts[0] {
	case "count":
		return float64(stepMetrics.Count), nil
	case "success_count":
		return float64(stepMetrics.SuccessCount), nil
	case "failure_count":
		return float64(stepMetrics.FailureCount), nil
	case "success_rate":
		if stepMetrics.Count == 0 {
			return 0, nil
		}
		return float64(stepMetrics.SuccessCount) / float64(stepMetrics.Count), nil
	case "failure_rate":
		if stepMetrics.Count == 0 {
			return 0, nil
		}
		return float64(stepMetrics.FailureCount) / float64(stepMetrics.Count), nil
	case "duration":
		if len(parts) < 2 || stepMetrics.Duration == nil {
			return 0, fmt.Errorf("invalid duration metric path")
		}
		return a.getDurationMetricValue(stepMetrics.Duration, parts[1])
	}

	// Check custom metrics
	if value, ok := stepMetrics.CustomMetrics[parts[0]]; ok {
		return value, nil
	}

	return 0, fmt.Errorf("unknown metric: %s", parts[0])
}

// getDurationMetricValue extracts a duration metric value.
func (a *DefaultMetricsAggregator) getDurationMetricValue(duration *types.DurationMetrics, metric string) (float64, error) {
	switch metric {
	case "min":
		return float64(duration.Min.Milliseconds()), nil
	case "max":
		return float64(duration.Max.Milliseconds()), nil
	case "avg":
		return float64(duration.Avg.Milliseconds()), nil
	case "p50":
		return float64(duration.P50.Milliseconds()), nil
	case "p90":
		return float64(duration.P90.Milliseconds()), nil
	case "p95":
		return float64(duration.P95.Milliseconds()), nil
	case "p99":
		return float64(duration.P99.Milliseconds()), nil
	default:
		return 0, fmt.Errorf("unknown duration metric: %s", metric)
	}
}

// getAggregatedStepMetricValue gets an aggregated metric value across all steps.
func (a *DefaultMetricsAggregator) getAggregatedStepMetricValue(metrics *types.AggregatedMetrics, metricName string) (float64, error) {
	// Common metric patterns
	switch {
	case strings.HasSuffix(metricName, "_duration"):
		// Aggregate duration across all steps
		var totalDuration time.Duration
		var count int64
		for _, stepMetrics := range metrics.StepMetrics {
			if stepMetrics.Duration != nil {
				totalDuration += stepMetrics.Duration.Avg * time.Duration(stepMetrics.Count)
				count += stepMetrics.Count
			}
		}
		if count == 0 {
			return 0, nil
		}
		return float64((totalDuration / time.Duration(count)).Milliseconds()), nil

	case strings.HasSuffix(metricName, "_failed"):
		// Calculate failure rate
		var totalFailed, totalCount int64
		for _, stepMetrics := range metrics.StepMetrics {
			totalFailed += stepMetrics.FailureCount
			totalCount += stepMetrics.Count
		}
		if totalCount == 0 {
			return 0, nil
		}
		return float64(totalFailed) / float64(totalCount), nil
	}

	return 0, fmt.Errorf("unknown metric: %s", metricName)
}

// evaluateCondition evaluates a threshold condition.
func (a *DefaultMetricsAggregator) evaluateCondition(value float64, condition string) (bool, error) {
	// Parse condition (e.g., "p95 < 500", "rate < 0.01")
	condition = strings.TrimSpace(condition)

	// Extract operator and threshold value
	var op string
	var threshold float64

	for _, operator := range []string{"<=", ">=", "!=", "==", "<", ">"} {
		if strings.Contains(condition, operator) {
			parts := strings.SplitN(condition, operator, 2)
			if len(parts) == 2 {
				op = operator
				thresholdStr := strings.TrimSpace(parts[1])
				var err error
				threshold, err = strconv.ParseFloat(thresholdStr, 64)
				if err != nil {
					return false, fmt.Errorf("invalid threshold value: %s", thresholdStr)
				}
				break
			}
		}
	}

	if op == "" {
		return false, fmt.Errorf("invalid condition format: %s", condition)
	}

	// Evaluate
	switch op {
	case "<":
		return value < threshold, nil
	case "<=":
		return value <= threshold, nil
	case ">":
		return value > threshold, nil
	case ">=":
		return value >= threshold, nil
	case "==":
		return value == threshold, nil
	case "!=":
		return value != threshold, nil
	default:
		return false, fmt.Errorf("unknown operator: %s", op)
	}
}

// GenerateSummary generates a summary report.
// Requirements: 5.6
func (a *DefaultMetricsAggregator) GenerateSummary(ctx context.Context, metrics *types.AggregatedMetrics) (*ExecutionSummary, error) {
	if metrics == nil {
		return nil, fmt.Errorf("metrics cannot be nil")
	}

	summary := &ExecutionSummary{
		ExecutionID:     metrics.ExecutionID,
		TotalVUs:        metrics.TotalVUs,
		TotalIterations: metrics.TotalIterations,
		Duration:        metrics.Duration.String(),
	}

	// Calculate totals from step metrics
	var totalRequests, totalSuccess, totalFailure int64
	var totalDuration time.Duration
	var p95Durations []time.Duration
	var p99Durations []time.Duration

	for _, stepMetrics := range metrics.StepMetrics {
		totalRequests += stepMetrics.Count
		totalSuccess += stepMetrics.SuccessCount
		totalFailure += stepMetrics.FailureCount

		if stepMetrics.Duration != nil {
			totalDuration += stepMetrics.Duration.Avg * time.Duration(stepMetrics.Count)
			p95Durations = append(p95Durations, stepMetrics.Duration.P95)
			p99Durations = append(p99Durations, stepMetrics.Duration.P99)
		}
	}

	summary.TotalRequests = totalRequests

	if totalRequests > 0 {
		summary.SuccessRate = float64(totalSuccess) / float64(totalRequests)
		summary.ErrorRate = float64(totalFailure) / float64(totalRequests)
		summary.AvgDuration = (totalDuration / time.Duration(totalRequests)).String()
	}

	// Calculate max P95 and P99
	if len(p95Durations) > 0 {
		sort.Slice(p95Durations, func(i, j int) bool {
			return p95Durations[i] > p95Durations[j]
		})
		summary.P95Duration = p95Durations[0].String()
	}

	if len(p99Durations) > 0 {
		sort.Slice(p99Durations, func(i, j int) bool {
			return p99Durations[i] > p99Durations[j]
		})
		summary.P99Duration = p99Durations[0].String()
	}

	// Count threshold results
	for _, result := range metrics.Thresholds {
		if result.Passed {
			summary.ThresholdsPassed++
		} else {
			summary.ThresholdsFailed++
		}
	}

	return summary, nil
}
