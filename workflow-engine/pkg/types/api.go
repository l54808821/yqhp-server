// Package types provides API request/response types for REST communication.
// These types are used by both api/rest and internal/slave packages.
package types

// ============================================================================
// Slave 通信相关的请求/响应类型
// ============================================================================

// SlaveRegisterRequest 表示 Slave 注册请求
type SlaveRegisterRequest struct {
	SlaveID      string            `json:"slave_id"`
	SlaveType    string            `json:"slave_type"`
	Capabilities []string          `json:"capabilities"`
	Labels       map[string]string `json:"labels,omitempty"`
	Address      string            `json:"address"`
	Resources    *APIResourceInfo  `json:"resources,omitempty"`
}

// SlaveRegisterResponse 表示 Slave 注册响应
type SlaveRegisterResponse struct {
	Accepted          bool   `json:"accepted"`
	AssignedID        string `json:"assigned_id,omitempty"`
	Error             string `json:"error,omitempty"`
	HeartbeatInterval int64  `json:"heartbeat_interval_ms"`
	MasterID          string `json:"master_id,omitempty"`
	Version           string `json:"version,omitempty"`
}

// APIResourceInfo 表示 Slave 资源信息 (API 版本，带 JSON tag)
type APIResourceInfo struct {
	CPUCores    int     `json:"cpu_cores"`
	MemoryMB    int64   `json:"memory_mb"`
	MaxVUs      int     `json:"max_vus"`
	CurrentLoad float64 `json:"current_load"`
}

// SlaveHeartbeatRequest 表示心跳请求
type SlaveHeartbeatRequest struct {
	SlaveID   string              `json:"slave_id"`
	Status    *APISlaveStatusInfo `json:"status"`
	Timestamp int64               `json:"timestamp"`
}

// APISlaveStatusInfo 表示当前 Slave 状态信息 (API 版本)
type APISlaveStatusInfo struct {
	State       string           `json:"state"`
	Load        float64          `json:"load"`
	ActiveTasks int              `json:"active_tasks"`
	LastSeen    int64            `json:"last_seen"`
	Metrics     *APISlaveMetrics `json:"metrics,omitempty"`
}

// APISlaveMetrics 表示 Slave 指标 (API 版本)
type APISlaveMetrics struct {
	CPUUsage    float64 `json:"cpu_usage"`
	MemoryUsage float64 `json:"memory_usage"`
	ActiveVUs   int     `json:"active_vus"`
	Throughput  float64 `json:"throughput"`
}

// SlaveHeartbeatResponse 表示心跳响应
type SlaveHeartbeatResponse struct {
	Commands  []*ControlCommand `json:"commands,omitempty"`
	Timestamp int64             `json:"timestamp"`
}

// ControlCommand 表示来自 Master 的控制命令
type ControlCommand struct {
	Type        string            `json:"type"`
	ExecutionID string            `json:"execution_id"`
	Params      map[string]string `json:"params,omitempty"`
}

// PendingTasksResponse 表示 Slave 的待执行任务响应
type PendingTasksResponse struct {
	Tasks []*TaskAssignment `json:"tasks"`
}

// TaskAssignment 表示分配给 Slave 的任务
type TaskAssignment struct {
	TaskID      string               `json:"task_id"`
	ExecutionID string               `json:"execution_id"`
	Workflow    *Workflow            `json:"workflow"`
	Segment     *APIExecutionSegment `json:"segment,omitempty"`
	Options     *APIExecutionOptions `json:"options,omitempty"`
}

// APIExecutionSegment 定义执行的一部分 (API 版本，带 JSON tag)
type APIExecutionSegment struct {
	Start float64 `json:"start"` // 0.0 到 1.0
	End   float64 `json:"end"`   // 0.0 到 1.0
}

// APIExecutionOptions 定义执行参数 (API 版本，时间用毫秒)
type APIExecutionOptions struct {
	VUs           int         `json:"vus"`
	DurationMs    int64       `json:"duration_ms"`
	Iterations    int         `json:"iterations"`
	ExecutionMode string      `json:"execution_mode"`
	Stages        []*APIStage `json:"stages,omitempty"`
}

// APIStage 定义执行阶段 (API 版本)
type APIStage struct {
	DurationMs int64  `json:"duration_ms"`
	Target     int    `json:"target"`
	Name       string `json:"name,omitempty"`
}

// TaskResultRequest 表示任务结果提交请求
type TaskResultRequest struct {
	TaskID      string                   `json:"task_id"`
	ExecutionID string                   `json:"execution_id"`
	SlaveID     string                   `json:"slave_id"`
	Status      string                   `json:"status"`
	Result      map[string]interface{}   `json:"result,omitempty"`
	Errors      []*ExecutionErrorRequest `json:"errors,omitempty"`
}

// ExecutionErrorRequest 表示执行错误
type ExecutionErrorRequest struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	StepID    string `json:"step_id,omitempty"`
	Timestamp int64  `json:"timestamp"`
}

// TaskResultResponse 表示任务结果响应
type TaskResultResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
}

// MetricsReportRequest 表示指标报告请求
type MetricsReportRequest struct {
	SlaveID     string                      `json:"slave_id"`
	ExecutionID string                      `json:"execution_id"`
	Timestamp   int64                       `json:"timestamp"`
	Metrics     *APIMetricsData             `json:"metrics"`
	StepMetrics map[string]*StepMetricsData `json:"step_metrics,omitempty"`
}

// APIMetricsData 表示指标数据 (API 版本)
type APIMetricsData struct {
	TotalVUs        int   `json:"total_vus"`
	TotalIterations int64 `json:"total_iterations"`
	DurationMs      int64 `json:"duration_ms"`
}

// StepMetricsData 表示步骤指标数据
type StepMetricsData struct {
	StepID        string               `json:"step_id"`
	Count         int64                `json:"count"`
	SuccessCount  int64                `json:"success_count"`
	FailureCount  int64                `json:"failure_count"`
	Duration      *DurationMetricsData `json:"duration,omitempty"`
	CustomMetrics map[string]float64   `json:"custom_metrics,omitempty"`
}

// DurationMetricsData 表示时长指标数据
type DurationMetricsData struct {
	MinNs int64 `json:"min_ns"`
	MaxNs int64 `json:"max_ns"`
	AvgNs int64 `json:"avg_ns"`
	P50Ns int64 `json:"p50_ns"`
	P90Ns int64 `json:"p90_ns"`
	P95Ns int64 `json:"p95_ns"`
	P99Ns int64 `json:"p99_ns"`
}

// MetricsReportResponse 表示指标报告响应
type MetricsReportResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

// SlaveUnregisterRequest 表示 Slave 注销请求
type SlaveUnregisterRequest struct {
	SlaveID string `json:"slave_id"`
	Reason  string `json:"reason,omitempty"`
}

// SlaveUnregisterResponse 表示 Slave 注销响应
type SlaveUnregisterResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
}

// ErrorResponse represents an error response.
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

// HealthResponse represents a health check response.
type HealthResponse struct {
	Status    string `json:"status"`
	Timestamp string `json:"timestamp"`
}
