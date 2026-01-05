package flow

import (
	"context"
	"sync"

	"github.com/grafana/k6/workflow-engine/pkg/types"
)

const (
	// ParallelExecutorType is the type identifier for parallel executor.
	ParallelExecutorType = "parallel"

	// DefaultMaxConcurrent 默认最大并发数
	DefaultMaxConcurrent = 10
)

// ParallelConfig Parallel 步骤配置
type ParallelConfig struct {
	Steps         []types.Step `yaml:"steps" json:"steps"`
	MaxConcurrent int          `yaml:"max_concurrent,omitempty" json:"max_concurrent,omitempty"`
	FailFast      bool         `yaml:"fail_fast,omitempty" json:"fail_fast,omitempty"`
}

// ParallelOutput Parallel 步骤输出
type ParallelOutput struct {
	TotalSteps   int                          `json:"total_steps"`
	Completed    int                          `json:"completed"`
	Failed       int                          `json:"failed"`
	Results      map[string]*types.StepResult `json:"results"`
	FailFast     bool                         `json:"fail_fast"`
	TerminatedBy string                       `json:"terminated_by"` // completed, fail_fast, error
}

// ParallelExecutor executes steps in parallel.
type ParallelExecutor struct {
	stepExecutor StepExecutorFunc
}

// NewParallelExecutor creates a new parallel executor.
func NewParallelExecutor(stepExecutor StepExecutorFunc) *ParallelExecutor {
	return &ParallelExecutor{
		stepExecutor: stepExecutor,
	}
}

// Execute executes steps in parallel.
func (e *ParallelExecutor) Execute(ctx context.Context, config *ParallelConfig, execCtx *FlowExecutionContext) (*ParallelOutput, error) {
	maxConcurrent := config.MaxConcurrent
	if maxConcurrent <= 0 {
		maxConcurrent = DefaultMaxConcurrent
	}

	output := &ParallelOutput{
		TotalSteps: len(config.Steps),
		Results:    make(map[string]*types.StepResult),
		FailFast:   config.FailFast,
	}

	if len(config.Steps) == 0 {
		output.TerminatedBy = "completed"
		return output, nil
	}

	// 创建信号量控制并发
	sem := make(chan struct{}, maxConcurrent)

	// 用于收集结果
	resultsChan := make(chan *types.StepResult, len(config.Steps))

	// 用于 fail_fast 取消
	cancelCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	var wg sync.WaitGroup
	var firstError error
	var errorMu sync.Mutex
	var failFastTriggered bool

	for i := range config.Steps {
		step := &config.Steps[i]

		wg.Add(1)
		go func(s *types.Step) {
			defer wg.Done()

			// 获取信号量
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-cancelCtx.Done():
				return
			}

			// 检查是否已经触发 fail_fast
			errorMu.Lock()
			if failFastTriggered {
				errorMu.Unlock()
				return
			}
			errorMu.Unlock()

			// 为每个并行步骤创建独立的上下文副本（变量隔离）
			stepCtx := &FlowExecutionContext{
				Variables: make(map[string]any),
				Results:   make(map[string]*types.StepResult),
			}
			// 复制变量
			for k, v := range execCtx.Variables {
				stepCtx.Variables[k] = v
			}
			for k, v := range execCtx.Results {
				stepCtx.Results[k] = v
			}

			// 执行步骤
			result, err := e.stepExecutor(cancelCtx, s, stepCtx)
			if err != nil {
				result = &types.StepResult{
					StepID: s.ID,
					Status: types.ResultStatusFailed,
					Error:  err,
				}
			}

			// 发送结果
			select {
			case resultsChan <- result:
			case <-cancelCtx.Done():
				return
			}

			// 检查 fail_fast
			if config.FailFast && (result.Status == types.ResultStatusFailed || result.Status == types.ResultStatusTimeout) {
				errorMu.Lock()
				if !failFastTriggered {
					failFastTriggered = true
					firstError = result.Error
					cancel()
				}
				errorMu.Unlock()
			}
		}(step)
	}

	// 等待所有 goroutine 完成
	go func() {
		wg.Wait()
		close(resultsChan)
	}()

	// 收集结果
	for result := range resultsChan {
		output.Results[result.StepID] = result
		output.Completed++
		if result.Status == types.ResultStatusFailed || result.Status == types.ResultStatusTimeout {
			output.Failed++
		}
	}

	// 将结果写入执行上下文
	for stepID, result := range output.Results {
		execCtx.SetResult(stepID, result)
	}

	// 设置 parallel_results 变量
	execCtx.SetVariable("parallel_results", output.Results)

	// 确定终止原因
	if failFastTriggered {
		output.TerminatedBy = "fail_fast"
		return output, firstError
	}

	output.TerminatedBy = "completed"
	return output, nil
}
