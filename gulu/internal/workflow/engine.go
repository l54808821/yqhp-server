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

// ExecuteWorkflowBlocking 阻塞式执行工作流
func (e *Engine) ExecuteWorkflowBlocking(ctx context.Context, req *types.ExecuteWorkflowRequest) (*types.ExecuteWorkflowResponse, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if e.embeddedEngine == nil {
		return nil, nil
	}

	return e.embeddedEngine.ExecuteWorkflowBlocking(ctx, req)
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

	// 从全局配置获取输出配置
	if globalEngine != nil && globalEngine.config != nil && len(globalEngine.config.Outputs) > 0 {
		for _, out := range globalEngine.config.Outputs {
			workflow.Options.Outputs = append(workflow.Options.Outputs, types.OutputConfig{
				Type:    out.Type,
				URL:     out.URL,
				Options: out.Options,
			})
		}
	}

	return workflow
}

// convertStep 转换单个步骤
func convertStep(s Step) types.Step {
	return convertStepWithOptions(s, false)
}

// mapStepType 将前端步骤类型映射为执行器类型
func mapStepType(frontendType string) string {
	switch frontendType {
	case "database":
		return "db"
	default:
		return frontendType
	}
}

// convertStepWithOptions 转换单个步骤，支持调试模式选项
func convertStepWithOptions(s Step, debugMode bool) types.Step {
	step := types.Step{
		ID:       s.ID,
		Type:     mapStepType(s.Type),
		Name:     s.Name,
		Config:   s.Config,
		Disabled: s.Disabled,
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

	// 转换条件分支（新结构）
	if s.Type == "condition" && len(s.Branches) > 0 {
		branches := make([]types.ConditionBranch, 0, len(s.Branches))
		for _, br := range s.Branches {
			branchSteps := make([]types.Step, 0, len(br.Steps))
			for _, bs := range br.Steps {
				branchSteps = append(branchSteps, convertStepWithOptions(bs, debugMode))
			}
			branches = append(branches, types.ConditionBranch{
				ID:         br.ID,
				Name:       br.Name,
				Kind:       types.ConditionBranchKind(br.Kind),
				Expression: br.Expression,
				Steps:      branchSteps,
			})
		}
		step.Branches = branches
	}

	// 转换子步骤（用于 loop 以及旧式结构）
	if len(s.Children) > 0 {
		children := make([]types.Step, len(s.Children))
		for i, cs := range s.Children {
			children[i] = convertStepWithOptions(cs, debugMode)
		}
		step.Children = children
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

	// 转换引用工作流中的子步骤
	if s.Type == "ref_workflow" && s.Config != nil {
		if wfDef, ok := s.Config["workflow_definition"].(map[string]any); ok {
			if rawSteps, ok := wfDef["steps"].([]Step); ok {
				convertedSteps := make([]map[string]any, len(rawSteps))
				for i, cs := range rawSteps {
					engineStep := convertStepWithOptions(cs, debugMode)
					convertedSteps[i] = stepToMap(engineStep)
				}
				wfDef["steps"] = convertedSteps
			}
		}
	}

	// 转换前置处理器
	if len(s.PreProcessors) > 0 {
		preProcessors := make([]types.Processor, len(s.PreProcessors))
		for i, p := range s.PreProcessors {
			preProcessors[i] = types.Processor{
				ID:      p.ID,
				Type:    p.Type,
				Enabled: p.Enabled,
				Name:    p.Name,
				Config:  p.Config,
			}
		}
		step.PreProcessors = preProcessors
	}

	// 转换后置处理器
	if len(s.PostProcessors) > 0 {
		postProcessors := make([]types.Processor, len(s.PostProcessors))
		for i, p := range s.PostProcessors {
			postProcessors[i] = types.Processor{
				ID:      p.ID,
				Type:    p.Type,
				Enabled: p.Enabled,
				Name:    p.Name,
				Config:  p.Config,
			}
		}
		step.PostProcessors = postProcessors
	}

	return step
}

// stepToMap 将 types.Step 转换为 map 以便 JSON 序列化后被引擎执行器解析
func stepToMap(s types.Step) map[string]any {
	m := map[string]any{
		"id":   s.ID,
		"type": s.Type,
		"name": s.Name,
	}
	if s.Disabled {
		m["disabled"] = true
	}
	if s.Config != nil {
		m["config"] = s.Config
	}
	if s.Timeout > 0 {
		m["timeout"] = s.Timeout.String()
	}
	if s.OnError != "" {
		m["on_error"] = string(s.OnError)
	}
	if len(s.Branches) > 0 {
		branches := make([]map[string]any, len(s.Branches))
		for i, br := range s.Branches {
			brMap := map[string]any{
				"id":   br.ID,
				"kind": string(br.Kind),
			}
			if br.Name != "" {
				brMap["name"] = br.Name
			}
			if br.Expression != "" {
				brMap["expression"] = br.Expression
			}
			if len(br.Steps) > 0 {
				brSteps := make([]map[string]any, len(br.Steps))
				for j, bs := range br.Steps {
					brSteps[j] = stepToMap(bs)
				}
				brMap["steps"] = brSteps
			}
			branches[i] = brMap
		}
		m["branches"] = branches
	}
	if len(s.Children) > 0 {
		children := make([]map[string]any, len(s.Children))
		for i, cs := range s.Children {
			children[i] = stepToMap(cs)
		}
		m["children"] = children
	}
	if s.Loop != nil {
		loopMap := map[string]any{
			"mode": s.Loop.Mode,
		}
		if s.Loop.Count > 0 {
			loopMap["count"] = s.Loop.Count
		}
		if s.Loop.Items != nil {
			loopMap["items"] = s.Loop.Items
		}
		if s.Loop.ItemVar != "" {
			loopMap["item_var"] = s.Loop.ItemVar
		}
		if s.Loop.Condition != "" {
			loopMap["condition"] = s.Loop.Condition
		}
		if s.Loop.MaxIterations > 0 {
			loopMap["max_iterations"] = s.Loop.MaxIterations
		}
		if len(s.Loop.Steps) > 0 {
			loopSteps := make([]map[string]any, len(s.Loop.Steps))
			for i, ls := range s.Loop.Steps {
				loopSteps[i] = stepToMap(ls)
			}
			loopMap["steps"] = loopSteps
		}
		m["loop"] = loopMap
	}
	if len(s.PreProcessors) > 0 {
		procs := make([]map[string]any, len(s.PreProcessors))
		for i, p := range s.PreProcessors {
			procs[i] = map[string]any{"id": p.ID, "type": p.Type, "enabled": p.Enabled, "name": p.Name, "config": p.Config}
		}
		m["preProcessors"] = procs
	}
	if len(s.PostProcessors) > 0 {
		procs := make([]map[string]any, len(s.PostProcessors))
		for i, p := range s.PostProcessors {
			procs[i] = map[string]any{"id": p.ID, "type": p.Type, "enabled": p.Enabled, "name": p.Name, "config": p.Config}
		}
		m["postProcessors"] = procs
	}
	return m
}
