// Package workflow 提供 workflow-engine 的集成
package workflow

import (
	"context"
	"sync"

	"yqhp/gulu/internal/config"
	"yqhp/workflow-engine/pkg/engine"
	"yqhp/workflow-engine/pkg/types"
)

// Engine 工作流引擎管理器
type Engine struct {
	config         *config.WorkflowEngineConfig
	embeddedEngine *engine.Engine
	started        bool
	mu             sync.RWMutex
}

var (
	globalEngine *Engine
	engineOnce   sync.Once
)

// Init 初始化工作流引擎
func Init(cfg *config.WorkflowEngineConfig) error {
	var initErr error
	engineOnce.Do(func() {
		globalEngine = &Engine{
			config: cfg,
		}

		if cfg.Embedded {
			initErr = globalEngine.startEmbeddedEngine()
		}
	})
	return initErr
}

// GetEngine 获取全局引擎实例
func GetEngine() *Engine {
	return globalEngine
}

// startEmbeddedEngine 启动内置引擎
func (e *Engine) startEmbeddedEngine() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.started {
		return nil
	}

	// 创建引擎配置
	cfg := &engine.Config{
		GRPCAddress:      e.config.GRPCAddress,
		Standalone:       e.config.Standalone,
		MaxExecutions:    e.config.MaxExecutions,
		HeartbeatTimeout: e.config.HeartbeatTimeout,
	}

	// 创建并启动引擎
	e.embeddedEngine = engine.New(cfg)
	if err := e.embeddedEngine.Start(); err != nil {
		return err
	}

	e.started = true
	return nil
}

// Stop 停止工作流引擎
func (e *Engine) Stop() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if !e.started || e.embeddedEngine == nil {
		return nil
	}

	if err := e.embeddedEngine.Stop(); err != nil {
		return err
	}

	e.started = false
	return nil
}

// IsEmbedded 是否使用内置引擎
func (e *Engine) IsEmbedded() bool {
	return e.config.Embedded
}

// GetExternalURL 获取外部引擎地址
func (e *Engine) GetExternalURL() string {
	return e.config.ExternalURL
}

// GetSlaves 获取所有 Slave
func (e *Engine) GetSlaves(ctx context.Context) ([]*types.SlaveInfo, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if e.embeddedEngine == nil {
		return nil, nil
	}

	return e.embeddedEngine.GetSlaves(ctx)
}

// SubmitWorkflow 提交工作流执行
func (e *Engine) SubmitWorkflow(ctx context.Context, workflow *types.Workflow) (string, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if e.embeddedEngine == nil {
		return "", nil
	}

	return e.embeddedEngine.SubmitWorkflow(ctx, workflow)
}

// GetExecutionStatus 获取执行状态
func (e *Engine) GetExecutionStatus(ctx context.Context, executionID string) (*types.ExecutionState, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if e.embeddedEngine == nil {
		return nil, nil
	}

	return e.embeddedEngine.GetExecutionStatus(ctx, executionID)
}

// StopExecution 停止执行
func (e *Engine) StopExecution(ctx context.Context, executionID string) error {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if e.embeddedEngine == nil {
		return nil
	}

	return e.embeddedEngine.StopExecution(ctx, executionID)
}

// GetMetrics 获取执行指标
func (e *Engine) GetMetrics(ctx context.Context, executionID string) (*types.AggregatedMetrics, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if e.embeddedEngine == nil {
		return nil, nil
	}

	return e.embeddedEngine.GetMetrics(ctx, executionID)
}
