package slave

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/grafana/k6/workflow-engine/internal/executor"
	"github.com/grafana/k6/workflow-engine/internal/hook"
	"github.com/grafana/k6/workflow-engine/pkg/types"
)

// TaskEngine handles workflow task execution.
// Requirements: 6.1, 6.3, 6.4
type TaskEngine struct {
	registry   *executor.Registry
	hookRunner *hook.Runner
	maxVUs     int

	// VU management
	vuPool     *VUPool
	activeVUs  atomic.Int32
	iterations atomic.Int64

	// Metrics collection
	collector *MetricsCollector

	// State
	running atomic.Bool
	mu      sync.RWMutex
}

// NewTaskEngine creates a new task engine.
func NewTaskEngine(registry *executor.Registry, maxVUs int) *TaskEngine {
	return &TaskEngine{
		registry:   registry,
		hookRunner: hook.NewRunner(registry),
		maxVUs:     maxVUs,
		vuPool:     NewVUPool(maxVUs),
		collector:  NewMetricsCollector(),
	}
}

// Execute executes a task and returns the result.
// Requirements: 6.1, 6.3, 6.4
func (e *TaskEngine) Execute(ctx context.Context, task *types.Task) (*types.TaskResult, error) {
	if task == nil || task.Workflow == nil {
		return nil, fmt.Errorf("invalid task: task or workflow is nil")
	}

	e.running.Store(true)
	defer e.running.Store(false)

	result := &types.TaskResult{
		TaskID:      task.ID,
		ExecutionID: task.ExecutionID,
		Status:      types.ExecutionStatusRunning,
		Errors:      make([]types.ExecutionError, 0),
	}

	// Determine execution parameters based on workflow options
	opts := task.Workflow.Options
	vus := e.calculateVUs(opts, task.Segment)
	iterations := e.calculateIterations(opts, task.Segment)
	duration := opts.Duration

	// Execute based on mode
	var execErr error
	switch opts.ExecutionMode {
	case types.ModeConstantVUs, "":
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

	// Collect final metrics
	result.Metrics = e.collector.GetMetrics()

	return result, execErr
}

// Stop stops the task engine.
func (e *TaskEngine) Stop(ctx context.Context) error {
	e.running.Store(false)
	e.vuPool.StopAll()
	return nil
}

// GetMetrics returns the current metrics.
func (e *TaskEngine) GetMetrics() *types.SlaveMetrics {
	return &types.SlaveMetrics{
		ActiveVUs:  int(e.activeVUs.Load()),
		Throughput: e.collector.GetThroughput(),
	}
}

// calculateVUs calculates the number of VUs for this segment.
func (e *TaskEngine) calculateVUs(opts types.ExecutionOptions, segment types.ExecutionSegment) int {
	totalVUs := opts.VUs
	if totalVUs <= 0 {
		totalVUs = 1
	}

	// Calculate VUs for this segment
	segmentSize := segment.End - segment.Start
	vus := int(float64(totalVUs) * segmentSize)
	if vus < 1 {
		vus = 1
	}

	// Cap at max VUs
	if vus > e.maxVUs {
		vus = e.maxVUs
	}

	return vus
}

// calculateIterations calculates the number of iterations for this segment.
func (e *TaskEngine) calculateIterations(opts types.ExecutionOptions, segment types.ExecutionSegment) int {
	totalIterations := opts.Iterations
	if totalIterations <= 0 {
		return 0 // Duration-based execution
	}

	// Calculate iterations for this segment
	segmentSize := segment.End - segment.Start
	iterations := int(float64(totalIterations) * segmentSize)
	if iterations < 1 {
		iterations = 1
	}

	return iterations
}

// executeConstantVUs executes with a constant number of VUs.
// Requirements: 6.1.1
func (e *TaskEngine) executeConstantVUs(ctx context.Context, task *types.Task, vus int, duration time.Duration, iterations int) error {
	var wg sync.WaitGroup
	errChan := make(chan error, vus)

	// Create a context with timeout if duration is specified
	execCtx := ctx
	var cancel context.CancelFunc
	if duration > 0 {
		execCtx, cancel = context.WithTimeout(ctx, duration)
		defer cancel()
	}

	// Start VUs
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

			err := e.runVU(execCtx, task, vu, iterations)
			if err != nil && err != context.Canceled && err != context.DeadlineExceeded {
				errChan <- err
			}
		}(vu)
	}

	// Wait for all VUs to complete
	wg.Wait()
	close(errChan)

	// Collect errors
	var firstErr error
	for err := range errChan {
		if firstErr == nil {
			firstErr = err
		}
	}

	return firstErr
}

