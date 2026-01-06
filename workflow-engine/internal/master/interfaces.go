package master

import (
	"context"

	"yqhp/workflow-engine/pkg/types"
)

// SlaveRegistry 管理 Slave 的注册和状态。
// Requirements: 5.2, 12.2, 13.1
type SlaveRegistry interface {
	// Register 注册一个新的 Slave。
	Register(ctx context.Context, slave *types.SlaveInfo) error

	// Unregister 注销一个 Slave。
	Unregister(ctx context.Context, slaveID string) error

	// UpdateStatus 更新 Slave 的状态。
	UpdateStatus(ctx context.Context, slaveID string, status *types.SlaveStatus) error

	// GetSlave 返回单个 Slave 的信息。
	GetSlave(ctx context.Context, slaveID string) (*types.SlaveInfo, error)

	// GetSlaveStatus 返回 Slave 的当前状态。
	GetSlaveStatus(ctx context.Context, slaveID string) (*types.SlaveStatus, error)

	// ListSlaves 列出所有匹配过滤条件的 Slave。
	ListSlaves(ctx context.Context, filter *SlaveFilter) ([]*types.SlaveInfo, error)

	// GetOnlineSlaves 返回所有在线的 Slave。
	GetOnlineSlaves(ctx context.Context) ([]*types.SlaveInfo, error)

	// WatchSlaves 监听 Slave 事件。
	WatchSlaves(ctx context.Context) (<-chan *types.SlaveEvent, error)
}

// SlaveFilter 定义 Slave 过滤条件。
type SlaveFilter struct {
	Types        []types.SlaveType  // 按类型过滤
	Labels       map[string]string  // 按标签过滤
	Capabilities []string           // 按能力过滤
	States       []types.SlaveState // 按状态过滤
}

// Scheduler 处理任务分发到 Slave。
// Requirements: 5.3, 13.1, 13.2, 13.3
type Scheduler interface {
	// Schedule 将工作流执行分发到 Slave。
	Schedule(ctx context.Context, workflow *types.Workflow, slaves []*types.SlaveInfo) (*types.ExecutionPlan, error)

	// Reschedule 处理 Slave 故障时的任务重新分配。
	Reschedule(ctx context.Context, failedSlaveID string, plan *types.ExecutionPlan) (*types.ExecutionPlan, error)

	// SelectSlaves 根据选择策略选择 Slave。
	SelectSlaves(ctx context.Context, selector *types.SlaveSelector) ([]*types.SlaveInfo, error)
}

// MetricsAggregator 聚合来自多个 Slave 的指标。
// Requirements: 5.4, 5.6, 6.6
type MetricsAggregator interface {
	// Aggregate 聚合来自多个 Slave 的指标。
	Aggregate(ctx context.Context, executionID string, slaveMetrics []*types.Metrics) (*types.AggregatedMetrics, error)

	// EvaluateThresholds 根据聚合指标评估阈值。
	EvaluateThresholds(ctx context.Context, metrics *types.AggregatedMetrics, thresholds []types.Threshold) ([]types.ThresholdResult, error)

	// GenerateSummary 生成摘要报告。
	GenerateSummary(ctx context.Context, metrics *types.AggregatedMetrics) (*ExecutionSummary, error)
}

// ExecutionSummary 包含执行的摘要信息。
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
