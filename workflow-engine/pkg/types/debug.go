package types

// DebugStepRequest 单步调试请求
type DebugStepRequest struct {
	// 节点配置
	NodeConfig *DebugNodeConfig `json:"nodeConfig"`
	// 环境 ID
	EnvID int64 `json:"envId,omitempty"`
	// 变量上下文
	Variables map[string]interface{} `json:"variables,omitempty"`
	// 环境变量
	EnvVars map[string]interface{} `json:"envVars,omitempty"`
	// 会话 ID
	SessionID string `json:"sessionId,omitempty"`
}

// DebugNodeConfig 调试节点配置
type DebugNodeConfig struct {
	ID             string                 `json:"id"`
	Type           string                 `json:"type"`
	Name           string                 `json:"name"`
	Config         map[string]interface{} `json:"config"`
	PreProcessors  []KeywordConfig        `json:"preProcessors,omitempty"`
	PostProcessors []KeywordConfig        `json:"postProcessors,omitempty"`
}

// KeywordConfig 关键字配置
type KeywordConfig struct {
	ID      string                 `json:"id"`
	Type    string                 `json:"type"`
	Enabled bool                   `json:"enabled"`
	Name    string                 `json:"name,omitempty"`
	Config  map[string]interface{} `json:"config"`
}

// DebugStepResponse 单步调试响应
type DebugStepResponse struct {
	Success          bool                 `json:"success"`
	Response         *HTTPResponseData    `json:"response,omitempty"`
	ScriptResult     *ScriptResponseData  `json:"scriptResult,omitempty"`
	AssertionResults []AssertionResult    `json:"assertionResults,omitempty"`
	ConsoleLogs      []ConsoleLogEntry    `json:"consoleLogs,omitempty"`
	ActualRequest    *ActualRequest       `json:"actualRequest,omitempty"`
	Error            string               `json:"error,omitempty"`
}
