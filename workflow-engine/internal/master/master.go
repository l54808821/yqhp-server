package master

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/grafana/k6/workflow-engine/pkg/types"
)

// Master defines the interface for a master node.
// Requirements: 5.1, 5.7
type Master interface {
	// Start initializes and starts the master node.
	Start(ctx context.Context) error

	// Stop gracefully shuts down the master node.
	Stop(ctx context.Context) error

	// SubmitWorkflow submits a workflow for execution.
	SubmitWorkflow(ctx context.Context, workflow *types.Workflow) (string, error)

	// GetExecutionStatus returns the execution status.
	GetExecutionStatus(ctx context.Context, executionID string) (*types.ExecutionState, error)

	// StopExecution stops a running execution.
	StopExecution(ctx context.Context, executionID string) error

	// PauseExecution pauses a running execution.
	PauseExecution(ctx context.Context, executionID string) error

	// ResumeExecution resumes a paused execution.
	ResumeExecution(ctx context.Context, executionID string) error

	// ScaleExecution scales the VU count for an execution.
	ScaleExecution(ctx context.Context, executionID string, targetVUs int) error

	// GetMetrics returns aggregated metrics for an execution.
	GetMetrics(ctx context.Context, executionID string) (*types.AggregatedMetrics, error)

	// GetSlaves returns all registered slaves.
	GetSlaves(ctx context.Context) ([]*types.SlaveInfo, error)
}

// Config holds the configuration for a master node.
type Config struct {
	// ID is the unique identifier for this master.
	ID string

	// Address is the address this master listens on.
	Address string

	// HeartbeatTimeout is the timeout for slave heartbeats.
	HeartbeatTimeout time.Duration

	// HealthCheckInterval is the interval for health checks.
	HealthCheckInterval time.Duration

	// StandaloneMode enables single-node execution without slaves.
	StandaloneMode bool

	// MaxConcurrentExecutions is the maximum number of concurrent executions.
	MaxConcurrentExecutions int
}

// DefaultConfig returns a default master configuration.
func DefaultConfig() *Config {
	return &Config{
		ID:                      uuid.New().String(),
		Address:                 ":8080",
		HeartbeatTimeout:        30 * time.Second,
		HealthCheckInterval:     10 * time.Second,
		StandaloneMode:          false,
		MaxConcurrentExecutions: 100,
	}
}

// WorkflowMaster implements the Master interface.
// Requirements: 5.1, 5.7
type WorkflowMaster struct {
	config *Config

	// Registry for slave management
	registry SlaveRegistry

	// Scheduler for task distribution
	scheduler Scheduler

	// Metrics aggregator
	aggregator MetricsAggregator

	// Execution state management
	executions     map[string]*ExecutionInfo
	executionMu    sync.RWMutex
	executionCount atomic.Int32

	// State management
	state    atomic.Value // MasterState
	started  atomic.Bool
	stopOnce sync.Once
	stopped  chan struct{}

	// Health check
	healthCtx    context.Context
	healthCancel context.CancelFunc

	// Synchronization
	mu sync.RWMutex
}

// MasterState represents the state of the master node.
type MasterState string

const (
	// MasterStateStarting indicates the master is starting.
	MasterStateStarting MasterState = "starting"
	// MasterStateRunning indicates the master is running.
	MasterStateRunning MasterState = "running"
	// MasterStateStopping indicates the master is stopping.
	MasterStateStopping MasterState = "stopping"
	// MasterStateStopped indicates the master is stopped.
	MasterStateStopped MasterState = "stopped"
)

// ExecutionInfo holds information about a workflow execution.
type ExecutionInfo struct {
	ID        string
	Workflow  *types.Workflow
	State     *types.ExecutionState
	Plan      *types.ExecutionPlan
	StartTime time.Time
	EndTime   *time.Time

	// Control channels
	stopCh   chan struct{}
	pauseCh  chan struct{}
	resumeCh chan struct{}

	// Synchronization
	mu sync.RWMutex
}

// NewWorkflowMaster creates a new workflow master.
func NewWorkflowMaster(config *Config, registry SlaveRegistry, scheduler Scheduler, aggregator MetricsAggregator) *WorkflowMaster {
	if config == nil {
		config = DefaultConfig()
	}

	m := &WorkflowMaster{
		config:     config,
		registry:   registry,
		scheduler:  scheduler,
		aggregator: aggregator,
		executions: make(map[string]*ExecutionInfo),
		stopped:    make(chan struct{}),
	}

	m.state.Store(MasterStateStopped)

	return m
}

