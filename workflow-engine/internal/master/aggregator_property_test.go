// Package master provides property-based tests for the metrics aggregator.
// Requirements: 5.4, 9.1, 9.2 - Metrics aggregation correctness
// Property 8: For any set of metrics collected from multiple slaves, the aggregated metrics
// should equal the mathematical sum/average of individual metrics (as appropriate for each metric type).
package master

import (
	"context"
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
	"github.com/stretchr/testify/assert"

	"github.com/grafana/k6/workflow-engine/pkg/types"
)

// TestMetricsAggregationProperty tests Property 8: Metrics aggregation correctness.
// aggregate(metrics) == sum/avg(individual_metrics)
func TestMetricsAggregationProperty(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	// Property: Count aggregation is sum of individual counts
	properties.Property("count aggregation is sum", prop.ForAll(
		func(counts []int64) bool {
			if len(counts) < 1 {
				return true
			}

			aggregator := NewDefaultMetricsAggregator()
			ctx := context.Background()

			slaveMetrics := make([]*types.Metrics, len(counts))
			var expectedTotal int64

			for i, count := range counts {
				if count < 0 {
					count = 0
				}
				expectedTotal += count
				slaveMetrics[i] = &types.Metrics{
					Timestamp: time.Now(),
					StepMetrics: map[string]*types.StepMetrics{
						"step-1": {
							StepID: "step-1",
							Count:  count,
						},
					},
				}
			}

			aggregated, err := aggregator.Aggregate(ctx, "exec-1", slaveMetrics)
			if err != nil {
				return false
			}

			if aggregated.StepMetrics["step-1"] == nil {
				return expectedTotal == 0
			}

			return aggregated.StepMetrics["step-1"].Count == expectedTotal
		},
		gen.SliceOfN(10, gen.Int64Range(0, 1000)),
	))

	// Property: Success count aggregation is sum
	properties.Property("success count aggregation is sum", prop.ForAll(
		func(successCounts []int64) bool {
			if len(successCounts) < 1 {
				return true
			}

			aggregator := NewDefaultMetricsAggregator()
			ctx := context.Background()

			slaveMetrics := make([]*types.Metrics, len(successCounts))
			var expectedTotal int64

			for i, count := range successCounts {
				if count < 0 {
					count = 0
				}
				expectedTotal += count
				slaveMetrics[i] = &types.Metrics{
					Timestamp: time.Now(),
					StepMetrics: map[string]*types.StepMetrics{
						"step-1": {
							StepID:       "step-1",
							Count:        count,
							SuccessCount: count,
						},
					},
				}
			}

			aggregated, err := aggregator.Aggregate(ctx, "exec-1", slaveMetrics)
			if err != nil {
				return false
			}

			if aggregated.StepMetrics["step-1"] == nil {
				return expectedTotal == 0
			}

			return aggregated.StepMetrics["step-1"].SuccessCount == expectedTotal
		},
		gen.SliceOfN(10, gen.Int64Range(0, 1000)),
	))

	// Property: Failure count aggregation is sum
	properties.Property("failure count aggregation is sum", prop.ForAll(
		func(failureCounts []int64) bool {
			if len(failureCounts) < 1 {
				return true
			}

			aggregator := NewDefaultMetricsAggregator()
			ctx := context.Background()

			slaveMetrics := make([]*types.Metrics, len(failureCounts))
			var expectedTotal int64

			for i, count := range failureCounts {
				if count < 0 {
					count = 0
				}
				expectedTotal += count
				slaveMetrics[i] = &types.Metrics{
					Timestamp: time.Now(),
					StepMetrics: map[string]*types.StepMetrics{
						"step-1": {
							StepID:       "step-1",
							Count:        count,
							FailureCount: count,
						},
					},
				}
			}

			aggregated, err := aggregator.Aggregate(ctx, "exec-1", slaveMetrics)
			if err != nil {
				return false
			}

			if aggregated.StepMetrics["step-1"] == nil {
				return expectedTotal == 0
			}

			return aggregated.StepMetrics["step-1"].FailureCount == expectedTotal
		},
		gen.SliceOfN(10, gen.Int64Range(0, 1000)),
	))

	properties.TestingRun(t)
}

