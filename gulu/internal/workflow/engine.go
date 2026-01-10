// Package workflow 提供 workflow-engine 的集成
package workflow

import (
	"context"
	"sync"
	"time"

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

// ConvertToEngineWorkflow 将 gulu 的工作流定义转换为 workflow-engine 的工作流类型
func ConvertToEngineWorkflow(def *WorkflowDefinition, executionID string) *types.Workflow {
	return ConvertToEngineWorkflowWithOptions(def, executionID, false)
}

// ConvertToEngineWorkflowForDebug 将工作流定义转换为调试模式的工作流
// 调试模式下，所有步骤失败后立即停止（abort 策略）
func ConvertToEngineWorkflowForDebug(def *WorkflowDefinition, executionID string) *types.Workflow {
	return ConvertToEngineWorkflowWithOptions(def, executionID, true)
}

// ConvertToEngineWorkflowWithOptions 将工作流定义转换为引擎工作流，支持选项
func ConvertToEngineWorkflowWithOptions(def *WorkflowDefinition, executionID string, debugMode bool) *types.Workflow {
	if def == nil {
		return nil
	}

	// 转换步骤
	steps := make([]types.Step, len(def.Steps))
	for i, s := range def.Steps {
		steps[i] = convertStepWithOptions(s, debugMode)
	}

	// 创建工作流
	workflow := &types.Workflow{
		ID:          executionID,
		Name:        def.Name,
		Description: def.Description,
		Variables:   def.Variables,
		Steps:       steps,
		Options: types.ExecutionOptions{
			VUs:           1, // 默认1个虚拟用户
			Iterations:    1, // 默认执行1次
			ExecutionMode: "constant-vus",
		},
	}

	return workflow
}

// convertStep 转换单个步骤
func convertStep(s Step) types.Step {
	return convertStepWithOptions(s, false)
}

// convertStepWithOptions 转换单个步骤，支持调试模式选项
func convertStepWithOptions(s Step, debugMode bool) types.Step {
	step := types.Step{
		ID:     s.ID,
		Type:   s.Type,
		Name:   s.Name,
		Config: s.Config,
	}

	// 转换超时
	if s.Timeout != "" {
		if d, err := time.ParseDuration(s.Timeout); err == nil {
			step.Timeout = d
		}
	}

	// 转换错误策略
	// 调试模式下强制使用 abort 策略，失败立即停止
	if debugMode {
		step.OnError = types.ErrorStrategyAbort
	} else {
		switch s.OnError {
		case "continue":
			step.OnError = types.ErrorStrategyContinue
		case "skip":
			step.OnError = types.ErrorStrategySkip
		case "retry":
			step.OnError = types.ErrorStrategyRetry
		default:
			step.OnError = types.ErrorStrategyAbort
		}
	}

	// 转换条件配置
	if s.Condition != nil {
		thenSteps := make([]types.Step, len(s.Condition.Then))
		for i, ts := range s.Condition.Then {
			thenSteps[i] = convertStepWithOptions(ts, debugMode)
		}
		elseSteps := make([]types.Step, len(s.Condition.Else))
		for i, es := range s.Condition.Else {
			elseSteps[i] = convertStepWithOptions(es, debugMode)
		}
		step.Condition = &types.Condition{
			Expression: s.Condition.Expression,
			Then:       thenSteps,
			Else:       elseSteps,
		}
	}

	// 转换循环配置
	if s.Loop != nil {
		// 优先使用 Children 字段，如果没有则使用 Loop.Steps
		var loopSteps []types.Step
		if len(s.Children) > 0 {
			loopSteps = make([]types.Step, len(s.Children))
			for i, cs := range s.Children {
				loopSteps[i] = convertStepWithOptions(cs, debugMode)
			}
		} else if len(s.Loop.Steps) > 0 {
			loopSteps = make([]types.Step, len(s.Loop.Steps))
			for i, ls := range s.Loop.Steps {
				loopSteps[i] = convertStepWithOptions(ls, debugMode)
			}
		}

		step.Loop = &types.Loop{
			Mode:              s.Loop.Mode,
			Count:             s.Loop.Count,
			Items:             s.Loop.Items,
			ItemVar:           s.Loop.ItemVar,
			Condition:         s.Loop.Condition,
			MaxIterations:     s.Loop.MaxIterations,
			BreakCondition:    s.Loop.BreakCondition,
			ContinueCondition: s.Loop.ContinueCondition,
			Steps:             loopSteps,
		}
	}

	return step
}
