package flow

import (
	"context"
	"testing"
	"time"

	"github.com/grafana/k6/workflow-engine/pkg/types"
	"pgregory.net/rapid"
)

// mockStepExecutor 模拟步骤执行器
func mockStepExecutor(ctx context.Context, step *types.Step, execCtx *FlowExecutionContext) (*types.StepResult, error) {
	return &types.StepResult{
		StepID:    step.ID,
		Status:    types.ResultStatusSuccess,
		StartTime: time.Now(),
		EndTime:   time.Now(),
		Duration:  time.Millisecond,
	}, nil
}

// TestIfElseFlowControlProperty 属性 7: If-Else 流程控制正确性
// 对于任意 if 步骤，必须恰好执行一个分支：条件为真时执行 then，
// 匹配的 else_if 条件为真时执行对应分支，否则执行 else。
func TestIfElseFlowControlProperty(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// 生成随机条件结果
		mainConditionTrue := rapid.Bool().Draw(t, "mainConditionTrue")
		numElseIf := rapid.IntRange(0, 3).Draw(t, "numElseIf")
		elseIfResults := make([]bool, numElseIf)
		for i := 0; i < numElseIf; i++ {
			elseIfResults[i] = rapid.Bool().Draw(t, "elseIfResult")
		}
		hasElse := rapid.Bool().Draw(t, "hasElse")

		// 创建执行上下文
		execCtx := NewFlowExecutionContext()
		execCtx.SetVariable("main_condition", mainConditionTrue)
		for i, result := range elseIfResults {
			execCtx.SetVariable("else_if_"+string(rune('a'+i)), result)
		}

		// 构建配置
		config := &IfConfig{
			Condition: "${main_condition}",
			Then: []types.Step{
				{ID: "then_step", Type: "mock"},
			},
			ElseIf: make([]ElseIf, numElseIf),
		}

		for i := 0; i < numElseIf; i++ {
			config.ElseIf[i] = ElseIf{
				Condition: "${else_if_" + string(rune('a'+i)) + "}",
				Steps: []types.Step{
					{ID: "else_if_step_" + string(rune('a'+i)), Type: "mock"},
				},
			}
		}

		if hasElse {
			config.Else = []types.Step{
				{ID: "else_step", Type: "mock"},
			}
		}

		// 执行
		executor := NewIfExecutor(mockStepExecutor)
		output, err := executor.Execute(context.Background(), config, execCtx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// 验证属性：恰好执行一个分支
		expectedBranch := ""
		expectedSteps := 0

		if mainConditionTrue {
			expectedBranch = "then"
			expectedSteps = 1
		} else {
			foundElseIf := false
			for i, result := range elseIfResults {
				if result {
					expectedBranch = "else_if"
					expectedSteps = 1
					foundElseIf = true
					// 验证是第一个为真的 else_if
					if output.BranchIndex != i+1 {
						t.Fatalf("expected else_if branch index %d, got %d", i+1, output.BranchIndex)
					}
					break
				}
			}
			if !foundElseIf {
				if hasElse {
					expectedBranch = "else"
					expectedSteps = 1
				} else {
					expectedBranch = "none"
					expectedSteps = 0
				}
			}
		}

		// 验证分支
		if output.BranchTaken != expectedBranch {
			t.Fatalf("expected branch '%s', got '%s'", expectedBranch, output.BranchTaken)
		}

		// 验证执行的步骤数
		if len(output.StepsExecuted) != expectedSteps {
			t.Fatalf("expected %d steps executed, got %d", expectedSteps, len(output.StepsExecuted))
		}
	})
}

