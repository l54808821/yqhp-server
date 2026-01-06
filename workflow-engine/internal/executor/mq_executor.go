package executor

import (
	"context"
	"fmt"
	"strings"
	"time"

	"yqhp/workflow-engine/pkg/types"
)

const (
	// MQExecutorType 是消息队列执行器的类型标识符。
	MQExecutorType = "mq"

	// 消息队列操作的默认超时时间。
	defaultMQTimeout = 30 * time.Second
)

// MQType 消息队列类型
type MQType string

const (
	MQTypeKafka    MQType = "kafka"
	MQTypeRabbitMQ MQType = "rabbitmq"
	MQTypeRedis    MQType = "redis"
	MQTypeMQTT     MQType = "mqtt"
)

// MQConfig 消息队列配置
type MQConfig struct {
	Type    MQType         `yaml:"type" json:"type"`                             // 类型: kafka/rabbitmq/redis/mqtt
	Broker  string         `yaml:"broker" json:"broker"`                         // 消息代理地址
	Topic   string         `yaml:"topic,omitempty" json:"topic,omitempty"`       // 主题
	Queue   string         `yaml:"queue,omitempty" json:"queue,omitempty"`       // 队列
	GroupID string         `yaml:"group_id,omitempty" json:"group_id,omitempty"` // 消费者组 ID
	Auth    MQAuthConfig   `yaml:"auth,omitempty" json:"auth,omitempty"`         // 认证配置
	Options map[string]any `yaml:"options,omitempty" json:"options,omitempty"`   // 其他选项
	Timeout time.Duration  `yaml:"timeout,omitempty" json:"timeout,omitempty"`   // 超时时间
}

// MQAuthConfig 消息队列认证配置
type MQAuthConfig struct {
	Username string `yaml:"username,omitempty" json:"username,omitempty"`
	Password string `yaml:"password,omitempty" json:"password,omitempty"`
	Token    string `yaml:"token,omitempty" json:"token,omitempty"`
}

// MQOperation 消息队列操作
type MQOperation struct {
	Action  string            `yaml:"action" json:"action"`                       // 操作: publish/subscribe/consume/ack
	Message string            `yaml:"message,omitempty" json:"message,omitempty"` // 消息内容
	Topic   string            `yaml:"topic,omitempty" json:"topic,omitempty"`     // 主题（覆盖配置）
	Queue   string            `yaml:"queue,omitempty" json:"queue,omitempty"`     // 队列（覆盖配置）
	Timeout time.Duration     `yaml:"timeout,omitempty" json:"timeout,omitempty"` // 超时时间
	Count   int               `yaml:"count,omitempty" json:"count,omitempty"`     // 消费消息数量
	Format  string            `yaml:"format,omitempty" json:"format,omitempty"`   // 格式: json/protobuf/avro
	Key     string            `yaml:"key,omitempty" json:"key,omitempty"`         // 消息键
	Headers map[string]string `yaml:"headers,omitempty" json:"headers,omitempty"` // 消息头
}

// MQAdapter 消息队列适配器接口
type MQAdapter interface {
	// Connect 连接到消息代理
	Connect(ctx context.Context, config *MQConfig) error
	// Publish 发布消息
	Publish(ctx context.Context, op *MQOperation) (*MQResult, error)
	// Consume 消费消息
	Consume(ctx context.Context, op *MQOperation) (*MQResult, error)
	// Close 关闭连接
	Close(ctx context.Context) error
	// IsConnected 检查是否已连接
	IsConnected() bool
}

// MQResult 消息队列操作结果
type MQResult struct {
	Success  bool        `json:"success"`
	Messages []MQMessage `json:"messages,omitempty"`
	Count    int         `json:"count"`
	Error    string      `json:"error,omitempty"`
}

// MQMessage 消息
type MQMessage struct {
	ID        string            `json:"id,omitempty"`
	Key       string            `json:"key,omitempty"`
	Value     string            `json:"value"`
	Headers   map[string]string `json:"headers,omitempty"`
	Timestamp time.Time         `json:"timestamp,omitempty"`
	Topic     string            `json:"topic,omitempty"`
	Partition int               `json:"partition,omitempty"`
	Offset    int64             `json:"offset,omitempty"`
}

// MQExecutor 执行消息队列操作。
type MQExecutor struct {
	*BaseExecutor
	config   *MQConfig
	adapters map[MQType]MQAdapter
}

// NewMQExecutor 创建一个新的消息队列执行器。
func NewMQExecutor() *MQExecutor {
	return &MQExecutor{
		BaseExecutor: NewBaseExecutor(MQExecutorType),
		adapters:     make(map[MQType]MQAdapter),
	}
}

// RegisterAdapter 注册 MQ 适配器
func (e *MQExecutor) RegisterAdapter(mqType MQType, adapter MQAdapter) {
	e.adapters[mqType] = adapter
}

