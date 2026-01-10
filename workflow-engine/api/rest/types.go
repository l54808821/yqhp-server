// Package rest provides the REST API server for the workflow execution engine.
package rest

import (
	"time"

	"yqhp/workflow-engine/pkg/types"
)

// ErrorResponse represents an error response.
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

// SuccessResponse represents a generic success response.
type SuccessResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
}

// HealthResponse represents a health check response.
type HealthResponse struct {
	Status    string `json:"status"`
	Timestamp string `json:"timestamp"`
}

// ReadyResponse represents a readiness check response.
type ReadyResponse struct {
	Ready     bool   `json:"ready"`
	Status    string `json:"status"`
	Timestamp string `json:"timestamp"`
}

// WorkflowSubmitRequest represents a workflow submission request.
type WorkflowSubmitRequest struct {
	Workflow *types.Workflow `json:"workflow"`
	YAML     string          `json:"yaml,omitempty"`
}

// WorkflowSubmitResponse represents a workflow submission response.
type WorkflowSubmitResponse struct {
	ExecutionID string `json:"execution_id"`
	WorkflowID  string `json:"workflow_id"`
	Status      string `json:"status"`
}

// WorkflowResponse represents a workflow response.
type WorkflowResponse struct {
	ID          string                  `json:"id"`
	Name        string                  `json:"name"`
	Description string                  `json:"description,omitempty"`
	Status      string                  `json:"status"`
	CreatedAt   string                  `json:"created_at,omitempty"`
	Options     *types.ExecutionOptions `json:"options,omitempty"`
}

// ExecutionResponse represents an execution status response.
type ExecutionResponse struct {
	ID          string                             `json:"id"`
	WorkflowID  string                             `json:"workflow_id"`
	Status      string                             `json:"status"`
	Progress    float64                            `json:"progress"`
	StartTime   string                             `json:"start_time"`
	EndTime     string                             `json:"end_time,omitempty"`
	SlaveStates map[string]*SlaveExecutionResponse `json:"slave_states,omitempty"`
	Errors      []ExecutionErrorResponse           `json:"errors,omitempty"`
}

// SlaveExecutionResponse represents a slave's execution state.
type SlaveExecutionResponse struct {
	SlaveID        string  `json:"slave_id"`
	Status         string  `json:"status"`
	CompletedVUs   int     `json:"completed_vus"`
	CompletedIters int     `json:"completed_iters"`
	SegmentStart   float64 `json:"segment_start"`
	SegmentEnd     float64 `json:"segment_end"`
}

// ExecutionErrorResponse represents an execution error.
type ExecutionErrorResponse struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	StepID    string `json:"step_id,omitempty"`
	Timestamp string `json:"timestamp"`
}

// ExecutionListResponse represents a list of executions.
type ExecutionListResponse struct {
	Executions []*ExecutionResponse `json:"executions"`
	Total      int                  `json:"total"`
}

// ScaleRequest represents a scale request.
type ScaleRequest struct {
	TargetVUs int `json:"target_vus"`
}

// SlaveResponse represents a slave node response.
type SlaveResponse struct {
	ID           string               `json:"id"`
	Type         string               `json:"type"`
	Address      string               `json:"address"`
	Capabilities []string             `json:"capabilities,omitempty"`
	Labels       map[string]string    `json:"labels,omitempty"`
	Status       *SlaveStatusResponse `json:"status,omitempty"`
}

// SlaveStatusResponse represents a slave's status.
type SlaveStatusResponse struct {
	State       string  `json:"state"`
	Load        float64 `json:"load"`
	ActiveTasks int     `json:"active_tasks"`
	LastSeen    string  `json:"last_seen"`
}

// SlaveListResponse represents a list of slaves.
type SlaveListResponse struct {
	Slaves []*SlaveResponse `json:"slaves"`
	Total  int              `json:"total"`
}

