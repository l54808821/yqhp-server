package keyword

import (
	"context"
	"sync/atomic"
	"testing"

	"pgregory.net/rapid"
)

// orderTrackingKeyword tracks execution order for property testing.
type orderTrackingKeyword struct {
	BaseKeyword
	counter *int64
	records *[]int64
}

func newOrderTrackingKeyword(name string, counter *int64, records *[]int64) *orderTrackingKeyword {
	return &orderTrackingKeyword{
		BaseKeyword: NewBaseKeyword(name, CategoryAction, "tracks execution order"),
		counter:     counter,
		records:     records,
	}
}

func (k *orderTrackingKeyword) Execute(ctx context.Context, execCtx *ExecutionContext, params map[string]any) (*Result, error) {
	order := atomic.AddInt64(k.counter, 1)
	*k.records = append(*k.records, order)
	return NewSuccessResult("executed", order), nil
}

func (k *orderTrackingKeyword) Validate(params map[string]any) error {
	return nil
}

// TestProperty_PrePostScriptExecutionOrder tests Property 1:
// For any step with pre_scripts and post_scripts, the execution order must be:
// all pre_scripts in order -> main step -> all post_scripts in order.
//
// **Property 1: 前后置脚本执行顺序**
// **Validates: Requirements 1.2, 1.3**
func TestProperty_PrePostScriptExecutionOrder(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate random number of pre and post scripts (0-10 each)
		numPreScripts := rapid.IntRange(0, 10).Draw(t, "numPreScripts")
		numPostScripts := rapid.IntRange(0, 10).Draw(t, "numPostScripts")

		// Setup tracking
		var counter int64
		var preRecords, mainRecords, postRecords []int64

		// Create registry with tracking keywords
		registry := NewRegistry()

		// Register pre-script keywords
		preActions := make([]Action, numPreScripts)
		for i := 0; i < numPreScripts; i++ {
			name := rapid.StringMatching(`pre_[a-z]{3,8}`).Draw(t, "preKeywordName")
			// Ensure unique name
			for registry.Has(name) {
				name = rapid.StringMatching(`pre_[a-z]{3,8}`).Draw(t, "preKeywordName")
			}
			kw := newOrderTrackingKeyword(name, &counter, &preRecords)
			registry.MustRegister(kw)
			preActions[i] = Action{Keyword: name}
		}

		// Register post-script keywords
		postActions := make([]Action, numPostScripts)
		for i := 0; i < numPostScripts; i++ {
			name := rapid.StringMatching(`post_[a-z]{3,8}`).Draw(t, "postKeywordName")
			// Ensure unique name
			for registry.Has(name) {
				name = rapid.StringMatching(`post_[a-z]{3,8}`).Draw(t, "postKeywordName")
			}
			kw := newOrderTrackingKeyword(name, &counter, &postRecords)
			registry.MustRegister(kw)
			postActions[i] = Action{Keyword: name}
		}

		// Execute step
		executor := NewScriptExecutor(registry)
		execCtx := NewExecutionContext()

		mainAction := func() error {
			order := atomic.AddInt64(&counter, 1)
			mainRecords = append(mainRecords, order)
			return nil
		}

		_, err := executor.ExecuteStep(context.Background(), execCtx, preActions, mainAction, postActions)
		if err != nil {
			t.Fatalf("execution failed: %v", err)
		}

		// Verify Property 1: Execution order must be pre -> main -> post

		// 1. All pre_scripts must execute before main
		for _, preOrder := range preRecords {
			for _, mainOrder := range mainRecords {
				if preOrder >= mainOrder {
					t.Errorf("pre_script (order %d) executed after or at same time as main (order %d)", preOrder, mainOrder)
				}
			}
		}

		// 2. Main must execute before all post_scripts
		for _, mainOrder := range mainRecords {
			for _, postOrder := range postRecords {
				if mainOrder >= postOrder {
					t.Errorf("main (order %d) executed after or at same time as post_script (order %d)", mainOrder, postOrder)
				}
			}
		}

		// 3. Pre_scripts must execute in order
		for i := 1; i < len(preRecords); i++ {
			if preRecords[i-1] >= preRecords[i] {
				t.Errorf("pre_scripts not in order: %d >= %d", preRecords[i-1], preRecords[i])
			}
		}

		// 4. Post_scripts must execute in order
		for i := 1; i < len(postRecords); i++ {
			if postRecords[i-1] >= postRecords[i] {
				t.Errorf("post_scripts not in order: %d >= %d", postRecords[i-1], postRecords[i])
			}
		}

		// 5. Total execution count must match
		expectedTotal := int64(numPreScripts + 1 + numPostScripts) // pre + main + post
		if counter != expectedTotal {
			t.Errorf("expected %d executions, got %d", expectedTotal, counter)
		}
	})
}

