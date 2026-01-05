package master

import (
	"context"

	"github.com/grafana/k6/workflow-engine/pkg/types"
)

// SlaveRegistry manages slave registration and status.
// Requirements: 5.2, 12.2, 13.1
type SlaveRegistry interface {
	// Register registers a new slave.
	Register(ctx context.Context, slave *types.SlaveInfo) error

	// Unregister unregisters a slave.
	Unregister(ctx context.Context, slaveID string) error

	// UpdateStatus updates a slave's status.
	UpdateStatus(ctx context.Context, slaveID string, status *types.SlaveStatus) error

	// GetSlave returns a single slave's information.
	GetSlave(ctx context.Context, slaveID string) (*types.SlaveInfo, error)

	// GetSlaveStatus returns a slave's current status.
	GetSlaveStatus(ctx context.Context, slaveID string) (*types.SlaveStatus, error)

	// ListSlaves lists all slaves matching the filter.
	ListSlaves(ctx context.Context, filter *SlaveFilter) ([]*types.SlaveInfo, error)

	// GetOnlineSlaves returns all online slaves.
	GetOnlineSlaves(ctx context.Context) ([]*types.SlaveInfo, error)

	// WatchSlaves watches for slave events.
	WatchSlaves(ctx context.Context) (<-chan *types.SlaveEvent, error)
}

// SlaveFilter defines slave filtering criteria.
type SlaveFilter struct {
	Types        []types.SlaveType  // Filter by type
	Labels       map[string]string  // Filter by labels
	Capabilities []string           // Filter by capabilities
	States       []types.SlaveState // Filter by state
}

// Scheduler handles task distribution to slaves.
// Requirements: 5.3, 13.1, 13.2, 13.3
type Scheduler interface {
	// Schedule distributes workflow execution to slaves.
	Schedule(ctx context.Context, workflow *types.Workflow, slaves []*types.SlaveInfo) (*types.ExecutionPlan, error)

	// Reschedule handles task redistribution on slave failure.
	Reschedule(ctx context.Context, failedSlaveID string, plan *types.ExecutionPlan) (*types.ExecutionPlan, error)

	// SelectSlaves selects slaves based on the selection strategy.
	SelectSlaves(ctx context.Context, selector *types.SlaveSelector) ([]*types.SlaveInfo, error)
}

// MetricsAggregator aggregates metrics from multiple slaves.
// Requirements: 5.4, 5.6, 6.6
type MetricsAggregator interface {
	// Aggregate aggregates metrics from multiple slaves.
	Aggregate(ctx context.Context, executionID string, slaveMetrics []*types.Metrics) (*types.AggregatedMetrics, error)

	// EvaluateThresholds evaluates thresholds against aggregated metrics.
	EvaluateThresholds(ctx context.Context, metrics *types.AggregatedMetrics, thresholds []types.Threshold) ([]types.ThresholdResult, error)

	// GenerateSummary generates a summary report.
	GenerateSummary(ctx context.Context, metrics *types.AggregatedMetrics) (*ExecutionSummary, error)
}

// ExecutionSummary contains a summary of the execution.
type ExecutionSummary struct {
	ExecutionID      string
	WorkflowID       string
	Duration         string
	TotalVUs         int
	TotalIterations  int64
	TotalRequests    int64
	SuccessRate      float64
	ErrorRate        float64
	AvgDuration      string
	P95Duration      string
	P99Duration      string
	ThresholdsPassed int
	ThresholdsFailed int
}
