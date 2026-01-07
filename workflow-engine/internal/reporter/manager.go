package reporter

import (
	"context"
	"fmt"
	"sync"

	"yqhp/workflow-engine/pkg/types"
)

// Manager manages multiple reporters for an execution.
// Requirements: 9.1.1
type Manager struct {
	registry  *Registry
	reporters []Reporter
	mu        sync.RWMutex
	started   bool
}

// NewManager creates a new reporter manager.
func NewManager(registry *Registry) *Manager {
	if registry == nil {
		registry = NewRegistry()
	}
	return &Manager{
		registry:  registry,
		reporters: make([]Reporter, 0),
	}
}

// AddReporter adds a reporter to the manager.
func (m *Manager) AddReporter(reporter Reporter) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.started {
		return fmt.Errorf("管理器启动后无法添加报告器")
	}

	m.reporters = append(m.reporters, reporter)
	return nil
}

// AddReporterFromConfig creates and adds a reporter from configuration.
func (m *Manager) AddReporterFromConfig(ctx context.Context, config *ReporterConfig) error {
	if !config.Enabled {
		return nil
	}

	reporter, err := m.registry.Create(config.Type, config.Config)
	if err != nil {
		return fmt.Errorf("创建报告器 %s 失败: %w", config.Type, err)
	}

	if err := reporter.Init(ctx, config.Config); err != nil {
		return fmt.Errorf("初始化报告器 %s 失败: %w", config.Type, err)
	}

	return m.AddReporter(reporter)
}

// Start marks the manager as started.
func (m *Manager) Start(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.started {
		return fmt.Errorf("管理器已启动")
	}

	m.started = true
	return nil
}

// Report sends metrics to all reporters.
func (m *Manager) Report(ctx context.Context, metrics *types.Metrics) error {
	m.mu.RLock()
	reporters := make([]Reporter, len(m.reporters))
	copy(reporters, m.reporters)
	m.mu.RUnlock()

	var errs []error
	for _, reporter := range reporters {
		if err := reporter.Report(ctx, metrics); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", reporter.Name(), err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("报告错误: %v", errs)
	}
	return nil
}

// Flush flushes all reporters.
func (m *Manager) Flush(ctx context.Context) error {
	m.mu.RLock()
	reporters := make([]Reporter, len(m.reporters))
	copy(reporters, m.reporters)
	m.mu.RUnlock()

	var errs []error
	for _, reporter := range reporters {
		if err := reporter.Flush(ctx); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", reporter.Name(), err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("刷新错误: %v", errs)
	}
	return nil
}

// Close closes all reporters.
func (m *Manager) Close(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var errs []error
	for _, reporter := range m.reporters {
		if err := reporter.Close(ctx); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", reporter.Name(), err))
		}
	}

	m.reporters = nil
	m.started = false

	if len(errs) > 0 {
		return fmt.Errorf("关闭错误: %v", errs)
	}
	return nil
}

// GetReporters returns all registered reporters.
func (m *Manager) GetReporters() []Reporter {
	m.mu.RLock()
	defer m.mu.RUnlock()

	reporters := make([]Reporter, len(m.reporters))
	copy(reporters, m.reporters)
	return reporters
}

// GetReporterCount returns the number of registered reporters.
func (m *Manager) GetReporterCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.reporters)
}

// IsStarted returns whether the manager has started.
func (m *Manager) IsStarted() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.started
}
