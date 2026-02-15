package types

import "time"

// ResultStatus represents the status of a step execution result.
type ResultStatus string

const (
	// ResultStatusSuccess indicates successful execution.
	ResultStatusSuccess ResultStatus = "success"
	// ResultStatusFailed indicates failed execution.
	ResultStatusFailed ResultStatus = "failed"
	// ResultStatusSkipped indicates the step was skipped.
	ResultStatusSkipped ResultStatus = "skipped"
	// ResultStatusTimeout indicates the step timed out.
	ResultStatusTimeout ResultStatus = "timeout"
)

// StepResult contains the result of a step execution.
// 推荐使用 NewStepResult 创建，然后在执行过程中逐步填充 Output，
// 最后用 defer result.Finish() 自动设置 EndTime 和 Duration。
type StepResult struct {
	StepID    string
	Status    ResultStatus
	StartTime time.Time
	EndTime   time.Time
	Duration  time.Duration
	Output    any
	Error     error
	Metrics   map[string]float64
}

// NewStepResult 创建一个初始状态为 success 的 StepResult。
// 配合 defer result.Finish() 使用，在执行过程中逐步填充数据。
func NewStepResult(stepID string) *StepResult {
	return &StepResult{
		StepID:    stepID,
		Status:    ResultStatusSuccess,
		StartTime: time.Now(),
		Metrics:   make(map[string]float64),
	}
}

// Fail 标记步骤为失败。
func (r *StepResult) Fail(err error) {
	r.Status = ResultStatusFailed
	r.Error = err
}

// Timeout 标记步骤为超时。
func (r *StepResult) Timeout(err error) {
	r.Status = ResultStatusTimeout
	r.Error = err
}

// Finish 完成结果构建，设置 EndTime 和 Duration。
// 通常在 Execute 方法开头用 defer result.Finish() 调用。
func (r *StepResult) Finish() {
	r.EndTime = time.Now()
	r.Duration = r.EndTime.Sub(r.StartTime)
}

// AddMetric 添加一个指标。
func (r *StepResult) AddMetric(key string, value float64) {
	if r.Metrics == nil {
		r.Metrics = make(map[string]float64)
	}
	r.Metrics[key] = value
}

// IsSuccess 判断步骤是否成功。
func (r *StepResult) IsSuccess() bool {
	return r.Status == ResultStatusSuccess
}

// ExecutionContext holds runtime state for step execution.
type ExecutionContext struct {
	Variables map[string]any
	Results   map[string]*StepResult
	VU        *VirtualUser
	Iteration int
}

// VirtualUser represents a virtual user in load testing.
type VirtualUser struct {
	ID        int
	Iteration int
	StartTime time.Time
}

// ExecutionState represents the state of a workflow execution.
type ExecutionState struct {
	ID                string
	WorkflowID        string
	Status            ExecutionStatus
	StartTime         time.Time
	EndTime           *time.Time
	Progress          float64
	SlaveStates       map[string]*SlaveExecutionState
	AggregatedMetrics *AggregatedMetrics
	Errors            []ExecutionError
}

// ExecutionStatus represents the status of an execution.
type ExecutionStatus string

const (
	// ExecutionStatusPending indicates the execution is pending.
	ExecutionStatusPending ExecutionStatus = "pending"
	// ExecutionStatusRunning indicates the execution is running.
	ExecutionStatusRunning ExecutionStatus = "running"
	// ExecutionStatusPaused indicates the execution is paused.
	ExecutionStatusPaused ExecutionStatus = "paused"
	// ExecutionStatusCompleted indicates the execution completed.
	ExecutionStatusCompleted ExecutionStatus = "completed"
	// ExecutionStatusFailed indicates the execution failed.
	ExecutionStatusFailed ExecutionStatus = "failed"
	// ExecutionStatusAborted indicates the execution was aborted.
	ExecutionStatusAborted ExecutionStatus = "aborted"
)

// SlaveExecutionState tracks execution state for each slave.
type SlaveExecutionState struct {
	SlaveID        string
	Status         ExecutionStatus
	Segment        ExecutionSegment
	CompletedVUs   int
	CompletedIters int
	CurrentMetrics *Metrics
}

// ExecutionError represents an error during execution.
type ExecutionError struct {
	Code      ErrorCode
	Message   string
	StepID    string
	Cause     error
	Timestamp time.Time
}

// ErrorCode represents the type of error.
type ErrorCode string

const (
	// ErrCodeParsing indicates a parsing error.
	ErrCodeParsing ErrorCode = "PARSING_ERROR"
	// ErrCodeValidation indicates a validation error.
	ErrCodeValidation ErrorCode = "VALIDATION_ERROR"
	// ErrCodeExecution indicates an execution error.
	ErrCodeExecution ErrorCode = "EXECUTION_ERROR"
	// ErrCodeTimeout indicates a timeout error.
	ErrCodeTimeout ErrorCode = "TIMEOUT_ERROR"
	// ErrCodeConnection indicates a connection error.
	ErrCodeConnection ErrorCode = "CONNECTION_ERROR"
	// ErrCodeAuthentication indicates an authentication error.
	ErrCodeAuthentication ErrorCode = "AUTH_ERROR"
)