// MetricsResponse represents metrics for an execution.
type MetricsResponse struct {
	ExecutionID     string                          `json:"execution_id"`
	TotalVUs        int                             `json:"total_vus"`
	TotalIterations int64                           `json:"total_iterations"`
	Duration        string                          `json:"duration"`
	StepMetrics     map[string]*StepMetricsResponse `json:"step_metrics,omitempty"`
	Thresholds      []*ThresholdResultResponse      `json:"thresholds,omitempty"`
}

// StepMetricsResponse represents metrics for a step.
type StepMetricsResponse struct {
	StepID       string                   `json:"step_id"`
	Count        int64                    `json:"count"`
	SuccessCount int64                    `json:"success_count"`
	FailureCount int64                    `json:"failure_count"`
	Duration     *DurationMetricsResponse `json:"duration,omitempty"`
}

// DurationMetricsResponse represents duration metrics.
type DurationMetricsResponse struct {
	Min string `json:"min"`
	Max string `json:"max"`
	Avg string `json:"avg"`
	P50 string `json:"p50"`
	P90 string `json:"p90"`
	P95 string `json:"p95"`
	P99 string `json:"p99"`
}

// ThresholdResultResponse represents a threshold evaluation result.
type ThresholdResultResponse struct {
	Metric    string  `json:"metric"`
	Condition string  `json:"condition"`
	Passed    bool    `json:"passed"`
	Value     float64 `json:"value"`
}

// Helper functions for converting types

// formatTime formats a time.Time to RFC3339 string.
func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339)
}

// formatDuration formats a time.Duration to string.
func formatDuration(d time.Duration) string {
	return d.String()
}

// toExecutionResponse converts an ExecutionState to ExecutionResponse.
func toExecutionResponse(state *types.ExecutionState) *ExecutionResponse {
	if state == nil {
		return nil
	}

	resp := &ExecutionResponse{
		ID:         state.ID,
		WorkflowID: state.WorkflowID,
		Status:     string(state.Status),
		Progress:   state.Progress,
		StartTime:  formatTime(state.StartTime),
	}

	if state.EndTime != nil {
		resp.EndTime = formatTime(*state.EndTime)
	}

	if len(state.SlaveStates) > 0 {
		resp.SlaveStates = make(map[string]*SlaveExecutionResponse)
		for id, ss := range state.SlaveStates {
			resp.SlaveStates[id] = &SlaveExecutionResponse{
				SlaveID:        ss.SlaveID,
				Status:         string(ss.Status),
				CompletedVUs:   ss.CompletedVUs,
				CompletedIters: ss.CompletedIters,
				SegmentStart:   ss.Segment.Start,
				SegmentEnd:     ss.Segment.End,
			}
		}
	}

	if len(state.Errors) > 0 {
		resp.Errors = make([]ExecutionErrorResponse, len(state.Errors))
		for i, err := range state.Errors {
			resp.Errors[i] = ExecutionErrorResponse{
				Code:      string(err.Code),
				Message:   err.Message,
				StepID:    err.StepID,
				Timestamp: formatTime(err.Timestamp),
			}
		}
	}

	return resp
}

// toSlaveResponse converts a SlaveInfo to SlaveResponse.
func toSlaveResponse(info *types.SlaveInfo, status *types.SlaveStatus) *SlaveResponse {
	if info == nil {
		return nil
	}

	resp := &SlaveResponse{
		ID:           info.ID,
		Type:         string(info.Type),
		Address:      info.Address,
		Capabilities: info.Capabilities,
		Labels:       info.Labels,
	}

	if status != nil {
		resp.Status = &SlaveStatusResponse{
			State:       string(status.State),
			Load:        status.Load,
			ActiveTasks: status.ActiveTasks,
			LastSeen:    formatTime(status.LastSeen),
		}
	}

	return resp
}

