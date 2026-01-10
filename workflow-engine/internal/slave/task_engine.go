package slave

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"yqhp/workflow-engine/internal/executor"
	"yqhp/workflow-engine/internal/hook"
	"yqhp/workflow-engine/pkg/types"
)

// TaskEngine 处理工作流任务执行。
// Requirements: 6.1, 6.3, 6.4
type TaskEngine struct {
	registry   *executor.Registry
	hookRunner *hook.Runner
	maxVUs     int

	// VU 管理
	vuPool     *VUPool
	activeVUs  atomic.Int32
	iterations atomic.Int64

	// 指标收集
	collector *MetricsCollector

	// 状态
	running atomic.Bool
	mu      sync.RWMutex
}

// NewTaskEngine 创建一个新的任务引擎。
func NewTaskEngine(registry *executor.Registry, maxVUs int) *TaskEngine {
	return &TaskEngine{
		registry:   registry,
		hookRunner: hook.NewRunner(registry),
		maxVUs:     maxVUs,
		vuPool:     NewVUPool(maxVUs),
		collector:  NewMetricsCollector(),
	}
}

// Execute 执行任务并返回结果。
// Requirements: 6.1, 6.3, 6.4
func (e *TaskEngine) Execute(ctx context.Context, task *types.Task) (*types.TaskResult, error) {
	if task == nil || task.Workflow == nil {
		return nil, fmt.Errorf("无效任务: task 或 workflow 为空")
	}

	fmt.Printf("[TaskEngine.Execute] 开始执行任务: %s, 工作流: %s\n", task.ID, task.Workflow.Name)
	fmt.Printf("[TaskEngine.Execute] 步骤数: %d\n", len(task.Workflow.Steps))
	for i, step := range task.Workflow.Steps {
		fmt.Printf("[TaskEngine.Execute] 步骤[%d]: id=%s, type=%s, name=%s\n", i, step.ID, step.Type, step.Name)
	}

	e.running.Store(true)
	defer e.running.Store(false)

	result := &types.TaskResult{
		TaskID:      task.ID,
		ExecutionID: task.ExecutionID,
		Status:      types.ExecutionStatusRunning,
		Errors:      make([]types.ExecutionError, 0),
	}

	// 根据工作流选项确定执行参数
	opts := task.Workflow.Options
	vus := e.calculateVUs(opts, task.Segment)
	iterations := e.calculateIterations(opts, task.Segment)
	duration := opts.Duration

	fmt.Printf("[TaskEngine.Execute] 执行参数: vus=%d, iterations=%d, duration=%v, mode=%s\n", vus, iterations, duration, opts.ExecutionMode)

	// 根据模式执行
	var execErr error
	switch opts.ExecutionMode {
	case types.ModeConstantVUs, "":
		fmt.Printf("[TaskEngine.Execute] 使用 ConstantVUs 模式执行\n")
		execErr = e.executeConstantVUs(ctx, task, vus, duration, iterations)
	case types.ModeRampingVUs:
		execErr = e.executeRampingVUs(ctx, task, opts.Stages)
	case types.ModePerVUIterations:
		execErr = e.executePerVUIterations(ctx, task, vus, iterations)
	case types.ModeSharedIterations:
		execErr = e.executeSharedIterations(ctx, task, vus, iterations)
	default:
		execErr = e.executeConstantVUs(ctx, task, vus, duration, iterations)
	}

	fmt.Printf("[TaskEngine.Execute] 执行完成, execErr=%v\n", execErr)

	if execErr != nil {
		result.Status = types.ExecutionStatusFailed
		result.Errors = append(result.Errors, types.ExecutionError{
			Code:      types.ErrCodeExecution,
			Message:   execErr.Error(),
			Timestamp: time.Now(),
		})
	} else {
		result.Status = types.ExecutionStatusCompleted
	}

	// 收集最终指标
	result.Metrics = e.collector.GetMetrics()

	// 设置迭代次数
	result.Iterations = e.iterations.Load()

	fmt.Printf("[TaskEngine.Execute] 返回结果: status=%s, iterations=%d\n", result.Status, result.Iterations)

	return result, execErr
}

