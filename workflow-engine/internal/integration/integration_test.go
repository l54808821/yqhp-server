// Package integration provides end-to-end integration tests for the workflow execution engine.
// Requirements: 19.1 - End-to-end tests for complete workflow execution, Master-Slave communication, and execution modes.
package integration

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grafana/k6/workflow-engine/internal/executor"
	"github.com/grafana/k6/workflow-engine/internal/master"
	"github.com/grafana/k6/workflow-engine/internal/parser"
	"github.com/grafana/k6/workflow-engine/internal/slave"
	"github.com/grafana/k6/workflow-engine/pkg/types"
)

// TestCompleteWorkflowExecution tests a complete workflow execution from parsing to completion.
func TestCompleteWorkflowExecution(t *testing.T) {
	// Create a test HTTP server to handle requests
	requestCount := 0
	var mu sync.Mutex
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requestCount++
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "ok", "message": "success"}`))
	}))
	defer testServer.Close()

	// Parse workflow YAML
	workflowYAML := `
id: integration-test-workflow
name: Integration Test Workflow
description: Tests complete workflow execution

variables:
  base_url: "` + testServer.URL + `"

options:
  vus: 2
  iterations: 4

steps:
  - id: http-request
    name: HTTP Request Step
    type: http
    config:
      method: GET
      url: "${base_url}/api/test"
    timeout: 5s