// TestProperty_PreScriptFailureStopsExecution tests that if a pre_script fails,
// subsequent scripts and main action are not executed.
// **Validates: Requirements 1.4**
func TestProperty_PreScriptFailureStopsExecution(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		numPreScripts := rapid.IntRange(2, 5).Draw(t, "numPreScripts")
		failAtIndex := rapid.IntRange(0, numPreScripts-1).Draw(t, "failAtIndex")

		var counter int64
		var executedKeywords []string

		registry := NewRegistry()

		// Create pre-script keywords, one of which will fail
		preActions := make([]Action, numPreScripts)
		for i := 0; i < numPreScripts; i++ {
			name := rapid.StringMatching(`pre_[a-z]{3,8}`).Draw(t, "preKeywordName")
			for registry.Has(name) {
				name = rapid.StringMatching(`pre_[a-z]{3,8}`).Draw(t, "preKeywordName")
			}

			var kw Keyword
			if i == failAtIndex {
				kw = &failingKeyword{
					BaseKeyword:      NewBaseKeyword(name, CategoryAction, "fails"),
					executedKeywords: &executedKeywords,
				}
			} else {
				kw = &trackingKeyword{
					BaseKeyword:      NewBaseKeyword(name, CategoryAction, "tracks"),
					counter:          &counter,
					executedKeywords: &executedKeywords,
				}
			}
			registry.MustRegister(kw)
			preActions[i] = Action{Keyword: name}
		}

		executor := NewScriptExecutor(registry)
		execCtx := NewExecutionContext()

		mainExecuted := false
		mainAction := func() error {
			mainExecuted = true
			return nil
		}

		_, err := executor.ExecuteStep(context.Background(), execCtx, preActions, mainAction, nil)

		// Should have error
		if err == nil {
			t.Error("expected error from failing pre_script")
		}

		// Main should not have executed
		if mainExecuted {
			t.Error("main action should not execute when pre_script fails")
		}

		// Only keywords before the failing one should have executed
		if len(executedKeywords) != failAtIndex+1 {
			t.Errorf("expected %d keywords to execute, got %d", failAtIndex+1, len(executedKeywords))
		}
	})
}

// trackingKeyword tracks execution without failing.
type trackingKeyword struct {
	BaseKeyword
	counter          *int64
	executedKeywords *[]string
}

func (k *trackingKeyword) Execute(ctx context.Context, execCtx *ExecutionContext, params map[string]any) (*Result, error) {
	atomic.AddInt64(k.counter, 1)
	*k.executedKeywords = append(*k.executedKeywords, k.Name())
	return NewSuccessResult("executed", nil), nil
}

func (k *trackingKeyword) Validate(params map[string]any) error {
	return nil
}

// failingKeyword always fails.
type failingKeyword struct {
	BaseKeyword
	executedKeywords *[]string
}

func (k *failingKeyword) Execute(ctx context.Context, execCtx *ExecutionContext, params map[string]any) (*Result, error) {
	*k.executedKeywords = append(*k.executedKeywords, k.Name())
	return NewFailureResult("intentional failure", nil), nil
}

func (k *failingKeyword) Validate(params map[string]any) error {
	return nil
}
