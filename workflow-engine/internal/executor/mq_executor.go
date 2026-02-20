package executor

import (
	"context"
	"fmt"
	"strings"
	"time"

	pkgExecutor "yqhp/workflow-engine/pkg/executor"
	"yqhp/workflow-engine/pkg/types"
)

const (
	MQExecutorType = "mq"
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
	Type    MQType         `yaml:"type" json:"type"`
	Broker  string         `yaml:"broker" json:"broker"`
	Topic   string         `yaml:"topic,omitempty" json:"topic,omitempty"`
	Queue   string         `yaml:"queue,omitempty" json:"queue,omitempty"`
	GroupID string         `yaml:"group_id,omitempty" json:"group_id,omitempty"`
	Auth    MQAuthConfig   `yaml:"auth,omitempty" json:"auth,omitempty"`
	Options map[string]any `yaml:"options,omitempty" json:"options,omitempty"`
	Timeout time.Duration  `yaml:"timeout,omitempty" json:"timeout,omitempty"`
}

// MQAuthConfig 消息队列认证配置
type MQAuthConfig struct {
	Username string `yaml:"username,omitempty" json:"username,omitempty"`
	Password string `yaml:"password,omitempty" json:"password,omitempty"`
	Token    string `yaml:"token,omitempty" json:"token,omitempty"`
}

// MQOperation 消息队列操作
type MQOperation struct {
	Action  string            `yaml:"action" json:"action"`
	Message string            `yaml:"message,omitempty" json:"message,omitempty"`
	Topic   string            `yaml:"topic,omitempty" json:"topic,omitempty"`
	Queue   string            `yaml:"queue,omitempty" json:"queue,omitempty"`
	Timeout time.Duration     `yaml:"timeout,omitempty" json:"timeout,omitempty"`
	Count   int               `yaml:"count,omitempty" json:"count,omitempty"`
	Format  string            `yaml:"format,omitempty" json:"format,omitempty"`
	Key     string            `yaml:"key,omitempty" json:"key,omitempty"`
	Headers map[string]string `yaml:"headers,omitempty" json:"headers,omitempty"`
}

// MQAdapter 消息队列适配器接口
type MQAdapter interface {
	Connect(ctx context.Context, config *MQConfig) error
	Publish(ctx context.Context, op *MQOperation) (*MQResult, error)
	Consume(ctx context.Context, op *MQOperation) (*MQResult, error)
	Close(ctx context.Context) error
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

	if options, ok := config["options"].(map[string]any); ok {
		e.config.Options = options
	}

	return nil
}

// Execute 执行消息队列操作步骤。
func (e *MQExecutor) Execute(ctx context.Context, step *types.Step, execCtx *ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(step.ID)
	output := &types.MQResponseData{}
	result.Output = output
	defer func() {
		result.Finish()
		output.Duration = result.Duration.Milliseconds()
	}()

	// 1. 执行前置处理器
	procExecutor := e.executePreProcessors(ctx, step, execCtx)

	// 2. 解析操作配置
	op, err := e.parseOperation(step.Config)
	if err != nil {
		output.Error = err.Error()
		result.Fail(err)
		return result, nil
	}

	// 3. 解析步骤级配置
	stepConfig := e.parseStepConfig(step.Config)

	// 4. 变量解析
	if execCtx != nil {
		evalCtx := execCtx.ToEvaluationContext()
		resolver := GetVariableResolver()
		op.Message = resolver.ResolveString(op.Message, evalCtx)
		op.Topic = resolver.ResolveString(op.Topic, evalCtx)
		op.Queue = resolver.ResolveString(op.Queue, evalCtx)
		op.Key = resolver.ResolveString(op.Key, evalCtx)
		stepConfig.Broker = resolver.ResolveString(stepConfig.Broker, evalCtx)
		for k, v := range op.Headers {
			op.Headers[k] = resolver.ResolveString(v, evalCtx)
		}
	}

	// 5. 获取适配器
	adapter, ok := e.adapters[stepConfig.Type]
	if !ok {
		adapter = NewInMemoryMQAdapter()
		e.adapters[stepConfig.Type] = adapter
	}

	// 6. 确保已连接
	if !adapter.IsConnected() {
		if connErr := adapter.Connect(ctx, stepConfig); connErr != nil {
			output.Error = fmt.Sprintf("连接消息队列失败: %s", connErr.Error())
			result.Fail(connErr)
			return result, nil
		}
	}

	// 7. 设置超时
	timeout := op.Timeout
	if timeout <= 0 {
		timeout = stepConfig.Timeout
	}
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	// 8. 合并 topic/queue
	if op.Topic == "" {
		op.Topic = stepConfig.Topic
	}
	if op.Queue == "" {
		op.Queue = stepConfig.Queue
	}

	output.Action = op.Action
	output.Topic = op.Topic
	output.Queue = op.Queue

	// 9. 执行操作
	var mqResult *MQResult
	switch op.Action {
	case "publish", "send":
		mqResult, err = adapter.Publish(ctx, op)
	case "consume", "receive":
		mqResult, err = adapter.Consume(ctx, op)
	default:
		err = NewConfigError(fmt.Sprintf("未知的消息队列操作: %s", op.Action), nil)
	}

	if err != nil {
		output.Error = err.Error()
		result.Fail(err)
		return result, nil
	}

	// 10. 填充输出
	output.Success = mqResult.Success
	output.Count = mqResult.Count
	if mqResult.Error != "" {
		output.Error = mqResult.Error
	}
	if len(mqResult.Messages) > 0 {
		msgs := make([]types.MQMessageData, len(mqResult.Messages))
		for i, m := range mqResult.Messages {
			ts := ""
			if !m.Timestamp.IsZero() {
				ts = m.Timestamp.Format(time.RFC3339)
			}
			msgs[i] = types.MQMessageData{
				ID:        m.ID,
				Key:       m.Key,
				Value:     m.Value,
				Headers:   m.Headers,
				Timestamp: ts,
				Topic:     m.Topic,
				Partition: m.Partition,
				Offset:    m.Offset,
			}
		}
		output.Messages = msgs
	}

	// 11. 执行后置处理器
	e.executePostProcessors(ctx, step, execCtx, procExecutor, output, result.StartTime)

	// 12. 收集日志和断言
	e.collectLogsAndAssertions(execCtx, output)

	return result, nil
}

