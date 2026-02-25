package slave

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"yqhp/workflow-engine/internal/executor"
	"yqhp/workflow-engine/internal/hook"
	metricsengine "yqhp/workflow-engine/internal/metrics/engine"
	summaryoutput "yqhp/workflow-engine/internal/output/summary"
	"yqhp/workflow-engine/pkg/controlsurface"
	"yqhp/workflow-engine/pkg/logger"
	"yqhp/workflow-engine/pkg/metrics"
	"yqhp/workflow-engine/pkg/output"
	"yqhp/workflow-engine/pkg/types"
)

// TaskEngine 处理工作流任务执行。
// Orchestration follows k6's cmd/run.go pattern:
// VU goroutines → samplesChan → OutputManager → [Outputs + MetricsEngine + Summary]
type TaskEngine struct {
	registry   *executor.Registry
	hookRunner *hook.Runner
	maxVUs     int

	// VU 管理
	vuPool     *VUPool
	activeVUs  atomic.Int32
	iterations atomic.Int64

	// 指标收集 (feeds samples into the pipeline)
	collector *MetricsCollector

	// k6-style metrics pipeline
	metricsEngine *metricsengine.MetricsEngine
	summaryOutput *summaryoutput.Output
	outputManager *output.Manager
	samplesChan   chan metrics.SampleContainer
	outputWait    func()
	outputFinish  func(error)

	// 采样日志收集器
	sampleLogCollector *SampleLogCollector

	// 控制面
	controlSurface *controlsurface.ControlSurface

	// 状态
	running    atomic.Bool
	paused     atomic.Bool
	pauseCh    chan struct{}
	errors     []string
	errorsMu   sync.Mutex
	cancelFunc context.CancelFunc
	mu         sync.RWMutex
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

// SetupMetricsPipeline sets up the k6-style metrics pipeline:
// samplesChan → OutputManager → [configured outputs + MetricsEngine Ingester + Summary Output]
func (e *TaskEngine) SetupMetricsPipeline(ctx context.Context, executionID string, configs []types.OutputConfig, params output.Params) error {
	// 1. Create samplesChan
	e.samplesChan = output.NewSamplesChannel(1000)

	// 2. Create configured outputs (InfluxDB, Kafka, etc.)
	var outputs []output.Output
	if len(configs) > 0 {
		configured, err := output.CreateOutputsFromConfig(ctx, configs, params)
		if err != nil {
			return err
		}
		outputs = append(outputs, configured...)
	}

	// 3. Create MetricsEngine + Ingester (as an Output in the pipeline)
	registry := metrics.NewRegistry()
	e.metricsEngine = metricsengine.NewMetricsEngine(registry)
	ingester := e.metricsEngine.CreateIngester()
	outputs = append(outputs, ingester)

	// 4. Create Summary Output (also an Output in the pipeline)
	e.summaryOutput = summaryoutput.New()
	outputs = append(outputs, e.summaryOutput)

	// 5. Connect MetricsCollector's sample emitter to the pipeline
	e.collector.SetSamplesChannel(e.samplesChan, params.Tags)

	// 6. Create and start Output Manager
	e.outputManager = output.NewManager(outputs, params.Logger)
	wait, finish, err := e.outputManager.Start(e.samplesChan)
	if err != nil {
		return err
	}
	e.outputWait = wait
	e.outputFinish = finish

	// 7. Start time-series collection (1s snapshots)
	e.metricsEngine.StartTimeSeriesCollection(
		func() int64 { return int64(e.activeVUs.Load()) },
		func() int64 { return e.iterations.Load() },
	)

	// 8. Register ControlSurface for REST API access
	e.controlSurface = &controlsurface.ControlSurface{
		RunCtx:        ctx,
		MetricsEngine: e.metricsEngine,
		SummaryOutput: e.summaryOutput,
		SamplesChan:   e.samplesChan,
		GetStatus: func() *controlsurface.ExecutionStatus {
			status := "running"
			if !e.running.Load() {
				status = "completed"
			}
			if e.paused.Load() {
				status = "paused"
			}
			return &controlsurface.ExecutionStatus{
				Status:     status,
				Running:    e.running.Load(),
				Paused:     e.paused.Load(),
				VUs:        int64(e.activeVUs.Load()),
				Iterations: e.iterations.Load(),
			}
		},
		ScaleVUs:        e.ScaleVUs,
		StopExecution:   func() error { e.running.Store(false); if e.cancelFunc != nil { e.cancelFunc() }; return nil },
		PauseExecution:  func() error { e.paused.Store(true); return nil },
		ResumeExecution: func() error { e.paused.Store(false); if e.pauseCh != nil { select { case e.pauseCh <- struct{}{}: default: } }; return nil },
		GetVUs:          func() int64 { return int64(e.activeVUs.Load()) },
		GetIterations:   func() int64 { return e.iterations.Load() },
		GetErrors:       func() []string { e.errorsMu.Lock(); defer e.errorsMu.Unlock(); r := make([]string, len(e.errors)); copy(r, e.errors); return r },
		GetSampleLogs:   func() interface{} { if e.sampleLogCollector != nil { return e.sampleLogCollector.GetLogs() }; return nil },
	}
	controlsurface.Register(executionID, e.controlSurface)

	return nil
}

// ScaleVUs dynamically adjusts the number of active VUs.
const maxAllowedVUs = 100000

// ScaleVUs dynamically adjusts the target number of VUs.
func (e *TaskEngine) ScaleVUs(newVUs int) error {
	if newVUs < 0 {
		return fmt.Errorf("VU count cannot be negative")
	}
	if newVUs > maxAllowedVUs {
		return fmt.Errorf("VU count %d exceeds hard limit %d", newVUs, maxAllowedVUs)
	}
	e.mu.Lock()
	e.maxVUs = newVUs
	e.mu.Unlock()
	logger.Info("VUs scaled to %d", newVUs)
	return nil
}

// addError records an execution error.
func (e *TaskEngine) addError(msg string) {
	e.errorsMu.Lock()
	e.errors = append(e.errors, msg)
	e.errorsMu.Unlock()
}

// GetSamplesChannel returns the samples channel for external metric emission.
func (e *TaskEngine) GetSamplesChannel() chan metrics.SampleContainer {
	return e.samplesChan
}

// Execute runs a task with the full k6-style metrics pipeline.
// Pipeline: VU goroutines → samplesChan → OutputManager → [Outputs + MetricsEngine + Summary]
func (e *TaskEngine) Execute(ctx context.Context, task *types.Task) (*types.TaskResult, error) {
	if task == nil || task.Workflow == nil {
		return nil, fmt.Errorf("invalid task: task or workflow is nil")
	}

	logger.Debug("TaskEngine.Execute] starting task: %s, workflow: %s", task.ID, task.Workflow.Name)

	// Create cancellable context for stop support
	execCtx, cancel := context.WithCancel(ctx)
	e.cancelFunc = cancel
	defer cancel()

	e.running.Store(true)
	e.errors = nil

	// Setup metrics pipeline if not already set up
	if e.samplesChan == nil {
		if err := e.SetupMetricsPipeline(execCtx, task.ExecutionID, task.Workflow.Options.Outputs, output.Params{
			ExecutionID:  task.ExecutionID,
			WorkflowName: task.Workflow.Name,
			Tags:         task.Workflow.Options.Tags,
		}); err != nil {
			logger.Warn("Failed to setup metrics pipeline: %v", err)
		}
	}

	// Setup sample log collector based on sampling mode
	samplingMode := task.Workflow.Options.SamplingMode
	if samplingMode != "" && samplingMode != types.SamplingModeNone {
		e.sampleLogCollector = NewSampleLogCollector(task.ExecutionID, samplingMode, nil)
	}

	result := &types.TaskResult{
		TaskID:      task.ID,
		ExecutionID: task.ExecutionID,
		Status:      types.ExecutionStatusRunning,
		Errors:      make([]types.ExecutionError, 0),
	}

	opts := task.Workflow.Options
	vus := e.calculateVUs(opts, task.Segment)
	iterations := e.calculateIterations(opts, task.Segment)
	duration := opts.Duration

	logger.Debug("TaskEngine.Execute] params: vus=%d, iterations=%d, duration=%v, mode=%s", vus, iterations, duration, opts.ExecutionMode)

	// Run the execution mode
	var execErr error
	switch opts.ExecutionMode {
	case types.ModeConstantVUs, "":
		execErr = e.executeConstantVUs(execCtx, task, vus, duration, iterations)
	case types.ModeRampingVUs:
		execErr = e.executeRampingVUs(execCtx, task, opts.Stages)
	case types.ModePerVUIterations:
		execErr = e.executePerVUIterations(execCtx, task, vus, iterations)
	case types.ModeSharedIterations:
		execErr = e.executeSharedIterations(execCtx, task, vus, iterations)
	default:
		execErr = e.executeConstantVUs(execCtx, task, vus, duration, iterations)
	}

	e.running.Store(false)

	// --- Pipeline shutdown (k6 pattern) ---

	// 1. Stop time-series collection
	if e.metricsEngine != nil {
		e.metricsEngine.StopTimeSeriesCollection()
	}

	// 2. Close samples channel to signal end of metrics
	if e.samplesChan != nil {
		close(e.samplesChan)
		e.samplesChan = nil
	}

	// 3. Wait for OutputManager to finish distributing all buffered samples
	if e.outputWait != nil {
		e.outputWait()
	}

	// 4. Determine final status
	status := "completed"
	if execErr != nil {
		status = "failed"
		result.Status = types.ExecutionStatusFailed
		result.Errors = append(result.Errors, types.ExecutionError{
			Code:      types.ErrCodeExecution,
			Message:   execErr.Error(),
			Timestamp: time.Now(),
		})
	} else {
		result.Status = types.ExecutionStatusCompleted
	}

	// 5. Stop all outputs so that their internal PeriodicFlusher
	//    performs a final flush before we read metrics for the report.
	if e.outputFinish != nil {
		e.outputFinish(execErr)
	}

	// 6. Generate final report from Summary Output (now all sinks are fully flushed)
	if e.summaryOutput != nil && e.metricsEngine != nil {
		report := e.summaryOutput.GenerateReport(
			e.metricsEngine,
			task.ExecutionID,
			task.Workflow.ID,
			task.Workflow.Name,
			status,
			e.iterations.Load(),
			vus,
		)
		result.Report = report

		// Store report in ControlSurface for REST API access
		if e.controlSurface != nil {
			e.controlSurface.FinalReport = report
		}
	}

	// 7. Flush sample logs
	if e.sampleLogCollector != nil {
		e.sampleLogCollector.Flush()
	}

	// 8. Collect legacy metrics (for backward compat during transition)
	result.Metrics = e.collector.GetMetrics()
	result.Iterations = e.iterations.Load()

	logger.Debug("TaskEngine.Execute] completed: status=%s, iterations=%d", result.Status, result.Iterations)

	// Note: ControlSurface is intentionally NOT unregistered here so that
	// the final report can still be retrieved via REST API after completion.
	// It will be cleaned up when a new execution starts or after a timeout.

	return result, execErr
}

// Stop stops the task engine gracefully.
func (e *TaskEngine) Stop(ctx context.Context) error {
	e.running.Store(false)
	if e.cancelFunc != nil {
		e.cancelFunc()
	}
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

// executeConstantVUs runs VUs with support for dynamic scaling via ScaleVUs().
func (e *TaskEngine) executeConstantVUs(ctx context.Context, task *types.Task, vus int, duration time.Duration, iterations int) error {
	logger.Debug("executeConstantVUs] start: vus=%d, duration=%v, iterations=%d", vus, duration, iterations)

	var wg sync.WaitGroup

	// Always create a cancellable context so the VU controller can be stopped
	// when all VUs finish their iterations (prevents infinite VU respawning).
	execCtx, stopAll := context.WithCancel(ctx)
	defer stopAll()

	if duration > 0 {
		var durationCancel context.CancelFunc
		execCtx, durationCancel = context.WithTimeout(execCtx, duration)
		defer durationCancel()
	}

	// Guard: once stopping is true, startVU becomes a no-op to prevent
	// the controller from launching new VUs after wg.Wait() returns.
	var stopping atomic.Bool

	// Track per-VU cancel functions for dynamic scaling
	var vuMu sync.Mutex
	vuCancels := make(map[int]context.CancelFunc)
	nextVUID := 0

	startVU := func() {
		if stopping.Load() {
			return
		}

		vuMu.Lock()
		id := nextVUID
		nextVUID++
		vuMu.Unlock()

		vu := e.vuPool.Acquire(id)
		if vu == nil {
			vu = &types.VirtualUser{ID: id, StartTime: time.Now()}
		}

		vuCtx, vuCancel := context.WithCancel(execCtx)

		vuMu.Lock()
		vuCancels[id] = vuCancel
		vuMu.Unlock()

		wg.Add(1)
		e.activeVUs.Add(1)

		go func(vu *types.VirtualUser, ctx context.Context, id int) {
			defer wg.Done()
			defer e.activeVUs.Add(-1)
			defer e.vuPool.Release(vu)
			defer func() {
				vuMu.Lock()
				delete(vuCancels, id)
				vuMu.Unlock()
			}()

			e.runVU(ctx, task, vu, iterations)
		}(vu, vuCtx, id)
	}

	stopOneVU := func() {
		vuMu.Lock()
		defer vuMu.Unlock()
		maxID := -1
		for id := range vuCancels {
			if id > maxID {
				maxID = id
			}
		}
		if maxID >= 0 {
			if cancel, ok := vuCancels[maxID]; ok {
				cancel()
			}
		}
	}

	// Launch initial VUs
	for i := 0; i < vus; i++ {
		startVU()
	}

	// VU controller: dynamically adjusts VU count based on maxVUs.
	// Only useful for duration-based execution or explicit ScaleVUs calls.
	controllerDone := make(chan struct{})
	go func() {
		defer close(controllerDone)
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-execCtx.Done():
				return
			case <-ticker.C:
				if stopping.Load() {
					return
				}

				e.mu.RLock()
				target := e.maxVUs
				e.mu.RUnlock()

				current := int(e.activeVUs.Load())

				if target > current {
					for i := 0; i < target-current; i++ {
						startVU()
					}
					logger.Debug("executeConstantVUs] scaled up: %d -> %d VUs", current, target)
				} else if target < current {
					for i := 0; i < current-target; i++ {
						stopOneVU()
					}
					logger.Debug("executeConstantVUs] scaled down: %d -> %d VUs", current, target)
				}
			}
		}
	}()

	wg.Wait()
	stopping.Store(true)
	stopAll()
	<-controllerDone

	return nil
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
			logger.Debug("runVU] VU %d 发生 panic: %v\n", vu.ID, r)
		}
	}()

	logger.Debug("runVU] VU %d 开始, maxIterations=%d\n", vu.ID, maxIterations)
	iteration := 0
	for {
		select {
		case <-ctx.Done():
			logger.Debug("runVU] VU %d 上下文取消，正常结束\n", vu.ID)
			return nil
		default:
			if maxIterations > 0 && iteration >= maxIterations {
				logger.Debug("runVU] VU %d 达到最大迭代次数 %d\n", vu.ID, maxIterations)
				return nil
			}

			vu.Iteration = iteration
			logger.Debug("runVU] VU %d 执行迭代 %d\n", vu.ID, iteration)
			err := e.executeWorkflowIteration(ctx, task, vu)
			if err != nil {
				if ctx.Err() != nil {
					logger.Debug("runVU] VU %d 测试结束，忽略迭代 %d 的上下文错误\n", vu.ID, iteration)
					return nil
				}
				logger.Debug("runVU] VU %d iteration %d failed: %v", vu.ID, iteration, err)
				e.addError(fmt.Sprintf("VU %d iter %d: %v", vu.ID, iteration, err))
				if maxIterations > 0 {
					return err
				}
			}

			iteration++
			e.iterations.Add(1)
		}
	}
}

