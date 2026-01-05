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
