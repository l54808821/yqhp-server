package types

// MQResponseData 统一的 MQ 响应数据结构
// 用于单步调试和流程执行的响应返回
type MQResponseData struct {
	// 操作结果
	Success bool   `json:"success"`
	Action  string `json:"action"`

	// 耗时（毫秒）
	Duration int64 `json:"duration"`

	// 目标
	Topic string `json:"topic,omitempty"`
	Queue string `json:"queue,omitempty"`

	// 消息列表（消费时返回）
	Messages []MQMessageData `json:"messages,omitempty"`
	Count    int             `json:"count"`

	// 控制台日志
	ConsoleLogs []ConsoleLogEntry `json:"consoleLogs,omitempty"`

	// 断言结果
	Assertions []AssertionResult `json:"assertions,omitempty"`

	// 错误信息
	Error string `json:"error,omitempty"`
}

// MQMessageData 单条 MQ 消息数据
type MQMessageData struct {
	ID        string            `json:"id,omitempty"`
	Key       string            `json:"key,omitempty"`
	Value     string            `json:"value"`
	Headers   map[string]string `json:"headers,omitempty"`
	Timestamp string            `json:"timestamp,omitempty"`
	Topic     string            `json:"topic,omitempty"`
	Partition int               `json:"partition,omitempty"`
	Offset    int64             `json:"offset,omitempty"`
}

// ToMap 转换为 map（用于后置处理器脚本中访问响应数据）
func (r *MQResponseData) ToMap() map[string]interface{} {
	result := map[string]interface{}{
		"success":  r.Success,
		"action":   r.Action,
		"duration": r.Duration,
		"count":    r.Count,
	}

	if r.Topic != "" {
		result["topic"] = r.Topic
	}
	if r.Queue != "" {
		result["queue"] = r.Queue
	}
	if r.Error != "" {
		result["error"] = r.Error
	}

	if len(r.Messages) > 0 {
		msgs := make([]map[string]interface{}, len(r.Messages))
		for i, m := range r.Messages {
			msg := map[string]interface{}{
				"value": m.Value,
			}
			if m.ID != "" {
				msg["id"] = m.ID
			}
			if m.Key != "" {
				msg["key"] = m.Key
			}
			if m.Topic != "" {
				msg["topic"] = m.Topic
			}
			if m.Headers != nil {
				msg["headers"] = m.Headers
			}
			if m.Timestamp != "" {
				msg["timestamp"] = m.Timestamp
			}
			msgs[i] = msg
		}
		result["messages"] = msgs
	}

	return result
}
