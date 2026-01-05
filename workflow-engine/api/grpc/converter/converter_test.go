package converter

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pb "github.com/grafana/k6/workflow-engine/api/grpc/proto"
	"github.com/grafana/k6/workflow-engine/pkg/types"
)

func TestSlaveInfoToProto(t *testing.T) {
	info := &types.SlaveInfo{
		ID:           "slave-1",
		Type:         types.SlaveTypeWorker,
		Address:      "localhost:9091",
		Capabilities: []string{"http_executor", "script_executor"},
		Labels:       map[string]string{"region": "us-east"},
		Resources: &types.ResourceInfo{
			CPUCores:    4,
			MemoryMB:    8192,
			MaxVUs:      100,
			CurrentLoad: 25.5,
		},
	}

	req := SlaveInfoToProto(info)

	assert.Equal(t, "slave-1", req.SlaveId)
	assert.Equal(t, "worker", req.SlaveType)
	assert.Equal(t, "localhost:9091", req.Address)
	assert.Equal(t, []string{"http_executor", "script_executor"}, req.Capabilities)
	assert.Equal(t, "us-east", req.Labels["region"])
	assert.Equal(t, int32(4), req.Resources.CpuCores)
	assert.Equal(t, int64(8192), req.Resources.MemoryMb)
	assert.Equal(t, int32(100), req.Resources.MaxVus)
	assert.Equal(t, 25.5, req.Resources.CurrentLoad)
}

func TestSlaveInfoToProtoNil(t *testing.T) {
	req := SlaveInfoToProto(nil)
	assert.Nil(t, req)
}

func TestProtoToSlaveInfo(t *testing.T) {
	req := &pb.RegisterRequest{
		SlaveId:      "slave-1",
		SlaveType:    "worker",
		Address:      "localhost:9091",
		Capabilities: []string{"http_executor"},
		Labels:       map[string]string{"env": "prod"},
		Resources: &pb.ResourceInfo{
			CpuCores:    8,
			MemoryMb:    16384,
			MaxVus:      200,
			CurrentLoad: 50.0,
		},
	}

	info := ProtoToSlaveInfo(req)

	assert.Equal(t, "slave-1", info.ID)
	assert.Equal(t, types.SlaveTypeWorker, info.Type)
	assert.Equal(t, "localhost:9091", info.Address)
	assert.Equal(t, []string{"http_executor"}, info.Capabilities)
	assert.Equal(t, "prod", info.Labels["env"])
	assert.Equal(t, 8, info.Resources.CPUCores)
	assert.Equal(t, int64(16384), info.Resources.MemoryMB)
	assert.Equal(t, 200, info.Resources.MaxVUs)
	assert.Equal(t, 50.0, info.Resources.CurrentLoad)
}

func TestProtoToSlaveInfoNil(t *testing.T) {
	info := ProtoToSlaveInfo(nil)
	assert.Nil(t, info)
}

func TestSlaveStatusRoundTrip(t *testing.T) {
	original := &types.SlaveStatus{
		State:       types.SlaveStateOnline,
		Load:        75.5,
		ActiveTasks: 10,
		LastSeen:    time.Now().Truncate(time.Nanosecond),
		Metrics: &types.SlaveMetrics{
			CPUUsage:    50.0,
			MemoryUsage: 60.0,
			ActiveVUs:   50,
			Throughput:  1000.0,
		},
	}

	pbStatus := SlaveStatusToProto(original)
	result := ProtoToSlaveStatus(pbStatus)

	assert.Equal(t, original.State, result.State)
	assert.Equal(t, original.Load, result.Load)
	assert.Equal(t, original.ActiveTasks, result.ActiveTasks)
	assert.Equal(t, original.Metrics.CPUUsage, result.Metrics.CPUUsage)
	assert.Equal(t, original.Metrics.MemoryUsage, result.Metrics.MemoryUsage)
	assert.Equal(t, original.Metrics.ActiveVUs, result.Metrics.ActiveVUs)
	assert.Equal(t, original.Metrics.Throughput, result.Metrics.Throughput)
}

func TestTaskRoundTrip(t *testing.T) {
	original := &types.Task{
		ID:          "task-1",
		ExecutionID: "exec-1",
		Workflow: &types.Workflow{
			ID:   "workflow-1",
			Name: "Test Workflow",
			Steps: []types.Step{
				{
					ID:   "step-1",
					Name: "HTTP Request",
					Type: "http",
				},
			},
		},
		Segment: types.ExecutionSegment{
			Start: 0.0,
			End:   0.5,
		},
	}

	pbTask, err := TaskToProto(original)
	require.NoError(t, err)

	result, err := ProtoToTask(pbTask)
	require.NoError(t, err)

	assert.Equal(t, original.ID, result.ID)
	assert.Equal(t, original.ExecutionID, result.ExecutionID)
	assert.Equal(t, original.Segment.Start, result.Segment.Start)
	assert.Equal(t, original.Segment.End, result.Segment.End)
	assert.Equal(t, original.Workflow.ID, result.Workflow.ID)
	assert.Equal(t, original.Workflow.Name, result.Workflow.Name)
}

func TestTaskToProtoNil(t *testing.T) {
	pbTask, err := TaskToProto(nil)
	assert.NoError(t, err)
	assert.Nil(t, pbTask)
}

func TestProtoToTaskNil(t *testing.T) {
	task, err := ProtoToTask(nil)
	assert.NoError(t, err)
	assert.Nil(t, task)
}

