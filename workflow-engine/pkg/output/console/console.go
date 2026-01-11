package console

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"yqhp/workflow-engine/pkg/metrics"
	"yqhp/workflow-engine/pkg/output"
)

func init() {
	output.Register("console", New)
}

// Output 控制台输出
type Output struct {
	params    output.Params
	registry  *metrics.Registry
	mu        sync.Mutex
	runStatus output.RunStatus
	stopCh    chan struct{}
	wg        sync.WaitGroup

	// 统计数据
	totalRequests atomic.Int64
	successCount  atomic.Int64
	failureCount  atomic.Int64
	totalDuration atomic.Int64 // 纳秒
	startTime     time.Time
}

// New 创建控制台输出
func New(params output.Params) (output.Output, error) {
	return &Output{
		params:   params,
		registry: metrics.NewRegistry(),
		stopCh:   make(chan struct{}),
	}, nil
}

// Description 返回描述
func (o *Output) Description() string {
	return "console"
}

// Start 启动输出
func (o *Output) Start() error {
	o.startTime = time.Now()
	return nil
}

// Stop 停止输出
func (o *Output) Stop() error {
	close(o.stopCh)
	o.wg.Wait()

	// 打印最终汇总
	o.printSummary()
	return nil
}

// AddMetricSamples 添加指标样本
func (o *Output) AddMetricSamples(containers []metrics.SampleContainer) {
	for _, container := range containers {
		for _, sample := range container.GetSamples() {
			// 更新统计
			switch sample.Metric.Name {
			case "http_reqs", "iterations":
				o.totalRequests.Add(1)
			case "http_req_failed":
				if sample.Value != 0 {
					o.failureCount.Add(1)
				} else {
					o.successCount.Add(1)
				}
			case "http_req_duration":
				o.totalDuration.Add(int64(sample.Value * 1e6)) // ms -> ns
			}

			// 添加到 registry 的 sink
			m := o.registry.Get(sample.Metric.Name)
			if m == nil {
				m = o.registry.NewMetric(sample.Metric.Name, sample.Metric.Type, sample.Metric.Contains)
			}
			if m.Sink != nil {
				m.Sink.Add(sample)
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

// printSummary 打印汇总信息
func (o *Output) printSummary() {
	duration := time.Since(o.startTime).Seconds()
	total := o.totalRequests.Load()
	success := o.successCount.Load()
	failure := o.failureCount.Load()

	fmt.Println("\n========== 执行汇总 ==========")
	fmt.Printf("运行时长:     %.2fs\n", duration)
	fmt.Printf("总请求数:     %d\n", total)
	fmt.Printf("成功请求:     %d\n", success)
	fmt.Printf("失败请求:     %d\n", failure)

	if total > 0 {
		fmt.Printf("成功率:       %.2f%%\n", float64(success)/float64(total)*100)
		fmt.Printf("RPS:          %.2f\n", float64(total)/duration)
	}

	// 打印各指标的统计
	fmt.Println("\n---------- 指标详情 ----------")
	for name, m := range o.registry.All() {
		if m.Sink != nil && !m.Sink.IsEmpty() {
			stats := m.Sink.Format(duration)
			fmt.Printf("\n%s:\n", name)
			for k, v := range stats {
				if m.Contains == metrics.Time {
					fmt.Printf("  %s: %.2fms\n", k, v)
				} else {
					fmt.Printf("  %s: %.2f\n", k, v)
				}
			}
		}
	}
	fmt.Println("==============================")
}

// GetStats 获取当前统计数据
func (o *Output) GetStats() map[string]interface{} {
	duration := time.Since(o.startTime).Seconds()
	total := o.totalRequests.Load()
	success := o.successCount.Load()
	failure := o.failureCount.Load()

	stats := map[string]interface{}{
		"duration":     duration,
		"total":        total,
		"success":      success,
		"failure":      failure,
		"success_rate": 0.0,
		"rps":          0.0,
	}

	if total > 0 {
		stats["success_rate"] = float64(success) / float64(total) * 100
		stats["rps"] = float64(total) / duration
	}

	return stats
}