// toMetricsResponse converts AggregatedMetrics to MetricsResponse.
func toMetricsResponse(metrics *types.AggregatedMetrics) *MetricsResponse {
	if metrics == nil {
		return nil
	}

	resp := &MetricsResponse{
		ExecutionID:     metrics.ExecutionID,
		TotalVUs:        metrics.TotalVUs,
		TotalIterations: metrics.TotalIterations,
		Duration:        formatDuration(metrics.Duration),
	}

	if len(metrics.StepMetrics) > 0 {
		resp.StepMetrics = make(map[string]*StepMetricsResponse)
		for id, sm := range metrics.StepMetrics {
			stepResp := &StepMetricsResponse{
				StepID:       sm.StepID,
				Count:        sm.Count,
				SuccessCount: sm.SuccessCount,
				FailureCount: sm.FailureCount,
			}
			if sm.Duration != nil {
				stepResp.Duration = &DurationMetricsResponse{
					Min: formatDuration(sm.Duration.Min),
					Max: formatDuration(sm.Duration.Max),
					Avg: formatDuration(sm.Duration.Avg),
					P50: formatDuration(sm.Duration.P50),
					P90: formatDuration(sm.Duration.P90),
					P95: formatDuration(sm.Duration.P95),
					P99: formatDuration(sm.Duration.P99),
				}
			}
			resp.StepMetrics[id] = stepResp
		}
	}

	if len(metrics.Thresholds) > 0 {
		resp.Thresholds = make([]*ThresholdResultResponse, len(metrics.Thresholds))
		for i, t := range metrics.Thresholds {
			resp.Thresholds[i] = &ThresholdResultResponse{
				Metric:    t.Metric,
				Condition: t.Condition,
				Passed:    t.Passed,
				Value:     t.Value,
			}
		}
	}

	return resp
}

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
	Resources    *ResourceInfo     `json:"resources,omitempty"`
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

// ResourceInfo 表示 Slave 资源信息
type ResourceInfo struct {
	CPUCores    int     `json:"cpu_cores"`
	MemoryMB    int64   `json:"memory_mb"`
	MaxVUs      int     `json:"max_vus"`
	CurrentLoad float64 `json:"current_load"`
}

// SlaveHeartbeatRequest 表示心跳请求
type SlaveHeartbeatRequest struct {
	SlaveID   string           `json:"slave_id"`
	Status    *SlaveStatusInfo `json:"status"`
	Timestamp int64            `json:"timestamp"`
}

// SlaveStatusInfo 表示当前 Slave 状态信息
type SlaveStatusInfo struct {
	State       string        `json:"state"`
	Load        float64       `json:"load"`
	ActiveTasks int           `json:"active_tasks"`
	LastSeen    int64         `json:"last_seen"`
	Metrics     *SlaveMetrics `json:"metrics,omitempty"`
}

// SlaveMetrics 表示 Slave 指标
type SlaveMetrics struct {
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
	TaskID      string            `json:"task_id"`
	ExecutionID string            `json:"execution_id"`
	Workflow    *types.Workflow   `json:"workflow"`
	Segment     *ExecutionSegment `json:"segment,omitempty"`
	Options     *ExecutionOptions `json:"options,omitempty"`
}

// ExecutionSegment 定义执行的一部分
type ExecutionSegment struct {
	Start float64 `json:"start"` // 0.0 到 1.0
	End   float64 `json:"end"`   // 0.0 到 1.0
}

// ExecutionOptions 定义执行参数
type ExecutionOptions struct {
	VUs           int      `json:"vus"`
	DurationMs    int64    `json:"duration_ms"`
	Iterations    int      `json:"iterations"`
	ExecutionMode string   `json:"execution_mode"`
	Stages        []*Stage `json:"stages,omitempty"`
}

// Stage 定义执行阶段
type Stage struct {
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
	Metrics     *MetricsData                `json:"metrics"`
	StepMetrics map[string]*StepMetricsData `json:"step_metrics,omitempty"`
}

// MetricsData 表示指标数据
type MetricsData struct {
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