func TestExecutionOptionsRoundTrip(t *testing.T) {
	original := &types.ExecutionOptions{
		VUs:           100,
		Duration:      5 * time.Minute,
		Iterations:    1000,
		ExecutionMode: types.ModeConstantVUs,
		Stages: []types.Stage{
			{Duration: 1 * time.Minute, Target: 50, Name: "ramp-up"},
			{Duration: 3 * time.Minute, Target: 100, Name: "steady"},
			{Duration: 1 * time.Minute, Target: 0, Name: "ramp-down"},
		},
	}

	pbOpts := ExecutionOptionsToProto(original)
	result := ProtoToExecutionOptions(pbOpts)

	assert.Equal(t, original.VUs, result.VUs)
	assert.Equal(t, original.Duration, result.Duration)
	assert.Equal(t, original.Iterations, result.Iterations)
	assert.Equal(t, original.ExecutionMode, result.ExecutionMode)
	assert.Len(t, result.Stages, 3)
	assert.Equal(t, original.Stages[0].Duration, result.Stages[0].Duration)
	assert.Equal(t, original.Stages[0].Target, result.Stages[0].Target)
	assert.Equal(t, original.Stages[0].Name, result.Stages[0].Name)
}

func TestTaskResultRoundTrip(t *testing.T) {
	original := &types.TaskResult{
		TaskID:      "task-1",
		ExecutionID: "exec-1",
		SlaveID:     "slave-1",
		Status:      types.ExecutionStatusCompleted,
		Errors: []types.ExecutionError{
			{
				Code:      types.ErrCodeExecution,
				Message:   "test error",
				StepID:    "step-1",
				Timestamp: time.Now().Truncate(time.Nanosecond),
			},
		},
	}

	pbUpdate, err := TaskResultToProto(original)
	require.NoError(t, err)

	result, err := ProtoToTaskResult(pbUpdate, "slave-1")
	require.NoError(t, err)

	assert.Equal(t, original.TaskID, result.TaskID)
	assert.Equal(t, original.ExecutionID, result.ExecutionID)
	assert.Equal(t, original.SlaveID, result.SlaveID)
	assert.Equal(t, original.Status, result.Status)
	assert.Len(t, result.Errors, 1)
	assert.Equal(t, original.Errors[0].Code, result.Errors[0].Code)
	assert.Equal(t, original.Errors[0].Message, result.Errors[0].Message)
	assert.Equal(t, original.Errors[0].StepID, result.Errors[0].StepID)
}

func TestStepMetricsRoundTrip(t *testing.T) {
	original := &types.StepMetrics{
		StepID:       "step-1",
		Count:        1000,
		SuccessCount: 990,
		FailureCount: 10,
		Duration: &types.DurationMetrics{
			Min: 10 * time.Millisecond,
			Max: 500 * time.Millisecond,
			Avg: 100 * time.Millisecond,
			P50: 80 * time.Millisecond,
			P90: 200 * time.Millisecond,
			P95: 300 * time.Millisecond,
			P99: 450 * time.Millisecond,
		},
		CustomMetrics: map[string]float64{
			"custom_metric": 42.0,
		},
	}

	pbMetrics := StepMetricsToProto(original)
	result := ProtoToStepMetrics(pbMetrics)

	assert.Equal(t, original.StepID, result.StepID)
	assert.Equal(t, original.Count, result.Count)
	assert.Equal(t, original.SuccessCount, result.SuccessCount)
	assert.Equal(t, original.FailureCount, result.FailureCount)
	assert.Equal(t, original.Duration.Min, result.Duration.Min)
	assert.Equal(t, original.Duration.Max, result.Duration.Max)
	assert.Equal(t, original.Duration.Avg, result.Duration.Avg)
	assert.Equal(t, original.Duration.P50, result.Duration.P50)
	assert.Equal(t, original.Duration.P90, result.Duration.P90)
	assert.Equal(t, original.Duration.P95, result.Duration.P95)
	assert.Equal(t, original.Duration.P99, result.Duration.P99)
	assert.Equal(t, 42.0, result.CustomMetrics["custom_metric"])
}

func TestExecutionStatusConversion(t *testing.T) {
	testCases := []struct {
		status   types.ExecutionStatus
		expected pb.TaskStatus
	}{
		{types.ExecutionStatusPending, pb.TaskStatus_TASK_STATUS_PENDING},
		{types.ExecutionStatusRunning, pb.TaskStatus_TASK_STATUS_RUNNING},
		{types.ExecutionStatusPaused, pb.TaskStatus_TASK_STATUS_PAUSED},
		{types.ExecutionStatusCompleted, pb.TaskStatus_TASK_STATUS_COMPLETED},
		{types.ExecutionStatusFailed, pb.TaskStatus_TASK_STATUS_FAILED},
		{types.ExecutionStatusAborted, pb.TaskStatus_TASK_STATUS_ABORTED},
	}

	for _, tc := range testCases {
		pbStatus := ExecutionStatusToProtoTaskStatus(tc.status)
		assert.Equal(t, tc.expected, pbStatus)

		result := ProtoTaskStatusToExecutionStatus(pbStatus)
		assert.Equal(t, tc.status, result)
	}
}

func TestCommandTypeConversion(t *testing.T) {
	testCases := []struct {
		cmdType  string
		expected pb.CommandType
	}{
		{"stop", pb.CommandType_COMMAND_STOP},
		{"pause", pb.CommandType_COMMAND_PAUSE},
		{"resume", pb.CommandType_COMMAND_RESUME},
		{"scale", pb.CommandType_COMMAND_SCALE},
		{"unknown", pb.CommandType_COMMAND_UNKNOWN},
	}

	for _, tc := range testCases {
		pbCmd := CommandTypeToProto(tc.cmdType)
		assert.Equal(t, tc.expected, pbCmd)

		result := ProtoToCommandType(pbCmd)
		if tc.cmdType == "unknown" {
			assert.Equal(t, "unknown", result)
		} else {
			assert.Equal(t, tc.cmdType, result)
		}
	}
}