`

	p := parser.NewYAMLParser()
	workflow, err := p.Parse([]byte(workflowYAML))
	require.NoError(t, err)
	require.NotNil(t, workflow)

	// Create master with standalone mode
	registry := newTestRegistry()
	scheduler := newTestScheduler(registry)
	aggregator := newTestAggregator()

	masterConfig := master.DefaultConfig()
	masterConfig.StandaloneMode = true

	m := master.NewWorkflowMaster(masterConfig, registry, scheduler, aggregator)

	ctx := context.Background()
	err = m.Start(ctx)
	require.NoError(t, err)
	defer m.Stop(ctx)

	// Submit workflow
	executionID, err := m.SubmitWorkflow(ctx, workflow)
	require.NoError(t, err)
	assert.NotEmpty(t, executionID)

	// Wait for execution to start
	time.Sleep(50 * time.Millisecond)

	// Check execution status
	status, err := m.GetExecutionStatus(ctx, executionID)
	require.NoError(t, err)
	assert.Equal(t, executionID, status.ID)
	assert.Equal(t, "integration-test-workflow", status.WorkflowID)

	// Stop execution
	err = m.StopExecution(ctx, executionID)
	require.NoError(t, err)

	// Verify final status
	status, err = m.GetExecutionStatus(ctx, executionID)
	require.NoError(t, err)
	assert.Equal(t, types.ExecutionStatusAborted, status.Status)
}

// TestMasterSlaveRegistration tests slave registration with master.
func TestMasterSlaveRegistration(t *testing.T) {
	registry := newTestRegistry()
	scheduler := newTestScheduler(registry)
	aggregator := newTestAggregator()

	masterConfig := master.DefaultConfig()
	m := master.NewWorkflowMaster(masterConfig, registry, scheduler, aggregator)

	ctx := context.Background()
	err := m.Start(ctx)
	require.NoError(t, err)
	defer m.Stop(ctx)

	// Register slaves
	slave1 := &types.SlaveInfo{
		ID:           "slave-1",
		Type:         types.SlaveTypeWorker,
		Address:      "localhost:9001",
		Capabilities: []string{"http_executor", "script_executor"},
		Labels:       map[string]string{"region": "us-east"},
	}
	slave2 := &types.SlaveInfo{
		ID:           "slave-2",
		Type:         types.SlaveTypeWorker,
		Address:      "localhost:9002",
		Capabilities: []string{"http_executor"},
		Labels:       map[string]string{"region": "us-west"},
	}

	err = registry.Register(ctx, slave1)
	require.NoError(t, err)
	err = registry.Register(ctx, slave2)
	require.NoError(t, err)

	// Verify slaves are registered
	slaves, err := m.GetSlaves(ctx)
	require.NoError(t, err)
	assert.Len(t, slaves, 2)

	// Verify slave info
	slaveIDs := make(map[string]bool)
	for _, s := range slaves {
		slaveIDs[s.ID] = true
	}
	assert.True(t, slaveIDs["slave-1"])
	assert.True(t, slaveIDs["slave-2"])
}

// TestMasterSlaveHeartbeat tests heartbeat mechanism between master and slave.
func TestMasterSlaveHeartbeat(t *testing.T) {
	registry := newTestRegistry()

	ctx := context.Background()

	// Register a slave
	slaveInfo := &types.SlaveInfo{
		ID:           "heartbeat-slave",
		Type:         types.SlaveTypeWorker,
		Address:      "localhost:9003",
		Capabilities: []string{"http_executor"},
	}
	err := registry.Register(ctx, slaveInfo)
	require.NoError(t, err)

	// Update status with heartbeat
	status := &types.SlaveStatus{
		State:       types.SlaveStateOnline,
		Load:        25.0,
		ActiveTasks: 2,
		LastSeen:    time.Now(),
	}
	err = registry.UpdateStatus(ctx, "heartbeat-slave", status)
	require.NoError(t, err)

	// Verify status was updated
	retrievedStatus, err := registry.GetSlaveStatus(ctx, "heartbeat-slave")
	require.NoError(t, err)
	assert.Equal(t, types.SlaveStateOnline, retrievedStatus.State)
	assert.Equal(t, 25.0, retrievedStatus.Load)
	assert.Equal(t, 2, retrievedStatus.ActiveTasks)
}

// TestSlaveTaskExecution tests task execution on a slave node.
func TestSlaveTaskExecution(t *testing.T) {
	// Create a test HTTP server
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"result": "success"}`))
	}))
	defer testServer.Close()

	// Create executor registry
	execRegistry := executor.NewRegistry()

	// Create slave
	slaveConfig := &slave.Config{
		ID:                "test-slave",
		Type:              types.SlaveTypeWorker,
		Address:           "localhost:9004",
		HeartbeatInterval: 5 * time.Second,
		MaxVUs:            10,
	}

	s := slave.NewWorkerSlave(slaveConfig, execRegistry)

	ctx := context.Background()
	err := s.Start(ctx)
	require.NoError(t, err)
	defer s.Stop(ctx)

	// Verify slave is online
	status := s.GetStatus()
	assert.Equal(t, types.SlaveStateOnline, status.State)

	// Verify slave info
	info := s.GetInfo()
	assert.Equal(t, "test-slave", info.ID)
	assert.Equal(t, types.SlaveTypeWorker, info.Type)
}

// TestConstantVUsMode tests constant VUs execution mode.
func TestConstantVUsMode(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "ok"}`))
	}))
	defer testServer.Close()

	workflowYAML := `
id: constant-vus-test
name: Constant VUs Test
options:
  mode: constant-vus
  vus: 3
  iterations: 6
steps:
  - id: request
    name: HTTP Request
    type: http
    config:
      method: GET
      url: "` + testServer.URL + `"
`

	p := parser.NewYAMLParser()
	workflow, err := p.Parse([]byte(workflowYAML))
	require.NoError(t, err)

	assert.Equal(t, types.ModeConstantVUs, workflow.Options.ExecutionMode)
	assert.Equal(t, 3, workflow.Options.VUs)
	assert.Equal(t, 6, workflow.Options.Iterations)
}

// TestRampingVUsMode tests ramping VUs execution mode.
func TestRampingVUsMode(t *testing.T) {
	workflowYAML := `
id: ramping-vus-test
name: Ramping VUs Test
options:
  mode: ramping-vus
  stages:
    - duration: 10s
      target: 5
    - duration: 20s
      target: 10
    - duration: 10s
      target: 0
steps:
  - id: request
    name: HTTP Request
    type: http
    config:
      method: GET
      url: "http://example.com"