// Stop 停止任务引擎。
func (e *TaskEngine) Stop(ctx context.Context) error {
	e.running.Store(false)
	e.vuPool.StopAll()
	return nil
}

// GetMetrics 返回当前指标。
func (e *TaskEngine) GetMetrics() *types.SlaveMetrics {
	return &types.SlaveMetrics{
		ActiveVUs:  int(e.activeVUs.Load()),
		Throughput: e.collector.GetThroughput(),
	}
}

// GetCurrentMetrics 返回当前收集的指标数据。
func (e *TaskEngine) GetCurrentMetrics() *types.Metrics {
	return e.collector.GetMetrics()
}

// GetIterations 返回当前迭代次数。
func (e *TaskEngine) GetIterations() int64 {
	return e.iterations.Load()
}

// GetActiveVUs 返回当前活跃的 VU 数量。
func (e *TaskEngine) GetActiveVUs() int {
	return int(e.activeVUs.Load())
}

// calculateVUs 计算此分段的 VU 数量。
func (e *TaskEngine) calculateVUs(opts types.ExecutionOptions, segment types.ExecutionSegment) int {
	totalVUs := opts.VUs
	if totalVUs <= 0 {
		totalVUs = 1
	}

	// 计算此分段的 VU 数量
	segmentSize := segment.End - segment.Start
	vus := int(float64(totalVUs) * segmentSize)
	if vus < 1 {
		vus = 1
	}

	// 限制在最大 VU 数以内
	if vus > e.maxVUs {
		vus = e.maxVUs
	}

	return vus
}

// calculateIterations 计算此分段的迭代次数。
func (e *TaskEngine) calculateIterations(opts types.ExecutionOptions, segment types.ExecutionSegment) int {
	totalIterations := opts.Iterations
	if totalIterations <= 0 {
		return 0 // 基于时长的执行
	}

	// 计算此分段的迭代次数
	segmentSize := segment.End - segment.Start
	iterations := int(float64(totalIterations) * segmentSize)
	if iterations < 1 {
		iterations = 1
	}

	return iterations
}

// executeConstantVUs 使用固定数量的 VU 执行。
// Requirements: 6.1.1
func (e *TaskEngine) executeConstantVUs(ctx context.Context, task *types.Task, vus int, duration time.Duration, iterations int) error {
	fmt.Printf("[executeConstantVUs] 开始: vus=%d, duration=%v, iterations=%d\n", vus, duration, iterations)

	var wg sync.WaitGroup
	errChan := make(chan error, vus)

	// 如果指定了时长，创建带超时的上下文
	execCtx := ctx
	var cancel context.CancelFunc
	if duration > 0 {
		execCtx, cancel = context.WithTimeout(ctx, duration)
		defer cancel()
	}

	// 启动 VU
	for i := 0; i < vus; i++ {
		vu := e.vuPool.Acquire(i)
		if vu == nil {
			fmt.Printf("[executeConstantVUs] 无法获取 VU %d\n", i)
			continue
		}

		wg.Add(1)
		e.activeVUs.Add(1)

		go func(vu *types.VirtualUser) {
			defer wg.Done()
			defer e.activeVUs.Add(-1)
			defer e.vuPool.Release(vu)

			fmt.Printf("[executeConstantVUs] VU %d 开始执行\n", vu.ID)
			err := e.runVU(execCtx, task, vu, iterations)
			fmt.Printf("[executeConstantVUs] VU %d 执行完成, err=%v\n", vu.ID, err)
			if err != nil && err != context.Canceled && err != context.DeadlineExceeded {
				errChan <- err
			}
		}(vu)
	}

	// 等待所有 VU 完成
	fmt.Printf("[executeConstantVUs] 等待所有 VU 完成...\n")
	wg.Wait()
	close(errChan)
	fmt.Printf("[executeConstantVUs] 所有 VU 已完成\n")

	// 收集错误
	var firstErr error
	for err := range errChan {
		if firstErr == nil {
			firstErr = err
		}
	}

	return firstErr
}

