package extractor

import (
	"context"
	"testing"

	"yqhp/workflow-engine/internal/keyword"
)

func TestExtract_DelegateToJSONPath(t *testing.T) {
	kw := Extract()
	ctx := context.Background()
	execCtx := keyword.NewExecutionContext()
	execCtx.SetResponse(&keyword.ResponseData{
		Body: `{"name": "test"}`,
	})

	result, err := kw.Execute(ctx, execCtx, map[string]any{
		"json_path": "$.name",
		"to":        "result",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.Success {
		t.Errorf("expected success, got: %s", result.Message)
	}

	val, _ := execCtx.GetVariable("result")
	if val != "test" {
		t.Errorf("got %v, want 'test'", val)
	}
}

func TestExtract_DelegateToRegex(t *testing.T) {
	kw := Extract()
	ctx := context.Background()
	execCtx := keyword.NewExecutionContext()
	execCtx.SetResponse(&keyword.ResponseData{
		Body: "Hello 123 World",
	})

	result, err := kw.Execute(ctx, execCtx, map[string]any{
		"regex": `\d+`,
		"to":    "number",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.Success {
		t.Errorf("expected success, got: %s", result.Message)
	}

	val, _ := execCtx.GetVariable("number")
	if val != "123" {
		t.Errorf("got %v, want '123'", val)
	}
}

func TestExtract_DelegateToHeader(t *testing.T) {
	kw := Extract()
	ctx := context.Background()
	execCtx := keyword.NewExecutionContext()
	execCtx.SetResponse(&keyword.ResponseData{
		Headers: map[string]string{"Content-Type": "application/json"},
	})

	result, err := kw.Execute(ctx, execCtx, map[string]any{
		"header": "Content-Type",
		"to":     "contentType",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.Success {
		t.Errorf("expected success, got: %s", result.Message)
	}

	val, _ := execCtx.GetVariable("contentType")
	if val != "application/json" {
		t.Errorf("got %v, want 'application/json'", val)
	}
}

func TestExtract_AutoDetectJSONPath(t *testing.T) {
	kw := Extract()
	ctx := context.Background()
	execCtx := keyword.NewExecutionContext()
	execCtx.SetResponse(&keyword.ResponseData{
		Body: `{"name": "auto"}`,
	})

	result, err := kw.Execute(ctx, execCtx, map[string]any{
		"expression": "$.name",
		"to":         "result",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.Success {
		t.Errorf("expected success, got: %s", result.Message)
	}

	val, _ := execCtx.GetVariable("result")
	if val != "auto" {
		t.Errorf("got %v, want 'auto'", val)
	}
}

func TestExtract_AutoDetectRegex(t *testing.T) {
	kw := Extract()
	ctx := context.Background()
	execCtx := keyword.NewExecutionContext()
	execCtx.SetResponse(&keyword.ResponseData{
		Body: "Hello 456 World",
	})

	result, err := kw.Execute(ctx, execCtx, map[string]any{
		"expression": `\d+`,
		"to":         "number",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.Success {
		t.Errorf("expected success, got: %s", result.Message)
	}

	val, _ := execCtx.GetVariable("number")
	if val != "456" {
		t.Errorf("got %v, want '456'", val)
	}
}

func TestExtract_Validate(t *testing.T) {
	kw := Extract()

	// Missing to
	err := kw.Validate(map[string]any{"json_path": "$.name"})
	if err == nil {
		t.Error("expected error for missing to")
	}

	// Missing extraction method
	err = kw.Validate(map[string]any{"to": "result"})
	if err == nil {
		t.Error("expected error for missing extraction method")
	}

	// Valid
	err = kw.Validate(map[string]any{"json_path": "$.name", "to": "result"})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRegisterAllExtractors(t *testing.T) {
	registry := keyword.NewRegistry()
	RegisterAllExtractors(registry)

	expected := []string{"extract", "json_path", "regex", "header", "cookie", "status", "body"}
	for _, name := range expected {
		if !registry.Has(name) {
			t.Errorf("expected keyword '%s' to be registered", name)
		}
	}
}
