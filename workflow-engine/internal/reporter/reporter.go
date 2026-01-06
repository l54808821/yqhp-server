// Package reporter 提供工作流执行引擎的报告框架。
// Requirements: 9.1.1
package reporter

import (
	"context"
	"fmt"
	"sync"

	"yqhp/workflow-engine/pkg/types"
)

// Reporter 定义了报告输出的接口。
// Requirements: 9.1.1
type Reporter interface {
	// Name 返回报告器名称。
	Name() string

	// Init 使用配置初始化报告器。
	Init(ctx context.Context, config map[string]any) error

	// Report 发送指标报告。
	Report(ctx context.Context, metrics *types.Metrics) error

	// Flush 刷新所有缓冲数据。
	Flush(ctx context.Context) error

	// Close 关闭报告器并释放资源。
	Close(ctx context.Context) error
}

// ReporterType 定义报告器类型。
type ReporterType string

const (
	// ReporterTypeConsole 输出到控制台。
	ReporterTypeConsole ReporterType = "console"
	// ReporterTypeJSON 输出到 JSON 文件。
	ReporterTypeJSON ReporterType = "json"
	// ReporterTypeCSV 输出到 CSV 文件。
	ReporterTypeCSV ReporterType = "csv"
	// ReporterTypePrometheus 推送到 Prometheus。
	ReporterTypePrometheus ReporterType = "prometheus"
	// ReporterTypeInfluxDB 写入 InfluxDB。
	ReporterTypeInfluxDB ReporterType = "influxdb"
	// ReporterTypeWebhook 发送到 Webhook URL。
	ReporterTypeWebhook ReporterType = "webhook"
)

// ReporterConfig 保存报告器的配置。
type ReporterConfig struct {
	Type    ReporterType   `yaml:"type"`
	Enabled bool           `yaml:"enabled"`
	Config  map[string]any `yaml:"config,omitempty"`
}

// ReporterFactory 创建特定类型的报告器。
type ReporterFactory func(config map[string]any) (Reporter, error)

// Registry 管理报告器的注册和创建。
// Requirements: 9.1.1
type Registry struct {
	factories map[ReporterType]ReporterFactory
	mu        sync.RWMutex
}

// NewRegistry 创建一个新的报告器注册表。
func NewRegistry() *Registry {
	return &Registry{
		factories: make(map[ReporterType]ReporterFactory),
	}
}

// Register 为指定类型注册报告器工厂。
func (r *Registry) Register(reporterType ReporterType, factory ReporterFactory) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.factories[reporterType]; exists {
		return fmt.Errorf("报告器类型已注册: %s", reporterType)
	}

	r.factories[reporterType] = factory
	return nil
}

// Unregister 移除报告器工厂。
func (r *Registry) Unregister(reporterType ReporterType) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.factories, reporterType)
}

// Create 创建指定类型的报告器。
func (r *Registry) Create(reporterType ReporterType, config map[string]any) (Reporter, error) {
	r.mu.RLock()
	factory, exists := r.factories[reporterType]
	r.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("未知的报告器类型: %s", reporterType)
	}

	return factory(config)
}

// ListTypes 返回所有已注册的报告器类型。
func (r *Registry) ListTypes() []ReporterType {
	r.mu.RLock()
	defer r.mu.RUnlock()

	types := make([]ReporterType, 0, len(r.factories))
	for t := range r.factories {
		types = append(types, t)
	}
	return types
}

// HasType 检查报告器类型是否已注册。
func (r *Registry) HasType(reporterType ReporterType) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, exists := r.factories[reporterType]
	return exists
}
