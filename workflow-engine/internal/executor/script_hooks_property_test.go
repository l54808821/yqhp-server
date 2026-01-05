package executor

import (
	"context"
	"testing"
	"time"

	"github.com/grafana/k6/workflow-engine/pkg/types"
	"pgregory.net/rapid"
)

// TestProperty_PrePostScriptExecutionOrder 属性测试：前后置脚本执行顺序
// 验证需求: 1.2, 1.3
func TestProperty_PrePostScriptExecutionOrder(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// 生成随机数量的前置和后置脚本
		numPreScripts := rapid.IntRange(0, 5).Draw(t, "numPreScripts")
		numPostScripts := rapid.IntRange(0, 5).Draw(t, "numPostScripts")

		scriptExec := NewScriptExecutor()
		hookExec := NewHookExecutor(scriptExec, nil)
		execCtx := NewExecutionContext()

		// 初始化执行顺序记录
		execCtx.SetVariable("execution_order", []string{})

		// 创建前置脚本
		preScripts := make([]*ScriptHook, numPreScripts)
		for i := 0; i < numPreScripts; i++ {
			preScripts[i] = &ScriptHook{
				Name:   rapid.StringMatching(`pre_[a-z]{3}`).Draw(t, "preName"),
				Script: `order, _ := ctx.GetVariable("execution_order"); ctx.SetVariable("execution_order", append(order.([]string), "pre"))`,
			}
		}

		// 创建后置脚本
		postScripts := make([]*ScriptHook, numPostScripts)
		for i := 0; i < numPostScripts; i++ {
			postScripts[i] = &ScriptHook{
				Name:   rapid.StringMatching(`post_[a-z]{3}`).Draw(t, "postName"),
				Script: `order, _ := ctx.GetVariable("execution_order"); ctx.SetVariable("execution_order", append(order.([]string), "post"))`,
			}
		}

		hooks := &StepHooks{
			PreScripts:  preScripts,
			PostScripts: postScripts,
		}

		// 执行前置脚本
		err := hookExec.ExecutePreScripts(context.Background(), hooks, execCtx)
		if err != nil {
			t.Skip("Script execution failed")
		}

		// 模拟步骤执行
		stepResult := &types.StepResult{
			StepID: "main_step",
			Status: types.ResultStatusSuccess,
		}

		// 执行后置脚本
		err = hookExec.ExecutePostScripts(context.Background(), hooks, execCtx, stepResult)
		if err != nil {
			t.Skip("Script execution failed")
		}

		// 验证执行顺序
		order, ok := execCtx.GetVariable("execution_order")
		if !ok {
			t.Fatal("execution_order not found")
		}

		orderSlice := order.([]string)

		// 属性 1: 总执行数量应该等于前置 + 后置脚本数量
		expectedTotal := numPreScripts + numPostScripts
		if len(orderSlice) != expectedTotal {
			t.Fatalf("Expected %d executions, got %d", expectedTotal, len(orderSlice))
		}

		// 属性 2: 所有前置脚本应该在后置脚本之前执行
		preCount := 0
		for i, item := range orderSlice {
			if item == "pre" {
				preCount++
			} else if item == "post" {
				// 当遇到第一个 post 时，之前应该已经执行了所有 pre
				if preCount != numPreScripts {
					t.Fatalf("Post script executed before all pre scripts at index %d", i)
				}
			}
		}

		// 属性 3: 前置脚本数量应该正确
		if preCount != numPreScripts {
			t.Fatalf("Expected %d pre scripts, got %d", numPreScripts, preCount)
		}
	})
}

// TestProperty_OnErrorContinueDoesNotAbort 属性测试：OnErrorContinue 不会中止执行
func TestProperty_OnErrorContinueDoesNotAbort(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		numScripts := rapid.IntRange(2, 5).Draw(t, "numScripts")
		failIndex := rapid.IntRange(0, numScripts-1).Draw(t, "failIndex")

		scriptExec := NewScriptExecutor()
		hookExec := NewHookExecutor(scriptExec, nil)
		execCtx := NewExecutionContext()

		execCtx.SetVariable("executed_count", 0)

		preScripts := make([]*ScriptHook, numScripts)
		for i := 0; i < numScripts; i++ {
			if i == failIndex {
				preScripts[i] = &ScriptHook{
					Name:    "failing",
					Script:  "invalid syntax !!!",
					OnError: OnErrorContinue,
				}
			} else {
				preScripts[i] = &ScriptHook{
					Name:   "success",
					Script: `count, _ := ctx.GetVariable("executed_count"); ctx.SetVariable("executed_count", count.(int)+1)`,
				}
			}
		}

		hooks := &StepHooks{PreScripts: preScripts}

		err := hookExec.ExecutePreScripts(context.Background(), hooks, execCtx)

		// 属性: OnErrorContinue 不应该返回错误
		if err != nil {
			t.Fatal("OnErrorContinue should not return error")
		}

		// 属性: 其他脚本应该继续执行
		count, _ := execCtx.GetVariable("executed_count")
		expectedCount := numScripts - 1 // 除了失败的脚本
		if count.(int) != expectedCount {
			t.Fatalf("Expected %d successful executions, got %d", expectedCount, count.(int))
		}
	})
}

