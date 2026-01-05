package extractor

import (
	"context"
	"testing"

	"github.com/grafana/k6/workflow-engine/internal/keyword"
)

func TestJSONPath_Execute(t *testing.T) {
	kw := JSONPath()
	ctx := context.Background()

	tests := []struct {
		name       string
		body       string
		expression string
		to         string
		want       any
		wantErr    bool
	}{
		{
			name:       "simple field",
			body:       `{"name": "test", "value": 123}`,
			expression: "$.name",
			to:         "result",
			want:       "test",
		},
		{
			name:       "nested field",
			body:       `{"user": {"name": "john", "age": 30}}`,
			expression: "$.user.name",
			to:         "username",
			want:       "john",
		},
		{
			name:       "array index",
			body:       `{"items": [1, 2, 3, 4, 5]}`,
			expression: "$.items[2]",
			to:         "item",
			want:       float64(3),
		},
		{
			name:       "array all",
			body:       `{"items": [1, 2, 3]}`,
			expression: "$.items[*]",
			to:         "items",
			want:       []any{float64(1), float64(2), float64(3)},
		},
		{
			name:       "deep nested",
			body:       `{"a": {"b": {"c": {"d": "deep"}}}}`,
			expression: "$.a.b.c.d",
			to:         "deep",
			want:       "deep",
		},
		{
			name:       "no match",
			body:       `{"name": "test"}`,
			expression: "$.nonexistent",
			to:         "result",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			execCtx := keyword.NewExecutionContext()
			execCtx.SetResponse(&keyword.ResponseData{
				Body: tt.body,
			})

			result, err := kw.Execute(ctx, execCtx, map[string]any{
				"expression": tt.expression,
				"to":         tt.to,
			})
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

			// Check variable was set
			val, ok := execCtx.GetVariable(tt.to)
			if !ok {
				t.Errorf("variable '%s' not set", tt.to)
				return
			}

			// Compare results
			if !compareValues(val, tt.want) {
				t.Errorf("got %v (%T), want %v (%T)", val, val, tt.want, tt.want)
			}
		})
	}
}

func TestJSONPath_WithDefault(t *testing.T) {
	kw := JSONPath()
	ctx := context.Background()
	execCtx := keyword.NewExecutionContext()
	execCtx.SetResponse(&keyword.ResponseData{
		Body: `{"name": "test"}`,
	})

	result, err := kw.Execute(ctx, execCtx, map[string]any{
		"expression": "$.nonexistent",
		"to":         "result",
		"default":    "default_value",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.Success {
		t.Error("expected success with default value")
	}

	val, ok := execCtx.GetVariable("result")
	if !ok {
		t.Error("variable 'result' not set")
	}
	if val != "default_value" {
		t.Errorf("got %v, want 'default_value'", val)
	}
}

func TestJSONPath_WithIndex(t *testing.T) {
	kw := JSONPath()
	ctx := context.Background()
	execCtx := keyword.NewExecutionContext()
	execCtx.SetResponse(&keyword.ResponseData{
		Body: `{"items": ["a", "b", "c"]}`,
	})

	result, err := kw.Execute(ctx, execCtx, map[string]any{
		"expression": "$.items[*]",
		"to":         "item",
		"index":      1,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.Success {
		t.Errorf("expected success, got: %s", result.Message)
	}

	val, _ := execCtx.GetVariable("item")
	if val != "b" {
		t.Errorf("got %v, want 'b'", val)
	}
}

func TestJSONPath_WithDirectData(t *testing.T) {
	kw := JSONPath()
	ctx := context.Background()
	execCtx := keyword.NewExecutionContext()

	data := map[string]any{
		"user": map[string]any{
			"name": "direct",
		},
	}

	result, err := kw.Execute(ctx, execCtx, map[string]any{
		"expression": "$.user.name",
		"to":         "name",
		"data":       data,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.Success {
		t.Errorf("expected success, got: %s", result.Message)
	}

	val, _ := execCtx.GetVariable("name")
	if val != "direct" {
		t.Errorf("got %v, want 'direct'", val)
	}
}

func TestJSONPath_Validate(t *testing.T) {
	kw := JSONPath()

	// Missing expression
	err := kw.Validate(map[string]any{"to": "result"})
	if err == nil {
		t.Error("expected error for missing expression")
	}

	// Missing to
	err = kw.Validate(map[string]any{"expression": "$.name"})
	if err == nil {
		t.Error("expected error for missing to")
	}

	// Valid with expression
	err = kw.Validate(map[string]any{"expression": "$.name", "to": "result"})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Valid with json_path
	err = kw.Validate(map[string]any{"json_path": "$.name", "to": "result"})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func compareValues(a, b any) bool {
	// Handle slice comparison
	aSlice, aIsSlice := a.([]any)
	bSlice, bIsSlice := b.([]any)
	if aIsSlice && bIsSlice {
		if len(aSlice) != len(bSlice) {
			return false
		}
		for i := range aSlice {
			if !compareValues(aSlice[i], bSlice[i]) {
				return false
			}
		}
		return true
	}

	return a == b
}
