package master

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grafana/k6/workflow-engine/pkg/types"
)

func setupSchedulerTest() (*WorkflowScheduler, *InMemorySlaveRegistry) {
	registry := NewInMemorySlaveRegistry()
	scheduler := NewWorkflowScheduler(registry)
	return scheduler, registry
}

func TestNewWorkflowScheduler(t *testing.T) {
	scheduler, _ := setupSchedulerTest()
	assert.NotNil(t, scheduler)
}

func TestScheduleBasic(t *testing.T) {
	scheduler, _ := setupSchedulerTest()
	ctx := context.Background()

	workflow := &types.Workflow{
		ID:   "test-workflow",
		Name: "Test Workflow",
	}

	slaves := []*types.SlaveInfo{
		{ID: "slave-1", Type: types.SlaveTypeWorker},
		{ID: "slave-2", Type: types.SlaveTypeWorker},
	}

	plan, err := scheduler.Schedule(ctx, workflow, slaves)
	require.NoError(t, err)
	assert.NotNil(t, plan)
	assert.Len(t, plan.Assignments, 2)

	// Check segments
	assert.Equal(t, 0.0, plan.Assignments[0].Segment.Start)
	assert.Equal(t, 0.5, plan.Assignments[0].Segment.End)
	assert.Equal(t, 0.5, plan.Assignments[1].Segment.Start)
	assert.Equal(t, 1.0, plan.Assignments[1].Segment.End)
}

func TestScheduleNilWorkflow(t *testing.T) {
	scheduler, _ := setupSchedulerTest()
	ctx := context.Background()

	slaves := []*types.SlaveInfo{{ID: "slave-1"}}

	_, err := scheduler.Schedule(ctx, nil, slaves)
	assert.Error(t, err)
}

func TestScheduleNoSlaves(t *testing.T) {
	scheduler, _ := setupSchedulerTest()
	ctx := context.Background()

	workflow := &types.Workflow{ID: "test"}

	_, err := scheduler.Schedule(ctx, workflow, []*types.SlaveInfo{})
	assert.Error(t, err)
}

func TestScheduleSegmentsCoverFullRange(t *testing.T) {
	scheduler, _ := setupSchedulerTest()
	ctx := context.Background()

	workflow := &types.Workflow{ID: "test"}

	// Test with various slave counts
	for numSlaves := 1; numSlaves <= 10; numSlaves++ {
		slaves := make([]*types.SlaveInfo, numSlaves)
		for i := 0; i < numSlaves; i++ {
			slaves[i] = &types.SlaveInfo{ID: string(rune('a' + i))}
		}

		plan, err := scheduler.Schedule(ctx, workflow, slaves)
		require.NoError(t, err)

		// Verify segments cover [0, 1]
		var totalCoverage float64
		for _, assignment := range plan.Assignments {
			totalCoverage += assignment.Segment.End - assignment.Segment.Start
		}
		assert.InDelta(t, 1.0, totalCoverage, 0.0001, "segments should cover full range for %d slaves", numSlaves)

		// Verify first segment starts at 0
		assert.Equal(t, 0.0, plan.Assignments[0].Segment.Start)

		// Verify last segment ends at 1
		assert.Equal(t, 1.0, plan.Assignments[len(plan.Assignments)-1].Segment.End)
	}
}

func TestReschedule(t *testing.T) {
	scheduler, _ := setupSchedulerTest()
	ctx := context.Background()

	plan := &types.ExecutionPlan{
		ExecutionID: "exec-1",
		Assignments: []*types.SlaveAssignment{
			{SlaveID: "slave-1", Segment: types.ExecutionSegment{Start: 0.0, End: 0.33}},
			{SlaveID: "slave-2", Segment: types.ExecutionSegment{Start: 0.33, End: 0.66}},
			{SlaveID: "slave-3", Segment: types.ExecutionSegment{Start: 0.66, End: 1.0}},
		},
	}

	// Reschedule after slave-2 fails
	newPlan, err := scheduler.Reschedule(ctx, "slave-2", plan)
	require.NoError(t, err)
	assert.Len(t, newPlan.Assignments, 2)

	// Verify remaining slaves
	slaveIDs := make([]string, len(newPlan.Assignments))
	for i, a := range newPlan.Assignments {
		slaveIDs[i] = a.SlaveID
	}
	assert.Contains(t, slaveIDs, "slave-1")
	assert.Contains(t, slaveIDs, "slave-3")
	assert.NotContains(t, slaveIDs, "slave-2")
}

func TestRescheduleNilPlan(t *testing.T) {
	scheduler, _ := setupSchedulerTest()
	ctx := context.Background()

	_, err := scheduler.Reschedule(ctx, "slave-1", nil)
	assert.Error(t, err)
}

func TestRescheduleNoRemainingSlaves(t *testing.T) {
	scheduler, _ := setupSchedulerTest()
	ctx := context.Background()

	plan := &types.ExecutionPlan{
		Assignments: []*types.SlaveAssignment{
			{SlaveID: "slave-1", Segment: types.ExecutionSegment{Start: 0.0, End: 1.0}},
		},
	}

	_, err := scheduler.Reschedule(ctx, "slave-1", plan)
	assert.Error(t, err)
}

