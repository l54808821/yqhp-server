// Package slave provides property-based tests for the metrics collector.
// Requirements: 9.5 - Percentile calculation correctness
// Property 10: For any set of duration samples, the calculated percentiles (p50, p90, p95, p99)
// should match the mathematically correct percentile values.
package slave

import (
	"math"
	"sort"
	"testing"
	"time"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
	"github.com/stretchr/testify/assert"

	"github.com/grafana/k6/workflow-engine/pkg/types"
)

// TestPercentileCalculationProperty tests Property 10: Percentile calculation correctness.
// calculated_percentile == correct_percentile
func TestPercentileCalculationProperty(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	// Property: P50 is calculated correctly
	properties.Property("P50 is calculated correctly", prop.ForAll(
		func(samples []int) bool {
			if len(samples) < 10 {
				return true // Skip small samples
			}

			collector := NewMetricsCollector()

			// Record samples
			for _, s := range samples {
				duration := time.Duration(s) * time.Millisecond
				collector.RecordStep("step-1", &types.StepResult{
					StepID:   "step-1",
					Status:   types.ResultStatusSuccess,
					Duration: duration,
				})
			}

			metrics := collector.GetMetrics()
			if metrics.StepMetrics["step-1"] == nil {
				return false
			}

			calculatedP50 := metrics.StepMetrics["step-1"].Duration.P50

			// Calculate expected P50
			sortedSamples := make([]int, len(samples))
			copy(sortedSamples, samples)
			sort.Ints(sortedSamples)
			expectedP50 := calculatePercentile(sortedSamples, 50)

			// Allow 5% tolerance due to different interpolation methods
			tolerance := float64(expectedP50) * 0.05
			if tolerance < 1 {
				tolerance = 1
			}

			diff := math.Abs(float64(calculatedP50.Milliseconds()) - float64(expectedP50))
			return diff <= tolerance
		},
		gen.SliceOfN(100, gen.IntRange(1, 1000)),
	))

	// Property: P90 is calculated correctly
	properties.Property("P90 is calculated correctly", prop.ForAll(
		func(samples []int) bool {
			if len(samples) < 10 {
				return true
			}

			collector := NewMetricsCollector()

			for _, s := range samples {
				duration := time.Duration(s) * time.Millisecond
				collector.RecordStep("step-1", &types.StepResult{
					StepID:   "step-1",
					Status:   types.ResultStatusSuccess,
					Duration: duration,
				})
			}

			metrics := collector.GetMetrics()
			if metrics.StepMetrics["step-1"] == nil {
				return false
			}

			calculatedP90 := metrics.StepMetrics["step-1"].Duration.P90

			sortedSamples := make([]int, len(samples))
			copy(sortedSamples, samples)
			sort.Ints(sortedSamples)
			expectedP90 := calculatePercentile(sortedSamples, 90)

			tolerance := float64(expectedP90) * 0.05
			if tolerance < 1 {
				tolerance = 1
			}

			diff := math.Abs(float64(calculatedP90.Milliseconds()) - float64(expectedP90))
			return diff <= tolerance
		},
		gen.SliceOfN(100, gen.IntRange(1, 1000)),
	))

	// Property: P95 is calculated correctly
	properties.Property("P95 is calculated correctly", prop.ForAll(
		func(samples []int) bool {
			if len(samples) < 20 {
				return true
			}

			collector := NewMetricsCollector()

			for _, s := range samples {
				duration := time.Duration(s) * time.Millisecond
				collector.RecordStep("step-1", &types.StepResult{
					StepID:   "step-1",
					Status:   types.ResultStatusSuccess,
					Duration: duration,
				})
			}

			metrics := collector.GetMetrics()
			if metrics.StepMetrics["step-1"] == nil {
				return false
			}

			calculatedP95 := metrics.StepMetrics["step-1"].Duration.P95

			sortedSamples := make([]int, len(samples))
			copy(sortedSamples, samples)
			sort.Ints(sortedSamples)
			expectedP95 := calculatePercentile(sortedSamples, 95)

			tolerance := float64(expectedP95) * 0.05
			if tolerance < 1 {
				tolerance = 1
			}

			diff := math.Abs(float64(calculatedP95.Milliseconds()) - float64(expectedP95))
			return diff <= tolerance
		},
		gen.SliceOfN(100, gen.IntRange(1, 1000)),
	))

	// Property: P99 is calculated correctly
	properties.Property("P99 is calculated correctly", prop.ForAll(
		func(samples []int) bool {
			if len(samples) < 100 {
				return true
			}

			collector := NewMetricsCollector()

			for _, s := range samples {
				duration := time.Duration(s) * time.Millisecond
				collector.RecordStep("step-1", &types.StepResult{
					StepID:   "step-1",
					Status:   types.ResultStatusSuccess,
					Duration: duration,
				})
			}

			metrics := collector.GetMetrics()
			if metrics.StepMetrics["step-1"] == nil {
				return false
			}

			calculatedP99 := metrics.StepMetrics["step-1"].Duration.P99

			sortedSamples := make([]int, len(samples))
			copy(sortedSamples, samples)
			sort.Ints(sortedSamples)
			expectedP99 := calculatePercentile(sortedSamples, 99)

			tolerance := float64(expectedP99) * 0.05
			if tolerance < 1 {
				tolerance = 1
			}

			diff := math.Abs(float64(calculatedP99.Milliseconds()) - float64(expectedP99))
			return diff <= tolerance
		},
		gen.SliceOfN(200, gen.IntRange(1, 1000)),
	))

	properties.TestingRun(t)
}