// TestDurationAggregationProperty tests duration metrics aggregation.
func TestDurationAggregationProperty(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	// Property: Min duration is the minimum across all slaves
	properties.Property("min duration is global minimum", prop.ForAll(
		func(minDurations []int) bool {
			if len(minDurations) < 1 {
				return true
			}

			aggregator := NewDefaultMetricsAggregator()
			ctx := context.Background()

			slaveMetrics := make([]*types.Metrics, len(minDurations))
			globalMin := minDurations[0]

			for i, minDur := range minDurations {
				if minDur < 1 {
					minDur = 1
				}
				if minDur < globalMin {
					globalMin = minDur
				}
				slaveMetrics[i] = &types.Metrics{
					Timestamp: time.Now(),
					StepMetrics: map[string]*types.StepMetrics{
						"step-1": {
							StepID: "step-1",
							Count:  10,
							Duration: &types.DurationMetrics{
								Min: time.Duration(minDur) * time.Millisecond,
								Max: time.Duration(minDur+100) * time.Millisecond,
								Avg: time.Duration(minDur+50) * time.Millisecond,
							},
						},
					},
				}
			}

			aggregated, err := aggregator.Aggregate(ctx, "exec-1", slaveMetrics)
			if err != nil {
				return false
			}

			if aggregated.StepMetrics["step-1"] == nil || aggregated.StepMetrics["step-1"].Duration == nil {
				return false
			}

			expectedMin := time.Duration(globalMin) * time.Millisecond
			return aggregated.StepMetrics["step-1"].Duration.Min == expectedMin
		},
		gen.SliceOfN(10, gen.IntRange(1, 1000)),
	))

	// Property: Max duration is the maximum across all slaves
	properties.Property("max duration is global maximum", prop.ForAll(
		func(maxDurations []int) bool {
			if len(maxDurations) < 1 {
				return true
			}

			aggregator := NewDefaultMetricsAggregator()
			ctx := context.Background()

			slaveMetrics := make([]*types.Metrics, len(maxDurations))
			globalMax := maxDurations[0]

			for i, maxDur := range maxDurations {
				if maxDur < 1 {
					maxDur = 1
				}
				if maxDur > globalMax {
					globalMax = maxDur
				}
				slaveMetrics[i] = &types.Metrics{
					Timestamp: time.Now(),
					StepMetrics: map[string]*types.StepMetrics{
						"step-1": {
							StepID: "step-1",
							Count:  10,
							Duration: &types.DurationMetrics{
								Min: time.Duration(1) * time.Millisecond,
								Max: time.Duration(maxDur) * time.Millisecond,
								Avg: time.Duration(maxDur/2) * time.Millisecond,
							},
						},
					},
				}
			}

			aggregated, err := aggregator.Aggregate(ctx, "exec-1", slaveMetrics)
			if err != nil {
				return false
			}

			if aggregated.StepMetrics["step-1"] == nil || aggregated.StepMetrics["step-1"].Duration == nil {
				return false
			}

			expectedMax := time.Duration(globalMax) * time.Millisecond
			return aggregated.StepMetrics["step-1"].Duration.Max == expectedMax
		},
		gen.SliceOfN(10, gen.IntRange(1, 1000)),
	))

	properties.TestingRun(t)
}

// TestWeightedAverageProperty tests weighted average calculation.
func TestWeightedAverageProperty(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	// Property: Average is weighted by count
	properties.Property("average is weighted by count", prop.ForAll(
		func(data []struct {
			Count int64
			Avg   int
		}) bool {
			if len(data) < 1 {
				return true
			}

			aggregator := NewDefaultMetricsAggregator()
			ctx := context.Background()

			slaveMetrics := make([]*types.Metrics, len(data))
			var totalCount int64
			var weightedSum float64

			for i, d := range data {
				count := d.Count
				if count < 1 {
					count = 1
				}
				avg := d.Avg
				if avg < 1 {
					avg = 1
				}

				totalCount += count
				weightedSum += float64(count) * float64(avg)

				slaveMetrics[i] = &types.Metrics{
					Timestamp: time.Now(),
					StepMetrics: map[string]*types.StepMetrics{
						"step-1": {
							StepID: "step-1",
							Count:  count,
							Duration: &types.DurationMetrics{
								Min: time.Duration(1) * time.Millisecond,
								Max: time.Duration(avg*2) * time.Millisecond,
								Avg: time.Duration(avg) * time.Millisecond,
							},
						},
					},
				}
			}

			aggregated, err := aggregator.Aggregate(ctx, "exec-1", slaveMetrics)
			if err != nil {
				return false
			}

			if aggregated.StepMetrics["step-1"] == nil || aggregated.StepMetrics["step-1"].Duration == nil {
				return false
			}

			expectedAvg := weightedSum / float64(totalCount)
			actualAvg := float64(aggregated.StepMetrics["step-1"].Duration.Avg.Milliseconds())

			// Allow 1ms tolerance
			return math.Abs(actualAvg-expectedAvg) <= 1
		},
		gen.SliceOfN(10, gen.Struct(reflect.TypeOf(struct {
			Count int64
			Avg   int
		}{}), map[string]gopter.Gen{
			"Count": gen.Int64Range(1, 100),
			"Avg":   gen.IntRange(1, 1000),
		})),
	))

	properties.TestingRun(t)
}