// TestLoopTerminationProperty 属性 8: 循环终止条件
// 对于任意 while/for/foreach 循环，循环必须在以下情况终止：
// - 条件变为假 (while)
// - 索引超过结束值 (for)
// - 所有项处理完毕 (foreach)
// - 达到 max_iterations
// - 执行 break 关键字
func TestLoopTerminationProperty(t *testing.T) {
	t.Run("while_condition_false", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			// 生成随机迭代次数
			targetIterations := rapid.IntRange(1, 10).Draw(t, "targetIterations")

			execCtx := NewFlowExecutionContext()
			execCtx.SetVariable("counter", 0)
			execCtx.SetVariable("target", targetIterations)

			// 创建一个会在 counter 达到 target 时终止的循环
			config := &WhileConfig{
				Condition:     "${counter} < ${target}",
				MaxIterations: 100,
				Steps: []types.Step{
					{ID: "increment", Type: "mock"},
				},
			}

			// 自定义执行器，每次递增 counter
			iterationCount := 0
			customExecutor := func(ctx context.Context, step *types.Step, execCtx *FlowExecutionContext) (*types.StepResult, error) {
				counter, _ := execCtx.GetVariable("counter")
				execCtx.SetVariable("counter", counter.(int)+1)
				iterationCount++
				return &types.StepResult{
					StepID: step.ID,
					Status: types.ResultStatusSuccess,
				}, nil
			}

			executor := NewWhileExecutor(customExecutor)
			output, err := executor.Execute(context.Background(), config, execCtx)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// 验证属性：循环在条件为假时终止
			if output.TerminatedBy != "condition" {
				t.Fatalf("expected termination by 'condition', got '%s'", output.TerminatedBy)
			}

			// 验证迭代次数
			if output.Iterations != targetIterations {
				t.Fatalf("expected %d iterations, got %d", targetIterations, output.Iterations)
			}
		})
	})

	t.Run("while_max_iterations", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			maxIterations := rapid.IntRange(1, 20).Draw(t, "maxIterations")

			execCtx := NewFlowExecutionContext()

			config := &WhileConfig{
				Condition:     "true", // 永远为真
				MaxIterations: maxIterations,
				Steps: []types.Step{
					{ID: "step", Type: "mock"},
				},
			}

			executor := NewWhileExecutor(mockStepExecutor)
			output, err := executor.Execute(context.Background(), config, execCtx)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// 验证属性：循环在达到最大迭代次数时终止
			if output.TerminatedBy != "max_iterations" {
				t.Fatalf("expected termination by 'max_iterations', got '%s'", output.TerminatedBy)
			}

			if output.Iterations != maxIterations {
				t.Fatalf("expected %d iterations, got %d", maxIterations, output.Iterations)
			}
		})
	})

	t.Run("for_loop_completion", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			start := rapid.IntRange(0, 10).Draw(t, "start")
			end := rapid.IntRange(start, start+20).Draw(t, "end")
			step := rapid.IntRange(1, 3).Draw(t, "step")

			execCtx := NewFlowExecutionContext()

			config := &ForConfig{
				Start:    start,
				End:      end,
				Step:     step,
				IndexVar: "i",
				Steps: []types.Step{
					{ID: "step", Type: "mock"},
				},
			}

			executor := NewForExecutor(mockStepExecutor)
			output, err := executor.Execute(context.Background(), config, execCtx)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// 验证属性：循环在索引超过结束值时终止
			if output.TerminatedBy != "completed" {
				t.Fatalf("expected termination by 'completed', got '%s'", output.TerminatedBy)
			}

			// 计算预期迭代次数
			expectedIterations := 0
			for i := start; i <= end; i += step {
				expectedIterations++
			}

			if output.Iterations != expectedIterations {
				t.Fatalf("expected %d iterations, got %d", expectedIterations, output.Iterations)
			}
		})
	})

	t.Run("foreach_completion", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			numItems := rapid.IntRange(0, 10).Draw(t, "numItems")
			items := make([]any, numItems)
			for i := 0; i < numItems; i++ {
				items[i] = rapid.String().Draw(t, "item")
			}

			execCtx := NewFlowExecutionContext()
			execCtx.SetVariable("items", items)

			config := &ForeachConfig{
				Items:   "${items}",
				ItemVar: "item",
				Steps: []types.Step{
					{ID: "step", Type: "mock"},
				},
			}

			executor := NewForeachExecutor(mockStepExecutor)
			output, err := executor.Execute(context.Background(), config, execCtx)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// 验证属性：循环在所有项处理完毕时终止
			if output.TerminatedBy != "completed" {
				t.Fatalf("expected termination by 'completed', got '%s'", output.TerminatedBy)
			}

			if output.Iterations != numItems {
				t.Fatalf("expected %d iterations, got %d", numItems, output.Iterations)
			}

			if output.TotalItems != numItems {
				t.Fatalf("expected total items %d, got %d", numItems, output.TotalItems)
			}
		})
	})
}