// TestPercentileOrderingProperty tests that percentiles are ordered correctly.
func TestPercentileOrderingProperty(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	// Property: P50 <= P90 <= P95 <= P99
	properties.Property("percentiles are ordered correctly", prop.ForAll(
		func(samples []int) bool {
			if len(samples) < 100 {
				return true
			}

			collector := NewMetricsCollector()

			for _, s := range samples {
				duration := time.Duration(s) * time.Millisecond
				collector.RecordStep("step-1", &types.StepResult{
					StepID:   "step-1",
					Status:   types.ResultStatusSuccess,
					Duration: duration,
				})
			}

			metrics := collector.GetMetrics()
			if metrics.StepMetrics["step-1"] == nil {
				return false
			}

			dm := metrics.StepMetrics["step-1"].Duration

			// P50 <= P90 <= P95 <= P99
			return dm.P50 <= dm.P90 && dm.P90 <= dm.P95 && dm.P95 <= dm.P99
		},
		gen.SliceOfN(200, gen.IntRange(1, 1000)),
	))

	// Property: Min <= P50 and P99 <= Max
	properties.Property("percentiles are within min/max bounds", prop.ForAll(
		func(samples []int) bool {
			if len(samples) < 100 {
				return true
			}

			collector := NewMetricsCollector()

			for _, s := range samples {
				duration := time.Duration(s) * time.Millisecond
				collector.RecordStep("step-1", &types.StepResult{
					StepID:   "step-1",
					Status:   types.ResultStatusSuccess,
					Duration: duration,
				})
			}

			metrics := collector.GetMetrics()
			if metrics.StepMetrics["step-1"] == nil {
				return false
			}

			dm := metrics.StepMetrics["step-1"].Duration

			return dm.Min <= dm.P50 && dm.P99 <= dm.Max
		},
		gen.SliceOfN(200, gen.IntRange(1, 1000)),
	))

	properties.TestingRun(t)
}

