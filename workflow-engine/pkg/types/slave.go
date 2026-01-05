package types

import "time"

// SlaveSelector defines slave selection strategy.
type SlaveSelector struct {
	Mode         SelectionMode     `yaml:"mode"`
	SlaveIDs     []string          `yaml:"slave_ids,omitempty"`    // Manual mode
	Labels       map[string]string `yaml:"labels,omitempty"`       // Label mode
	Capabilities []string          `yaml:"capabilities,omitempty"` // Capability mode
	MinSlaves    int               `yaml:"min_slaves,omitempty"`   // Auto mode
	MaxSlaves    int               `yaml:"max_slaves,omitempty"`   // Auto mode
}

// SelectionMode defines how slaves are selected.
type SelectionMode string

const (
	// SelectionModeManual specifies slaves by ID.
	SelectionModeManual SelectionMode = "manual"
	// SelectionModeLabel selects slaves by labels.
	SelectionModeLabel SelectionMode = "label"
	// SelectionModeCapability selects slaves by capabilities.
	SelectionModeCapability SelectionMode = "capability"
	// SelectionModeAuto uses automatic load balancing.
	SelectionModeAuto SelectionMode = "auto"
)

// ExecutionSegment defines a portion of the total execution.
type ExecutionSegment struct {
	Start float64 `yaml:"start"` // 0.0 to 1.0
	End   float64 `yaml:"end"`   // 0.0 to 1.0
}

// SlaveType defines the type of slave node.
type SlaveType string

const (
	// SlaveTypeWorker executes workflows.
	SlaveTypeWorker SlaveType = "worker"
	// SlaveTypeGateway acts as API gateway.
	SlaveTypeGateway SlaveType = "gateway"
	// SlaveTypeAggregator aggregates results.
	SlaveTypeAggregator SlaveType = "aggregator"
)

// SlaveInfo contains slave registration information.
type SlaveInfo struct {
	ID           string
	Type         SlaveType
	Address      string
	Capabilities []string
	Labels       map[string]string
	Resources    *ResourceInfo
}

// ResourceInfo contains resource information.
type ResourceInfo struct {
	CPUCores    int   `yaml:"cpu_cores"`
	MemoryMB    int64 `yaml:"memory_mb"`
	MaxVUs      int   `yaml:"max_vus"`
	CurrentLoad float64
}

// SlaveStatus represents the current status of a slave.
type SlaveStatus struct {
	State       SlaveState
	Load        float64       // CPU/memory load 0-100
	ActiveTasks int           // Current active task count
	LastSeen    time.Time     // Last heartbeat time
	Metrics     *SlaveMetrics // Node metrics
}

// SlaveState represents the state of a slave node.
type SlaveState string

const (
	// SlaveStateOnline indicates the slave is online.
	SlaveStateOnline SlaveState = "online"
	// SlaveStateOffline indicates the slave is offline.
	SlaveStateOffline SlaveState = "offline"
	// SlaveStateBusy indicates the slave is busy.
	SlaveStateBusy SlaveState = "busy"
	// SlaveStateDraining indicates the slave is draining.
	SlaveStateDraining SlaveState = "draining"
	// SlaveStateMaintenance indicates the slave is under maintenance.
	SlaveStateMaintenance SlaveState = "maintenance"
)

// SlaveMetrics contains metrics for a slave node.
type SlaveMetrics struct {
	CPUUsage    float64
	MemoryUsage float64
	ActiveVUs   int
	Throughput  float64
}

// SlaveEvent represents a slave lifecycle event.
type SlaveEvent struct {
	Type    SlaveEventType
	SlaveID string
	Slave   *SlaveInfo
}

// SlaveEventType defines the type of slave event.
type SlaveEventType string

const (
	// SlaveEventRegistered indicates a slave was registered.
	SlaveEventRegistered SlaveEventType = "registered"
	// SlaveEventUnregistered indicates a slave was unregistered.
	SlaveEventUnregistered SlaveEventType = "unregistered"
	// SlaveEventOnline indicates a slave came online.
	SlaveEventOnline SlaveEventType = "online"
	// SlaveEventOffline indicates a slave went offline.
	SlaveEventOffline SlaveEventType = "offline"
	// SlaveEventUpdated indicates a slave was updated.
	SlaveEventUpdated SlaveEventType = "updated"
)