// executeWorkflowIteration 执行单次工作流迭代。
func (e *TaskEngine) executeWorkflowIteration(ctx context.Context, task *types.Task, vu *types.VirtualUser) error {
	workflow := task.Workflow

	logger.Debug("executeWorkflowIteration] VU %d 开始执行工作流迭代, 步骤数: %d\n", vu.ID, len(workflow.Steps))

	// 创建执行上下文
	execCtx := executor.NewExecutionContext().
		WithVU(vu).
		WithIteration(vu.Iteration).
		WithWorkflowID(workflow.ID).
		WithExecutionID(task.ExecutionID)

	// 从工作流中获取回调并设置到执行上下文
	if task.Workflow != nil && task.Workflow.Callback != nil {
		execCtx.WithCallback(task.Workflow.Callback)
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

	logger.Debug("executeWorkflowIteration] VU %d 工作流迭代完成, error=%v\n", vu.ID, result.Error)

	// 保存最终变量快照到工作流（用于调试上下文缓存）
	if execCtx.Variables != nil {
		finalVars := make(map[string]any, len(execCtx.Variables))
		for k, v := range execCtx.Variables {
			finalVars[k] = v
		}
		workflow.FinalVariables = finalVars
	}

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

	// 从执行上下文获取回调
	callback := execCtx.GetCallback()

	logger.Debug("executeStepsWithContext] 开始执行 %d 个步骤, parentID=%s, iteration=%d\n", len(steps), parentID, iteration)

	for i := range steps {
		step := &steps[i]

		// 跳过禁用的步骤
		if step.Disabled {
			logger.Debug("executeStepsWithContext] 跳过禁用的步骤[%d]: id=%s, name=%s\n", i, step.ID, step.Name)
			// 触发步骤跳过回调
			if callback != nil {
				callback.OnStepSkipped(ctx, step, "步骤已禁用", parentID, iteration)
			}
			continue
		}

		logger.Debug("executeStepsWithContext] 执行步骤[%d]: id=%s, type=%s, name=%s\n", i, step.ID, step.Type, step.Name)

		// 触发步骤开始回调
		if callback != nil {
			callback.OnStepStart(ctx, step, parentID, iteration)
			callback.OnProgress(ctx, i+1, len(steps), step.Name)
		}

		// 获取此步骤类型的执行器
		execType := step.Type
		// 如果是 HTTP 类型且配置了使用标准库引擎，则切换执行器类型
		if execType == "http" && opts != nil && opts.HTTPEngine == types.HTTPEngineStandard {
			execType = "http-std"
		}

		exec, err := e.registry.GetOrError(execType)
		if err != nil {
			logger.Debug("executeStepsWithContext] 获取执行器失败: type=%s, err=%v\n", execType, err)
			// 获取执行器失败也构造一个 StepResult，确保回调层拿到完整信息
			failedResult := executor.CreateFailedResult(step.ID, time.Now(), err)
			if callback != nil {
				callback.OnStepComplete(ctx, step, failedResult, parentID, iteration)
				callback.OnStepFailed(ctx, step, err, 0, parentID, iteration)
			}
			return results, err
		}

		logger.Debug("executeStepsWithContext] 找到执行器: type=%s\n", execType)

		// 使用钩子执行步骤
		result := e.hookRunner.ExecuteStepWithHooks(
			ctx,
			step,
			execCtx,
			func(ctx context.Context, s *types.Step, ec *executor.ExecutionContext) (*types.StepResult, error) {
				return e.executeStepWithCallback(ctx, exec, s, ec, opts, parentID, iteration)
			},
		)

		logger.Debug("executeStepsWithContext] 步骤[%d] 执行完成: status=%v, error=%v\n", i, result.StepResult != nil && result.StepResult.Status == types.ResultStatusSuccess, result.Error)

		results = append(results, result)

		// 将结果存储到上下文中
		if result.StepResult != nil {
			execCtx.SetResult(step.ID, result.StepResult)

			// 触发回调：不管成功还是失败，都先调 OnStepComplete 以传递完整的 StepResult（含 Output）
			// 对于流程引擎来说，节点"执行完毕"就是 Complete，失败只是执行结果的一种状态
			if callback != nil {
				// 统一走 OnStepComplete，让回调层拿到完整的 StepResult（含 Output、Error 等）
				callback.OnStepComplete(ctx, step, result.StepResult, parentID, iteration)

				// 如果步骤失败，额外通知 OnStepFailed（给需要它的消费者）
				if result.StepResult.Status != types.ResultStatusSuccess {
					var errMsg error
					if result.Error != nil {
						errMsg = result.Error
					} else if result.StepResult.Error != nil {
						errMsg = result.StepResult.Error
					}
					callback.OnStepFailed(ctx, step, errMsg, result.StepResult.Duration, parentID, iteration)
				}
			}
		}

		// 处理错误策略
		// 检查步骤是否失败（通过 error 或 status）
		stepFailed := result.Error != nil ||
			(result.StepResult != nil && (result.StepResult.Status == types.ResultStatusFailed || result.StepResult.Status == types.ResultStatusTimeout))

		if stepFailed {
			stepErr := result.Error
			if stepErr == nil && result.StepResult != nil && result.StepResult.Error != nil {
				stepErr = result.StepResult.Error
			}
			if stepErr == nil {
				stepErr = fmt.Errorf("执行失败")
			}
			wrappedErr := fmt.Errorf("[%s] %w", step.Name, stepErr)

			switch step.OnError {
			case types.ErrorStrategyContinue, types.ErrorStrategySkip:
				continue
			default:
				return results, wrappedErr
			}
		}
	}

	logger.Debug("executeStepsWithContext] 所有步骤执行完成")

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
			e.collector.RecordStep(step.ID, step.Name, result)
			e.recordSampleLog(step.ID, step.Name, result)
			logger.Debug("executeStepWithCallback] 步骤 %s 执行器发生 panic: %v\n", step.ID, r)
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
		e.collector.RecordStep(step.ID, step.Name, result)
		e.recordSampleLog(step.ID, step.Name, result)
	} else if err != nil {
		result = executor.CreateFailedResult(step.ID, startTime, err)
		e.collector.RecordStep(step.ID, step.Name, result)
		e.recordSampleLog(step.ID, step.Name, result)
	}

	return result, err
}

// recordSampleLog 有条件地记录采样日志
func (e *TaskEngine) recordSampleLog(stepID, stepName string, result *types.StepResult) {
	if e.sampleLogCollector == nil || result == nil {
		return
	}
	if e.sampleLogCollector.ShouldSample(result) {
		e.sampleLogCollector.Record(stepID, stepName, result)
	}
}

// recordWorkflowMetrics 记录工作流执行的指标。
// 注意：步骤指标已在 executeStep 中记录，此函数保留用于
// 将来可能的用途（例如记录工作流级别的指标）。
func (e *TaskEngine) recordWorkflowMetrics(result *hook.WorkflowExecutionResult) {
	// 步骤指标已在 executeStep 中记录，无需重复记录
}
