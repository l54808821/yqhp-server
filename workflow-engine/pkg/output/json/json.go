package json

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"yqhp/workflow-engine/pkg/metrics"
	"yqhp/workflow-engine/pkg/output"
)

func init() {
	output.Register("json", New)
}

// Output JSON 文件输出
type Output struct {
	params    output.Params
	file      *os.File
	writer    *bufio.Writer
	encoder   *json.Encoder
	mu        sync.Mutex
	runStatus output.RunStatus
}

// New 创建 JSON 输出
func New(params output.Params) (output.Output, error) {
	return &Output{
		params: params,
	}, nil
}

// Description 返回描述
func (o *Output) Description() string {
	return fmt.Sprintf("json (%s)", o.params.ConfigArgument)
}

// Start 启动输出
func (o *Output) Start() error {
	o.mu.Lock()
	defer o.mu.Unlock()

	filename := o.params.ConfigArgument
	if filename == "" {
		filename = fmt.Sprintf("metrics_%s.json", time.Now().Format("20060102_150405"))
	}

	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("创建 JSON 文件失败: %w", err)
	}

	o.file = file
	o.writer = bufio.NewWriter(file)
	o.encoder = json.NewEncoder(o.writer)

	// 写入开始标记
	_, _ = o.writer.WriteString("[\n")

	return nil
}

// Stop 停止输出
func (o *Output) Stop() error {
	o.mu.Lock()
	defer o.mu.Unlock()

	if o.file == nil {
		return nil
	}

	// 写入结束标记
	_, _ = o.writer.WriteString("\n]")
	_ = o.writer.Flush()
	return o.file.Close()
}

// AddMetricSamples 添加指标样本
func (o *Output) AddMetricSamples(containers []metrics.SampleContainer) {
	o.mu.Lock()
	defer o.mu.Unlock()

	if o.encoder == nil {
		return
	}

	for _, container := range containers {
		for _, sample := range container.GetSamples() {
			entry := map[string]interface{}{
				"type":   "Point",
				"metric": sample.Metric.Name,
				"data": map[string]interface{}{
					"time":  sample.Time.UnixMilli(),
					"value": sample.Value,
					"tags":  sample.Tags,
				},
			}
			if err := o.encoder.Encode(entry); err != nil && o.params.Logger != nil {
				o.params.Logger.Error("写入 JSON 失败: %v", err)
			}
		}
	}
}

// SetRunStatus 设置运行状态
func (o *Output) SetRunStatus(status output.RunStatus) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.runStatus = status
}
