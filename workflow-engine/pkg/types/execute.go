package types

// ExecuteWorkflowRequest 执行工作流请求
type ExecuteWorkflowRequest struct {
	// 工作流定义（JSON 字符串或已解析的工作流对象）
	Workflow *Workflow `json:"workflow,omitempty"`
	// 工作流定义（JSON 字符串格式）
	WorkflowJSON string `json:"workflowJson,omitempty"`
	// 单步快捷方式：传入单个步骤，自动包装为工作流
	Step *Step `json:"step,omitempty"`
	// 环境 ID
	EnvID int64 `json:"envId,omitempty"`
	// 变量
	Variables map[string]interface{} `json:"variables,omitempty"`
	// 环境变量
	EnvVars map[string]interface{} `json:"envVars,omitempty"`
	// 超时时间（秒）
	Timeout int `json:"timeout,omitempty"`
	// 执行模式：debug（失败即停止）或 normal（继续执行）
	Mode string `json:"mode,omitempty"`
	// 执行器类型：local 或 remote
	ExecutorType string `json:"executorType,omitempty"`
	// 指定的 Slave ID（远程执行时使用）
	SlaveID string `json:"slaveId,omitempty"`
	// 会话 ID（用于 SSE 事件关联）
	SessionID string `json:"sessionId,omitempty"`
	// 选中的步骤 ID（用于选择性执行）
	SelectedSteps []string `json:"selectedSteps,omitempty"`
	// 回调 URL（用于接收执行事件）
	CallbackURL string `json:"callbackUrl,omitempty"`
	// 是否使用 SSE 流式响应
	Stream bool `json:"stream,omitempty"`
}

// ExecuteWorkflowResponse 执行工作流响应
type ExecuteWorkflowResponse struct {
	// 是否成功
	Success bool `json:"success"`
	// 执行 ID
	ExecutionID string `json:"executionId,omitempty"`
	// 会话 ID
	SessionID string `json:"sessionId,omitempty"`
	// 执行汇总（阻塞模式时返回）
	Summary *ExecuteSummary `json:"summary,omitempty"`
	// 错误信息
	Error string `json:"error,omitempty"`
}

// ExecuteSummary 执行汇总（用于 API 响应）
type ExecuteSummary struct {
	SessionID     string                `json:"sessionId"`
	TotalSteps    int                   `json:"totalSteps"`
	SuccessSteps  int                   `json:"successSteps"`
	FailedSteps   int                   `json:"failedSteps"`
	TotalDuration int64                 `json:"totalDurationMs"`
	Status        string                `json:"status"` // success, failed, timeout, stopped
	Steps         []StepExecutionResult `json:"steps,omitempty"` // 步骤执行详情
}

// StepExecutionResult 步骤执行结果
type StepExecutionResult struct {
	StepID     string      `json:"stepId"`
	StepName   string      `json:"stepName"`
	StepType   string      `json:"stepType"`
	Success    bool        `json:"success"`
	DurationMs int64       `json:"durationMs"`
	Result     interface{} `json:"result,omitempty"` // HTTPResponseData / ScriptResponseData / 其他
	Error      string      `json:"error,omitempty"`
}

// SSEEvent SSE 事件
type SSEEvent struct {
	ID    string      `json:"id,omitempty"`
	Type  string      `json:"type"`
	Data  interface{} `json:"data"`
	Retry int         `json:"retry,omitempty"`
}

// ExecutionEventType 执行事件类型
type ExecutionEventType string

const (
	// 连接事件
	EventTypeConnected ExecutionEventType = "connected"
	// 工作流开始
	EventTypeWorkflowStarted ExecutionEventType = "workflow_started"
	// 步骤开始
	EventTypeStepStarted ExecutionEventType = "step_started"
	// 步骤完成
	EventTypeStepCompleted ExecutionEventType = "step_completed"
	// 步骤失败
	EventTypeStepFailed ExecutionEventType = "step_failed"
	// 工作流完成
	EventTypeWorkflowCompleted ExecutionEventType = "workflow_completed"
	// 需要人机交互
	EventTypeInteractionRequired ExecutionEventType = "ai_interaction_required"
	// 心跳
	EventTypeHeartbeat ExecutionEventType = "heartbeat"
	// 错误
	EventTypeError ExecutionEventType = "error"
)

// StepStartedEvent 步骤开始事件数据
type StepStartedEvent struct {
	StepID   string `json:"stepId"`
	StepName string `json:"stepName"`
	StepType string `json:"stepType"`
	Index    int    `json:"index"`
}

// StepCompletedEvent 步骤完成事件数据
type StepCompletedEvent struct {
	StepID     string      `json:"stepId"`
	StepName   string      `json:"stepName"`
	Success    bool        `json:"success"`
	DurationMs int64       `json:"durationMs"`
	Result     interface{} `json:"result,omitempty"`
}

// WorkflowCompletedEvent 工作流完成事件数据
type WorkflowCompletedEvent struct {
	Status       string `json:"status"`
	TotalSteps   int    `json:"totalSteps"`
	SuccessSteps int    `json:"successSteps"`
	FailedSteps  int    `json:"failedSteps"`
	DurationMs   int64  `json:"durationMs"`
}

// InteractionRequiredEvent 人机交互事件数据
type InteractionRequiredEvent struct {
	StepID  string `json:"stepId"`
	Prompt  string `json:"prompt"`
	Timeout int    `json:"timeout"` // 秒
}

// ExecuteInteractionResponse 执行交互响应（扩展版，包含会话和步骤信息）
type ExecuteInteractionResponse struct {
	SessionID string `json:"sessionId"`
	StepID    string `json:"stepId"`
	Value     string `json:"value"`
	Skipped   bool   `json:"skipped"`
}
