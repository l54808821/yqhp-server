// Package workflow 提供 workflow-engine 的集成
package workflow

import (
	"context"
	"sync"

	"yqhp/gulu/internal/config"
	"yqhp/workflow-engine/pkg/engine"
	"yqhp/workflow-engine/pkg/logger"
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

		// 根据配置启用调试日志
		if cfg.Debug {
			logger.EnableDebug()
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
		HTTPAddress:      e.config.HTTPAddress,
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

// GetSlaveStatus 获取单个 Slave 的运行时状态
func (e *Engine) GetSlaveStatus(ctx context.Context, slaveID string) (*types.SlaveStatus, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if e.embeddedEngine == nil {
		return nil, nil
	}

	return e.embeddedEngine.GetSlaveStatus(ctx, slaveID)
}

// WatchSlaves 监听 Slave 事件
func (e *Engine) WatchSlaves(ctx context.Context) (<-chan *types.SlaveEvent, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if e.embeddedEngine == nil {
		return nil, nil
	}

	return e.embeddedEngine.WatchSlaves(ctx)
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

// GetPerformanceReport retrieves the final performance report from the engine.
func (e *Engine) GetPerformanceReport(ctx context.Context, executionID string) (*types.PerformanceTestReport, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if e.embeddedEngine == nil {
		return nil, nil
	}

	return e.embeddedEngine.GetPerformanceReport(ctx, executionID)
}

// GetRealtimeMetrics retrieves realtime metrics snapshot from the engine.
func (e *Engine) GetRealtimeMetrics(ctx context.Context, executionID string) (interface{}, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if e.embeddedEngine == nil {
		return nil, nil
	}

	return e.embeddedEngine.GetRealtimeMetrics(ctx, executionID)
}

// GetTimeSeries retrieves time-series data from the engine.
func (e *Engine) GetTimeSeries(ctx context.Context, executionID string) (interface{}, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if e.embeddedEngine == nil {
		return nil, nil
	}

	return e.embeddedEngine.GetTimeSeries(ctx, executionID)
}

// ScaleVUs adjusts the VU count for a running execution.
func (e *Engine) ScaleVUs(ctx context.Context, executionID string, vus int) error {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if e.embeddedEngine == nil {
		return nil
	}

	return e.embeddedEngine.ScaleVUs(ctx, executionID, vus)
}

// PauseExecution pauses a running execution.
func (e *Engine) PauseExecution(ctx context.Context, executionID string) error {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if e.embeddedEngine == nil {
		return nil
	}

	return e.embeddedEngine.PauseExecution(ctx, executionID)
}

// ResumeExecution resumes a paused execution.
func (e *Engine) ResumeExecution(ctx context.Context, executionID string) error {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if e.embeddedEngine == nil {
		return nil
	}

	return e.embeddedEngine.ResumeExecution(ctx, executionID)
}

// ExecuteWorkflowBlocking 阻塞式执行工作流
func (e *Engine) ExecuteWorkflowBlocking(ctx context.Context, req *types.ExecuteWorkflowRequest) (*types.ExecuteWorkflowResponse, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if e.embeddedEngine == nil {
		return nil, nil
	}

	return e.embeddedEngine.ExecuteWorkflowBlocking(ctx, req)
}

// ConvertToEngineWorkflow 将 gulu 的工作流定义转换为 workflow-engine 的工作流类型。
// 由于 WorkflowDefinition 已直接使用 types.Step，此函数仅组装 Workflow 外壳。
func ConvertToEngineWorkflow(def *WorkflowDefinition, executionID string) *types.Workflow {
	return convertToWorkflow(def, executionID, false)
}

// ConvertToEngineWorkflowForDebug 转换为调试模式的工作流：
// 所有步骤强制使用 abort 错误策略，失败立即停止。
func ConvertToEngineWorkflowForDebug(def *WorkflowDefinition, executionID string) *types.Workflow {
	return convertToWorkflow(def, executionID, true)
}

func convertToWorkflow(def *WorkflowDefinition, executionID string, debugMode bool) *types.Workflow {
	if def == nil {
		return nil
	}

	if debugMode {
		applyDebugErrorStrategy(def.Steps)
	}

	wf := &types.Workflow{
		ID:          executionID,
		Name:        def.Name,
		Description: def.Description,
		Variables:   def.Variables,
		Steps:       def.Steps,
		Options: types.ExecutionOptions{
			VUs:           1,
			Iterations:    1,
			ExecutionMode: "constant-vus",
		},
	}

	if globalEngine != nil && globalEngine.config != nil && len(globalEngine.config.Outputs) > 0 {
		for _, out := range globalEngine.config.Outputs {
			wf.Options.Outputs = append(wf.Options.Outputs, types.OutputConfig{
				Type:    out.Type,
				URL:     out.URL,
				Options: out.Options,
			})
		}
	}

	return wf
}

// applyDebugErrorStrategy 递归地将所有步骤的错误策略设为 abort
func applyDebugErrorStrategy(steps []types.Step) {
	for i := range steps {
		steps[i].OnError = types.ErrorStrategyAbort
		applyDebugErrorStrategy(steps[i].Children)
		if steps[i].Loop != nil {
			applyDebugErrorStrategy(steps[i].Loop.Steps)
		}
		for j := range steps[i].Branches {
			applyDebugErrorStrategy(steps[i].Branches[j].Steps)
		}
	}
}
