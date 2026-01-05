package slave

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/grafana/k6/workflow-engine/internal/executor"
	"github.com/grafana/k6/workflow-engine/pkg/types"
)

// Slave defines the interface for a slave node.
// Requirements: 12.1, 12.4
type Slave interface {
	// Start initializes and starts the slave node.
	Start(ctx context.Context) error

	// Stop gracefully shuts down the slave node.
	Stop(ctx context.Context) error

	// Connect connects to the master node.
	Connect(ctx context.Context, masterAddr string) error

	// Disconnect disconnects from the master node.
	Disconnect(ctx context.Context) error

	// ExecuteTask executes an assigned task.
	ExecuteTask(ctx context.Context, task *types.Task) (*types.TaskResult, error)

	// GetStatus returns the current slave status.
	GetStatus() *types.SlaveStatus

	// GetInfo returns the slave registration information.
	GetInfo() *types.SlaveInfo
}

// Config holds the configuration for a slave node.
type Config struct {
	// ID is the unique identifier for this slave.
	ID string

	// Type is the type of slave (worker, gateway, aggregator).
	Type types.SlaveType

	// Address is the address this slave listens on.
	Address string

	// MasterAddress is the address of the master node.
	MasterAddress string

	// Capabilities are the capabilities this slave supports.
	Capabilities []string

	// Labels are key-value labels for this slave.
	Labels map[string]string

	// HeartbeatInterval is the interval between heartbeats.
	HeartbeatInterval time.Duration

	// HeartbeatTimeout is the timeout for heartbeat responses.
	HeartbeatTimeout time.Duration

	// MaxVUs is the maximum number of virtual users this slave can handle.
	MaxVUs int

	// CPUCores is the number of CPU cores available.
	CPUCores int

	// MemoryMB is the amount of memory available in MB.
	MemoryMB int64
}

// DefaultConfig returns a default slave configuration.
func DefaultConfig() *Config {
	return &Config{
		Type:              types.SlaveTypeWorker,
		HeartbeatInterval: 5 * time.Second,
		HeartbeatTimeout:  10 * time.Second,
		MaxVUs:            100,
		CPUCores:          4,
		MemoryMB:          4096,
	}
}

// WorkerSlave implements the Slave interface for executing workflows.
// Requirements: 12.1, 12.4
type WorkerSlave struct {
	config   *Config
	registry *executor.Registry

	// State management
	state       atomic.Value // types.SlaveState
	connected   atomic.Bool
	masterAddr  string
	activeTasks atomic.Int32
	currentLoad atomic.Value // float64

	// Heartbeat management
	heartbeatCtx    context.Context
	heartbeatCancel context.CancelFunc
	lastHeartbeat   atomic.Value // time.Time

	// Metrics
	metrics *types.SlaveMetrics

	// Task execution
	taskEngine *TaskEngine

	// Synchronization
	mu       sync.RWMutex
	stopOnce sync.Once
	stopped  chan struct{}
}

// NewWorkerSlave creates a new worker slave.
func NewWorkerSlave(config *Config, registry *executor.Registry) *WorkerSlave {
	if config == nil {
		config = DefaultConfig()
	}

	s := &WorkerSlave{
		config:   config,
		registry: registry,
		metrics:  &types.SlaveMetrics{},
		stopped:  make(chan struct{}),
	}

	s.state.Store(types.SlaveStateOffline)
	s.currentLoad.Store(float64(0))
	s.lastHeartbeat.Store(time.Time{})

	return s
}

// Start initializes and starts the slave node.
// Requirements: 12.1
func (s *WorkerSlave) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Initialize task engine
	s.taskEngine = NewTaskEngine(s.registry, s.config.MaxVUs)

	// Set state to online
	s.state.Store(types.SlaveStateOnline)

	return nil
}

// Stop gracefully shuts down the slave node.
func (s *WorkerSlave) Stop(ctx context.Context) error {
	var err error
	s.stopOnce.Do(func() {
		// Stop heartbeat
		if s.heartbeatCancel != nil {
			s.heartbeatCancel()
		}

		// Stop task engine
		if s.taskEngine != nil {
			err = s.taskEngine.Stop(ctx)
		}

		// Set state to offline
		s.state.Store(types.SlaveStateOffline)
		s.connected.Store(false)

		close(s.stopped)
	})
	return err
}