// executeRampingVUs 使用递增/递减的 VU 数量执行。
// Requirements: 6.1.2
func (e *TaskEngine) executeRampingVUs(ctx context.Context, task *types.Task, stages []types.Stage) error {
	if len(stages) == 0 {
		return e.executeConstantVUs(ctx, task, 1, 0, 0)
	}

	var wg sync.WaitGroup
	vuCtxs := make(map[int]context.CancelFunc)
	var vuMu sync.Mutex

	currentVUs := 0

	for _, stage := range stages {
		targetVUs := stage.Target
		if targetVUs > e.maxVUs {
			targetVUs = e.maxVUs
		}

		// 计算递增速率
		stageStart := time.Now()
		stageDuration := stage.Duration
		if stageDuration <= 0 {
			stageDuration = time.Second
		}

		// 在此阶段内递增 VU
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()

	stageLoop:
		for {
			select {
			case <-ctx.Done():
				break stageLoop
			case <-ticker.C:
				elapsed := time.Since(stageStart)
				if elapsed >= stageDuration {
					break stageLoop
				}

				// 计算当前时间点的目标 VU 数
				progress := float64(elapsed) / float64(stageDuration)
				targetAtPoint := currentVUs + int(float64(targetVUs-currentVUs)*progress)

				vuMu.Lock()
				// 扩容
				for i := len(vuCtxs); i < targetAtPoint; i++ {
					vu := e.vuPool.Acquire(i)
					if vu == nil {
						continue
					}

					vuCtx, vuCancel := context.WithCancel(ctx)
					vuCtxs[i] = vuCancel

					wg.Add(1)
					e.activeVUs.Add(1)

					go func(vu *types.VirtualUser, ctx context.Context) {
						defer wg.Done()
						defer e.activeVUs.Add(-1)
						defer e.vuPool.Release(vu)

						e.runVU(ctx, task, vu, 0)
					}(vu, vuCtx)
				}

				// 缩容
				for i := len(vuCtxs) - 1; i >= targetAtPoint; i-- {
					if cancel, ok := vuCtxs[i]; ok {
						cancel()
						delete(vuCtxs, i)
					}
				}
				vuMu.Unlock()
			}
		}

		currentVUs = targetVUs
	}

	// 取消所有剩余的 VU
	vuMu.Lock()
	for _, cancel := range vuCtxs {
		cancel()
	}
	vuMu.Unlock()

	wg.Wait()
	return nil
}

// executePerVUIterations 每个 VU 执行固定次数的迭代。
// Requirements: 6.1.5
func (e *TaskEngine) executePerVUIterations(ctx context.Context, task *types.Task, vus int, iterationsPerVU int) error {
	if iterationsPerVU <= 0 {
		iterationsPerVU = 1
	}

	var wg sync.WaitGroup
	errChan := make(chan error, vus)

	for i := 0; i < vus; i++ {
		vu := e.vuPool.Acquire(i)
		if vu == nil {
			continue
		}

		wg.Add(1)
		e.activeVUs.Add(1)

		go func(vu *types.VirtualUser) {
			defer wg.Done()
			defer e.activeVUs.Add(-1)
			defer e.vuPool.Release(vu)

			err := e.runVU(ctx, task, vu, iterationsPerVU)
			if err != nil && err != context.Canceled {
				errChan <- err
			}
		}(vu)
	}

	wg.Wait()
	close(errChan)

	var firstErr error
	for err := range errChan {
		if firstErr == nil {
			firstErr = err
		}
	}

	return firstErr
}

