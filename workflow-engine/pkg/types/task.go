package types

// Task represents a task assigned to a slave.
type Task struct {
	ID          string
	ExecutionID string
	Workflow    *Workflow
	Segment     ExecutionSegment
}

// TaskResult contains the result of a task execution.
type TaskResult struct {
	TaskID      string
	ExecutionID string
	SlaveID     string
	Status      ExecutionStatus
	Metrics     *Metrics
	Errors      []ExecutionError
}

// ExecutionPlan describes how work is distributed.
type ExecutionPlan struct {
	ExecutionID string
	Assignments []*SlaveAssignment
}

// SlaveAssignment describes work assigned to a slave.
type SlaveAssignment struct {
	SlaveID  string
	Segment  ExecutionSegment
	Workflow *Workflow
}