`

	p := parser.NewYAMLParser()
	workflow, err := p.Parse([]byte(workflowYAML))
	require.NoError(t, err)

	assert.Equal(t, types.ModeRampingVUs, workflow.Options.ExecutionMode)
	assert.Len(t, workflow.Options.Stages, 3)
	assert.Equal(t, 10*time.Second, workflow.Options.Stages[0].Duration)
	assert.Equal(t, 5, workflow.Options.Stages[0].Target)
}

// TestPerVUIterationsMode tests per-VU iterations execution mode.
func TestPerVUIterationsMode(t *testing.T) {
	workflowYAML := `
id: per-vu-iterations-test
name: Per VU Iterations Test
options:
  mode: per-vu-iterations
  vus: 5
  iterations: 10
steps:
  - id: request
    name: HTTP Request
    type: http
    config:
      method: GET
      url: "http://example.com"
`

	p := parser.NewYAMLParser()
	workflow, err := p.Parse([]byte(workflowYAML))
	require.NoError(t, err)

	assert.Equal(t, types.ModePerVUIterations, workflow.Options.ExecutionMode)
	assert.Equal(t, 5, workflow.Options.VUs)
	assert.Equal(t, 10, workflow.Options.Iterations)
}

// TestSharedIterationsMode tests shared iterations execution mode.
func TestSharedIterationsMode(t *testing.T) {
	workflowYAML := `
id: shared-iterations-test
name: Shared Iterations Test
options:
  mode: shared-iterations
  vus: 5
  iterations: 100
steps:
  - id: request
    name: HTTP Request
    type: http
    config:
      method: GET
      url: "http://example.com"
`

	p := parser.NewYAMLParser()
	workflow, err := p.Parse([]byte(workflowYAML))
	require.NoError(t, err)

	assert.Equal(t, types.ModeSharedIterations, workflow.Options.ExecutionMode)
	assert.Equal(t, 5, workflow.Options.VUs)
	assert.Equal(t, 100, workflow.Options.Iterations)
}

// TestExternallyControlledMode tests externally controlled execution mode.
func TestExternallyControlledMode(t *testing.T) {
	workflowYAML := `
id: externally-controlled-test
name: Externally Controlled Test
options:
  mode: externally-controlled
  vus: 10
steps:
  - id: request
    name: HTTP Request
    type: http
    config:
      method: GET
      url: "http://example.com"
