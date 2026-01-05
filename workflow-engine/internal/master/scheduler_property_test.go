// Package master provides property-based tests for the scheduler.
// Requirements: 5.3 - Work segment distribution completeness
// Property 7: For any workflow execution distributed across N slaves, the union of all
// execution segments should equal the complete range [0, 1], with no overlap between segments.
package master

import (
	"context"
	"math"
	"sort"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
	"github.com/stretchr/testify/assert"

	"github.com/grafana/k6/workflow-engine/pkg/types"
)

// TestWorkSegmentDistributionProperty tests Property 7: Work segment distribution completeness.
// union(segments) == [0, 1] AND no_overlap(segments)
func TestWorkSegmentDistributionProperty(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	// Property: Segments cover the complete range [0, 1]
	properties.Property("segments cover complete range", prop.ForAll(
		func(slaveCount int) bool {
			if slaveCount < 1 {
				slaveCount = 1
			}
			if slaveCount > 20 {
				slaveCount = 20
			}

			scheduler, _ := setupSchedulerTest()
			ctx := context.Background()

			workflow := &types.Workflow{
				ID:   "test-workflow",
				Name: "Test Workflow",
			}

			slaves := make([]*types.SlaveInfo, slaveCount)
			for i := 0; i < slaveCount; i++ {
				slaves[i] = &types.SlaveInfo{
					ID:   string(rune('a' + i)),
					Type: types.SlaveTypeWorker,
				}
			}

			plan, err := scheduler.Schedule(ctx, workflow, slaves)
			if err != nil {
				return false
			}

			// Calculate total coverage
			var totalCoverage float64
			for _, assignment := range plan.Assignments {
				totalCoverage += assignment.Segment.End - assignment.Segment.Start
			}

			// Should cover exactly 1.0 (with small tolerance for floating point)
			return math.Abs(totalCoverage-1.0) < 0.0001
		},
		gen.IntRange(1, 20),
	))

	// Property: No overlap between segments
	properties.Property("no overlap between segments", prop.ForAll(
		func(slaveCount int) bool {
			if slaveCount < 2 {
				return true // No overlap possible with single slave
			}
			if slaveCount > 20 {
				slaveCount = 20
			}

			scheduler, _ := setupSchedulerTest()
			ctx := context.Background()

			workflow := &types.Workflow{
				ID:   "test-workflow",
				Name: "Test Workflow",
			}

			slaves := make([]*types.SlaveInfo, slaveCount)
			for i := 0; i < slaveCount; i++ {
				slaves[i] = &types.SlaveInfo{
					ID:   string(rune('a' + i)),
					Type: types.SlaveTypeWorker,
				}
			}

			plan, err := scheduler.Schedule(ctx, workflow, slaves)
			if err != nil {
				return false
			}

			// Check for overlaps
			for i, a1 := range plan.Assignments {
				for j, a2 := range plan.Assignments {
					if i != j {
						if segmentsOverlap(a1.Segment, a2.Segment) {
							return false
						}
					}
				}
			}

			return true
		},
		gen.IntRange(2, 20),
	))

	// Property: First segment starts at 0
	properties.Property("first segment starts at 0", prop.ForAll(
		func(slaveCount int) bool {
			if slaveCount < 1 {
				slaveCount = 1
			}
			if slaveCount > 20 {
				slaveCount = 20
			}

			scheduler, _ := setupSchedulerTest()
			ctx := context.Background()

			workflow := &types.Workflow{
				ID:   "test-workflow",
				Name: "Test Workflow",
			}

			slaves := make([]*types.SlaveInfo, slaveCount)
			for i := 0; i < slaveCount; i++ {
				slaves[i] = &types.SlaveInfo{
					ID:   string(rune('a' + i)),
					Type: types.SlaveTypeWorker,
				}
			}

			plan, err := scheduler.Schedule(ctx, workflow, slaves)
			if err != nil {
				return false
			}

			// Find minimum start
			minStart := 1.0
			for _, assignment := range plan.Assignments {
				if assignment.Segment.Start < minStart {
					minStart = assignment.Segment.Start
				}
			}

			return math.Abs(minStart) < 0.0001
		},
		gen.IntRange(1, 20),
	))

	// Property: Last segment ends at 1
	properties.Property("last segment ends at 1", prop.ForAll(
		func(slaveCount int) bool {
			if slaveCount < 1 {
				slaveCount = 1
			}
			if slaveCount > 20 {
				slaveCount = 20
			}

			scheduler, _ := setupSchedulerTest()
			ctx := context.Background()

			workflow := &types.Workflow{
				ID:   "test-workflow",
				Name: "Test Workflow",
			}

			slaves := make([]*types.SlaveInfo, slaveCount)
			for i := 0; i < slaveCount; i++ {
				slaves[i] = &types.SlaveInfo{
					ID:   string(rune('a' + i)),
					Type: types.SlaveTypeWorker,
				}
			}

			plan, err := scheduler.Schedule(ctx, workflow, slaves)
			if err != nil {
				return false
			}

			// Find maximum end
			maxEnd := 0.0
			for _, assignment := range plan.Assignments {
				if assignment.Segment.End > maxEnd {
					maxEnd = assignment.Segment.End
				}
			}

			return math.Abs(maxEnd-1.0) < 0.0001
		},
		gen.IntRange(1, 20),
	))

	properties.TestingRun(t)
}