// executeSharedIterations 在所有 VU 之间分配迭代次数。
// Requirements: 6.1.6
func (e *TaskEngine) executeSharedIterations(ctx context.Context, task *types.Task, vus int, totalIterations int) error {
	if totalIterations <= 0 {
		totalIterations = 1
	}

	var wg sync.WaitGroup
	errChan := make(chan error, vus)
	iterChan := make(chan int, totalIterations)

	// 填充迭代通道
	for i := 0; i < totalIterations; i++ {
		iterChan <- i
	}
	close(iterChan)

	for i := 0; i < vus; i++ {
		vu := e.vuPool.Acquire(i)
		if vu == nil {
			continue
		}

		wg.Add(1)
		e.activeVUs.Add(1)

		go func(vu *types.VirtualUser) {
			defer wg.Done()
			defer e.activeVUs.Add(-1)
			defer e.vuPool.Release(vu)

			for iter := range iterChan {
				select {
				case <-ctx.Done():
					return
				default:
					vu.Iteration = iter
					err := e.executeWorkflowIteration(ctx, task, vu)
					if err != nil {
						errChan <- err
					}
				}
			}
		}(vu)
	}

	wg.Wait()
	close(errChan)

	var firstErr error
	for err := range errChan {
		if firstErr == nil {
			firstErr = err
		}
	}

	return firstErr
}

// runVU 运行虚拟用户，直到上下文取消或迭代完成。
func (e *TaskEngine) runVU(ctx context.Context, task *types.Task, vu *types.VirtualUser, maxIterations int) (err error) {
	// 添加 panic 恢复，防止单个 VU 的 panic 影响其他 VU
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("VU %d panic: %v", vu.ID, r)
			fmt.Printf("[runVU] VU %d 发生 panic: %v\n", vu.ID, r)
		}
	}()

	fmt.Printf("[runVU] VU %d 开始, maxIterations=%d\n", vu.ID, maxIterations)
	iteration := 0
	for {
		select {
		case <-ctx.Done():
			fmt.Printf("[runVU] VU %d 上下文取消\n", vu.ID)
			return ctx.Err()
		default:
			if maxIterations > 0 && iteration >= maxIterations {
				fmt.Printf("[runVU] VU %d 达到最大迭代次数 %d\n", vu.ID, maxIterations)
				return nil
			}

			vu.Iteration = iteration
			fmt.Printf("[runVU] VU %d 执行迭代 %d\n", vu.ID, iteration)
			err := e.executeWorkflowIteration(ctx, task, vu)
			if err != nil {
				fmt.Printf("[runVU] VU %d 迭代 %d 执行失败: %v\n", vu.ID, iteration, err)
				return err
			}

			iteration++
			e.iterations.Add(1)
		}
	}
}

// executeWorkflowIteration 执行单次工作流迭代。
func (e *TaskEngine) executeWorkflowIteration(ctx context.Context, task *types.Task, vu *types.VirtualUser) error {
	workflow := task.Workflow

	fmt.Printf("[executeWorkflowIteration] VU %d 开始执行工作流迭代, 步骤数: %d\n", vu.ID, len(workflow.Steps))

	// 创建执行上下文
	execCtx := executor.NewExecutionContext().
		WithVU(vu).
		WithIteration(vu.Iteration).
		WithWorkflowID(workflow.ID).
		WithExecutionID(task.ExecutionID)

	// 设置回调到执行上下文（用于循环等嵌套场景）
	if workflow.Options.Callback != nil {
		execCtx.WithCallback(workflow.Options.Callback)
	}

	// 复制工作流变量
	if workflow.Variables != nil {
		for k, v := range workflow.Variables {
			execCtx.SetVariable(k, v)
		}
	}

	// 获取执行选项
	opts := &workflow.Options

	// 使用钩子执行工作流
	result := e.hookRunner.ExecuteWorkflowWithHooks(
		ctx,
		workflow,
		execCtx,
		func(ctx context.Context, wf *types.Workflow, ec *executor.ExecutionContext) ([]*hook.StepExecutionResult, error) {
			return e.executeStepsWithOptions(ctx, wf.Steps, ec, opts)
		},
	)

	fmt.Printf("[executeWorkflowIteration] VU %d 工作流迭代完成, error=%v\n", vu.ID, result.Error)

	// 记录指标
	e.recordWorkflowMetrics(result)

	return result.Error
}