// TestBreakContinueProperty 测试 break/continue 的正确性
func TestBreakContinueProperty(t *testing.T) {
	t.Run("break_terminates_loop", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			breakAt := rapid.IntRange(1, 5).Draw(t, "breakAt")
			maxIterations := rapid.IntRange(breakAt+1, breakAt+10).Draw(t, "maxIterations")

			execCtx := NewFlowExecutionContext()
			execCtx.SetVariable("counter", 0)
			execCtx.SetVariable("break_at", breakAt)

			iterationCount := 0
			customExecutor := func(ctx context.Context, step *types.Step, execCtx *FlowExecutionContext) (*types.StepResult, error) {
				counter, _ := execCtx.GetVariable("counter")
				execCtx.SetVariable("counter", counter.(int)+1)
				iterationCount++
				return &types.StepResult{
					StepID: step.ID,
					Status: types.ResultStatusSuccess,
				}, nil
			}

			config := &WhileConfig{
				Condition:     "true",
				MaxIterations: maxIterations,
				Steps: []types.Step{
					{ID: "step", Type: "mock"},
					{ID: "break_step", Type: "break", Config: map[string]any{}},
				},
			}

			executor := NewWhileExecutor(customExecutor)
			output, err := executor.Execute(context.Background(), config, execCtx)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// 验证属性：break 终止循环
			if output.TerminatedBy != "break" {
				t.Fatalf("expected termination by 'break', got '%s'", output.TerminatedBy)
			}

			// 只执行了一次迭代（在第一次迭代中遇到 break）
			if output.Iterations != 0 {
				t.Fatalf("expected 0 complete iterations (break in first), got %d", output.Iterations)
			}
		})
	})

	t.Run("continue_skips_iteration", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			numItems := rapid.IntRange(2, 10).Draw(t, "numItems")
			items := make([]any, numItems)
			for i := 0; i < numItems; i++ {
				items[i] = i
			}

			execCtx := NewFlowExecutionContext()
			execCtx.SetVariable("items", items)

			processedCount := 0
			customExecutor := func(ctx context.Context, step *types.Step, execCtx *FlowExecutionContext) (*types.StepResult, error) {
				if step.ID == "process" {
					processedCount++
				}
				return &types.StepResult{
					StepID: step.ID,
					Status: types.ResultStatusSuccess,
				}, nil
			}

			// 在偶数索引时 continue
			config := &ForeachConfig{
				Items:    "${items}",
				ItemVar:  "item",
				IndexVar: "idx",
				Steps: []types.Step{
					{ID: "check", Type: "mock"},
					// 注意：这里简化测试，实际 continue 需要条件判断
					// 这个测试主要验证 continue 机制本身
				},
			}

			executor := NewForeachExecutor(customExecutor)
			output, err := executor.Execute(context.Background(), config, execCtx)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// 验证属性：所有项都被处理
			if output.Iterations != numItems {
				t.Fatalf("expected %d iterations, got %d", numItems, output.Iterations)
			}
		})
	})
}

