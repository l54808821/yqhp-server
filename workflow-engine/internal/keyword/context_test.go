package keyword

import (
	"sync"
	"testing"
)

func TestExecutionContext_Variables(t *testing.T) {
	ctx := NewExecutionContext()

	// Test SetVariable and GetVariable
	ctx.SetVariable("name", "test")
	val, ok := ctx.GetVariable("name")
	if !ok {
		t.Error("expected variable to exist")
	}
	if val != "test" {
		t.Errorf("expected 'test', got '%v'", val)
	}

	// Test GetVariable for nonexistent
	_, ok = ctx.GetVariable("nonexistent")
	if ok {
		t.Error("expected variable to not exist")
	}

	// Test DeleteVariable
	ctx.DeleteVariable("name")
	_, ok = ctx.GetVariable("name")
	if ok {
		t.Error("expected variable to be deleted")
	}
}

func TestExecutionContext_GetVariables(t *testing.T) {
	ctx := NewExecutionContext()
	ctx.SetVariable("a", 1)
	ctx.SetVariable("b", 2)

	vars := ctx.GetVariables()
	if len(vars) != 2 {
		t.Errorf("expected 2 variables, got %d", len(vars))
	}

	// Verify it's a copy
	vars["c"] = 3
	if _, ok := ctx.GetVariable("c"); ok {
		t.Error("GetVariables should return a copy")
	}
}

func TestExecutionContext_WithVars(t *testing.T) {
	initial := map[string]any{
		"x": 10,
		"y": 20,
	}
	ctx := NewExecutionContextWithVars(initial)

	val, ok := ctx.GetVariable("x")
	if !ok || val != 10 {
		t.Errorf("expected x=10, got %v", val)
	}

	val, ok = ctx.GetVariable("y")
	if !ok || val != 20 {
		t.Errorf("expected y=20, got %v", val)
	}
}

func TestExecutionContext_Response(t *testing.T) {
	ctx := NewExecutionContext()

	resp := &ResponseData{
		Status: 200,
		Body:   `{"message": "ok"}`,
		Data:   map[string]any{"message": "ok"},
	}
	ctx.SetResponse(resp)

	got := ctx.GetResponse()
	if got.Status != 200 {
		t.Errorf("expected status 200, got %d", got.Status)
	}

	// Verify response is also available as variable
	val, ok := ctx.GetVariable("response")
	if !ok {
		t.Error("expected response to be available as variable")
	}
	respVar, ok := val.(*ResponseData)
	if !ok || respVar.Status != 200 {
		t.Error("response variable should match")
	}
}

func TestExecutionContext_Metadata(t *testing.T) {
	ctx := NewExecutionContext()

	ctx.SetMetadata("key", "value")
	val, ok := ctx.GetMetadata("key")
	if !ok {
		t.Error("expected metadata to exist")
	}
	if val != "value" {
		t.Errorf("expected 'value', got '%v'", val)
	}

	_, ok = ctx.GetMetadata("nonexistent")
	if ok {
		t.Error("expected metadata to not exist")
	}
}

func TestExecutionContext_Clone(t *testing.T) {
	ctx := NewExecutionContext()
	ctx.SetVariable("a", 1)
	ctx.SetMetadata("m", "meta")
	ctx.SetResponse(&ResponseData{Status: 200})

	clone := ctx.Clone()

	// Verify clone has same values
	val, _ := clone.GetVariable("a")
	if val != 1 {
		t.Error("clone should have same variable")
	}

	meta, _ := clone.GetMetadata("m")
	if meta != "meta" {
		t.Error("clone should have same metadata")
	}

	if clone.GetResponse().Status != 200 {
		t.Error("clone should have same response")
	}

	// Verify clone is independent
	clone.SetVariable("a", 2)
	val, _ = ctx.GetVariable("a")
	if val != 1 {
		t.Error("original should not be affected by clone changes")
	}
}

func TestExecutionContext_Merge(t *testing.T) {
	ctx1 := NewExecutionContext()
	ctx1.SetVariable("a", 1)
	ctx1.SetVariable("b", 2)

	ctx2 := NewExecutionContext()
	ctx2.SetVariable("b", 20)
	ctx2.SetVariable("c", 30)

	ctx1.Merge(ctx2)

	val, _ := ctx1.GetVariable("a")
	if val != 1 {
		t.Error("a should remain 1")
	}

	val, _ = ctx1.GetVariable("b")
	if val != 20 {
		t.Error("b should be overwritten to 20")
	}

	val, _ = ctx1.GetVariable("c")
	if val != 30 {
		t.Error("c should be added as 30")
	}
}

func TestExecutionContext_Concurrent(t *testing.T) {
	ctx := NewExecutionContext()
	var wg sync.WaitGroup

	// Concurrent writes
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			ctx.SetVariable("key", n)
		}(i)
	}

	// Concurrent reads
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx.GetVariable("key")
		}()
	}

	wg.Wait()
	// If we get here without panic, concurrent access is safe
}

func TestResult(t *testing.T) {
	// Test success result
	success := NewSuccessResult("ok", map[string]any{"id": 1})
	if !success.Success {
		t.Error("expected success to be true")
	}
	if success.Message != "ok" {
		t.Errorf("expected message 'ok', got '%s'", success.Message)
	}

	// Test failure result
	failure := NewFailureResult("failed", nil)
	if failure.Success {
		t.Error("expected success to be false")
	}
	if failure.Message != "failed" {
		t.Errorf("expected message 'failed', got '%s'", failure.Message)
	}
}