// executeSteps 执行步骤列表。
func (e *TaskEngine) executeSteps(ctx context.Context, steps []types.Step, execCtx *executor.ExecutionContext) ([]*hook.StepExecutionResult, error) {
	return e.executeStepsWithOptions(ctx, steps, execCtx, nil)
}

// executeStepsWithOptions 执行步骤列表，支持执行选项。
func (e *TaskEngine) executeStepsWithOptions(ctx context.Context, steps []types.Step, execCtx *executor.ExecutionContext, opts *types.ExecutionOptions) ([]*hook.StepExecutionResult, error) {
	return e.executeStepsWithContext(ctx, steps, execCtx, opts, "", 0)
}

// executeStepsWithContext 执行步骤列表，支持父步骤上下文（用于循环等嵌套场景）
func (e *TaskEngine) executeStepsWithContext(ctx context.Context, steps []types.Step, execCtx *executor.ExecutionContext, opts *types.ExecutionOptions, parentID string, iteration int) ([]*hook.StepExecutionResult, error) {
	results := make([]*hook.StepExecutionResult, 0, len(steps))

	fmt.Printf("[executeStepsWithContext] 开始执行 %d 个步骤, parentID=%s, iteration=%d\n", len(steps), parentID, iteration)

	for i := range steps {
		step := &steps[i]

		select {
		case <-ctx.Done():
			return results, ctx.Err()
		default:
		}

		fmt.Printf("[executeStepsWithContext] 执行步骤[%d]: id=%s, type=%s, name=%s\n", i, step.ID, step.Type, step.Name)

		// 触发步骤开始回调
		if opts != nil && opts.Callback != nil {
			opts.Callback.OnStepStart(ctx, step, parentID, iteration)
			opts.Callback.OnProgress(ctx, i+1, len(steps), step.Name)
		}

		// 获取此步骤类型的执行器
		execType := step.Type
		// 如果是 HTTP 类型且配置了使用标准库引擎，则切换执行器类型
		if execType == "http" && opts != nil && opts.HTTPEngine == types.HTTPEngineStandard {
			execType = "http-std"
		}

		exec, err := e.registry.GetOrError(execType)
		if err != nil {
			fmt.Printf("[executeStepsWithContext] 获取执行器失败: type=%s, err=%v\n", execType, err)
			// 触发步骤失败回调
			if opts != nil && opts.Callback != nil {
				opts.Callback.OnStepFailed(ctx, step, err, 0, parentID, iteration)
			}
			return results, err
		}

		fmt.Printf("[executeStepsWithContext] 找到执行器: type=%s\n", execType)

		// 使用钩子执行步骤
		result := e.hookRunner.ExecuteStepWithHooks(
			ctx,
			step,
			execCtx,
			func(ctx context.Context, s *types.Step, ec *executor.ExecutionContext) (*types.StepResult, error) {
				return e.executeStepWithCallback(ctx, exec, s, ec, opts, parentID, iteration)
			},
		)

		fmt.Printf("[executeStepsWithContext] 步骤[%d] 执行完成: status=%v, error=%v\n", i, result.StepResult != nil && result.StepResult.Status == types.ResultStatusSuccess, result.Error)

		results = append(results, result)

		// 将结果存储到上下文中
		if result.StepResult != nil {
			execCtx.SetResult(step.ID, result.StepResult)

			// 触发步骤完成/失败回调
			if opts != nil && opts.Callback != nil {
				if result.StepResult.Status == types.ResultStatusSuccess {
					opts.Callback.OnStepComplete(ctx, step, result.StepResult, parentID, iteration)
				} else {
					var errMsg error
					if result.Error != nil {
						errMsg = result.Error
					} else if result.StepResult.Error != nil {
						errMsg = result.StepResult.Error
					}
					opts.Callback.OnStepFailed(ctx, step, errMsg, result.StepResult.Duration, parentID, iteration)
				}
			}
		}

		// 处理错误策略
		// 检查步骤是否失败（通过 error 或 status）
		stepFailed := result.Error != nil ||
			(result.StepResult != nil && (result.StepResult.Status == types.ResultStatusFailed || result.StepResult.Status == types.ResultStatusTimeout))

		if stepFailed {
			switch step.OnError {
			case types.ErrorStrategyAbort:
				if result.Error != nil {
					return results, result.Error
				}
				// 如果没有 error 但状态是失败，构造一个错误返回
				if result.StepResult != nil && result.StepResult.Error != nil {
					return results, result.StepResult.Error
				}
				return results, fmt.Errorf("步骤 %s 执行失败", step.Name)
			case types.ErrorStrategyContinue, types.ErrorStrategySkip:
				continue
			default:
				// 默认也是 abort
				if result.Error != nil {
					return results, result.Error
				}
				if result.StepResult != nil && result.StepResult.Error != nil {
					return results, result.StepResult.Error
				}
				return results, fmt.Errorf("步骤 %s 执行失败", step.Name)
			}
		}
	}

	fmt.Printf("[executeStepsWithContext] 所有步骤执行完成\n")

	return results, nil
}