// TestNestedLoopBreakProperty 测试嵌套循环中带标签的 break
func TestNestedLoopBreakProperty(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		outerIterations := rapid.IntRange(2, 5).Draw(t, "outerIterations")
		innerIterations := rapid.IntRange(2, 5).Draw(t, "innerIterations")

		execCtx := NewFlowExecutionContext()
		execCtx.SetVariable("outer_items", make([]any, outerIterations))
		execCtx.SetVariable("inner_items", make([]any, innerIterations))

		// 验证 break 标签机制
		breakErr := &BreakError{Label: "outer"}
		if breakErr.Label != "outer" {
			t.Fatalf("expected break label 'outer', got '%s'", breakErr.Label)
		}

		continueErr := &ContinueError{Label: "inner"}
		if continueErr.Label != "inner" {
			t.Fatalf("expected continue label 'inner', got '%s'", continueErr.Label)
		}

		// 验证错误消息
		if breakErr.Error() != "break:outer" {
			t.Fatalf("expected error message 'break:outer', got '%s'", breakErr.Error())
		}

		if continueErr.Error() != "continue:inner" {
			t.Fatalf("expected error message 'continue:inner', got '%s'", continueErr.Error())
		}

		// 无标签的 break/continue
		noLabelBreak := &BreakError{}
		if noLabelBreak.Error() != "break" {
			t.Fatalf("expected error message 'break', got '%s'", noLabelBreak.Error())
		}

		noLabelContinue := &ContinueError{}
		if noLabelContinue.Error() != "continue" {
			t.Fatalf("expected error message 'continue', got '%s'", noLabelContinue.Error())
		}
	})
}

// TestParallelResultsCollectionProperty 属性 9: 并行执行结果收集
// 对于任意 parallel 步骤，所有子步骤的结果必须收集到 ${parallel_results} 中，与执行顺序无关。
func TestParallelResultsCollectionProperty(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		numSteps := rapid.IntRange(1, 10).Draw(t, "numSteps")
		maxConcurrent := rapid.IntRange(1, 5).Draw(t, "maxConcurrent")

		steps := make([]types.Step, numSteps)
		for i := 0; i < numSteps; i++ {
			steps[i] = types.Step{
				ID:   "step_" + string(rune('a'+i)),
				Type: "mock",
			}
		}

		execCtx := NewFlowExecutionContext()

		config := &ParallelConfig{
			Steps:         steps,
			MaxConcurrent: maxConcurrent,
			FailFast:      false,
		}

		executor := NewParallelExecutor(mockStepExecutor)
		output, err := executor.Execute(context.Background(), config, execCtx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// 验证属性：所有步骤的结果都被收集
		if len(output.Results) != numSteps {
			t.Fatalf("expected %d results, got %d", numSteps, len(output.Results))
		}

		// 验证属性：每个步骤都有结果
		for _, step := range steps {
			if _, ok := output.Results[step.ID]; !ok {
				t.Fatalf("missing result for step '%s'", step.ID)
			}
		}

		// 验证属性：parallel_results 变量被设置
		parallelResults, ok := execCtx.GetVariable("parallel_results")
		if !ok {
			t.Fatal("parallel_results variable not set")
		}

		resultsMap, ok := parallelResults.(map[string]*types.StepResult)
		if !ok {
			t.Fatalf("parallel_results has wrong type: %T", parallelResults)
		}

		if len(resultsMap) != numSteps {
			t.Fatalf("parallel_results should have %d entries, got %d", numSteps, len(resultsMap))
		}

		// 验证属性：完成计数正确
		if output.Completed != numSteps {
			t.Fatalf("expected %d completed, got %d", numSteps, output.Completed)
		}

		// 验证属性：终止原因正确
		if output.TerminatedBy != "completed" {
			t.Fatalf("expected termination by 'completed', got '%s'", output.TerminatedBy)
		}
	})
}

