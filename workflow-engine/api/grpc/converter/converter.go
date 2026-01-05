// Package converter provides conversion functions between protobuf and internal types.
package converter

import (
	"encoding/json"
	"time"

	pb "yqhp/workflow-engine/api/grpc/proto"
	"yqhp/workflow-engine/pkg/types"
)

// SlaveInfoToProto converts types.SlaveInfo to protobuf RegisterRequest.
func SlaveInfoToProto(info *types.SlaveInfo) *pb.RegisterRequest {
	if info == nil {
		return nil
	}

	req := &pb.RegisterRequest{
		SlaveId:      info.ID,
		SlaveType:    string(info.Type),
		Capabilities: info.Capabilities,
		Labels:       info.Labels,
		Address:      info.Address,
	}

	if info.Resources != nil {
		req.Resources = &pb.ResourceInfo{
			CpuCores:    int32(info.Resources.CPUCores),
			MemoryMb:    info.Resources.MemoryMB,
			MaxVus:      int32(info.Resources.MaxVUs),
			CurrentLoad: info.Resources.CurrentLoad,
		}
	}

	return req
}

// ProtoToSlaveInfo converts protobuf RegisterRequest to types.SlaveInfo.
func ProtoToSlaveInfo(req *pb.RegisterRequest) *types.SlaveInfo {
	if req == nil {
		return nil
	}

	info := &types.SlaveInfo{
		ID:           req.SlaveId,
		Type:         types.SlaveType(req.SlaveType),
		Address:      req.Address,
		Capabilities: req.Capabilities,
		Labels:       req.Labels,
	}

	if req.Resources != nil {
		info.Resources = &types.ResourceInfo{
			CPUCores:    int(req.Resources.CpuCores),
			MemoryMB:    req.Resources.MemoryMb,
			MaxVUs:      int(req.Resources.MaxVus),
			CurrentLoad: req.Resources.CurrentLoad,
		}
	}

	return info
}

// SlaveStatusToProto converts types.SlaveStatus to protobuf SlaveStatus.
func SlaveStatusToProto(status *types.SlaveStatus) *pb.SlaveStatus {
	if status == nil {
		return nil
	}

	pbStatus := &pb.SlaveStatus{
		State:       string(status.State),
		Load:        status.Load,
		ActiveTasks: int32(status.ActiveTasks),
		LastSeen:    status.LastSeen.UnixNano(),
	}

	if status.Metrics != nil {
		pbStatus.Metrics = &pb.SlaveMetrics{
			CpuUsage:    status.Metrics.CPUUsage,
			MemoryUsage: status.Metrics.MemoryUsage,
			ActiveVus:   int32(status.Metrics.ActiveVUs),
			Throughput:  status.Metrics.Throughput,
		}
	}

	return pbStatus
}

// ProtoToSlaveStatus converts protobuf SlaveStatus to types.SlaveStatus.
func ProtoToSlaveStatus(pbStatus *pb.SlaveStatus) *types.SlaveStatus {
	if pbStatus == nil {
		return nil
	}

	status := &types.SlaveStatus{
		State:       types.SlaveState(pbStatus.State),
		Load:        pbStatus.Load,
		ActiveTasks: int(pbStatus.ActiveTasks),
		LastSeen:    time.Unix(0, pbStatus.LastSeen),
	}

	if pbStatus.Metrics != nil {
		status.Metrics = &types.SlaveMetrics{
			CPUUsage:    pbStatus.Metrics.CpuUsage,
			MemoryUsage: pbStatus.Metrics.MemoryUsage,
			ActiveVUs:   int(pbStatus.Metrics.ActiveVus),
			Throughput:  pbStatus.Metrics.Throughput,
		}
	}

	return status
}

