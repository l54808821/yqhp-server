package output

import (
	"context"
	"fmt"
	"time"

	"yqhp/workflow-engine/pkg/metrics"
	"yqhp/workflow-engine/pkg/types"
)

// CreateOutputsFromConfig 根据配置创建输出实例
func CreateOutputsFromConfig(ctx context.Context, configs []types.OutputConfig, params Params) ([]Output, error) {
	outputs := make([]Output, 0, len(configs))

	for _, cfg := range configs {
		// 构建配置参数
		configArg := cfg.URL
		if configArg == "" && len(cfg.Options) > 0 {
			// 从 options 构建配置字符串
			for k, v := range cfg.Options {
				if configArg == "" {
					configArg = fmt.Sprintf("%s=%s", k, v)
				} else {
					configArg = fmt.Sprintf("%s&%s=%s", configArg, k, v)
				}
			}
		}

		p := Params{
			OutputType:     cfg.Type,
			ConfigArgument: configArg,
			Logger:         params.Logger,
			ExecutionID:    params.ExecutionID,
			WorkflowName:   params.WorkflowName,
			Tags:           params.Tags,
		}

		out, err := Create(ctx, cfg.Type, p)
		if err != nil {
			// 关闭已创建的输出
			for _, o := range outputs {
				_ = o.Stop()
			}
			return nil, fmt.Errorf("创建输出 %s 失败: %w", cfg.Type, err)
		}
		outputs = append(outputs, out)
	}

	return outputs, nil
}

// SampleEmitter 用于发送指标样本
type SampleEmitter struct {
	samplesChan chan metrics.SampleContainer
	registry    *metrics.Registry
	tags        map[string]string
}

// NewSampleEmitter 创建样本发送器
func NewSampleEmitter(samplesChan chan metrics.SampleContainer, tags map[string]string) *SampleEmitter {
	return &SampleEmitter{
		samplesChan: samplesChan,
		registry:    metrics.NewRegistry(),
		tags:        tags,
	}
}

// Emit 发送单个样本
func (e *SampleEmitter) Emit(metricName string, metricType metrics.MetricType, value float64, tags map[string]string) {
	if e.samplesChan == nil {
		return
	}

	// 获取或创建指标
	m := e.registry.Get(metricName)
	if m == nil {
		m = e.registry.NewMetric(metricName, metricType, metrics.Default)
	}

	// 合并标签
	allTags := make(map[string]string)
	for k, v := range e.tags {
		allTags[k] = v
	}
	for k, v := range tags {
		allTags[k] = v
	}

	sample := metrics.Sample{
		Metric: m,
		Time:   time.Now(),
		Value:  value,
		Tags:   allTags,
	}

	// 非阻塞发送
	select {
	case e.samplesChan <- metrics.Samples{sample}:
	default:
		// 通道满了，丢弃样本
	}
}

// EmitCounter 发送计数器样本
func (e *SampleEmitter) EmitCounter(name string, value float64, tags map[string]string) {
	e.Emit(name, metrics.Counter, value, tags)
}

// EmitGauge 发送仪表盘样本
func (e *SampleEmitter) EmitGauge(name string, value float64, tags map[string]string) {
	e.Emit(name, metrics.Gauge, value, tags)
}

// EmitRate 发送比率样本（value != 0 表示成功）
func (e *SampleEmitter) EmitRate(name string, success bool, tags map[string]string) {
	value := 0.0
	if success {
		value = 1.0
	}
	e.Emit(name, metrics.Rate, value, tags)
}

// EmitTrend 发送趋势样本（如延迟）
func (e *SampleEmitter) EmitTrend(name string, value float64, tags map[string]string) {
	e.Emit(name, metrics.Trend, value, tags)
}

// EmitHTTPMetrics 发送 HTTP 请求相关的指标
func (e *SampleEmitter) EmitHTTPMetrics(
	duration time.Duration,
	success bool,
	statusCode int,
	method string,
	url string,
	tags map[string]string,
) {
	// 合并标签
	allTags := make(map[string]string)
	for k, v := range tags {
		allTags[k] = v
	}
	allTags["method"] = method
	allTags["url"] = url
	allTags["status"] = fmt.Sprintf("%d", statusCode)

	// 请求计数
	e.EmitCounter("http_reqs", 1, allTags)

	// 请求时长
	e.EmitTrend("http_req_duration", float64(duration.Milliseconds()), allTags)

	// 请求失败率
	e.EmitRate("http_req_failed", !success, allTags)

	// 状态码分布
	e.EmitCounter(fmt.Sprintf("http_req_status_%d", statusCode), 1, allTags)
}

// EmitIterationMetrics 发送迭代相关的指标
func (e *SampleEmitter) EmitIterationMetrics(duration time.Duration, success bool, vuID int, iteration int) {
	tags := map[string]string{
		"vu":        fmt.Sprintf("%d", vuID),
		"iteration": fmt.Sprintf("%d", iteration),
	}

	// 迭代计数
	e.EmitCounter("iterations", 1, tags)

	// 迭代时长
	e.EmitTrend("iteration_duration", float64(duration.Milliseconds()), tags)

	// 迭代成功率
	e.EmitRate("iteration_success", success, tags)
}

// Close 关闭发送器
func (e *SampleEmitter) Close() {
	// 不关闭 channel，由 manager 负责
}
