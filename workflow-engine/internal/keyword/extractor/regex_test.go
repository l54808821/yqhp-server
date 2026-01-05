package extractor

import (
	"context"
	"testing"

	"yqhp/workflow-engine/internal/keyword"
)

func TestRegex_Execute(t *testing.T) {
	kw := Regex()
	ctx := context.Background()

	tests := []struct {
		name    string
		body    string
		pattern string
		group   int
		to      string
		want    string
		wantErr bool
	}{
		{
			name:    "simple match",
			body:    "Hello World 123",
			pattern: `\d+`,
			to:      "number",
			want:    "123",
		},
		{
			name:    "capture group",
			body:    "name: John, age: 30",
			pattern: `name: (\w+)`,
			group:   1,
			to:      "name",
			want:    "John",
		},
		{
			name:    "email extraction",
			body:    "Contact: test@example.com for info",
			pattern: `[\w.-]+@[\w.-]+\.\w+`,
			to:      "email",
			want:    "test@example.com",
		},
		{
			name:    "no match",
			body:    "Hello World",
			pattern: `\d+`,
			to:      "number",
			wantErr: true,
		},
		{
			name:    "multiple groups",
			body:    "2024-01-15",
			pattern: `(\d{4})-(\d{2})-(\d{2})`,
			group:   2,
			to:      "month",
			want:    "01",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			execCtx := keyword.NewExecutionContext()
			execCtx.SetResponse(&keyword.ResponseData{
				Body: tt.body,
			})

			params := map[string]any{
				"expression": tt.pattern,
				"to":         tt.to,
			}
			if tt.group > 0 {
				params["group"] = tt.group
			}

			result, err := kw.Execute(ctx, execCtx, params)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tt.wantErr {
				if result.Success {
					t.Error("expected failure")
				}
				return
			}

			if !result.Success {
				t.Errorf("expected success, got failure: %s", result.Message)
				return
			}

			val, ok := execCtx.GetVariable(tt.to)
			if !ok {
				t.Errorf("variable '%s' not set", tt.to)
				return
			}

			if val != tt.want {
				t.Errorf("got %v, want %v", val, tt.want)
			}
		})
	}
}

func TestRegex_WithDefault(t *testing.T) {
	kw := Regex()
	ctx := context.Background()
	execCtx := keyword.NewExecutionContext()
	execCtx.SetResponse(&keyword.ResponseData{
		Body: "Hello World",
	})

	result, err := kw.Execute(ctx, execCtx, map[string]any{
		"expression": `\d+`,
		"to":         "number",
		"default":    "0",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.Success {
		t.Error("expected success with default value")
	}

	val, _ := execCtx.GetVariable("number")
	if val != "0" {
		t.Errorf("got %v, want '0'", val)
	}
}

func TestRegex_WithIndex(t *testing.T) {
	kw := Regex()
	ctx := context.Background()
	execCtx := keyword.NewExecutionContext()
	execCtx.SetResponse(&keyword.ResponseData{
		Body: "a1 b2 c3",
	})

	result, err := kw.Execute(ctx, execCtx, map[string]any{
		"expression": `\w(\d)`,
		"to":         "digit",
		"index":      1,
		"group":      1,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.Success {
		t.Errorf("expected success, got: %s", result.Message)
	}

	val, _ := execCtx.GetVariable("digit")
	if val != "2" {
		t.Errorf("got %v, want '2'", val)
	}
}

func TestRegex_Validate(t *testing.T) {
	kw := Regex()

	// Missing expression
	err := kw.Validate(map[string]any{"to": "result"})
	if err == nil {
		t.Error("expected error for missing expression")
	}

	// Missing to
	err = kw.Validate(map[string]any{"expression": `\d+`})
	if err == nil {
		t.Error("expected error for missing to")
	}

	// Valid with expression
	err = kw.Validate(map[string]any{"expression": `\d+`, "to": "result"})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Valid with regex
	err = kw.Validate(map[string]any{"regex": `\d+`, "to": "result"})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Valid with pattern
	err = kw.Validate(map[string]any{"pattern": `\d+`, "to": "result"})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}