// TaskToProto converts types.Task to protobuf TaskAssignment.
func TaskToProto(task *types.Task) (*pb.TaskAssignment, error) {
	if task == nil {
		return nil, nil
	}

	// Serialize workflow to JSON
	var workflowData []byte
	var err error
	if task.Workflow != nil {
		workflowData, err = json.Marshal(task.Workflow)
		if err != nil {
			return nil, err
		}
	}

	assignment := &pb.TaskAssignment{
		TaskId:       task.ID,
		ExecutionId:  task.ExecutionID,
		WorkflowData: workflowData,
		Segment: &pb.ExecutionSegment{
			Start: task.Segment.Start,
			End:   task.Segment.End,
		},
	}

	// Add execution options if workflow has them
	if task.Workflow != nil {
		assignment.Options = ExecutionOptionsToProto(&task.Workflow.Options)
	}

	return assignment, nil
}

// ProtoToTask converts protobuf TaskAssignment to types.Task.
func ProtoToTask(assignment *pb.TaskAssignment) (*types.Task, error) {
	if assignment == nil {
		return nil, nil
	}

	task := &types.Task{
		ID:          assignment.TaskId,
		ExecutionID: assignment.ExecutionId,
	}

	if assignment.Segment != nil {
		task.Segment = types.ExecutionSegment{
			Start: assignment.Segment.Start,
			End:   assignment.Segment.End,
		}
	}

	// Deserialize workflow from JSON
	if len(assignment.WorkflowData) > 0 {
		task.Workflow = &types.Workflow{}
		if err := json.Unmarshal(assignment.WorkflowData, task.Workflow); err != nil {
			return nil, err
		}
	}

	return task, nil
}

// ExecutionOptionsToProto converts types.ExecutionOptions to protobuf ExecutionOptions.
func ExecutionOptionsToProto(opts *types.ExecutionOptions) *pb.ExecutionOptions {
	if opts == nil {
		return nil
	}

	pbOpts := &pb.ExecutionOptions{
		Vus:           int32(opts.VUs),
		DurationMs:    opts.Duration.Milliseconds(),
		Iterations:    int32(opts.Iterations),
		ExecutionMode: string(opts.ExecutionMode),
	}

	for _, stage := range opts.Stages {
		pbOpts.Stages = append(pbOpts.Stages, &pb.Stage{
			DurationMs: stage.Duration.Milliseconds(),
			Target:     int32(stage.Target),
			Name:       stage.Name,
		})
	}

	return pbOpts
}

// ProtoToExecutionOptions converts protobuf ExecutionOptions to types.ExecutionOptions.
func ProtoToExecutionOptions(pbOpts *pb.ExecutionOptions) *types.ExecutionOptions {
	if pbOpts == nil {
		return nil
	}

	opts := &types.ExecutionOptions{
		VUs:           int(pbOpts.Vus),
		Duration:      time.Duration(pbOpts.DurationMs) * time.Millisecond,
		Iterations:    int(pbOpts.Iterations),
		ExecutionMode: types.ExecutionMode(pbOpts.ExecutionMode),
	}

	for _, pbStage := range pbOpts.Stages {
		opts.Stages = append(opts.Stages, types.Stage{
			Duration: time.Duration(pbStage.DurationMs) * time.Millisecond,
			Target:   int(pbStage.Target),
			Name:     pbStage.Name,
		})
	}

	return opts
}

// TaskResultToProto converts types.TaskResult to protobuf TaskUpdate.
func TaskResultToProto(result *types.TaskResult) (*pb.TaskUpdate, error) {
	if result == nil {
		return nil, nil
	}

	// Serialize metrics to JSON
	var resultData []byte
	var err error
	if result.Metrics != nil {
		resultData, err = json.Marshal(result.Metrics)
		if err != nil {
			return nil, err
		}
	}

	update := &pb.TaskUpdate{
		TaskId:      result.TaskID,
		ExecutionId: result.ExecutionID,
		Status:      ExecutionStatusToProtoTaskStatus(result.Status),
		ResultData:  resultData,
	}

	for _, execErr := range result.Errors {
		update.Errors = append(update.Errors, &pb.ExecutionError{
			Code:      string(execErr.Code),
			Message:   execErr.Message,
			StepId:    execErr.StepID,
			Timestamp: execErr.Timestamp.UnixNano(),
		})
	}

	return update, nil
}