// executeStep 执行单个步骤。
func (e *TaskEngine) executeStep(ctx context.Context, exec executor.Executor, step *types.Step, execCtx *executor.ExecutionContext) (result *types.StepResult, err error) {
	return e.executeStepWithCallback(ctx, exec, step, execCtx, nil, "", 0)
}

// executeStepWithCallback 执行单个步骤，支持回调。
func (e *TaskEngine) executeStepWithCallback(ctx context.Context, exec executor.Executor, step *types.Step, execCtx *executor.ExecutionContext, opts *types.ExecutionOptions, parentID string, iteration int) (result *types.StepResult, err error) {
	startTime := time.Now()

	// 添加 panic 恢复，防止执行器 panic 导致整个服务崩溃
	defer func() {
		if r := recover(); r != nil {
			panicErr := fmt.Errorf("executor panic: %v", r)
			result = executor.CreateFailedResult(step.ID, startTime, panicErr)
			err = panicErr
			e.collector.RecordStep(step.ID, result)
			fmt.Printf("[executeStepWithCallback] 步骤 %s 执行器发生 panic: %v\n", step.ID, r)
		}
	}()

	// 如果配置了超时，应用超时设置
	stepCtx := ctx
	var cancel context.CancelFunc
	if step.Timeout > 0 {
		stepCtx, cancel = context.WithTimeout(ctx, step.Timeout)
		defer cancel()
	}

	// 执行步骤
	result, err = exec.Execute(stepCtx, step, execCtx)

	// 记录步骤指标
	if result != nil {
		e.collector.RecordStep(step.ID, result)
	} else if err != nil {
		result = executor.CreateFailedResult(step.ID, startTime, err)
		e.collector.RecordStep(step.ID, result)
	}

	return result, err
}

// recordWorkflowMetrics 记录工作流执行的指标。
// 注意：步骤指标已在 executeStep 中记录，此函数保留用于
// 将来可能的用途（例如记录工作流级别的指标）。
func (e *TaskEngine) recordWorkflowMetrics(result *hook.WorkflowExecutionResult) {
	// 步骤指标已在 executeStep 中记录，无需重复记录
}