// Start initializes and starts the master node.
// Requirements: 5.1
func (m *WorkflowMaster) Start(ctx context.Context) error {
	if m.started.Load() {
		return fmt.Errorf("master already started")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.state.Store(MasterStateStarting)

	// Start health check goroutine
	m.healthCtx, m.healthCancel = context.WithCancel(context.Background())
	go m.healthCheckLoop()

	m.state.Store(MasterStateRunning)
	m.started.Store(true)

	return nil
}

// Stop gracefully shuts down the master node.
func (m *WorkflowMaster) Stop(ctx context.Context) error {
	var err error
	m.stopOnce.Do(func() {
		m.state.Store(MasterStateStopping)

		// Stop health check
		if m.healthCancel != nil {
			m.healthCancel()
		}

		// Stop all running executions
		m.executionMu.Lock()
		for _, exec := range m.executions {
			if exec.State.Status == types.ExecutionStatusRunning {
				close(exec.stopCh)
			}
		}
		m.executionMu.Unlock()

		m.state.Store(MasterStateStopped)
		m.started.Store(false)
		close(m.stopped)
	})
	return err
}

// SubmitWorkflow submits a workflow for execution.
// Requirements: 5.1, 5.3, 5.7
func (m *WorkflowMaster) SubmitWorkflow(ctx context.Context, workflow *types.Workflow) (string, error) {
	if workflow == nil {
		return "", fmt.Errorf("workflow cannot be nil")
	}

	if !m.started.Load() {
		return "", fmt.Errorf("master not started")
	}

	// Check concurrent execution limit
	if int(m.executionCount.Load()) >= m.config.MaxConcurrentExecutions {
		return "", fmt.Errorf("maximum concurrent executions reached: %d", m.config.MaxConcurrentExecutions)
	}

	// Generate execution ID
	executionID := uuid.New().String()

	// Create execution info
	execInfo := &ExecutionInfo{
		ID:        executionID,
		Workflow:  workflow,
		StartTime: time.Now(),
		stopCh:    make(chan struct{}),
		pauseCh:   make(chan struct{}),
		resumeCh:  make(chan struct{}),
		State: &types.ExecutionState{
			ID:          executionID,
			WorkflowID:  workflow.ID,
			Status:      types.ExecutionStatusPending,
			StartTime:   time.Now(),
			Progress:    0,
			SlaveStates: make(map[string]*types.SlaveExecutionState),
			Errors:      []types.ExecutionError{},
		},
	}

	// Store execution
	m.executionMu.Lock()
	m.executions[executionID] = execInfo
	m.executionCount.Add(1)
	m.executionMu.Unlock()

	// Schedule execution
	if err := m.scheduleExecution(ctx, execInfo); err != nil {
		m.executionMu.Lock()
		execInfo.State.Status = types.ExecutionStatusFailed
		execInfo.State.Errors = append(execInfo.State.Errors, types.ExecutionError{
			Code:      types.ErrCodeExecution,
			Message:   fmt.Sprintf("failed to schedule execution: %v", err),
			Timestamp: time.Now(),
		})
		m.executionMu.Unlock()
		return executionID, err
	}

	return executionID, nil
}

// scheduleExecution schedules a workflow execution across slaves.
func (m *WorkflowMaster) scheduleExecution(ctx context.Context, execInfo *ExecutionInfo) error {
	execInfo.mu.Lock()
	defer execInfo.mu.Unlock()

	// Get available slaves
	var slaves []*types.SlaveInfo
	var err error

	if m.config.StandaloneMode {
		// In standalone mode, create a virtual local slave
		slaves = []*types.SlaveInfo{{
			ID:           "local",
			Type:         types.SlaveTypeWorker,
			Address:      "localhost",
			Capabilities: []string{"http_executor", "script_executor"},
		}}
	} else {
		// Select slaves based on workflow options
		selector := execInfo.Workflow.Options.TargetSlaves
		if selector == nil {
			selector = &types.SlaveSelector{Mode: types.SelectionModeAuto}
		}
		slaves, err = m.scheduler.SelectSlaves(ctx, selector)
		if err != nil {
			return fmt.Errorf("failed to select slaves: %w", err)
		}
	}

	if len(slaves) == 0 {
		return fmt.Errorf("no available slaves for execution")
	}

	// Create execution plan
	plan, err := m.scheduler.Schedule(ctx, execInfo.Workflow, slaves)
	if err != nil {
		return fmt.Errorf("failed to create execution plan: %w", err)
	}

	plan.ExecutionID = execInfo.ID
	execInfo.Plan = plan

	// Initialize slave states
	for _, assignment := range plan.Assignments {
		execInfo.State.SlaveStates[assignment.SlaveID] = &types.SlaveExecutionState{
			SlaveID: assignment.SlaveID,
			Status:  types.ExecutionStatusPending,
			Segment: assignment.Segment,
		}
	}

	// Update status to running
	execInfo.State.Status = types.ExecutionStatusRunning

	// Start execution in background
	go m.runExecution(ctx, execInfo)

	return nil
}

// runExecution runs the workflow execution.
func (m *WorkflowMaster) runExecution(ctx context.Context, execInfo *ExecutionInfo) {
	defer func() {
		m.executionCount.Add(-1)
	}()

	// Create a context that can be cancelled
	execCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Monitor for stop signal
	go func() {
		select {
		case <-execInfo.stopCh:
			cancel()
		case <-execCtx.Done():
		}
	}()

	// In a real implementation, this would distribute tasks to slaves
	// and collect results. For now, we simulate the execution.
	m.simulateExecution(execCtx, execInfo)
}

// simulateExecution simulates workflow execution (placeholder for real implementation).
func (m *WorkflowMaster) simulateExecution(ctx context.Context, execInfo *ExecutionInfo) {
	execInfo.mu.Lock()
	execInfo.State.Status = types.ExecutionStatusRunning
	execInfo.mu.Unlock()

	// Wait for completion or cancellation
	select {
	case <-ctx.Done():
		execInfo.mu.Lock()
		execInfo.State.Status = types.ExecutionStatusAborted
		now := time.Now()
		execInfo.EndTime = &now
		execInfo.State.EndTime = &now
		execInfo.mu.Unlock()
	case <-execInfo.stopCh:
		execInfo.mu.Lock()
		execInfo.State.Status = types.ExecutionStatusAborted
		now := time.Now()
		execInfo.EndTime = &now
		execInfo.State.EndTime = &now
		execInfo.mu.Unlock()
	}
}

// GetExecutionStatus returns the execution status.
// Requirements: 5.1
func (m *WorkflowMaster) GetExecutionStatus(ctx context.Context, executionID string) (*types.ExecutionState, error) {
	m.executionMu.RLock()
	defer m.executionMu.RUnlock()

	execInfo, ok := m.executions[executionID]
	if !ok {
		return nil, fmt.Errorf("execution not found: %s", executionID)
	}

	execInfo.mu.RLock()
	defer execInfo.mu.RUnlock()

	// Return a copy of the state
	state := *execInfo.State
	return &state, nil
}

// StopExecution stops a running execution.
// Requirements: 5.1
func (m *WorkflowMaster) StopExecution(ctx context.Context, executionID string) error {
	m.executionMu.RLock()
	execInfo, ok := m.executions[executionID]
	m.executionMu.RUnlock()

	if !ok {
		return fmt.Errorf("execution not found: %s", executionID)
	}

	execInfo.mu.Lock()
	defer execInfo.mu.Unlock()

	if execInfo.State.Status != types.ExecutionStatusRunning &&
		execInfo.State.Status != types.ExecutionStatusPaused {
		return fmt.Errorf("execution is not running or paused: %s", execInfo.State.Status)
	}

	// Signal stop
	select {
	case <-execInfo.stopCh:
		// Already stopped
	default:
		close(execInfo.stopCh)
	}

	execInfo.State.Status = types.ExecutionStatusAborted
	now := time.Now()
	execInfo.EndTime = &now
	execInfo.State.EndTime = &now

	return nil
}

// PauseExecution pauses a running execution.
// Requirements: 6.2.5
func (m *WorkflowMaster) PauseExecution(ctx context.Context, executionID string) error {
	m.executionMu.RLock()
	execInfo, ok := m.executions[executionID]
	m.executionMu.RUnlock()

	if !ok {
		return fmt.Errorf("execution not found: %s", executionID)
	}

	execInfo.mu.Lock()
	defer execInfo.mu.Unlock()

	if execInfo.State.Status != types.ExecutionStatusRunning {
		return fmt.Errorf("execution is not running: %s", execInfo.State.Status)
	}

	// Signal pause
	select {
	case execInfo.pauseCh <- struct{}{}:
	default:
	}

	execInfo.State.Status = types.ExecutionStatusPaused

	return nil
}

// ResumeExecution resumes a paused execution.
// Requirements: 6.2.6
func (m *WorkflowMaster) ResumeExecution(ctx context.Context, executionID string) error {
	m.executionMu.RLock()
	execInfo, ok := m.executions[executionID]
	m.executionMu.RUnlock()

	if !ok {
		return fmt.Errorf("execution not found: %s", executionID)
	}

	execInfo.mu.Lock()
	defer execInfo.mu.Unlock()

	if execInfo.State.Status != types.ExecutionStatusPaused {
		return fmt.Errorf("execution is not paused: %s", execInfo.State.Status)
	}

	// Signal resume
	select {
	case execInfo.resumeCh <- struct{}{}:
	default:
	}

	execInfo.State.Status = types.ExecutionStatusRunning

	return nil
}

// ScaleExecution scales the VU count for an execution.
// Requirements: 6.2.1, 6.2.2, 6.2.3
func (m *WorkflowMaster) ScaleExecution(ctx context.Context, executionID string, targetVUs int) error {
	if targetVUs < 0 {
		return fmt.Errorf("target VUs cannot be negative: %d", targetVUs)
	}

	m.executionMu.RLock()
	execInfo, ok := m.executions[executionID]
	m.executionMu.RUnlock()

	if !ok {
		return fmt.Errorf("execution not found: %s", executionID)
	}

	execInfo.mu.Lock()
	defer execInfo.mu.Unlock()

	if execInfo.State.Status != types.ExecutionStatusRunning {
		return fmt.Errorf("execution is not running: %s", execInfo.State.Status)
	}

	// In a real implementation, this would send scale commands to slaves
	// For now, we just update the workflow options
	execInfo.Workflow.Options.VUs = targetVUs

	return nil
}

// GetMetrics returns aggregated metrics for an execution.
// Requirements: 5.4, 5.6
func (m *WorkflowMaster) GetMetrics(ctx context.Context, executionID string) (*types.AggregatedMetrics, error) {
	m.executionMu.RLock()
	execInfo, ok := m.executions[executionID]
	m.executionMu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("execution not found: %s", executionID)
	}

	execInfo.mu.RLock()
	defer execInfo.mu.RUnlock()

	// If we have aggregated metrics, return them
	if execInfo.State.AggregatedMetrics != nil {
		return execInfo.State.AggregatedMetrics, nil
	}

	// Otherwise, aggregate from slave states
	if m.aggregator != nil {
		slaveMetrics := make([]*types.Metrics, 0)
		for _, slaveState := range execInfo.State.SlaveStates {
			if slaveState.CurrentMetrics != nil {
				slaveMetrics = append(slaveMetrics, slaveState.CurrentMetrics)
			}
		}
		return m.aggregator.Aggregate(ctx, executionID, slaveMetrics)
	}

	return &types.AggregatedMetrics{
		ExecutionID: executionID,
		StepMetrics: make(map[string]*types.StepMetrics),
	}, nil
}

