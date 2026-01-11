package output

import (
	"context"

	"yqhp/workflow-engine/pkg/metrics"
)

// Output 定义输出插件接口
type Output interface {
	// Description 返回输出插件的描述
	Description() string

	// Start 启动输出插件
	Start() error

	// Stop 停止输出插件
	Stop() error

	// AddMetricSamples 添加指标样本
	AddMetricSamples(samples []metrics.SampleContainer)

	// SetRunStatus 设置运行状态（用于最终汇总）
	SetRunStatus(status RunStatus)
}

// RunStatus 表示测试运行状态
type RunStatus struct {
	Duration   float64 // 运行时长（秒）
	Iterations int64   // 总迭代次数
	VUs        int     // VU 数量
	Status     string  // 状态：running, completed, failed, aborted
	Error      error   // 错误信息
}

// Params 是创建 Output 时的参数
type Params struct {
	// OutputType 输出类型
	OutputType string

	// ConfigArgument 配置参数（如 URL、文件路径等）
	ConfigArgument string

	// Logger 日志记录器
	Logger Logger

	// ExecutionID 执行 ID
	ExecutionID string

	// WorkflowName 工作流名称
	WorkflowName string

	// Tags 全局标签
	Tags map[string]string
}

// Logger 日志接口
type Logger interface {
	Debug(format string, args ...interface{})
	Info(format string, args ...interface{})
	Warn(format string, args ...interface{})
	Error(format string, args ...interface{})
}

// Factory 是创建 Output 的工厂函数类型
type Factory func(params Params) (Output, error)

// registry 存储已注册的输出工厂
var registry = make(map[string]Factory)

// Register 注册输出工厂
func Register(name string, factory Factory) {
	registry[name] = factory
}

// Get 获取输出工厂
func Get(name string) (Factory, bool) {
	f, ok := registry[name]
	return f, ok
}

// List 列出所有已注册的输出类型
func List() []string {
	names := make([]string, 0, len(registry))
	for name := range registry {
		names = append(names, name)
	}
	return names
}

// Create 创建输出实例
func Create(ctx context.Context, outputType string, params Params) (Output, error) {
	factory, ok := Get(outputType)
	if !ok {
		return nil, &UnknownOutputError{Type: outputType}
	}
	params.OutputType = outputType
	return factory(params)
}

// UnknownOutputError 未知输出类型错误
type UnknownOutputError struct {
	Type string
}

func (e *UnknownOutputError) Error() string {
	return "未知的输出类型: " + e.Type
}