// Connect connects to the master node.
// Requirements: 12.1
func (s *WorkerSlave) Connect(ctx context.Context, masterAddr string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.connected.Load() {
		return fmt.Errorf("already connected to master")
	}

	s.masterAddr = masterAddr
	s.connected.Store(true)

	// Start heartbeat goroutine
	s.heartbeatCtx, s.heartbeatCancel = context.WithCancel(context.Background())
	go s.heartbeatLoop()

	return nil
}

// Disconnect disconnects from the master node.
func (s *WorkerSlave) Disconnect(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.connected.Load() {
		return nil
	}

	// Stop heartbeat
	if s.heartbeatCancel != nil {
		s.heartbeatCancel()
	}

	s.connected.Store(false)
	s.masterAddr = ""

	return nil
}

// ExecuteTask executes an assigned task.
// Requirements: 12.1
func (s *WorkerSlave) ExecuteTask(ctx context.Context, task *types.Task) (*types.TaskResult, error) {
	if task == nil {
		return nil, fmt.Errorf("task cannot be nil")
	}

	s.activeTasks.Add(1)
	defer s.activeTasks.Add(-1)

	// Update state to busy if we have active tasks
	if s.activeTasks.Load() > 0 {
		s.state.Store(types.SlaveStateBusy)
	}

	// Execute the task using the task engine
	result, err := s.taskEngine.Execute(ctx, task)

	// Update state back to online if no more active tasks
	if s.activeTasks.Load() == 0 {
		s.state.Store(types.SlaveStateOnline)
	}

	return result, err
}

// GetStatus returns the current slave status.
// Requirements: 12.4
func (s *WorkerSlave) GetStatus() *types.SlaveStatus {
	load, _ := s.currentLoad.Load().(float64)
	lastSeen, _ := s.lastHeartbeat.Load().(time.Time)
	state, _ := s.state.Load().(types.SlaveState)

	return &types.SlaveStatus{
		State:       state,
		Load:        load,
		ActiveTasks: int(s.activeTasks.Load()),
		LastSeen:    lastSeen,
		Metrics:     s.getMetrics(),
	}
}

// GetInfo returns the slave registration information.
// Requirements: 12.1
func (s *WorkerSlave) GetInfo() *types.SlaveInfo {
	return &types.SlaveInfo{
		ID:           s.config.ID,
		Type:         s.config.Type,
		Address:      s.config.Address,
		Capabilities: s.getCapabilities(),
		Labels:       s.config.Labels,
		Resources: &types.ResourceInfo{
			CPUCores:    s.config.CPUCores,
			MemoryMB:    s.config.MemoryMB,
			MaxVUs:      s.config.MaxVUs,
			CurrentLoad: s.currentLoad.Load().(float64),
		},
	}
}

// getCapabilities returns the capabilities of this slave.
// Requirements: 12.1
func (s *WorkerSlave) getCapabilities() []string {
	// Start with configured capabilities
	caps := make([]string, len(s.config.Capabilities))
	copy(caps, s.config.Capabilities)

	// Add capabilities based on registered executors
	if s.registry != nil {
		for _, execType := range s.registry.Types() {
			caps = append(caps, execType+"_executor")
		}
	}

	return caps
}

// getMetrics returns the current slave metrics.
func (s *WorkerSlave) getMetrics() *types.SlaveMetrics {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.taskEngine != nil {
		return s.taskEngine.GetMetrics()
	}

	return &types.SlaveMetrics{}
}

// heartbeatLoop sends periodic heartbeats to the master.
// Requirements: 12.4
func (s *WorkerSlave) heartbeatLoop() {
	ticker := time.NewTicker(s.config.HeartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.heartbeatCtx.Done():
			return
		case <-ticker.C:
			s.sendHeartbeat()
		}
	}
}

// sendHeartbeat sends a heartbeat to the master.
// Requirements: 12.4
func (s *WorkerSlave) sendHeartbeat() {
	s.lastHeartbeat.Store(time.Now())

	// Update load metrics
	s.updateLoad()

	// In a real implementation, this would send the heartbeat via gRPC
	// For now, we just update the local state
}

// updateLoad updates the current load metrics.
func (s *WorkerSlave) updateLoad() {
	// Calculate load based on active tasks and max VUs
	if s.config.MaxVUs > 0 {
		load := float64(s.activeTasks.Load()) / float64(s.config.MaxVUs) * 100
		s.currentLoad.Store(load)
	}
}

// IsConnected returns whether the slave is connected to a master.
func (s *WorkerSlave) IsConnected() bool {
	return s.connected.Load()
}

// GetMasterAddress returns the address of the connected master.
func (s *WorkerSlave) GetMasterAddress() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.masterAddr
}