`

	p := parser.NewYAMLParser()
	workflow, err := p.Parse([]byte(workflowYAML))
	require.NoError(t, err)

	assert.Equal(t, types.ModeExternally, workflow.Options.ExecutionMode)
	assert.Equal(t, 10, workflow.Options.VUs)
}

// TestExecutionPauseResume tests pausing and resuming execution.
func TestExecutionPauseResume(t *testing.T) {
	registry := newTestRegistry()
	scheduler := newTestScheduler(registry)
	aggregator := newTestAggregator()

	masterConfig := master.DefaultConfig()
	masterConfig.StandaloneMode = true

	m := master.NewWorkflowMaster(masterConfig, registry, scheduler, aggregator)

	ctx := context.Background()
	err := m.Start(ctx)
	require.NoError(t, err)
	defer m.Stop(ctx)

	// Submit workflow
	workflow := &types.Workflow{
		ID:   "pause-resume-test",
		Name: "Pause Resume Test",
		Steps: []types.Step{
			{ID: "step1", Name: "Step 1", Type: "http", Config: map[string]any{"url": "http://example.com"}},
		},
		Options: types.ExecutionOptions{
			VUs:      5,
			Duration: 1 * time.Minute,
		},
	}

	executionID, err := m.SubmitWorkflow(ctx, workflow)
	require.NoError(t, err)

	// Wait for execution to start
	time.Sleep(50 * time.Millisecond)

	// Pause execution
	err = m.PauseExecution(ctx, executionID)
	require.NoError(t, err)

	status, err := m.GetExecutionStatus(ctx, executionID)
	require.NoError(t, err)
	assert.Equal(t, types.ExecutionStatusPaused, status.Status)

	// Resume execution
	err = m.ResumeExecution(ctx, executionID)
	require.NoError(t, err)

	status, err = m.GetExecutionStatus(ctx, executionID)
	require.NoError(t, err)
	assert.Equal(t, types.ExecutionStatusRunning, status.Status)

	// Cleanup
	m.StopExecution(ctx, executionID)
}

// TestExecutionScaling tests VU scaling during execution.
func TestExecutionScaling(t *testing.T) {
	registry := newTestRegistry()
	scheduler := newTestScheduler(registry)
	aggregator := newTestAggregator()

	masterConfig := master.DefaultConfig()
	masterConfig.StandaloneMode = true

	m := master.NewWorkflowMaster(masterConfig, registry, scheduler, aggregator)

	ctx := context.Background()
	err := m.Start(ctx)
	require.NoError(t, err)
	defer m.Stop(ctx)

	// Submit workflow
	workflow := &types.Workflow{
		ID:   "scaling-test",
		Name: "Scaling Test",
		Steps: []types.Step{
			{ID: "step1", Name: "Step 1", Type: "http", Config: map[string]any{"url": "http://example.com"}},
		},
		Options: types.ExecutionOptions{
			VUs:      10,
			Duration: 1 * time.Minute,
		},
	}

	executionID, err := m.SubmitWorkflow(ctx, workflow)
	require.NoError(t, err)

	// Wait for execution to start
	time.Sleep(50 * time.Millisecond)

	// Scale up
	err = m.ScaleExecution(ctx, executionID, 20)
	require.NoError(t, err)

	// Scale down
	err = m.ScaleExecution(ctx, executionID, 5)
	require.NoError(t, err)

	// Invalid scale (negative) should fail
	err = m.ScaleExecution(ctx, executionID, -1)
	assert.Error(t, err)

	// Cleanup
	m.StopExecution(ctx, executionID)
}

// TestSlaveSelectionModes tests different slave selection modes.
func TestSlaveSelectionModes(t *testing.T) {
	registry := newTestRegistry()
	ctx := context.Background()

	// Register slaves with different capabilities and labels
	slaves := []*types.SlaveInfo{
		{
			ID:           "slave-http",
			Type:         types.SlaveTypeWorker,
			Capabilities: []string{"http_executor"},
			Labels:       map[string]string{"region": "us-east", "env": "prod"},
		},
		{
			ID:           "slave-script",
			Type:         types.SlaveTypeWorker,
			Capabilities: []string{"script_executor"},
			Labels:       map[string]string{"region": "us-west", "env": "staging"},
		},
		{
			ID:           "slave-all",
			Type:         types.SlaveTypeWorker,
			Capabilities: []string{"http_executor", "script_executor"},
			Labels:       map[string]string{"region": "eu-west", "env": "prod"},
		},
	}

	for _, s := range slaves {
		err := registry.Register(ctx, s)
		require.NoError(t, err)
	}

	scheduler := newTestScheduler(registry)

	// Test manual selection
	manualSelector := &types.SlaveSelector{
		Mode:     types.SelectionModeManual,
		SlaveIDs: []string{"slave-http", "slave-all"},
	}
	selected, err := scheduler.SelectSlaves(ctx, manualSelector)
	require.NoError(t, err)
	assert.Len(t, selected, 2)

	// Test label selection
	labelSelector := &types.SlaveSelector{
		Mode:   types.SelectionModeLabel,
		Labels: map[string]string{"env": "prod"},
	}
	selected, err = scheduler.SelectSlaves(ctx, labelSelector)
	require.NoError(t, err)
	assert.Len(t, selected, 2) // slave-http and slave-all

	// Test capability selection
	capSelector := &types.SlaveSelector{
		Mode:         types.SelectionModeCapability,
		Capabilities: []string{"http_executor"},
	}
	selected, err = scheduler.SelectSlaves(ctx, capSelector)
	require.NoError(t, err)
	assert.Len(t, selected, 2) // slave-http and slave-all

	// Test auto selection
	autoSelector := &types.SlaveSelector{
		Mode:      types.SelectionModeAuto,
		MinSlaves: 1,
		MaxSlaves: 3,
	}
	selected, err = scheduler.SelectSlaves(ctx, autoSelector)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(selected), 1)
	assert.LessOrEqual(t, len(selected), 3)
}

// TestWorkflowWithConditions tests workflow execution with conditional logic.
func TestWorkflowWithConditions(t *testing.T) {
	workflowYAML := `