// ProtoToTaskResult converts protobuf TaskUpdate to types.TaskResult.
func ProtoToTaskResult(update *pb.TaskUpdate, slaveID string) (*types.TaskResult, error) {
	if update == nil {
		return nil, nil
	}

	result := &types.TaskResult{
		TaskID:      update.TaskId,
		ExecutionID: update.ExecutionId,
		SlaveID:     slaveID,
		Status:      ProtoTaskStatusToExecutionStatus(update.Status),
	}

	// Deserialize metrics from JSON
	if len(update.ResultData) > 0 {
		result.Metrics = &types.Metrics{}
		if err := json.Unmarshal(update.ResultData, result.Metrics); err != nil {
			return nil, err
		}
	}

	for _, pbErr := range update.Errors {
		result.Errors = append(result.Errors, types.ExecutionError{
			Code:      types.ErrorCode(pbErr.Code),
			Message:   pbErr.Message,
			StepID:    pbErr.StepId,
			Timestamp: time.Unix(0, pbErr.Timestamp),
		})
	}

	return result, nil
}

// MetricsToProto converts types.Metrics to protobuf MetricsReport.
func MetricsToProto(slaveID, executionID string, metrics *types.Metrics) (*pb.MetricsReport, error) {
	if metrics == nil {
		return nil, nil
	}

	// Serialize full metrics to JSON
	metricsData, err := json.Marshal(metrics)
	if err != nil {
		return nil, err
	}

	report := &pb.MetricsReport{
		SlaveId:     slaveID,
		ExecutionId: executionID,
		Timestamp:   metrics.Timestamp.UnixNano(),
		MetricsData: metricsData,
		StepMetrics: make(map[string]*pb.StepMetrics),
	}

	for stepID, stepMetrics := range metrics.StepMetrics {
		report.StepMetrics[stepID] = StepMetricsToProto(stepMetrics)
	}

	return report, nil
}

// ProtoToMetrics converts protobuf MetricsReport to types.Metrics.
func ProtoToMetrics(report *pb.MetricsReport) (*types.Metrics, error) {
	if report == nil {
		return nil, nil
	}

	// Try to deserialize full metrics from JSON first
	if len(report.MetricsData) > 0 {
		metrics := &types.Metrics{}
		if err := json.Unmarshal(report.MetricsData, metrics); err != nil {
			return nil, err
		}
		return metrics, nil
	}

	// Otherwise, build from step metrics
	metrics := &types.Metrics{
		Timestamp:   time.Unix(0, report.Timestamp),
		StepMetrics: make(map[string]*types.StepMetrics),
	}

	for stepID, pbStepMetrics := range report.StepMetrics {
		metrics.StepMetrics[stepID] = ProtoToStepMetrics(pbStepMetrics)
	}

	return metrics, nil
}

// StepMetricsToProto converts types.StepMetrics to protobuf StepMetrics.
func StepMetricsToProto(metrics *types.StepMetrics) *pb.StepMetrics {
	if metrics == nil {
		return nil
	}

	pbMetrics := &pb.StepMetrics{
		StepId:        metrics.StepID,
		Count:         metrics.Count,
		SuccessCount:  metrics.SuccessCount,
		FailureCount:  metrics.FailureCount,
		CustomMetrics: metrics.CustomMetrics,
	}

	if metrics.Duration != nil {
		pbMetrics.Duration = &pb.DurationMetrics{
			MinNs: int64(metrics.Duration.Min),
			MaxNs: int64(metrics.Duration.Max),
			AvgNs: int64(metrics.Duration.Avg),
			P50Ns: int64(metrics.Duration.P50),
			P90Ns: int64(metrics.Duration.P90),
			P95Ns: int64(metrics.Duration.P95),
			P99Ns: int64(metrics.Duration.P99),
		}
	}

	return pbMetrics
}

