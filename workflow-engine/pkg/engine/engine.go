// Package engine 提供工作流引擎的公共 API
package engine

import (
	"context"
	"fmt"
	"sync"
	"time"

	"yqhp/workflow-engine/internal/master"
	"yqhp/workflow-engine/pkg/types"
)

// Config 引擎配置
type Config struct {
	// HTTPAddress HTTP 服务地址
	HTTPAddress string
	// Standalone 独立模式（无需 Slave 即可执行）
	Standalone bool
	// MaxExecutions 最大并发执行数
	MaxExecutions int
	// HeartbeatTimeout 心跳超时
	HeartbeatTimeout time.Duration
}

// DefaultConfig 返回默认配置
func DefaultConfig() *Config {
	return &Config{
		HTTPAddress:      ":8080",
		Standalone:       true,
		MaxExecutions:    100,
		HeartbeatTimeout: 30 * time.Second,
	}
}

// Engine 工作流引擎
type Engine struct {
	config   *Config
	master   *master.WorkflowMaster
	registry master.SlaveRegistry
	started  bool
	mu       sync.RWMutex
}

// New 创建新的工作流引擎
func New(cfg *Config) *Engine {
	if cfg == nil {
		cfg = DefaultConfig()
	}
	return &Engine{
		config: cfg,
	}
}

// Start 启动引擎
func (e *Engine) Start() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.started {
		return nil
	}

	// 创建 Master 配置
	masterCfg := &master.Config{
		Address:                 e.config.HTTPAddress,
		HeartbeatTimeout:        e.config.HeartbeatTimeout,
		HealthCheckInterval:     10 * time.Second,
		StandaloneMode:          e.config.Standalone,
		MaxConcurrentExecutions: e.config.MaxExecutions,
	}

	// 创建注册中心、调度器和聚合器
	e.registry = master.NewInMemorySlaveRegistry()
	scheduler := master.NewWorkflowScheduler(e.registry)
	aggregator := master.NewDefaultMetricsAggregator()

	// 创建并启动 Master
	e.master = master.NewWorkflowMaster(masterCfg, e.registry, scheduler, aggregator)

	ctx := context.Background()
	if err := e.master.Start(ctx); err != nil {
		return fmt.Errorf("启动 Master 失败: %w", err)
	}

	e.started = true
	return nil
}

// Stop 停止引擎
func (e *Engine) Stop() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if !e.started {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 停止 Master
	if e.master != nil {
		if err := e.master.Stop(ctx); err != nil {
			return fmt.Errorf("停止 Master 失败: %w", err)
		}
	}

	e.started = false
	return nil
}

// IsRunning 是否正在运行
func (e *Engine) IsRunning() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.started
}

// GetSlaves 获取所有 Slave
func (e *Engine) GetSlaves(ctx context.Context) ([]*types.SlaveInfo, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if e.master == nil {
		return nil, fmt.Errorf("引擎未启动")
	}

	return e.master.GetSlaves(ctx)
}

// SubmitWorkflow 提交工作流执行
func (e *Engine) SubmitWorkflow(ctx context.Context, workflow *types.Workflow) (string, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if e.master == nil {
		return "", fmt.Errorf("引擎未启动")
	}

	return e.master.SubmitWorkflow(ctx, workflow)
}

// GetExecutionStatus 获取执行状态
func (e *Engine) GetExecutionStatus(ctx context.Context, executionID string) (*types.ExecutionState, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if e.master == nil {
		return nil, fmt.Errorf("引擎未启动")
	}

	return e.master.GetExecutionStatus(ctx, executionID)
}

// StopExecution 停止执行
func (e *Engine) StopExecution(ctx context.Context, executionID string) error {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if e.master == nil {
		return fmt.Errorf("引擎未启动")
	}

	return e.master.StopExecution(ctx, executionID)
}

// GetMetrics 获取执行指标
func (e *Engine) GetMetrics(ctx context.Context, executionID string) (*types.AggregatedMetrics, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if e.master == nil {
		return nil, fmt.Errorf("引擎未启动")
	}

	return e.master.GetMetrics(ctx, executionID)
}

// ListExecutions 列出所有执行
func (e *Engine) ListExecutions(ctx context.Context) ([]*types.ExecutionState, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if e.master == nil {
		return nil, fmt.Errorf("引擎未启动")
	}

	return e.master.ListExecutions(ctx)
}