// Init 使用配置初始化消息队列执行器。
func (e *MQExecutor) Init(ctx context.Context, config map[string]any) error {
	if err := e.BaseExecutor.Init(ctx, config); err != nil {
		return err
	}

	e.config = &MQConfig{
		Type:    MQTypeKafka,
		Timeout: defaultMQTimeout,
		Options: make(map[string]any),
	}

	// 解析配置
	if mqType, ok := config["type"].(string); ok {
		e.config.Type = MQType(strings.ToLower(mqType))
	}
	if broker, ok := config["broker"].(string); ok {
		e.config.Broker = broker
	}
	if topic, ok := config["topic"].(string); ok {
		e.config.Topic = topic
	}
	if queue, ok := config["queue"].(string); ok {
		e.config.Queue = queue
	}
	if groupID, ok := config["group_id"].(string); ok {
		e.config.GroupID = groupID
	}
	if timeout, ok := config["timeout"].(string); ok {
		if d, err := time.ParseDuration(timeout); err == nil {
			e.config.Timeout = d
		}
	}

	// 解析认证配置
	if auth, ok := config["auth"].(map[string]any); ok {
		if username, ok := auth["username"].(string); ok {
			e.config.Auth.Username = username
		}
		if password, ok := auth["password"].(string); ok {
			e.config.Auth.Password = password
		}
		if token, ok := auth["token"].(string); ok {
			e.config.Auth.Token = token
		}
	}

	// 解析其他选项
	if options, ok := config["options"].(map[string]any); ok {
		e.config.Options = options
	}

	return nil
}

// Execute 执行消息队列操作步骤。
func (e *MQExecutor) Execute(ctx context.Context, step *types.Step, execCtx *ExecutionContext) (*types.StepResult, error) {
	startTime := time.Now()

	// 解析操作配置
	op, err := e.parseOperation(step.Config)
	if err != nil {
		return CreateFailedResult(step.ID, startTime, err), nil
	}

	// 解析步骤级配置
	stepConfig := e.parseStepConfig(step.Config)

	// 变量解析
	if execCtx != nil {
		evalCtx := execCtx.ToEvaluationContext()
		op.Message = resolveString(op.Message, evalCtx)
		op.Topic = resolveString(op.Topic, evalCtx)
		op.Queue = resolveString(op.Queue, evalCtx)
		op.Key = resolveString(op.Key, evalCtx)
		stepConfig.Broker = resolveString(stepConfig.Broker, evalCtx)
	}

	// 获取适配器
	adapter, ok := e.adapters[stepConfig.Type]
	if !ok {
		// 使用内存适配器作为默认
		adapter = NewInMemoryMQAdapter()
		e.adapters[stepConfig.Type] = adapter
	}

	// 确保已连接
	if !adapter.IsConnected() {
		if err := adapter.Connect(ctx, stepConfig); err != nil {
			return CreateFailedResult(step.ID, startTime, NewExecutionError(step.ID, "failed to connect to MQ", err)), nil
		}
	}

	// 设置超时
	timeout := op.Timeout
	if timeout <= 0 {
		timeout = stepConfig.Timeout
	}
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	// 合并 topic/queue
	if op.Topic == "" {
		op.Topic = stepConfig.Topic
	}
	if op.Queue == "" {
		op.Queue = stepConfig.Queue
	}

	// 执行操作
	var result *MQResult
	switch op.Action {
	case "publish", "send":
		result, err = adapter.Publish(ctx, op)
	case "consume", "receive":
		result, err = adapter.Consume(ctx, op)
	default:
		err = NewConfigError(fmt.Sprintf("unknown MQ action: %s", op.Action), nil)
	}

	if err != nil {
		return CreateFailedResult(step.ID, startTime, err), nil
	}

	return CreateSuccessResult(step.ID, startTime, result), nil
}

// parseOperation 解析操作配置
func (e *MQExecutor) parseOperation(config map[string]any) (*MQOperation, error) {
	op := &MQOperation{
		Headers: make(map[string]string),
	}

	if action, ok := config["action"].(string); ok {
		op.Action = strings.ToLower(action)
	} else {
		return nil, NewConfigError("MQ step requires 'action' configuration", nil)
	}

	if message, ok := config["message"].(string); ok {
		op.Message = message
	}
	if topic, ok := config["topic"].(string); ok {
		op.Topic = topic
	}
	if queue, ok := config["queue"].(string); ok {
		op.Queue = queue
	}
	if timeout, ok := config["timeout"].(string); ok {
		if d, err := time.ParseDuration(timeout); err == nil {
			op.Timeout = d
		}
	}
	if count, ok := config["count"].(int); ok {
		op.Count = count
	}
	if format, ok := config["format"].(string); ok {
		op.Format = format
	}
	if key, ok := config["key"].(string); ok {
		op.Key = key
	}
	if headers, ok := config["headers"].(map[string]any); ok {
		for k, v := range headers {
			if s, ok := v.(string); ok {
				op.Headers[k] = s
			}
		}
	}

	return op, nil
}

// parseStepConfig 解析步骤级配置
func (e *MQExecutor) parseStepConfig(config map[string]any) *MQConfig {
	stepConfig := &MQConfig{
		Type:    e.config.Type,
		Broker:  e.config.Broker,
		Topic:   e.config.Topic,
		Queue:   e.config.Queue,
		GroupID: e.config.GroupID,
		Auth:    e.config.Auth,
		Options: e.config.Options,
		Timeout: e.config.Timeout,
	}

	if mqType, ok := config["type"].(string); ok {
		stepConfig.Type = MQType(strings.ToLower(mqType))
	}
	if broker, ok := config["broker"].(string); ok {
		stepConfig.Broker = broker
	}
	if topic, ok := config["topic"].(string); ok {
		stepConfig.Topic = topic
	}
	if queue, ok := config["queue"].(string); ok {
		stepConfig.Queue = queue
	}
	if groupID, ok := config["group_id"].(string); ok {
		stepConfig.GroupID = groupID
	}

	return stepConfig
}

// Cleanup 释放消息队列执行器持有的资源。
func (e *MQExecutor) Cleanup(ctx context.Context) error {
	for _, adapter := range e.adapters {
		if err := adapter.Close(ctx); err != nil {
			return err
		}
	}
	return nil
}

// init 在默认注册表中注册消息队列执行器。
func init() {
	MustRegister(NewMQExecutor())
}