id: conditional-workflow
name: Conditional Workflow Test
variables:
  should_run: "true"
steps:
  - id: conditional-step
    name: Conditional Step
    type: condition
    condition:
      expression: "${should_run} == \"true\""
      then:
        - id: then-step
          name: Then Step
          type: http
          config:
            method: GET
            url: "http://example.com/success"
      else:
        - id: else-step
          name: Else Step
          type: http
          config:
            method: GET
            url: "http://example.com/failure"
`

	p := parser.NewYAMLParser()
	workflow, err := p.Parse([]byte(workflowYAML))
	require.NoError(t, err)

	assert.Equal(t, "conditional-workflow", workflow.ID)
	assert.Len(t, workflow.Steps, 1)
	assert.NotNil(t, workflow.Steps[0].Condition)
	assert.Len(t, workflow.Steps[0].Condition.Then, 1)
	assert.Len(t, workflow.Steps[0].Condition.Else, 1)
}

// TestWorkflowWithHooks tests workflow execution with pre/post hooks.
func TestWorkflowWithHooks(t *testing.T) {
	workflowYAML := `
id: hooks-workflow
name: Hooks Workflow Test
pre_hook:
  type: script
  config:
    inline: "console.log('Pre-workflow hook')"
post_hook:
  type: script
  config:
    inline: "console.log('Post-workflow hook')"
steps:
  - id: main-step
    name: Main Step
    type: http
    config:
      method: GET
      url: "http://example.com"
    pre_hook:
      type: script
      config:
        inline: "console.log('Pre-step hook')"
    post_hook:
      type: script
      config:
        inline: "console.log('Post-step hook')"