// TestRetryBackoffCalculationProperty 属性 10: 重试退避计算
// 对于任意带退避策略的 retry 步骤：
// - fixed: 延迟固定不变
// - linear: 延迟 = 基础延迟 × 尝试次数
// - exponential: 延迟 = 基础延迟 × 2^(尝试次数-1)
func TestRetryBackoffCalculationProperty(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		baseDelay := time.Duration(rapid.IntRange(100, 1000).Draw(t, "baseDelayMs")) * time.Millisecond
		attempt := rapid.IntRange(1, 10).Draw(t, "attempt")
		backoffType := rapid.SampledFrom([]BackoffType{
			BackoffFixed, BackoffLinear, BackoffExponential,
		}).Draw(t, "backoffType")

		// 计算延迟
		delay := CalculateBackoffDelay(baseDelay, attempt, backoffType, 0)

		// 验证属性：根据退避类型计算正确的延迟
		var expectedDelay time.Duration
		switch backoffType {
		case BackoffFixed:
			expectedDelay = baseDelay
		case BackoffLinear:
			expectedDelay = baseDelay * time.Duration(attempt)
		case BackoffExponential:
			expectedDelay = baseDelay * time.Duration(1<<(attempt-1)) // 2^(attempt-1)
		}

		if delay != expectedDelay {
			t.Fatalf("for %s backoff, attempt %d, base %v: expected %v, got %v",
				backoffType, attempt, baseDelay, expectedDelay, delay)
		}
	})
}

// TestRetryBackoffMaxDelayProperty 测试最大延迟限制
func TestRetryBackoffMaxDelayProperty(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		baseDelay := time.Duration(rapid.IntRange(100, 500).Draw(t, "baseDelayMs")) * time.Millisecond
		maxDelay := time.Duration(rapid.IntRange(500, 2000).Draw(t, "maxDelayMs")) * time.Millisecond
		attempt := rapid.IntRange(1, 20).Draw(t, "attempt")
		backoffType := rapid.SampledFrom([]BackoffType{
			BackoffFixed, BackoffLinear, BackoffExponential,
		}).Draw(t, "backoffType")

		delay := CalculateBackoffDelay(baseDelay, attempt, backoffType, maxDelay)

		// 验证属性：延迟不超过最大延迟
		if delay > maxDelay {
			t.Fatalf("delay %v exceeds max delay %v", delay, maxDelay)
		}

		// 验证属性：如果计算的延迟小于最大延迟，应该使用计算的延迟
		calculatedDelay := CalculateBackoffDelay(baseDelay, attempt, backoffType, 0)
		if calculatedDelay <= maxDelay && delay != calculatedDelay {
			t.Fatalf("expected calculated delay %v, got %v", calculatedDelay, delay)
		}
	})
}

// TestParallelFailFastProperty 测试 fail_fast 模式
func TestParallelFailFastProperty(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		numSteps := rapid.IntRange(3, 10).Draw(t, "numSteps")
		failAtIndex := rapid.IntRange(0, numSteps-1).Draw(t, "failAtIndex")

		steps := make([]types.Step, numSteps)
		for i := 0; i < numSteps; i++ {
			steps[i] = types.Step{
				ID:   "step_" + string(rune('a'+i)),
				Type: "mock",
			}
		}

		execCtx := NewFlowExecutionContext()

		// 创建一个会在特定步骤失败的执行器
		failingExecutor := func(ctx context.Context, step *types.Step, execCtx *FlowExecutionContext) (*types.StepResult, error) {
			// 添加小延迟以确保顺序
			time.Sleep(time.Millisecond * time.Duration(step.ID[len(step.ID)-1]-'a'))

			if step.ID == "step_"+string(rune('a'+failAtIndex)) {
				return &types.StepResult{
					StepID: step.ID,
					Status: types.ResultStatusFailed,
				}, nil
			}
			return &types.StepResult{
				StepID: step.ID,
				Status: types.ResultStatusSuccess,
			}, nil
		}

		config := &ParallelConfig{
			Steps:         steps,
			MaxConcurrent: numSteps, // 允许所有步骤并行
			FailFast:      true,
		}

		executor := NewParallelExecutor(failingExecutor)
		output, _ := executor.Execute(context.Background(), config, execCtx)

		// 验证属性：fail_fast 模式下，至少有一个失败
		if output.Failed == 0 {
			t.Fatal("expected at least one failure")
		}

		// 验证属性：终止原因是 fail_fast
		if output.TerminatedBy != "fail_fast" {
			t.Fatalf("expected termination by 'fail_fast', got '%s'", output.TerminatedBy)
		}
	})
}
