package executor

import (
	"context"
	"sync"
	"time"
)

// InMemoryMQAdapter 内存消息队列适配器（用于测试）
type InMemoryMQAdapter struct {
	mu        sync.RWMutex
	connected bool
	config    *MQConfig
	topics    map[string][]MQMessage
	queues    map[string][]MQMessage
	msgID     int64
}

// NewInMemoryMQAdapter 创建一个新的内存消息队列适配器。
func NewInMemoryMQAdapter() *InMemoryMQAdapter {
	return &InMemoryMQAdapter{
		topics: make(map[string][]MQMessage),
		queues: make(map[string][]MQMessage),
	}
}

// Connect 连接到内存消息队列。
func (a *InMemoryMQAdapter) Connect(ctx context.Context, config *MQConfig) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.config = config
	a.connected = true
	return nil
}

// Publish 发布消息到主题或队列。
func (a *InMemoryMQAdapter) Publish(ctx context.Context, op *MQOperation) (*MQResult, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if !a.connected {
		return &MQResult{
			Success: false,
			Error:   "not connected",
		}, nil
	}

	a.msgID++
	msg := MQMessage{
		ID:        string(rune(a.msgID)),
		Key:       op.Key,
		Value:     op.Message,
		Headers:   op.Headers,
		Timestamp: time.Now(),
		Topic:     op.Topic,
	}

	// 发布到主题
	if op.Topic != "" {
		a.topics[op.Topic] = append(a.topics[op.Topic], msg)
	}

	// 发布到队列
	if op.Queue != "" {
		a.queues[op.Queue] = append(a.queues[op.Queue], msg)
	}

	return &MQResult{
		Success:  true,
		Count:    1,
		Messages: []MQMessage{msg},
	}, nil
}

// Consume 从主题或队列消费消息。
func (a *InMemoryMQAdapter) Consume(ctx context.Context, op *MQOperation) (*MQResult, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if !a.connected {
		return &MQResult{
			Success: false,
			Error:   "not connected",
		}, nil
	}

	count := op.Count
	if count <= 0 {
		count = 1
	}

	var messages []MQMessage

	// 从主题消费
	if op.Topic != "" {
		if msgs, ok := a.topics[op.Topic]; ok && len(msgs) > 0 {
			n := min(count, len(msgs))
			messages = append(messages, msgs[:n]...)
			a.topics[op.Topic] = msgs[n:]
		}
	}

	// 从队列消费
	if op.Queue != "" {
		if msgs, ok := a.queues[op.Queue]; ok && len(msgs) > 0 {
			remaining := count - len(messages)
			if remaining > 0 {
				n := min(remaining, len(msgs))
				messages = append(messages, msgs[:n]...)
				a.queues[op.Queue] = msgs[n:]
			}
		}
	}

	return &MQResult{
		Success:  true,
		Count:    len(messages),
		Messages: messages,
	}, nil
}

// Close 关闭连接。
func (a *InMemoryMQAdapter) Close(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.connected = false
	return nil
}

// IsConnected 返回适配器是否已连接。
func (a *InMemoryMQAdapter) IsConnected() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.connected
}

// GetTopicMessages 返回主题中的消息（用于测试）。
func (a *InMemoryMQAdapter) GetTopicMessages(topic string) []MQMessage {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.topics[topic]
}

// GetQueueMessages 返回队列中的消息（用于测试）。
func (a *InMemoryMQAdapter) GetQueueMessages(queue string) []MQMessage {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.queues[queue]
}

// Clear 清除所有消息（用于测试）。
func (a *InMemoryMQAdapter) Clear() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.topics = make(map[string][]MQMessage)
	a.queues = make(map[string][]MQMessage)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