// executePreProcessors 执行前置处理器。
func (e *MQExecutor) executePreProcessors(ctx context.Context, step *types.Step, execCtx *ExecutionContext) *pkgExecutor.ProcessorExecutor {
	variables := make(map[string]interface{})
	envVars := make(map[string]interface{})
	if execCtx != nil && execCtx.Variables != nil {
		for k, v := range execCtx.Variables {
			variables[k] = v
		}
	}
	procExecutor := pkgExecutor.NewProcessorExecutor(variables, envVars)

	if len(step.PreProcessors) > 0 {
		preLogs := procExecutor.ExecuteProcessors(ctx, step.PreProcessors, "pre")
		execCtx.AppendLogs(preLogs)
		trackVariableChangesShared(execCtx, preLogs)

		if execCtx != nil && execCtx.Variables != nil {
			for k, v := range procExecutor.GetVariables() {
				execCtx.Variables[k] = v
			}
		}
	}

	return procExecutor
}

// executePostProcessors 执行后置处理器。
func (e *MQExecutor) executePostProcessors(ctx context.Context, step *types.Step, execCtx *ExecutionContext, procExecutor *pkgExecutor.ProcessorExecutor, output *types.MQResponseData, startTime time.Time) {
	if len(step.PostProcessors) == 0 {
		return
	}

	procExecutor.SetResponse(output.ToMap())

	postLogs := procExecutor.ExecuteProcessors(ctx, step.PostProcessors, "post")
	execCtx.AppendLogs(postLogs)
	trackVariableChangesShared(execCtx, postLogs)

	if execCtx != nil && execCtx.Variables != nil {
		for k, v := range procExecutor.GetVariables() {
			execCtx.Variables[k] = v
		}
	}
}

// collectLogsAndAssertions 收集日志和断言结果到 output 中。
func (e *MQExecutor) collectLogsAndAssertions(execCtx *ExecutionContext, output *types.MQResponseData) {
	if execCtx == nil {
		return
	}
	execCtx.CreateVariableSnapshotWithEnvVars(nil)

	allConsoleLogs := execCtx.FlushLogs()
	if len(allConsoleLogs) > 0 {
		output.ConsoleLogs = allConsoleLogs

		for _, entry := range allConsoleLogs {
			if entry.Type == types.LogTypeProcessor && entry.Processor != nil && entry.Processor.Type == "assertion" {
				output.Assertions = append(output.Assertions, types.AssertionResult{
					ID:      entry.Processor.ID,
					Name:    entry.Processor.Name,
					Passed:  entry.Processor.Success,
					Message: entry.Processor.Message,
				})
			}
		}
	}
}

// parseOperation 解析操作配置
func (e *MQExecutor) parseOperation(config map[string]any) (*MQOperation, error) {
	op := &MQOperation{
		Headers: make(map[string]string),
	}

	if action, ok := config["action"].(string); ok {
		op.Action = strings.ToLower(action)
	} else {
		return nil, NewConfigError("消息队列步骤需要配置 'action'（操作类型）", nil)
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
	} else if countF, ok := config["count"].(float64); ok {
		op.Count = int(countF)
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
		Type:    MQTypeKafka,
		Timeout: defaultMQTimeout,
		Options: make(map[string]any),
	}

	if e.config != nil {
		stepConfig.Type = e.config.Type
		stepConfig.Broker = e.config.Broker
		stepConfig.Topic = e.config.Topic
		stepConfig.Queue = e.config.Queue
		stepConfig.GroupID = e.config.GroupID
		stepConfig.Auth = e.config.Auth
		stepConfig.Options = e.config.Options
		stepConfig.Timeout = e.config.Timeout
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
	if timeout, ok := config["timeout"].(string); ok {
		if d, err := time.ParseDuration(timeout); err == nil {
			stepConfig.Timeout = d
		}
	}

	if auth, ok := config["auth"].(map[string]any); ok {
		if username, ok := auth["username"].(string); ok {
			stepConfig.Auth.Username = username
		}
		if password, ok := auth["password"].(string); ok {
			stepConfig.Auth.Password = password
		}
		if token, ok := auth["token"].(string); ok {
			stepConfig.Auth.Token = token
		}
	}

	if options, ok := config["options"].(map[string]any); ok {
		if stepConfig.Options == nil {
			stepConfig.Options = make(map[string]any)
		}
		for k, v := range options {
			stepConfig.Options[k] = v
		}
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