// ProtoToStepMetrics converts protobuf StepMetrics to types.StepMetrics.
func ProtoToStepMetrics(pbMetrics *pb.StepMetrics) *types.StepMetrics {
	if pbMetrics == nil {
		return nil
	}

	metrics := &types.StepMetrics{
		StepID:        pbMetrics.StepId,
		Count:         pbMetrics.Count,
		SuccessCount:  pbMetrics.SuccessCount,
		FailureCount:  pbMetrics.FailureCount,
		CustomMetrics: pbMetrics.CustomMetrics,
	}

	if pbMetrics.Duration != nil {
		metrics.Duration = &types.DurationMetrics{
			Min: time.Duration(pbMetrics.Duration.MinNs),
			Max: time.Duration(pbMetrics.Duration.MaxNs),
			Avg: time.Duration(pbMetrics.Duration.AvgNs),
			P50: time.Duration(pbMetrics.Duration.P50Ns),
			P90: time.Duration(pbMetrics.Duration.P90Ns),
			P95: time.Duration(pbMetrics.Duration.P95Ns),
			P99: time.Duration(pbMetrics.Duration.P99Ns),
		}
	}

	return metrics
}

// ExecutionStatusToProtoTaskStatus converts types.ExecutionStatus to protobuf TaskStatus.
func ExecutionStatusToProtoTaskStatus(status types.ExecutionStatus) pb.TaskStatus {
	switch status {
	case types.ExecutionStatusPending:
		return pb.TaskStatus_TASK_STATUS_PENDING
	case types.ExecutionStatusRunning:
		return pb.TaskStatus_TASK_STATUS_RUNNING
	case types.ExecutionStatusPaused:
		return pb.TaskStatus_TASK_STATUS_PAUSED
	case types.ExecutionStatusCompleted:
		return pb.TaskStatus_TASK_STATUS_COMPLETED
	case types.ExecutionStatusFailed:
		return pb.TaskStatus_TASK_STATUS_FAILED
	case types.ExecutionStatusAborted:
		return pb.TaskStatus_TASK_STATUS_ABORTED
	default:
		return pb.TaskStatus_TASK_STATUS_UNKNOWN
	}
}

// ProtoTaskStatusToExecutionStatus converts protobuf TaskStatus to types.ExecutionStatus.
func ProtoTaskStatusToExecutionStatus(status pb.TaskStatus) types.ExecutionStatus {
	switch status {
	case pb.TaskStatus_TASK_STATUS_PENDING:
		return types.ExecutionStatusPending
	case pb.TaskStatus_TASK_STATUS_RUNNING:
		return types.ExecutionStatusRunning
	case pb.TaskStatus_TASK_STATUS_PAUSED:
		return types.ExecutionStatusPaused
	case pb.TaskStatus_TASK_STATUS_COMPLETED:
		return types.ExecutionStatusCompleted
	case pb.TaskStatus_TASK_STATUS_FAILED:
		return types.ExecutionStatusFailed
	case pb.TaskStatus_TASK_STATUS_ABORTED:
		return types.ExecutionStatusAborted
	default:
		return types.ExecutionStatusPending
	}
}

// CommandTypeToProto converts a string command type to protobuf CommandType.
func CommandTypeToProto(cmdType string) pb.CommandType {
	switch cmdType {
	case "stop":
		return pb.CommandType_COMMAND_STOP
	case "pause":
		return pb.CommandType_COMMAND_PAUSE
	case "resume":
		return pb.CommandType_COMMAND_RESUME
	case "scale":
		return pb.CommandType_COMMAND_SCALE
	default:
		return pb.CommandType_COMMAND_UNKNOWN
	}
}

// ProtoToCommandType converts protobuf CommandType to string.
func ProtoToCommandType(cmdType pb.CommandType) string {
	switch cmdType {
	case pb.CommandType_COMMAND_STOP:
		return "stop"
	case pb.CommandType_COMMAND_PAUSE:
		return "pause"
	case pb.CommandType_COMMAND_RESUME:
		return "resume"
	case pb.CommandType_COMMAND_SCALE:
		return "scale"
	default:
		return "unknown"
	}
}
