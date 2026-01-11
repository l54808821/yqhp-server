package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"yqhp/workflow-engine/pkg/metrics"
	"yqhp/workflow-engine/pkg/output"
)

func init() {
	output.Register("kafka", New)
}

// Config Kafka 配置
type Config struct {
	// Brokers Kafka broker 地址列表
	Brokers []string
	// Topic 主题名
	Topic string
	// Format 消息格式: json, influx
	Format string
	// PushInterval 推送间隔
	PushInterval time.Duration
	// BatchSize 批量大小
	BatchSize int
	// Tags 全局标签
	Tags map[string]string
}

// Output Kafka 输出
type Output struct {
	params    output.Params
	config    Config
	producer  Producer
	buffer    []metrics.Sample
	mu        sync.Mutex
	runStatus output.RunStatus
	stopCh    chan struct{}
	wg        sync.WaitGroup
}

// Producer Kafka 生产者接口
type Producer interface {
	Send(ctx context.Context, topic string, key string, value []byte) error
	Close() error
}

// SimpleProducer 简单的 HTTP 代理生产者（用于演示，实际使用时应替换为真正的 Kafka 客户端）
type SimpleProducer struct {
	brokers []string
}

func (p *SimpleProducer) Send(ctx context.Context, topic string, key string, value []byte) error {
	// 这里应该使用真正的 Kafka 客户端
	// 为了避免引入额外依赖，这里只是一个占位实现
	// 实际使用时，请使用 github.com/segmentio/kafka-go 或 github.com/confluentinc/confluent-kafka-go
	return nil
}

func (p *SimpleProducer) Close() error {
	return nil
}

// New 创建 Kafka 输出
func New(params output.Params) (output.Output, error) {
	config, err := parseConfig(params.ConfigArgument)
	if err != nil {
		return nil, err
	}

	// 合并全局标签
	if params.Tags != nil {
		if config.Tags == nil {
			config.Tags = make(map[string]string)
		}
		for k, v := range params.Tags {
			config.Tags[k] = v
		}
	}

	return &Output{
		params:   params,
		config:   config,
		producer: &SimpleProducer{brokers: config.Brokers},
		buffer:   make([]metrics.Sample, 0, config.BatchSize),
		stopCh:   make(chan struct{}),
	}, nil
}

// parseConfig 解析配置字符串
// 格式: kafka=broker1:9092,broker2:9092?topic=metrics&format=json
func parseConfig(arg string) (Config, error) {
	config := Config{
		Format:       "json",
		PushInterval: time.Second,
		BatchSize:    1000,
		Topic:        "workflow-metrics",
	}

	if arg == "" {
		return config, fmt.Errorf("Kafka 配置不能为空")
	}

	// 解析 URL 格式
	if strings.Contains(arg, "?") {
		parts := strings.SplitN(arg, "?", 2)
		config.Brokers = strings.Split(parts[0], ",")
		if len(parts) > 1 {
			q, _ := url.ParseQuery(parts[1])
			if topic := q.Get("topic"); topic != "" {
				config.Topic = topic
			}
			if format := q.Get("format"); format != "" {
				config.Format = format
			}
		}
	} else {
		config.Brokers = strings.Split(arg, ",")
	}

	if len(config.Brokers) == 0 {
		return config, fmt.Errorf("至少需要一个 Kafka broker")
	}

	return config, nil
}

// Description 返回描述
func (o *Output) Description() string {
	return fmt.Sprintf("kafka (%s -> %s)", strings.Join(o.config.Brokers, ","), o.config.Topic)
}

// Start 启动输出
func (o *Output) Start() error {
	// 启动定期推送协程
	o.wg.Add(1)
	go o.pushLoop()
	return nil
}

// Stop 停止输出
func (o *Output) Stop() error {
	close(o.stopCh)
	o.wg.Wait()

	// 推送剩余数据
	o.mu.Lock()
	if len(o.buffer) > 0 {
		o.flush()
	}
	o.mu.Unlock()

	return o.producer.Close()
}

// pushLoop 定期推送数据
func (o *Output) pushLoop() {
	defer o.wg.Done()
	ticker := time.NewTicker(o.config.PushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-o.stopCh:
			return
		case <-ticker.C:
			o.mu.Lock()
			if len(o.buffer) > 0 {
				o.flush()
			}
			o.mu.Unlock()
		}
	}
}

// AddMetricSamples 添加指标样本
func (o *Output) AddMetricSamples(containers []metrics.SampleContainer) {
	o.mu.Lock()
	defer o.mu.Unlock()

	for _, container := range containers {
		for _, sample := range container.GetSamples() {
			o.buffer = append(o.buffer, sample)

			// 达到批量大小时推送
			if len(o.buffer) >= o.config.BatchSize {
				o.flush()
			}
		}
	}
}

// flush 推送缓冲区数据到 Kafka
func (o *Output) flush() {
	if len(o.buffer) == 0 {
		return
	}

	ctx := context.Background()

	for _, sample := range o.buffer {
		var value []byte
		var err error

		switch o.config.Format {
		case "json":
			value, err = o.formatJSON(sample)
		default:
			value, err = o.formatJSON(sample)
		}

		if err != nil {
			if o.params.Logger != nil {
				o.params.Logger.Error("格式化消息失败: %v", err)
			}
			continue
		}

		key := sample.Metric.Name
		if err := o.producer.Send(ctx, o.config.Topic, key, value); err != nil {
			if o.params.Logger != nil {
				o.params.Logger.Error("发送到 Kafka 失败: %v", err)
			}
		}
	}

	o.buffer = o.buffer[:0]
}

// formatJSON 格式化为 JSON
func (o *Output) formatJSON(sample metrics.Sample) ([]byte, error) {
	// 合并标签
	tags := make(map[string]string)
	for k, v := range o.config.Tags {
		tags[k] = v
	}
	for k, v := range sample.Tags {
		tags[k] = v
	}

	msg := map[string]interface{}{
		"metric":    sample.Metric.Name,
		"type":      string(sample.Metric.Type),
		"value":     sample.Value,
		"timestamp": sample.Time.UnixMilli(),
		"tags":      tags,
	}

	if o.params.ExecutionID != "" {
		msg["execution_id"] = o.params.ExecutionID
	}
	if o.params.WorkflowName != "" {
		msg["workflow"] = o.params.WorkflowName
	}

	return json.Marshal(msg)
}

// SetRunStatus 设置运行状态
func (o *Output) SetRunStatus(status output.RunStatus) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.runStatus = status
}