// TestMultipleStepsAggregationProperty tests aggregation with multiple steps.
func TestMultipleStepsAggregationProperty(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	// Property: Each step is aggregated independently
	properties.Property("steps are aggregated independently", prop.ForAll(
		func(stepCounts map[string]int64) bool {
			if len(stepCounts) < 1 {
				return true
			}

			aggregator := NewDefaultMetricsAggregator()
			ctx := context.Background()

			// Create metrics from two slaves
			slave1Metrics := &types.Metrics{
				Timestamp:   time.Now(),
				StepMetrics: make(map[string]*types.StepMetrics),
			}
			slave2Metrics := &types.Metrics{
				Timestamp:   time.Now(),
				StepMetrics: make(map[string]*types.StepMetrics),
			}

			for stepID, count := range stepCounts {
				if count < 0 {
					count = 0
				}
				slave1Metrics.StepMetrics[stepID] = &types.StepMetrics{
					StepID: stepID,
					Count:  count,
				}
				slave2Metrics.StepMetrics[stepID] = &types.StepMetrics{
					StepID: stepID,
					Count:  count,
				}
			}

			aggregated, err := aggregator.Aggregate(ctx, "exec-1", []*types.Metrics{slave1Metrics, slave2Metrics})
			if err != nil {
				return false
			}

			// Each step should have double the count
			for stepID, count := range stepCounts {
				if count < 0 {
					count = 0
				}
				if aggregated.StepMetrics[stepID] == nil {
					if count > 0 {
						return false
					}
					continue
				}
				if aggregated.StepMetrics[stepID].Count != count*2 {
					return false
				}
			}

			return true
		},
		gen.MapOf(
			gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 && len(s) <= 10 }),
			gen.Int64Range(0, 100),
		),
	))

	properties.TestingRun(t)
}

// BenchmarkMetricsAggregation benchmarks metrics aggregation.
func BenchmarkMetricsAggregation(b *testing.B) {
	aggregator := NewDefaultMetricsAggregator()
	ctx := context.Background()

	slaveMetrics := make([]*types.Metrics, 10)
	for i := 0; i < 10; i++ {
		slaveMetrics[i] = &types.Metrics{
			Timestamp: time.Now(),
			StepMetrics: map[string]*types.StepMetrics{
				"step-1": {
					StepID:       "step-1",
					Count:        100,
					SuccessCount: 95,
					FailureCount: 5,
					Duration: &types.DurationMetrics{
						Min: 10 * time.Millisecond,
						Max: 100 * time.Millisecond,
						Avg: 50 * time.Millisecond,
					},
				},
			},
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		aggregator.Aggregate(ctx, "exec-1", slaveMetrics)
	}
}

// TestMetricsAggregationSpecificCases tests specific edge cases.
func TestMetricsAggregationSpecificCases(t *testing.T) {
	testCases := []struct {
		name          string
		slaveCounts   []int64
		expectedTotal int64
	}{
		{
			name:          "single slave",
			slaveCounts:   []int64{100},
			expectedTotal: 100,
		},
		{
			name:          "two slaves equal",
			slaveCounts:   []int64{50, 50},
			expectedTotal: 100,
		},
		{
			name:          "three slaves unequal",
			slaveCounts:   []int64{30, 40, 30},
			expectedTotal: 100,
		},
		{
			name:          "with zero counts",
			slaveCounts:   []int64{0, 50, 50},
			expectedTotal: 100,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			aggregator := NewDefaultMetricsAggregator()
			ctx := context.Background()

			slaveMetrics := make([]*types.Metrics, len(tc.slaveCounts))
			for i, count := range tc.slaveCounts {
				slaveMetrics[i] = &types.Metrics{
					Timestamp: time.Now(),
					StepMetrics: map[string]*types.StepMetrics{
						"step-1": {
							StepID: "step-1",
							Count:  count,
						},
					},
				}
			}

			aggregated, err := aggregator.Aggregate(ctx, "exec-1", slaveMetrics)
			assert.NoError(t, err)
			assert.Equal(t, tc.expectedTotal, aggregated.StepMetrics["step-1"].Count)
		})
	}
}