// GetSlaves returns all registered slaves.
// Requirements: 5.2
func (m *WorkflowMaster) GetSlaves(ctx context.Context) ([]*types.SlaveInfo, error) {
	if m.registry == nil {
		return []*types.SlaveInfo{}, nil
	}
	return m.registry.ListSlaves(ctx, nil)
}

// healthCheckLoop periodically checks slave health.
func (m *WorkflowMaster) healthCheckLoop() {
	ticker := time.NewTicker(m.config.HealthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-m.healthCtx.Done():
			return
		case <-ticker.C:
			m.checkSlaveHealth()
		}
	}
}

// checkSlaveHealth checks the health of all registered slaves.
func (m *WorkflowMaster) checkSlaveHealth() {
	if m.registry == nil {
		return
	}

	ctx := context.Background()
	slaves, err := m.registry.ListSlaves(ctx, nil)
	if err != nil {
		return
	}

	now := time.Now()
	for _, slave := range slaves {
		status, err := m.registry.GetSlaveStatus(ctx, slave.ID)
		if err != nil {
			continue
		}

		// Check if heartbeat is stale
		if now.Sub(status.LastSeen) > m.config.HeartbeatTimeout {
			// Mark slave as offline
			_ = m.registry.UpdateStatus(ctx, slave.ID, &types.SlaveStatus{
				State:    types.SlaveStateOffline,
				LastSeen: status.LastSeen,
			})
		}
	}
}

// GetState returns the current master state.
func (m *WorkflowMaster) GetState() MasterState {
	return m.state.Load().(MasterState)
}

// IsRunning returns whether the master is running.
func (m *WorkflowMaster) IsRunning() bool {
	return m.started.Load() && m.GetState() == MasterStateRunning
}

// GetExecutionCount returns the number of active executions.
func (m *WorkflowMaster) GetExecutionCount() int {
	return int(m.executionCount.Load())
}

// ListExecutions returns all executions.
func (m *WorkflowMaster) ListExecutions(ctx context.Context) ([]*types.ExecutionState, error) {
	m.executionMu.RLock()
	defer m.executionMu.RUnlock()

	states := make([]*types.ExecutionState, 0, len(m.executions))
	for _, execInfo := range m.executions {
		execInfo.mu.RLock()
		state := *execInfo.State
		execInfo.mu.RUnlock()
		states = append(states, &state)
	}

	return states, nil
}
