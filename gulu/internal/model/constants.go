package model

// WorkflowType 工作流类型
type WorkflowType string

const (
	WorkflowTypeNormal         WorkflowType = "normal"          // 普通流程
	WorkflowTypePerformance    WorkflowType = "performance"     // 压测流程
	WorkflowTypeDataGeneration WorkflowType = "data_generation" // 造数流程
)

// IsValid 验证工作流类型是否有效
func (t WorkflowType) IsValid() bool {
	switch t {
	case WorkflowTypeNormal, WorkflowTypePerformance, WorkflowTypeDataGeneration:
		return true
	default:
		return false
	}
}

// ExecutionMode 执行模式
type ExecutionMode string

const (
	ExecutionModeDebug   ExecutionMode = "debug"   // 调试模式
	ExecutionModeExecute ExecutionMode = "execute" // 执行模式
)

// SourceType 执行来源类型
type SourceType string

const (
	SourceTypePerformance SourceType = "performance" // 性能测试
	SourceTypeTestPlan    SourceType = "test_plan"   // 测试计划
	SourceTypeDebug       SourceType = "debug"       // 调试
)

// ExecutionStatus 执行状态
type ExecutionStatus string

const (
	ExecutionStatusPending   ExecutionStatus = "pending"   // 等待中
	ExecutionStatusRunning   ExecutionStatus = "running"   // 运行中
	ExecutionStatusCompleted ExecutionStatus = "completed" // 已完成
	ExecutionStatusFailed    ExecutionStatus = "failed"    // 已失败
	ExecutionStatusStopped   ExecutionStatus = "stopped"   // 已停止
	ExecutionStatusPaused    ExecutionStatus = "paused"    // 已暂停
)

// IsTerminal 判断是否是终态
func (s ExecutionStatus) IsTerminal() bool {
	return s == ExecutionStatusCompleted || s == ExecutionStatusFailed || s == ExecutionStatusStopped
}