// TestMinMaxAvgProperty tests min, max, and average calculations.
func TestMinMaxAvgProperty(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	// Property: Min is the smallest value
	properties.Property("min is the smallest value", prop.ForAll(
		func(samples []int) bool {
			if len(samples) < 1 {
				return true
			}

			collector := NewMetricsCollector()

			minVal := samples[0]
			for _, s := range samples {
				if s < minVal {
					minVal = s
				}
				duration := time.Duration(s) * time.Millisecond
				collector.RecordStep("step-1", &types.StepResult{
					StepID:   "step-1",
					Status:   types.ResultStatusSuccess,
					Duration: duration,
				})
			}

			metrics := collector.GetMetrics()
			if metrics.StepMetrics["step-1"] == nil {
				return false
			}

			calculatedMin := metrics.StepMetrics["step-1"].Duration.Min
			return calculatedMin == time.Duration(minVal)*time.Millisecond
		},
		gen.SliceOfN(50, gen.IntRange(1, 1000)),
	))

	// Property: Max is the largest value
	properties.Property("max is the largest value", prop.ForAll(
		func(samples []int) bool {
			if len(samples) < 1 {
				return true
			}

			collector := NewMetricsCollector()

			maxVal := samples[0]
			for _, s := range samples {
				if s > maxVal {
					maxVal = s
				}
				duration := time.Duration(s) * time.Millisecond
				collector.RecordStep("step-1", &types.StepResult{
					StepID:   "step-1",
					Status:   types.ResultStatusSuccess,
					Duration: duration,
				})
			}

			metrics := collector.GetMetrics()
			if metrics.StepMetrics["step-1"] == nil {
				return false
			}

			calculatedMax := metrics.StepMetrics["step-1"].Duration.Max
			return calculatedMax == time.Duration(maxVal)*time.Millisecond
		},
		gen.SliceOfN(50, gen.IntRange(1, 1000)),
	))

	// Property: Average is correct
	properties.Property("average is calculated correctly", prop.ForAll(
		func(samples []int) bool {
			if len(samples) < 1 {
				return true
			}

			collector := NewMetricsCollector()

			var sum int64
			for _, s := range samples {
				sum += int64(s)
				duration := time.Duration(s) * time.Millisecond
				collector.RecordStep("step-1", &types.StepResult{
					StepID:   "step-1",
					Status:   types.ResultStatusSuccess,
					Duration: duration,
				})
			}

			expectedAvg := sum / int64(len(samples))

			metrics := collector.GetMetrics()
			if metrics.StepMetrics["step-1"] == nil {
				return false
			}

			calculatedAvg := metrics.StepMetrics["step-1"].Duration.Avg.Milliseconds()

			// Allow small tolerance for rounding
			diff := math.Abs(float64(calculatedAvg) - float64(expectedAvg))
			return diff <= 1
		},
		gen.SliceOfN(50, gen.IntRange(1, 1000)),
	))

	properties.TestingRun(t)
}

// Helper functions

// calculatePercentile calculates the percentile value from sorted samples.
func calculatePercentile(sortedSamples []int, percentile int) int {
	if len(sortedSamples) == 0 {
		return 0
	}

	index := (percentile * len(sortedSamples)) / 100
	if index >= len(sortedSamples) {
		index = len(sortedSamples) - 1
	}
	if index < 0 {
		index = 0
	}

	return sortedSamples[index]
}

// BenchmarkPercentileCalculation benchmarks percentile calculation.
func BenchmarkPercentileCalculation(b *testing.B) {
	collector := NewMetricsCollector()

	// Pre-populate with samples
	for i := 0; i < 1000; i++ {
		collector.RecordStep("step-1", &types.StepResult{
			StepID:   "step-1",
			Status:   types.ResultStatusSuccess,
			Duration: time.Duration(i) * time.Millisecond,
		})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		collector.GetMetrics()
	}
}

// TestPercentileCalculationSpecificCases tests specific edge cases.
func TestPercentileCalculationSpecificCases(t *testing.T) {
	testCases := []struct {
		name    string
		samples []int
	}{
		{
			name:    "sequential values",
			samples: []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10},
		},
		{
			name:    "all same values",
			samples: []int{5, 5, 5, 5, 5, 5, 5, 5, 5, 5},
		},
		{
			name:    "two distinct values",
			samples: []int{1, 1, 1, 1, 1, 10, 10, 10, 10, 10},
		},
		{
			name:    "large range",
			samples: []int{1, 100, 200, 300, 400, 500, 600, 700, 800, 900, 1000},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			collector := NewMetricsCollector()

			for _, s := range tc.samples {
				collector.RecordStep("step-1", &types.StepResult{
					StepID:   "step-1",
					Status:   types.ResultStatusSuccess,
					Duration: time.Duration(s) * time.Millisecond,
				})
			}

			metrics := collector.GetMetrics()
			assert.NotNil(t, metrics.StepMetrics["step-1"])

			dm := metrics.StepMetrics["step-1"].Duration

			// Verify ordering
			assert.True(t, dm.Min <= dm.P50)
			assert.True(t, dm.P50 <= dm.P90)
			assert.True(t, dm.P90 <= dm.P95)
			assert.True(t, dm.P95 <= dm.P99)
			assert.True(t, dm.P99 <= dm.Max)
		})
	}
}