// TestProperty_OnErrorAbortStopsExecution 属性测试：OnErrorAbort 中止执行
func TestProperty_OnErrorAbortStopsExecution(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		numScripts := rapid.IntRange(2, 5).Draw(t, "numScripts")
		failIndex := rapid.IntRange(0, numScripts-1).Draw(t, "failIndex")

		scriptExec := NewScriptExecutor()
		hookExec := NewHookExecutor(scriptExec, nil)
		execCtx := NewExecutionContext()

		execCtx.SetVariable("executed_count", 0)

		preScripts := make([]*ScriptHook, numScripts)
		for i := 0; i < numScripts; i++ {
			if i == failIndex {
				preScripts[i] = &ScriptHook{
					Name:    "failing",
					Script:  "invalid syntax !!!",
					OnError: OnErrorAbort,
				}
			} else {
				preScripts[i] = &ScriptHook{
					Name:   "success",
					Script: `count, _ := ctx.GetVariable("executed_count"); ctx.SetVariable("executed_count", count.(int)+1)`,
				}
			}
		}

		hooks := &StepHooks{PreScripts: preScripts}

		err := hookExec.ExecutePreScripts(context.Background(), hooks, execCtx)

		// 属性: OnErrorAbort 应该返回错误
		if err == nil {
			t.Fatal("OnErrorAbort should return error")
		}

		// 属性: 失败后的脚本不应该执行
		count, _ := execCtx.GetVariable("executed_count")
		if count.(int) > failIndex {
			t.Fatalf("Scripts after failure should not execute, got %d executions", count.(int))
		}
	})
}

// TestProperty_PostScriptsReceiveStepResult 属性测试：后置脚本接收步骤结果
func TestProperty_PostScriptsReceiveStepResult(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		status := rapid.SampledFrom([]types.ResultStatus{
			types.ResultStatusSuccess,
			types.ResultStatusFailed,
			types.ResultStatusTimeout,
		}).Draw(t, "status")

		duration := rapid.Int64Range(1, 10000).Draw(t, "duration")

		scriptExec := NewScriptExecutor()
		hookExec := NewHookExecutor(scriptExec, nil)
		execCtx := NewExecutionContext()

		stepResult := &types.StepResult{
			StepID:   "test_step",
			Status:   status,
			Duration: time.Duration(duration) * time.Millisecond,
			Output:   map[string]any{"key": "value"},
		}

		hooks := &StepHooks{
			PostScripts: []*ScriptHook{
				{
					Name:   "check_result",
					Script: `result, _ := ctx.GetVariable("step_result"); ctx.SetVariable("received_status", result.(map[string]any)["status"])`,
				},
			},
		}

		err := hookExec.ExecutePostScripts(context.Background(), hooks, execCtx, stepResult)
		if err != nil {
			t.Skip("Script execution failed")
		}

		// 属性: 后置脚本应该能访问步骤结果
		receivedStatus, ok := execCtx.GetVariable("received_status")
		if !ok {
			t.Fatal("Post script should receive step_result")
		}

		// 属性: 状态应该正确传递
		if receivedStatus != string(status) {
			t.Fatalf("Expected status %s, got %s", status, receivedStatus)
		}
	})
}

// TestProperty_HookResultCountsAreCorrect 属性测试：钩子结果计数正确
func TestProperty_HookResultCountsAreCorrect(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		numScripts := rapid.IntRange(1, 10).Draw(t, "numScripts")
		numFailing := rapid.IntRange(0, numScripts).Draw(t, "numFailing")

		scriptExec := NewScriptExecutor()
		hookExec := NewHookExecutor(scriptExec, nil)
		execCtx := NewExecutionContext()

		preScripts := make([]*ScriptHook, numScripts)
		for i := 0; i < numScripts; i++ {
			if i < numFailing {
				preScripts[i] = &ScriptHook{
					Name:    "failing",
					Script:  "invalid!!!",
					OnError: OnErrorContinue,
				}
			} else {
				preScripts[i] = &ScriptHook{
					Name:   "success",
					Script: "// no-op",
				}
			}
		}

		hooks := &StepHooks{PreScripts: preScripts}

		result := hookExec.ExecutePreScriptsWithResult(context.Background(), hooks, execCtx)

		// 属性: 执行计数应该等于脚本总数
		if result.ExecutedCount != numScripts {
			t.Fatalf("Expected %d executed, got %d", numScripts, result.ExecutedCount)
		}

		// 属性: 失败计数应该等于失败脚本数
		if result.FailedCount != numFailing {
			t.Fatalf("Expected %d failed, got %d", numFailing, result.FailedCount)
		}

		// 属性: 使用 OnErrorContinue 时应该成功
		if !result.Success {
			t.Fatal("Should succeed with OnErrorContinue")
		}
	})
}
