package master

import (
	"context"
	"testing"
	"time"

	"yqhp/workflow-engine/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockRegistry implements SlaveRegistry for testing.
type mockRegistry struct {
	slaves map[string]*types.SlaveInfo
	status map[string]*types.SlaveStatus
}

func newMockRegistry() *mockRegistry {
	return &mockRegistry{
		slaves: make(map[string]*types.SlaveInfo),
		status: make(map[string]*types.SlaveStatus),
	}
}

func (r *mockRegistry) Register(ctx context.Context, slave *types.SlaveInfo) error {
	r.slaves[slave.ID] = slave
	r.status[slave.ID] = &types.SlaveStatus{
		State:    types.SlaveStateOnline,
		LastSeen: time.Now(),
	}
	return nil
}

func (r *mockRegistry) Unregister(ctx context.Context, slaveID string) error {
	delete(r.slaves, slaveID)
	delete(r.status, slaveID)
	return nil
}

func (r *mockRegistry) UpdateStatus(ctx context.Context, slaveID string, status *types.SlaveStatus) error {
	r.status[slaveID] = status
	return nil
}

func (r *mockRegistry) GetSlave(ctx context.Context, slaveID string) (*types.SlaveInfo, error) {
	return r.slaves[slaveID], nil
}

func (r *mockRegistry) GetSlaveStatus(ctx context.Context, slaveID string) (*types.SlaveStatus, error) {
	return r.status[slaveID], nil
}

func (r *mockRegistry) ListSlaves(ctx context.Context, filter *SlaveFilter) ([]*types.SlaveInfo, error) {
	result := make([]*types.SlaveInfo, 0, len(r.slaves))
	for _, slave := range r.slaves {
		result = append(result, slave)
	}
	return result, nil
}

func (r *mockRegistry) GetOnlineSlaves(ctx context.Context) ([]*types.SlaveInfo, error) {
	result := make([]*types.SlaveInfo, 0)
	for id, slave := range r.slaves {
		if status, ok := r.status[id]; ok && status.State == types.SlaveStateOnline {
			result = append(result, slave)
		}
	}
	return result, nil
}

func (r *mockRegistry) WatchSlaves(ctx context.Context) (<-chan *types.SlaveEvent, error) {
	ch := make(chan *types.SlaveEvent)
	return ch, nil
}

// mockScheduler implements Scheduler for testing.
type mockScheduler struct {
	registry *mockRegistry
}

func newMockScheduler(registry *mockRegistry) *mockScheduler {
	return &mockScheduler{registry: registry}
}

func (s *mockScheduler) Schedule(ctx context.Context, workflow *types.Workflow, slaves []*types.SlaveInfo) (*types.ExecutionPlan, error) {
	assignments := make([]*types.SlaveAssignment, len(slaves))
	segmentSize := 1.0 / float64(len(slaves))

	for i, slave := range slaves {
		assignments[i] = &types.SlaveAssignment{
			SlaveID:  slave.ID,
			Workflow: workflow,
			Segment: types.ExecutionSegment{
				Start: float64(i) * segmentSize,
				End:   float64(i+1) * segmentSize,
			},
		}
	}

	return &types.ExecutionPlan{
		Assignments: assignments,
	}, nil
}

func (s *mockScheduler) Reschedule(ctx context.Context, failedSlaveID string, plan *types.ExecutionPlan) (*types.ExecutionPlan, error) {
	return plan, nil
}

func (s *mockScheduler) SelectSlaves(ctx context.Context, selector *types.SlaveSelector) ([]*types.SlaveInfo, error) {
	return s.registry.GetOnlineSlaves(ctx)
}

// mockAggregator implements MetricsAggregator for testing.
type mockAggregator struct{}

func newMockAggregator() *mockAggregator {
	return &mockAggregator{}
}

func (a *mockAggregator) Aggregate(ctx context.Context, executionID string, slaveMetrics []*types.Metrics) (*types.AggregatedMetrics, error) {
	return &types.AggregatedMetrics{
		ExecutionID: executionID,
		StepMetrics: make(map[string]*types.StepMetrics),
	}, nil
}

func (a *mockAggregator) EvaluateThresholds(ctx context.Context, metrics *types.AggregatedMetrics, thresholds []types.Threshold) ([]types.ThresholdResult, error) {
	return []types.ThresholdResult{}, nil
}

func (a *mockAggregator) GenerateSummary(ctx context.Context, metrics *types.AggregatedMetrics) (*ExecutionSummary, error) {
	return &ExecutionSummary{
		ExecutionID: metrics.ExecutionID,
	}, nil
}

func TestNewWorkflowMaster(t *testing.T) {
	registry := newMockRegistry()
	scheduler := newMockScheduler(registry)
	aggregator := newMockAggregator()

	master := NewWorkflowMaster(nil, registry, scheduler, aggregator)

	assert.NotNil(t, master)
	assert.Equal(t, MasterStateStopped, master.GetState())
	assert.False(t, master.IsRunning())
}

func TestMasterStartStop(t *testing.T) {
	registry := newMockRegistry()
	scheduler := newMockScheduler(registry)
	aggregator := newMockAggregator()

	master := NewWorkflowMaster(DefaultConfig(), registry, scheduler, aggregator)

	ctx := context.Background()

	// Start master
	err := master.Start(ctx)
	require.NoError(t, err)
	assert.True(t, master.IsRunning())
	assert.Equal(t, MasterStateRunning, master.GetState())

	// Starting again should fail
	err = master.Start(ctx)
	assert.Error(t, err)

	// Stop master
	err = master.Stop(ctx)
	require.NoError(t, err)
	assert.False(t, master.IsRunning())
	assert.Equal(t, MasterStateStopped, master.GetState())
}

func TestSubmitWorkflow(t *testing.T) {
	registry := newMockRegistry()
	scheduler := newMockScheduler(registry)
	aggregator := newMockAggregator()

	config := DefaultConfig()
	config.StandaloneMode = true // Use standalone mode for testing

	master := NewWorkflowMaster(config, registry, scheduler, aggregator)

	ctx := context.Background()

	// Start master
	err := master.Start(ctx)
	require.NoError(t, err)
	defer master.Stop(ctx)

	// Submit workflow
	workflow := &types.Workflow{
		ID:   "test-workflow",
		Name: "Test Workflow",
		Steps: []types.Step{
			{
				ID:   "step1",
				Name: "Step 1",
				Type: "http",
			},
		},
	}

	executionID, err := master.SubmitWorkflow(ctx, workflow)
	require.NoError(t, err)
	assert.NotEmpty(t, executionID)

	// Check execution status
	status, err := master.GetExecutionStatus(ctx, executionID)
	require.NoError(t, err)
	assert.Equal(t, executionID, status.ID)
	assert.Equal(t, "test-workflow", status.WorkflowID)
}

func TestSubmitWorkflowNil(t *testing.T) {
	master := NewWorkflowMaster(DefaultConfig(), nil, nil, nil)

	ctx := context.Background()
	err := master.Start(ctx)
	require.NoError(t, err)
	defer master.Stop(ctx)

	_, err = master.SubmitWorkflow(ctx, nil)
	assert.Error(t, err)
}

func TestSubmitWorkflowNotStarted(t *testing.T) {
	master := NewWorkflowMaster(DefaultConfig(), nil, nil, nil)

	ctx := context.Background()
	workflow := &types.Workflow{ID: "test"}

	_, err := master.SubmitWorkflow(ctx, workflow)
	assert.Error(t, err)
}

func TestGetExecutionStatusNotFound(t *testing.T) {
	master := NewWorkflowMaster(DefaultConfig(), nil, nil, nil)

	ctx := context.Background()
	err := master.Start(ctx)
	require.NoError(t, err)
	defer master.Stop(ctx)

	_, err = master.GetExecutionStatus(ctx, "non-existent")
	assert.Error(t, err)
}

func TestStopExecution(t *testing.T) {
	registry := newMockRegistry()
	scheduler := newMockScheduler(registry)
	aggregator := newMockAggregator()

	config := DefaultConfig()
	config.StandaloneMode = true

	master := NewWorkflowMaster(config, registry, scheduler, aggregator)

	ctx := context.Background()
	err := master.Start(ctx)
	require.NoError(t, err)
	defer master.Stop(ctx)

	// Submit workflow
	workflow := &types.Workflow{
		ID:   "test-workflow",
		Name: "Test Workflow",
		Steps: []types.Step{
			{ID: "step1", Name: "Step 1", Type: "http"},
		},
	}

	executionID, err := master.SubmitWorkflow(ctx, workflow)
	require.NoError(t, err)

	// Give it a moment to start
	time.Sleep(10 * time.Millisecond)

	// Stop execution
	err = master.StopExecution(ctx, executionID)
	require.NoError(t, err)

	// Check status
	status, err := master.GetExecutionStatus(ctx, executionID)
	require.NoError(t, err)
	assert.Equal(t, types.ExecutionStatusAborted, status.Status)
}

func TestStopExecutionNotFound(t *testing.T) {
	master := NewWorkflowMaster(DefaultConfig(), nil, nil, nil)

	ctx := context.Background()
	err := master.Start(ctx)
	require.NoError(t, err)
	defer master.Stop(ctx)

	err = master.StopExecution(ctx, "non-existent")
	assert.Error(t, err)
}

func TestPauseResumeExecution(t *testing.T) {
	registry := newMockRegistry()
	scheduler := newMockScheduler(registry)
	aggregator := newMockAggregator()

	config := DefaultConfig()
	config.StandaloneMode = true

	master := NewWorkflowMaster(config, registry, scheduler, aggregator)

	ctx := context.Background()
	err := master.Start(ctx)
	require.NoError(t, err)
	defer master.Stop(ctx)

	// Submit workflow
	workflow := &types.Workflow{
		ID:   "test-workflow",
		Name: "Test Workflow",
		Steps: []types.Step{
			{ID: "step1", Name: "Step 1", Type: "http"},
		},
	}

	executionID, err := master.SubmitWorkflow(ctx, workflow)
	require.NoError(t, err)

	// Give it a moment to start
	time.Sleep(10 * time.Millisecond)

	// Pause execution
	err = master.PauseExecution(ctx, executionID)
	require.NoError(t, err)

	status, err := master.GetExecutionStatus(ctx, executionID)
	require.NoError(t, err)
	assert.Equal(t, types.ExecutionStatusPaused, status.Status)

	// Resume execution
	err = master.ResumeExecution(ctx, executionID)
	require.NoError(t, err)

	status, err = master.GetExecutionStatus(ctx, executionID)
	require.NoError(t, err)
	assert.Equal(t, types.ExecutionStatusRunning, status.Status)
}

func TestScaleExecution(t *testing.T) {
	registry := newMockRegistry()
	scheduler := newMockScheduler(registry)
	aggregator := newMockAggregator()

	config := DefaultConfig()
	config.StandaloneMode = true

	master := NewWorkflowMaster(config, registry, scheduler, aggregator)

	ctx := context.Background()
	err := master.Start(ctx)
	require.NoError(t, err)
	defer master.Stop(ctx)

	// Submit workflow
	workflow := &types.Workflow{
		ID:   "test-workflow",
		Name: "Test Workflow",
		Options: types.ExecutionOptions{
			VUs: 10,
		},
		Steps: []types.Step{
			{ID: "step1", Name: "Step 1", Type: "http"},
		},
	}

	executionID, err := master.SubmitWorkflow(ctx, workflow)
	require.NoError(t, err)

	// Give it a moment to start
	time.Sleep(10 * time.Millisecond)

	// Scale execution
	err = master.ScaleExecution(ctx, executionID, 20)
	require.NoError(t, err)
}

func TestScaleExecutionNegativeVUs(t *testing.T) {
	master := NewWorkflowMaster(DefaultConfig(), nil, nil, nil)

	ctx := context.Background()
	err := master.Start(ctx)
	require.NoError(t, err)
	defer master.Stop(ctx)

	err = master.ScaleExecution(ctx, "some-id", -1)
	assert.Error(t, err)
}

func TestGetMetrics(t *testing.T) {
	registry := newMockRegistry()
	scheduler := newMockScheduler(registry)
	aggregator := newMockAggregator()

	config := DefaultConfig()
	config.StandaloneMode = true

	master := NewWorkflowMaster(config, registry, scheduler, aggregator)

	ctx := context.Background()
	err := master.Start(ctx)
	require.NoError(t, err)
	defer master.Stop(ctx)

	// Submit workflow
	workflow := &types.Workflow{
		ID:   "test-workflow",
		Name: "Test Workflow",
		Steps: []types.Step{
			{ID: "step1", Name: "Step 1", Type: "http"},
		},
	}

	executionID, err := master.SubmitWorkflow(ctx, workflow)
	require.NoError(t, err)

	// Get metrics
	metrics, err := master.GetMetrics(ctx, executionID)
	require.NoError(t, err)
	assert.Equal(t, executionID, metrics.ExecutionID)
}

func TestGetSlaves(t *testing.T) {
	registry := newMockRegistry()

	// Register some slaves
	ctx := context.Background()
	registry.Register(ctx, &types.SlaveInfo{
		ID:   "slave-1",
		Type: types.SlaveTypeWorker,
	})
	registry.Register(ctx, &types.SlaveInfo{
		ID:   "slave-2",
		Type: types.SlaveTypeWorker,
	})

	master := NewWorkflowMaster(DefaultConfig(), registry, nil, nil)

	err := master.Start(ctx)
	require.NoError(t, err)
	defer master.Stop(ctx)

	slaves, err := master.GetSlaves(ctx)
	require.NoError(t, err)
	assert.Len(t, slaves, 2)
}

func TestListExecutions(t *testing.T) {
	registry := newMockRegistry()
	scheduler := newMockScheduler(registry)
	aggregator := newMockAggregator()

	config := DefaultConfig()
	config.StandaloneMode = true

	master := NewWorkflowMaster(config, registry, scheduler, aggregator)

	ctx := context.Background()
	err := master.Start(ctx)
	require.NoError(t, err)
	defer master.Stop(ctx)

	// Submit multiple workflows
	for i := 0; i < 3; i++ {
		workflow := &types.Workflow{
			ID:    "test-workflow",
			Name:  "Test Workflow",
			Steps: []types.Step{{ID: "step1", Name: "Step 1", Type: "http"}},
		}
		_, err := master.SubmitWorkflow(ctx, workflow)
		require.NoError(t, err)
	}

	// List executions
	executions, err := master.ListExecutions(ctx)
	require.NoError(t, err)
	assert.Len(t, executions, 3)
}

func TestMaxConcurrentExecutions(t *testing.T) {
	registry := newMockRegistry()
	scheduler := newMockScheduler(registry)
	aggregator := newMockAggregator()

	config := DefaultConfig()
	config.StandaloneMode = true
	config.MaxConcurrentExecutions = 2

	master := NewWorkflowMaster(config, registry, scheduler, aggregator)

	ctx := context.Background()
	err := master.Start(ctx)
	require.NoError(t, err)
	defer master.Stop(ctx)

	workflow := &types.Workflow{
		ID:    "test-workflow",
		Name:  "Test Workflow",
		Steps: []types.Step{{ID: "step1", Name: "Step 1", Type: "http"}},
	}

	// Submit up to max
	_, err = master.SubmitWorkflow(ctx, workflow)
	require.NoError(t, err)
	_, err = master.SubmitWorkflow(ctx, workflow)
	require.NoError(t, err)

	// Third should fail
	_, err = master.SubmitWorkflow(ctx, workflow)
	assert.Error(t, err)
}
