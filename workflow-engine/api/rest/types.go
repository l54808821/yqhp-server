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
