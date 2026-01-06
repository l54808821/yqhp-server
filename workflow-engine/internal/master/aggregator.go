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

// DefaultMetricsAggregator 实现了 MetricsAggregator 接口。
// Requirements: 5.4, 5.6, 6.6
type DefaultMetricsAggregator struct{}

// NewDefaultMetricsAggregator 创建一个新的指标聚合器。
func NewDefaultMetricsAggregator() *DefaultMetricsAggregator {
	return &DefaultMetricsAggregator{}
}

// Aggregate 聚合来自多个 Slave 的指标。
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

	// 按步骤 ID 收集所有步骤指标
	stepMetricsMap := make(map[string][]*types.StepMetrics)

	for _, metrics := range slaveMetrics {
		if metrics == nil || metrics.StepMetrics == nil {
			continue
		}
		for stepID, stepMetrics := range metrics.StepMetrics {
			stepMetricsMap[stepID] = append(stepMetricsMap[stepID], stepMetrics)
		}
	}

	// 聚合每个步骤的指标
	for stepID, metricsSlice := range stepMetricsMap {
		aggregated.StepMetrics[stepID] = a.aggregateStepMetrics(stepID, metricsSlice)
	}

	// 计算总数
	for _, stepMetrics := range aggregated.StepMetrics {
		aggregated.TotalIterations += stepMetrics.Count
	}

	return aggregated, nil
}

// aggregateStepMetrics 聚合来自多个来源的单个步骤的指标。
func (a *DefaultMetricsAggregator) aggregateStepMetrics(stepID string, metricsSlice []*types.StepMetrics) *types.StepMetrics {
	if len(metricsSlice) == 0 {
		return &types.StepMetrics{StepID: stepID}
	}

	aggregated := &types.StepMetrics{
		StepID:        stepID,
		CustomMetrics: make(map[string]float64),
	}

	// 聚合计数
	for _, m := range metricsSlice {
		aggregated.Count += m.Count
		aggregated.SuccessCount += m.SuccessCount
		aggregated.FailureCount += m.FailureCount
	}

	// 聚合持续时间指标
	aggregated.Duration = a.aggregateDurationMetrics(metricsSlice)

	// 聚合自定义指标（求和）
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

// aggregateDurationMetrics 聚合来自多个来源的持续时间指标。
func (a *DefaultMetricsAggregator) aggregateDurationMetrics(metricsSlice []*types.StepMetrics) *types.DurationMetrics {
	if len(metricsSlice) == 0 {
		return nil
	}

	// 收集所有持续时间值用于百分位数计算
	var allDurations []time.Duration
	var totalDuration time.Duration
	var totalCount int64
	var minDuration, maxDuration time.Duration
	first := true

	for _, m := range metricsSlice {
		if m.Duration == nil {
			continue
		}

		// 跟踪最小/最大值
		if first || m.Duration.Min < minDuration {
			minDuration = m.Duration.Min
		}
		if first || m.Duration.Max > maxDuration {
			maxDuration = m.Duration.Max
		}
		first = false

		// 累加用于加权平均
		totalDuration += m.Duration.Avg * time.Duration(m.Count)
		totalCount += m.Count

		// 对于百分位数，我们需要从各个百分位数进行估算
		// 这是一个近似值，因为我们没有原始数据
		allDurations = append(allDurations, m.Duration.P50, m.Duration.P90, m.Duration.P95, m.Duration.P99)
	}

	if totalCount == 0 {
		return nil
	}

	// 计算加权平均
	avgDuration := totalDuration / time.Duration(totalCount)

	// 对持续时间排序用于百分位数估算
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

// estimatePercentile 从已排序的切片中估算百分位数。
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

// EvaluateThresholds 根据聚合指标评估阈值。
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

		// 获取指标值
		value, err := a.getMetricValue(metrics, threshold.Metric)
		if err != nil {
			result.Passed = false
			results[i] = result
			continue
		}

		result.Value = value

		// 评估条件
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

// getMetricValue 从聚合指标中提取指标值。
func (a *DefaultMetricsAggregator) getMetricValue(metrics *types.AggregatedMetrics, metricName string) (float64, error) {
	// 解析指标名称（例如 "http_req_duration"、"step_1.duration.p95"）
	parts := strings.Split(metricName, ".")

	// 处理全局指标
	switch metricName {
	case "total_iterations":
		return float64(metrics.TotalIterations), nil
	case "total_vus":
		return float64(metrics.TotalVUs), nil
	case "duration":
		return float64(metrics.Duration.Milliseconds()), nil
	}

	// 处理步骤特定指标
	if len(parts) >= 2 {
		stepID := parts[0]
		stepMetrics, ok := metrics.StepMetrics[stepID]
		if !ok {
			return 0, fmt.Errorf("step not found: %s", stepID)
		}

		return a.getStepMetricValue(stepMetrics, parts[1:])
	}

	// 处理聚合步骤指标（例如 "http_req_duration" -> 聚合所有 HTTP 步骤）
	return a.getAggregatedStepMetricValue(metrics, metricName)
}

// getStepMetricValue 从步骤指标中提取指标值。
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

	// 检查自定义指标
	if value, ok := stepMetrics.CustomMetrics[parts[0]]; ok {
		return value, nil
	}

	return 0, fmt.Errorf("unknown metric: %s", parts[0])
}

// getDurationMetricValue 提取持续时间指标值。
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

// getAggregatedStepMetricValue 获取所有步骤的聚合指标值。
func (a *DefaultMetricsAggregator) getAggregatedStepMetricValue(metrics *types.AggregatedMetrics, metricName string) (float64, error) {
	// 常见指标模式
	switch {
	case strings.HasSuffix(metricName, "_duration"):
		// 聚合所有步骤的持续时间
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
		// 计算失败率
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

// evaluateCondition 评估阈值条件。
func (a *DefaultMetricsAggregator) evaluateCondition(value float64, condition string) (bool, error) {
	// 解析条件（例如 "p95 < 500"、"rate < 0.01"）
	condition = strings.TrimSpace(condition)

	// 提取运算符和阈值
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

	// 评估
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

// GenerateSummary 生成摘要报告。
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

	// 从步骤指标计算总数
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

	// 计算最大 P95 和 P99
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

	// 统计阈值结果
	for _, result := range metrics.Thresholds {
		if result.Passed {
			summary.ThresholdsPassed++
		} else {
			summary.ThresholdsFailed++
		}
	}

	return summary, nil
}