func TestSelectSlavesManual(t *testing.T) {
	scheduler, registry := setupSchedulerTest()
	ctx := context.Background()

	// Register slaves
	registry.Register(ctx, &types.SlaveInfo{ID: "slave-1", Type: types.SlaveTypeWorker})
	registry.Register(ctx, &types.SlaveInfo{ID: "slave-2", Type: types.SlaveTypeWorker})
	registry.Register(ctx, &types.SlaveInfo{ID: "slave-3", Type: types.SlaveTypeWorker})

	selector := &types.SlaveSelector{
		Mode:     types.SelectionModeManual,
		SlaveIDs: []string{"slave-1", "slave-3"},
	}

	slaves, err := scheduler.SelectSlaves(ctx, selector)
	require.NoError(t, err)
	assert.Len(t, slaves, 2)
}

func TestSelectSlavesManualNotFound(t *testing.T) {
	scheduler, registry := setupSchedulerTest()
	ctx := context.Background()

	registry.Register(ctx, &types.SlaveInfo{ID: "slave-1", Type: types.SlaveTypeWorker})

	selector := &types.SlaveSelector{
		Mode:     types.SelectionModeManual,
		SlaveIDs: []string{"slave-1", "non-existent"},
	}

	_, err := scheduler.SelectSlaves(ctx, selector)
	assert.Error(t, err)
}

func TestSelectSlavesManualOffline(t *testing.T) {
	scheduler, registry := setupSchedulerTest()
	ctx := context.Background()

	registry.Register(ctx, &types.SlaveInfo{ID: "slave-1", Type: types.SlaveTypeWorker})
	registry.MarkOffline(ctx, "slave-1")

	selector := &types.SlaveSelector{
		Mode:     types.SelectionModeManual,
		SlaveIDs: []string{"slave-1"},
	}

	_, err := scheduler.SelectSlaves(ctx, selector)
	assert.Error(t, err)
}

func TestSelectSlavesByLabel(t *testing.T) {
	scheduler, registry := setupSchedulerTest()
	ctx := context.Background()

	registry.Register(ctx, &types.SlaveInfo{
		ID:     "slave-1",
		Type:   types.SlaveTypeWorker,
		Labels: map[string]string{"region": "us-east", "env": "prod"},
	})
	registry.Register(ctx, &types.SlaveInfo{
		ID:     "slave-2",
		Type:   types.SlaveTypeWorker,
		Labels: map[string]string{"region": "us-west", "env": "prod"},
	})
	registry.Register(ctx, &types.SlaveInfo{
		ID:     "slave-3",
		Type:   types.SlaveTypeWorker,
		Labels: map[string]string{"region": "us-east", "env": "dev"},
	})

	selector := &types.SlaveSelector{
		Mode:   types.SelectionModeLabel,
		Labels: map[string]string{"region": "us-east"},
	}

	slaves, err := scheduler.SelectSlaves(ctx, selector)
	require.NoError(t, err)
	assert.Len(t, slaves, 2)
}

func TestSelectSlavesByLabelNoMatch(t *testing.T) {
	scheduler, registry := setupSchedulerTest()
	ctx := context.Background()

	registry.Register(ctx, &types.SlaveInfo{
		ID:     "slave-1",
		Type:   types.SlaveTypeWorker,
		Labels: map[string]string{"region": "us-east"},
	})

	selector := &types.SlaveSelector{
		Mode:   types.SelectionModeLabel,
		Labels: map[string]string{"region": "eu-west"},
	}

	_, err := scheduler.SelectSlaves(ctx, selector)
	assert.Error(t, err)
}

func TestSelectSlavesByCapability(t *testing.T) {
	scheduler, registry := setupSchedulerTest()
	ctx := context.Background()

	registry.Register(ctx, &types.SlaveInfo{
		ID:           "slave-1",
		Type:         types.SlaveTypeWorker,
		Capabilities: []string{"http_executor", "script_executor"},
	})
	registry.Register(ctx, &types.SlaveInfo{
		ID:           "slave-2",
		Type:         types.SlaveTypeWorker,
		Capabilities: []string{"http_executor"},
	})
	registry.Register(ctx, &types.SlaveInfo{
		ID:           "slave-3",
		Type:         types.SlaveTypeWorker,
		Capabilities: []string{"grpc_executor"},
	})

	selector := &types.SlaveSelector{
		Mode:         types.SelectionModeCapability,
		Capabilities: []string{"http_executor"},
	}

	slaves, err := scheduler.SelectSlaves(ctx, selector)
	require.NoError(t, err)
	assert.Len(t, slaves, 2)
}

