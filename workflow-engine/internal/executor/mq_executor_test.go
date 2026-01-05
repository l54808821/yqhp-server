package executor

import (
	"context"
	"testing"
	"time"

	"github.com/grafana/k6/workflow-engine/pkg/types"
)

// TestMQExecutor_Publish tests message publishing.
func TestMQExecutor_Publish(t *testing.T) {
	executor := NewMQExecutor()
	ctx := context.Background()

	// 注册内存适配器
	adapter := NewInMemoryMQAdapter()
	executor.RegisterAdapter(MQTypeKafka, adapter)

	err := executor.Init(ctx, map[string]any{
		"type":   "kafka",
		"broker": "localhost:9092",
		"topic":  "test-topic",
	})
	if err != nil {
		t.Fatalf("failed to init executor: %v", err)
	}
	defer executor.Cleanup(ctx)

	step := &types.Step{
		ID: "test-publish",
		Config: map[string]any{
			"action":  "publish",
			"message": "Hello, MQ!",
			"key":     "test-key",
		},
	}

	result, err := executor.Execute(ctx, step, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Status != types.ResultStatusSuccess {
		t.Errorf("expected success, got %s: %s", result.Status, result.Error)
	}

	output := result.Output.(*MQResult)
	if !output.Success {
		t.Errorf("expected success, got error: %s", output.Error)
	}
	if output.Count != 1 {
		t.Errorf("expected 1 message, got %d", output.Count)
	}
}

// TestMQExecutor_Consume tests message consumption.
func TestMQExecutor_Consume(t *testing.T) {
	executor := NewMQExecutor()
	ctx := context.Background()

	adapter := NewInMemoryMQAdapter()
	executor.RegisterAdapter(MQTypeKafka, adapter)

	err := executor.Init(ctx, map[string]any{
		"type":   "kafka",
		"broker": "localhost:9092",
		"topic":  "test-topic",
	})
	if err != nil {
		t.Fatalf("failed to init executor: %v", err)
	}
	defer executor.Cleanup(ctx)

	// 先发布消息
	publishStep := &types.Step{
		ID: "test-publish",
		Config: map[string]any{
			"action":  "publish",
			"message": "Test message",
		},
	}
	executor.Execute(ctx, publishStep, nil)

	// 消费消息
	consumeStep := &types.Step{
		ID: "test-consume",
		Config: map[string]any{
			"action": "consume",
			"count":  1,
		},
	}

	result, err := executor.Execute(ctx, consumeStep, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Status != types.ResultStatusSuccess {
		t.Errorf("expected success, got %s: %s", result.Status, result.Error)
	}

	output := result.Output.(*MQResult)
	if !output.Success {
		t.Errorf("expected success, got error: %s", output.Error)
	}
	if output.Count != 1 {
		t.Errorf("expected 1 message, got %d", output.Count)
	}
	if len(output.Messages) != 1 {
		t.Errorf("expected 1 message, got %d", len(output.Messages))
	}
	if output.Messages[0].Value != "Test message" {
		t.Errorf("expected 'Test message', got %q", output.Messages[0].Value)
	}
}

// TestMQExecutor_PublishToQueue tests publishing to a queue.
func TestMQExecutor_PublishToQueue(t *testing.T) {
	executor := NewMQExecutor()
	ctx := context.Background()

	adapter := NewInMemoryMQAdapter()
	executor.RegisterAdapter(MQTypeRabbitMQ, adapter)

	err := executor.Init(ctx, map[string]any{
		"type":   "rabbitmq",
		"broker": "localhost:5672",
		"queue":  "test-queue",
	})
	if err != nil {
		t.Fatalf("failed to init executor: %v", err)
	}
	defer executor.Cleanup(ctx)

	step := &types.Step{
		ID: "test-publish",
		Config: map[string]any{
			"action":  "publish",
			"message": "Queue message",
		},
	}

	result, err := executor.Execute(ctx, step, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Status != types.ResultStatusSuccess {
		t.Errorf("expected success, got %s", result.Status)
	}

	// 验证消息在队列中
	msgs := adapter.GetQueueMessages("test-queue")
	if len(msgs) != 1 {
		t.Errorf("expected 1 message in queue, got %d", len(msgs))
	}
}

// TestMQExecutor_ConsumeMultiple tests consuming multiple messages.
func TestMQExecutor_ConsumeMultiple(t *testing.T) {
	executor := NewMQExecutor()
	ctx := context.Background()

	adapter := NewInMemoryMQAdapter()
	executor.RegisterAdapter(MQTypeKafka, adapter)

	err := executor.Init(ctx, map[string]any{
		"type":  "kafka",
		"topic": "test-topic",
	})
	if err != nil {
		t.Fatalf("failed to init executor: %v", err)
	}
	defer executor.Cleanup(ctx)

	// 发布多条消息
	for i := 0; i < 5; i++ {
		step := &types.Step{
			ID: "publish",
			Config: map[string]any{
				"action":  "publish",
				"message": "Message " + string(rune('A'+i)),
			},
		}
		executor.Execute(ctx, step, nil)
	}

	// 消费 3 条消息
	consumeStep := &types.Step{
		ID: "consume",
		Config: map[string]any{
			"action": "consume",
			"count":  3,
		},
	}

	result, err := executor.Execute(ctx, consumeStep, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := result.Output.(*MQResult)
	if output.Count != 3 {
		t.Errorf("expected 3 messages, got %d", output.Count)
	}

	// 验证剩余消息
	remaining := adapter.GetTopicMessages("test-topic")
	if len(remaining) != 2 {
		t.Errorf("expected 2 remaining messages, got %d", len(remaining))
	}
}

// TestMQExecutor_StepConfigOverride tests step-level config override.
func TestMQExecutor_StepConfigOverride(t *testing.T) {
	executor := NewMQExecutor()
	ctx := context.Background()

	adapter := NewInMemoryMQAdapter()
	executor.RegisterAdapter(MQTypeKafka, adapter)

	err := executor.Init(ctx, map[string]any{
		"type":  "kafka",
		"topic": "default-topic",
	})
	if err != nil {
		t.Fatalf("failed to init executor: %v", err)
	}
	defer executor.Cleanup(ctx)

	// 步骤级覆盖 topic
	step := &types.Step{
		ID: "publish",
		Config: map[string]any{
			"action":  "publish",
			"message": "Override message",
			"topic":   "override-topic",
		},
	}

	executor.Execute(ctx, step, nil)

	// 验证消息在覆盖的 topic 中
	msgs := adapter.GetTopicMessages("override-topic")
	if len(msgs) != 1 {
		t.Errorf("expected 1 message in override-topic, got %d", len(msgs))
	}

	// 默认 topic 应该为空
	defaultMsgs := adapter.GetTopicMessages("default-topic")
	if len(defaultMsgs) != 0 {
		t.Errorf("expected 0 messages in default-topic, got %d", len(defaultMsgs))
	}
}

// TestMQExecutor_WithHeaders tests publishing with headers.
func TestMQExecutor_WithHeaders(t *testing.T) {
	executor := NewMQExecutor()
	ctx := context.Background()

	adapter := NewInMemoryMQAdapter()
	executor.RegisterAdapter(MQTypeKafka, adapter)

	err := executor.Init(ctx, map[string]any{
		"type":  "kafka",
		"topic": "test-topic",
	})
	if err != nil {
		t.Fatalf("failed to init executor: %v", err)
	}
	defer executor.Cleanup(ctx)

	step := &types.Step{
		ID: "publish",
		Config: map[string]any{
			"action":  "publish",
			"message": "Message with headers",
			"headers": map[string]any{
				"Content-Type": "application/json",
				"X-Custom":     "custom-value",
			},
		},
	}

	result, err := executor.Execute(ctx, step, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := result.Output.(*MQResult)
	if len(output.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(output.Messages))
	}

	msg := output.Messages[0]
	if msg.Headers["Content-Type"] != "application/json" {
		t.Errorf("expected Content-Type header, got %v", msg.Headers)
	}
	if msg.Headers["X-Custom"] != "custom-value" {
		t.Errorf("expected X-Custom header, got %v", msg.Headers)
	}
}

// TestMQExecutor_InvalidAction tests invalid action.
func TestMQExecutor_InvalidAction(t *testing.T) {
	executor := NewMQExecutor()
	ctx := context.Background()

	adapter := NewInMemoryMQAdapter()
	executor.RegisterAdapter(MQTypeKafka, adapter)

	err := executor.Init(ctx, map[string]any{
		"type": "kafka",
	})
	if err != nil {
		t.Fatalf("failed to init executor: %v", err)
	}

	step := &types.Step{
		ID: "invalid",
		Config: map[string]any{
			"action": "invalid",
		},
	}

	result, err := executor.Execute(ctx, step, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Status != types.ResultStatusFailed {
		t.Errorf("expected failed status, got %s", result.Status)
	}
}

// TestMQExecutor_MissingAction tests missing action.
func TestMQExecutor_MissingAction(t *testing.T) {
	executor := NewMQExecutor()
	ctx := context.Background()

	err := executor.Init(ctx, map[string]any{})
	if err != nil {
		t.Fatalf("failed to init executor: %v", err)
	}

	step := &types.Step{
		ID:     "missing",
		Config: map[string]any{},
	}

	result, err := executor.Execute(ctx, step, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Status != types.ResultStatusFailed {
		t.Errorf("expected failed status, got %s", result.Status)
	}
}

// TestMQExecutor_VariableResolution tests variable resolution.
func TestMQExecutor_VariableResolution(t *testing.T) {
	executor := NewMQExecutor()
	ctx := context.Background()

	adapter := NewInMemoryMQAdapter()
	executor.RegisterAdapter(MQTypeKafka, adapter)

	err := executor.Init(ctx, map[string]any{
		"type":  "kafka",
		"topic": "test-topic",
	})
	if err != nil {
		t.Fatalf("failed to init executor: %v", err)
	}
	defer executor.Cleanup(ctx)

	// 创建执行上下文
	execCtx := NewExecutionContext()
	execCtx.SetVariable("user_id", "12345")
	execCtx.SetVariable("event_type", "user_created")

	step := &types.Step{
		ID: "publish",
		Config: map[string]any{
			"action":  "publish",
			"message": "User ${user_id} event: ${event_type}",
			"key":     "${user_id}",
		},
	}

	result, err := executor.Execute(ctx, step, execCtx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := result.Output.(*MQResult)
	if len(output.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(output.Messages))
	}

	msg := output.Messages[0]
	if msg.Value != "User 12345 event: user_created" {
		t.Errorf("expected resolved message, got %q", msg.Value)
	}
	if msg.Key != "12345" {
		t.Errorf("expected resolved key, got %q", msg.Key)
	}
}

// TestInMemoryMQAdapter_Clear tests clearing messages.
func TestInMemoryMQAdapter_Clear(t *testing.T) {
	adapter := NewInMemoryMQAdapter()
	ctx := context.Background()

	adapter.Connect(ctx, &MQConfig{})

	// 发布消息
	adapter.Publish(ctx, &MQOperation{
		Topic:   "topic1",
		Message: "msg1",
	})
	adapter.Publish(ctx, &MQOperation{
		Queue:   "queue1",
		Message: "msg2",
	})

	// 验证消息存在
	if len(adapter.GetTopicMessages("topic1")) != 1 {
		t.Error("expected 1 message in topic1")
	}
	if len(adapter.GetQueueMessages("queue1")) != 1 {
		t.Error("expected 1 message in queue1")
	}

	// 清除
	adapter.Clear()

	// 验证消息已清除
	if len(adapter.GetTopicMessages("topic1")) != 0 {
		t.Error("expected 0 messages in topic1 after clear")
	}
	if len(adapter.GetQueueMessages("queue1")) != 0 {
		t.Error("expected 0 messages in queue1 after clear")
	}
}

// TestMQExecutor_Timeout tests timeout handling.
func TestMQExecutor_Timeout(t *testing.T) {
	executor := NewMQExecutor()
	ctx := context.Background()

	adapter := NewInMemoryMQAdapter()
	executor.RegisterAdapter(MQTypeKafka, adapter)

	err := executor.Init(ctx, map[string]any{
		"type":    "kafka",
		"topic":   "test-topic",
		"timeout": "100ms",
	})
	if err != nil {
		t.Fatalf("failed to init executor: %v", err)
	}
	defer executor.Cleanup(ctx)

	// 消费空队列（应该立即返回）
	step := &types.Step{
		ID: "consume",
		Config: map[string]any{
			"action": "consume",
		},
	}

	start := time.Now()
	result, err := executor.Execute(ctx, step, nil)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Status != types.ResultStatusSuccess {
		t.Errorf("expected success, got %s", result.Status)
	}

	// 应该很快返回（内存适配器不会阻塞）
	if elapsed > time.Second {
		t.Errorf("expected quick return, took %v", elapsed)
	}
}