// TestSegmentContiguityProperty tests that segments are contiguous.
func TestSegmentContiguityProperty(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	// Property: Segments are contiguous (no gaps)
	properties.Property("segments are contiguous", prop.ForAll(
		func(slaveCount int) bool {
			if slaveCount < 2 {
				return true
			}
			if slaveCount > 20 {
				slaveCount = 20
			}

			scheduler, _ := setupSchedulerTest()
			ctx := context.Background()

			workflow := &types.Workflow{
				ID:   "test-workflow",
				Name: "Test Workflow",
			}

			slaves := make([]*types.SlaveInfo, slaveCount)
			for i := 0; i < slaveCount; i++ {
				slaves[i] = &types.SlaveInfo{
					ID:   string(rune('a' + i)),
					Type: types.SlaveTypeWorker,
				}
			}

			plan, err := scheduler.Schedule(ctx, workflow, slaves)
			if err != nil {
				return false
			}

			// Sort segments by start
			segments := make([]types.ExecutionSegment, len(plan.Assignments))
			for i, a := range plan.Assignments {
				segments[i] = a.Segment
			}
			sort.Slice(segments, func(i, j int) bool {
				return segments[i].Start < segments[j].Start
			})

			// Check contiguity
			for i := 0; i < len(segments)-1; i++ {
				if math.Abs(segments[i].End-segments[i+1].Start) > 0.0001 {
					return false
				}
			}

			return true
		},
		gen.IntRange(2, 20),
	))

	properties.TestingRun(t)
}

// TestEqualDistributionProperty tests that work is distributed equally.
func TestEqualDistributionProperty(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	// Property: Segments are approximately equal in size
	properties.Property("segments are approximately equal", prop.ForAll(
		func(slaveCount int) bool {
			if slaveCount < 2 {
				return true
			}
			if slaveCount > 20 {
				slaveCount = 20
			}

			scheduler, _ := setupSchedulerTest()
			ctx := context.Background()

			workflow := &types.Workflow{
				ID:   "test-workflow",
				Name: "Test Workflow",
			}

			slaves := make([]*types.SlaveInfo, slaveCount)
			for i := 0; i < slaveCount; i++ {
				slaves[i] = &types.SlaveInfo{
					ID:   string(rune('a' + i)),
					Type: types.SlaveTypeWorker,
				}
			}

			plan, err := scheduler.Schedule(ctx, workflow, slaves)
			if err != nil {
				return false
			}

			expectedSize := 1.0 / float64(slaveCount)

			for _, assignment := range plan.Assignments {
				size := assignment.Segment.End - assignment.Segment.Start
				// Allow 10% tolerance
				if math.Abs(size-expectedSize) > expectedSize*0.1 {
					return false
				}
			}

			return true
		},
		gen.IntRange(2, 20),
	))

	properties.TestingRun(t)
}

// Helper function to check segment overlap
func segmentsOverlap(a, b types.ExecutionSegment) bool {
	// Two segments overlap if one starts before the other ends
	// and the other starts before the first ends
	return a.Start < b.End && b.Start < a.End
}

// BenchmarkScheduling benchmarks the scheduling operation.
func BenchmarkScheduling(b *testing.B) {
	scheduler, _ := setupSchedulerTest()
	ctx := context.Background()

	workflow := &types.Workflow{
		ID:   "benchmark-workflow",
		Name: "Benchmark Workflow",
	}

	slaves := make([]*types.SlaveInfo, 10)
	for i := 0; i < 10; i++ {
		slaves[i] = &types.SlaveInfo{
			ID:   string(rune('a' + i)),
			Type: types.SlaveTypeWorker,
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		scheduler.Schedule(ctx, workflow, slaves)
	}
}

// TestWorkSegmentDistributionSpecificCases tests specific edge cases.
func TestWorkSegmentDistributionSpecificCases(t *testing.T) {
	testCases := []struct {
		name       string
		slaveCount int
	}{
		{name: "single slave", slaveCount: 1},
		{name: "two slaves", slaveCount: 2},
		{name: "three slaves", slaveCount: 3},
		{name: "five slaves", slaveCount: 5},
		{name: "ten slaves", slaveCount: 10},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			scheduler, _ := setupSchedulerTest()
			ctx := context.Background()

			workflow := &types.Workflow{
				ID:   "test-workflow",
				Name: "Test Workflow",
			}

			slaves := make([]*types.SlaveInfo, tc.slaveCount)
			for i := 0; i < tc.slaveCount; i++ {
				slaves[i] = &types.SlaveInfo{
					ID:   string(rune('a' + i)),
					Type: types.SlaveTypeWorker,
				}
			}

			plan, err := scheduler.Schedule(ctx, workflow, slaves)
			assert.NoError(t, err)
			assert.Len(t, plan.Assignments, tc.slaveCount)

			// Verify total coverage
			var totalCoverage float64
			for _, assignment := range plan.Assignments {
				totalCoverage += assignment.Segment.End - assignment.Segment.Start
			}
			assert.InDelta(t, 1.0, totalCoverage, 0.0001)

			// Verify first starts at 0
			assert.Equal(t, 0.0, plan.Assignments[0].Segment.Start)

			// Verify last ends at 1
			assert.Equal(t, 1.0, plan.Assignments[len(plan.Assignments)-1].Segment.End)
		})
	}
}