func TestSelectSlavesByCapabilityMultiple(t *testing.T) {
	scheduler, registry := setupSchedulerTest()
	ctx := context.Background()

	registry.Register(ctx, &types.SlaveInfo{
		ID:           "slave-1",
		Type:         types.SlaveTypeWorker,
		Capabilities: []string{"http_executor", "script_executor"},
	})
	registry.Register(ctx, &types.SlaveInfo{
		ID:           "slave-2",
		Type:         types.SlaveTypeWorker,
		Capabilities: []string{"http_executor"},
	})

	selector := &types.SlaveSelector{
		Mode:         types.SelectionModeCapability,
		Capabilities: []string{"http_executor", "script_executor"},
	}

	slaves, err := scheduler.SelectSlaves(ctx, selector)
	require.NoError(t, err)
	assert.Len(t, slaves, 1)
	assert.Equal(t, "slave-1", slaves[0].ID)
}

func TestSelectSlavesAuto(t *testing.T) {
	scheduler, registry := setupSchedulerTest()
	ctx := context.Background()

	// Register slaves with different loads
	registry.Register(ctx, &types.SlaveInfo{ID: "slave-1", Type: types.SlaveTypeWorker})
	registry.Register(ctx, &types.SlaveInfo{ID: "slave-2", Type: types.SlaveTypeWorker})
	registry.Register(ctx, &types.SlaveInfo{ID: "slave-3", Type: types.SlaveTypeWorker})

	registry.UpdateStatus(ctx, "slave-1", &types.SlaveStatus{State: types.SlaveStateOnline, Load: 80})
	registry.UpdateStatus(ctx, "slave-2", &types.SlaveStatus{State: types.SlaveStateOnline, Load: 20})
	registry.UpdateStatus(ctx, "slave-3", &types.SlaveStatus{State: types.SlaveStateOnline, Load: 50})

	selector := &types.SlaveSelector{
		Mode:      types.SelectionModeAuto,
		MaxSlaves: 2,
	}

	slaves, err := scheduler.SelectSlaves(ctx, selector)
	require.NoError(t, err)
	assert.Len(t, slaves, 2)

	// Should select lowest load slaves first
	assert.Equal(t, "slave-2", slaves[0].ID) // Load 20
	assert.Equal(t, "slave-3", slaves[1].ID) // Load 50
}

func TestSelectSlavesAutoMinSlaves(t *testing.T) {
	scheduler, registry := setupSchedulerTest()
	ctx := context.Background()

	registry.Register(ctx, &types.SlaveInfo{ID: "slave-1", Type: types.SlaveTypeWorker})

	selector := &types.SlaveSelector{
		Mode:      types.SelectionModeAuto,
		MinSlaves: 3,
	}

	_, err := scheduler.SelectSlaves(ctx, selector)
	assert.Error(t, err)
}

func TestSelectSlavesNilSelector(t *testing.T) {
	scheduler, registry := setupSchedulerTest()
	ctx := context.Background()

	registry.Register(ctx, &types.SlaveInfo{ID: "slave-1", Type: types.SlaveTypeWorker})

	slaves, err := scheduler.SelectSlaves(ctx, nil)
	require.NoError(t, err)
	assert.Len(t, slaves, 1)
}

func TestCalculateVUsPerSlave(t *testing.T) {
	scheduler, _ := setupSchedulerTest()

	tests := []struct {
		totalVUs  int
		numSlaves int
		expected  []int
	}{
		{10, 2, []int{5, 5}},
		{10, 3, []int{4, 3, 3}},
		{7, 3, []int{3, 2, 2}},
		{1, 3, []int{1, 0, 0}},
		{0, 3, []int{0, 0, 0}},
		{10, 0, []int{}},
	}

	for _, tt := range tests {
		result := scheduler.CalculateVUsPerSlave(tt.totalVUs, tt.numSlaves)
		assert.Equal(t, tt.expected, result, "totalVUs=%d, numSlaves=%d", tt.totalVUs, tt.numSlaves)

		// Verify sum equals total
		if tt.numSlaves > 0 {
			sum := 0
			for _, v := range result {
				sum += v
			}
			assert.Equal(t, tt.totalVUs, sum)
		}
	}
}

func TestCalculateIterationsPerSlave(t *testing.T) {
	scheduler, _ := setupSchedulerTest()

	tests := []struct {
		totalIters int64
		numSlaves  int
		expected   []int64
	}{
		{100, 2, []int64{50, 50}},
		{100, 3, []int64{34, 33, 33}},
		{7, 3, []int64{3, 2, 2}},
		{1, 3, []int64{1, 0, 0}},
		{0, 3, []int64{0, 0, 0}},
		{100, 0, []int64{}},
	}

	for _, tt := range tests {
		result := scheduler.CalculateIterationsPerSlave(tt.totalIters, tt.numSlaves)
		assert.Equal(t, tt.expected, result, "totalIters=%d, numSlaves=%d", tt.totalIters, tt.numSlaves)

		// Verify sum equals total
		if tt.numSlaves > 0 {
			var sum int64
			for _, v := range result {
				sum += v
			}
			assert.Equal(t, tt.totalIters, sum)
		}
	}
}