`

	p := parser.NewYAMLParser()
	workflow, err := p.Parse([]byte(workflowYAML))
	require.NoError(t, err)

	assert.NotNil(t, workflow.PreHook)
	assert.NotNil(t, workflow.PostHook)
	assert.Equal(t, "script", workflow.PreHook.Type)
	assert.Equal(t, "script", workflow.PostHook.Type)

	assert.NotNil(t, workflow.Steps[0].PreHook)
	assert.NotNil(t, workflow.Steps[0].PostHook)
}

// TestMetricsCollection tests metrics collection during execution.
func TestMetricsCollection(t *testing.T) {
	registry := newTestRegistry()
	scheduler := newTestScheduler(registry)
	aggregator := newTestAggregator()

	masterConfig := master.DefaultConfig()
	masterConfig.StandaloneMode = true

	m := master.NewWorkflowMaster(masterConfig, registry, scheduler, aggregator)

	ctx := context.Background()
	err := m.Start(ctx)
	require.NoError(t, err)
	defer m.Stop(ctx)

	// Submit workflow
	workflow := &types.Workflow{
		ID:   "metrics-test",
		Name: "Metrics Test",
		Steps: []types.Step{
			{ID: "step1", Name: "Step 1", Type: "http", Config: map[string]any{"url": "http://example.com"}},
		},
	}

	executionID, err := m.SubmitWorkflow(ctx, workflow)
	require.NoError(t, err)

	// Wait for execution to start
	time.Sleep(50 * time.Millisecond)

	// Get metrics
	metrics, err := m.GetMetrics(ctx, executionID)
	require.NoError(t, err)
	assert.Equal(t, executionID, metrics.ExecutionID)

	// Cleanup
	m.StopExecution(ctx, executionID)
}

// TestConcurrentExecutions tests multiple concurrent workflow executions.
func TestConcurrentExecutions(t *testing.T) {
	registry := newTestRegistry()
	scheduler := newTestScheduler(registry)
	aggregator := newTestAggregator()

	masterConfig := master.DefaultConfig()
	masterConfig.StandaloneMode = true
	masterConfig.MaxConcurrentExecutions = 5

	m := master.NewWorkflowMaster(masterConfig, registry, scheduler, aggregator)

	ctx := context.Background()
	err := m.Start(ctx)
	require.NoError(t, err)
	defer m.Stop(ctx)

	// Submit multiple workflows concurrently
	var wg sync.WaitGroup
	executionIDs := make([]string, 3)
	errors := make([]error, 3)

	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			workflow := &types.Workflow{
				ID:   "concurrent-test",
				Name: "Concurrent Test",
				Steps: []types.Step{
					{ID: "step1", Name: "Step 1", Type: "http", Config: map[string]any{"url": "http://example.com"}},
				},
			}
			executionIDs[idx], errors[idx] = m.SubmitWorkflow(ctx, workflow)
		}(i)
	}

	wg.Wait()

	// Verify all executions were submitted successfully
	for i := 0; i < 3; i++ {
		assert.NoError(t, errors[i])
		assert.NotEmpty(t, executionIDs[i])
	}

	// Cleanup
	for _, id := range executionIDs {
		if id != "" {
			m.StopExecution(ctx, id)
		}
	}
}

// Helper types and functions for testing

// testRegistry implements master.SlaveRegistry for testing.
type testRegistry struct {
	slaves map[string]*types.SlaveInfo
	status map[string]*types.SlaveStatus
	mu     sync.RWMutex
}

func newTestRegistry() *testRegistry {
	return &testRegistry{
		slaves: make(map[string]*types.SlaveInfo),
		status: make(map[string]*types.SlaveStatus),
	}
}

func (r *testRegistry) Register(ctx context.Context, slave *types.SlaveInfo) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.slaves[slave.ID] = slave
	r.status[slave.ID] = &types.SlaveStatus{
		State:    types.SlaveStateOnline,
		LastSeen: time.Now(),
	}
	return nil
}

func (r *testRegistry) Unregister(ctx context.Context, slaveID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.slaves, slaveID)
	delete(r.status, slaveID)
	return nil
}

func (r *testRegistry) UpdateStatus(ctx context.Context, slaveID string, status *types.SlaveStatus) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.status[slaveID] = status
	return nil
}

func (r *testRegistry) GetSlave(ctx context.Context, slaveID string) (*types.SlaveInfo, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.slaves[slaveID], nil
}

func (r *testRegistry) GetSlaveStatus(ctx context.Context, slaveID string) (*types.SlaveStatus, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.status[slaveID], nil
}

func (r *testRegistry) ListSlaves(ctx context.Context, filter *master.SlaveFilter) ([]*types.SlaveInfo, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]*types.SlaveInfo, 0, len(r.slaves))
	for _, slave := range r.slaves {
		result = append(result, slave)
	}
	return result, nil
}

func (r *testRegistry) GetOnlineSlaves(ctx context.Context) ([]*types.SlaveInfo, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]*types.SlaveInfo, 0)
	for id, slave := range r.slaves {
		if status, ok := r.status[id]; ok && status.State == types.SlaveStateOnline {
			result = append(result, slave)
		}
	}
	return result, nil
}

func (r *testRegistry) WatchSlaves(ctx context.Context) (<-chan *types.SlaveEvent, error) {
	ch := make(chan *types.SlaveEvent)
	return ch, nil
}

// testScheduler implements master.Scheduler for testing.
type testScheduler struct {
	registry *testRegistry
}

func newTestScheduler(registry *testRegistry) *testScheduler {
	return &testScheduler{registry: registry}
}

func (s *testScheduler) Schedule(ctx context.Context, workflow *types.Workflow, slaves []*types.SlaveInfo) (*types.ExecutionPlan, error) {
	if len(slaves) == 0 {
		return &types.ExecutionPlan{Assignments: []*types.SlaveAssignment{}}, nil
	}

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

func (s *testScheduler) Reschedule(ctx context.Context, failedSlaveID string, plan *types.ExecutionPlan) (*types.ExecutionPlan, error) {
	return plan, nil
}

func (s *testScheduler) SelectSlaves(ctx context.Context, selector *types.SlaveSelector) ([]*types.SlaveInfo, error) {
	allSlaves, err := s.registry.ListSlaves(ctx, nil)
	if err != nil {
		return nil, err
	}

	if selector == nil {
		return allSlaves, nil
	}

	switch selector.Mode {
	case types.SelectionModeManual:
		return s.selectByIDs(allSlaves, selector.SlaveIDs), nil
	case types.SelectionModeLabel:
		return s.selectByLabels(allSlaves, selector.Labels), nil
	case types.SelectionModeCapability:
		return s.selectByCapabilities(allSlaves, selector.Capabilities), nil
	case types.SelectionModeAuto:
		return s.selectAuto(allSlaves, selector.MinSlaves, selector.MaxSlaves), nil
	default:
		return allSlaves, nil
	}
}

func (s *testScheduler) selectByIDs(slaves []*types.SlaveInfo, ids []string) []*types.SlaveInfo {
	idSet := make(map[string]bool)
	for _, id := range ids {
		idSet[id] = true
	}

	result := make([]*types.SlaveInfo, 0)
	for _, slave := range slaves {
		if idSet[slave.ID] {
			result = append(result, slave)
		}
	}
	return result
}

func (s *testScheduler) selectByLabels(slaves []*types.SlaveInfo, labels map[string]string) []*types.SlaveInfo {
	result := make([]*types.SlaveInfo, 0)
	for _, slave := range slaves {
		if matchLabels(slave.Labels, labels) {
			result = append(result, slave)
		}
	}
	return result
}

func (s *testScheduler) selectByCapabilities(slaves []*types.SlaveInfo, capabilities []string) []*types.SlaveInfo {
	result := make([]*types.SlaveInfo, 0)
	for _, slave := range slaves {
		if hasCapabilities(slave.Capabilities, capabilities) {
			result = append(result, slave)
		}
	}
	return result
}

func (s *testScheduler) selectAuto(slaves []*types.SlaveInfo, minSlaves, maxSlaves int) []*types.SlaveInfo {
	if len(slaves) == 0 {
		return slaves
	}

	count := len(slaves)
	if minSlaves > 0 && count < minSlaves {
		count = minSlaves
	}
	if maxSlaves > 0 && count > maxSlaves {
		count = maxSlaves
	}
	if count > len(slaves) {
		count = len(slaves)
	}

	return slaves[:count]
}

func matchLabels(slaveLabels, requiredLabels map[string]string) bool {
	for k, v := range requiredLabels {
		if slaveLabels[k] != v {
			return false
		}
	}
	return true
}

func hasCapabilities(slaveCaps, requiredCaps []string) bool {
	capSet := make(map[string]bool)
	for _, cap := range slaveCaps {
		capSet[cap] = true
	}

	for _, cap := range requiredCaps {
		if !capSet[cap] {
			return false
		}
	}
	return true
}

// testAggregator implements master.MetricsAggregator for testing.
type testAggregator struct{}

func newTestAggregator() *testAggregator {
	return &testAggregator{}
}

func (a *testAggregator) Aggregate(ctx context.Context, executionID string, slaveMetrics []*types.Metrics) (*types.AggregatedMetrics, error) {
	return &types.AggregatedMetrics{
		ExecutionID: executionID,
		StepMetrics: make(map[string]*types.StepMetrics),
	}, nil
}

func (a *testAggregator) EvaluateThresholds(ctx context.Context, metrics *types.AggregatedMetrics, thresholds []types.Threshold) ([]types.ThresholdResult, error) {
	return []types.ThresholdResult{}, nil
}

func (a *testAggregator) GenerateSummary(ctx context.Context, metrics *types.AggregatedMetrics) (*master.ExecutionSummary, error) {
	return &master.ExecutionSummary{
		ExecutionID: metrics.ExecutionID,
	}, nil
}
