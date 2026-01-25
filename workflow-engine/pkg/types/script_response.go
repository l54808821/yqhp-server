package types

// ScriptResponseData 统一的脚本响应数据结构
// 用于单步调试和流程执行的脚本响应返回
type ScriptResponseData struct {
	// 脚本信息
	Script   string `json:"script"`   // 执行的脚本内容
	Language string `json:"language"` // 脚本语言

	// 执行结果
	Result     interface{} `json:"result"`          // 脚本返回值
	DurationMs int64       `json:"durationMs"`      // 执行耗时（毫秒）
	Error      string      `json:"error,omitempty"` // 错误信息

	// 变量（脚本执行后的变量状态）
	Variables map[string]interface{} `json:"variables,omitempty"`

	// 控制台日志
	ConsoleLogs []ConsoleLogEntry `json:"consoleLogs,omitempty"`

	// 断言结果（如果有）
	Assertions []AssertionResult `json:"assertions,omitempty"`
}

// ToMap 转换为 map（用于脚本中访问数据）
func (r *ScriptResponseData) ToMap() map[string]interface{} {
	result := map[string]interface{}{
		"script":     r.Script,
		"language":   r.Language,
		"result":     r.Result,
		"durationMs": r.DurationMs,
	}

	if r.Error != "" {
		result["error"] = r.Error
	}

	if r.Variables != nil {
		result["variables"] = r.Variables
	}

	if r.ConsoleLogs != nil {
		result["consoleLogs"] = r.ConsoleLogs
	}

	if r.Assertions != nil {
		result["assertions"] = r.Assertions
	}

	return result
}