// executeRampingVUs executes with ramping VU count.
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

		// Calculate ramp rate
		stageStart := time.Now()
		stageDuration := stage.Duration
		if stageDuration <= 0 {
			stageDuration = time.Second
		}

		// Ramp VUs during this stage
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

				// Calculate target VUs at this point
				progress := float64(elapsed) / float64(stageDuration)
				targetAtPoint := currentVUs + int(float64(targetVUs-currentVUs)*progress)

				vuMu.Lock()
				// Scale up
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

				// Scale down
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

	// Cancel all remaining VUs
	vuMu.Lock()
	for _, cancel := range vuCtxs {
		cancel()
	}
	vuMu.Unlock()

	wg.Wait()
	return nil
}

// executePerVUIterations executes a fixed number of iterations per VU.
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

// executeSharedIterations distributes iterations across all VUs.
// Requirements: 6.1.6
func (e *TaskEngine) executeSharedIterations(ctx context.Context, task *types.Task, vus int, totalIterations int) error {
	if totalIterations <= 0 {
		totalIterations = 1
	}

	var wg sync.WaitGroup
	errChan := make(chan error, vus)
	iterChan := make(chan int, totalIterations)

	// Fill iteration channel
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

// runVU runs a virtual user until context is cancelled or iterations complete.
func (e *TaskEngine) runVU(ctx context.Context, task *types.Task, vu *types.VirtualUser, maxIterations int) error {
	iteration := 0
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			if maxIterations > 0 && iteration >= maxIterations {
				return nil
			}

			vu.Iteration = iteration
			err := e.executeWorkflowIteration(ctx, task, vu)
			if err != nil {
				return err
			}

			iteration++
			e.iterations.Add(1)
		}
	}
}

// executeWorkflowIteration executes a single workflow iteration.
func (e *TaskEngine) executeWorkflowIteration(ctx context.Context, task *types.Task, vu *types.VirtualUser) error {
	workflow := task.Workflow

	// Create execution context
	execCtx := executor.NewExecutionContext().
		WithVU(vu).
		WithIteration(vu.Iteration).
		WithWorkflowID(workflow.ID).
		WithExecutionID(task.ExecutionID)

	// Copy workflow variables
	if workflow.Variables != nil {
		for k, v := range workflow.Variables {
			execCtx.SetVariable(k, v)
		}
	}

	// Execute workflow with hooks
	result := e.hookRunner.ExecuteWorkflowWithHooks(
		ctx,
		workflow,
		execCtx,
		func(ctx context.Context, wf *types.Workflow, ec *executor.ExecutionContext) ([]*hook.StepExecutionResult, error) {
			return e.executeSteps(ctx, wf.Steps, ec)
		},
	)

	// Record metrics
	e.recordWorkflowMetrics(result)

	return result.Error
}

// executeSteps executes a list of steps.
func (e *TaskEngine) executeSteps(ctx context.Context, steps []types.Step, execCtx *executor.ExecutionContext) ([]*hook.StepExecutionResult, error) {
	results := make([]*hook.StepExecutionResult, 0, len(steps))

	for i := range steps {
		step := &steps[i]

		select {
		case <-ctx.Done():
			return results, ctx.Err()
		default:
		}

		// Get executor for this step type
		exec, err := e.registry.GetOrError(step.Type)
		if err != nil {
			return results, err
		}

		// Execute step with hooks
		result := e.hookRunner.ExecuteStepWithHooks(
			ctx,
			step,
			execCtx,
			func(ctx context.Context, s *types.Step, ec *executor.ExecutionContext) (*types.StepResult, error) {
				return e.executeStep(ctx, exec, s, ec)
			},
		)

		results = append(results, result)

		// Store result in context
		if result.StepResult != nil {
			execCtx.SetResult(step.ID, result.StepResult)
		}

		// Handle error strategy
		if result.Error != nil {
			switch step.OnError {
			case types.ErrorStrategyAbort:
				return results, result.Error
			case types.ErrorStrategyContinue, types.ErrorStrategySkip:
				continue
			default:
				return results, result.Error
			}
		}
	}

	return results, nil
}

// executeStep executes a single step.
func (e *TaskEngine) executeStep(ctx context.Context, exec executor.Executor, step *types.Step, execCtx *executor.ExecutionContext) (*types.StepResult, error) {
	startTime := time.Now()

	// Apply timeout if configured
	stepCtx := ctx
	var cancel context.CancelFunc
	if step.Timeout > 0 {
		stepCtx, cancel = context.WithTimeout(ctx, step.Timeout)
		defer cancel()
	}

	// Execute the step
	result, err := exec.Execute(stepCtx, step, execCtx)

	// Record step metrics
	if result != nil {
		e.collector.RecordStep(step.ID, result)
	} else if err != nil {
		result = executor.CreateFailedResult(step.ID, startTime, err)
		e.collector.RecordStep(step.ID, result)
	}

	return result, err
}

// recordWorkflowMetrics records metrics for a workflow execution.
func (e *TaskEngine) recordWorkflowMetrics(result *hook.WorkflowExecutionResult) {
	for _, stepResult := range result.StepResults {
		if stepResult.StepResult != nil {
			e.collector.RecordStep(stepResult.StepResult.StepID, stepResult.StepResult)
		}
	}
}
