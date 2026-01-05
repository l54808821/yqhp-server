package assertion

import (
	"context"
	"testing"

	"github.com/grafana/k6/workflow-engine/internal/keyword"
)

func TestIsNull(t *testing.T) {
	kw := IsNull()
	ctx := context.Background()
	execCtx := keyword.NewExecutionContext()

	tests := []struct {
		name   string
		params map[string]any
		want   bool
	}{
		{"nil value", map[string]any{"actual": nil}, true},
		{"missing actual", map[string]any{}, true},
		{"non-nil string", map[string]any{"actual": "hello"}, false},
		{"non-nil int", map[string]any{"actual": 123}, false},
		{"empty string", map[string]any{"actual": ""}, false},
		{"zero int", map[string]any{"actual": 0}, false},
		{"nil slice", map[string]any{"actual": ([]int)(nil)}, true},
		{"nil map", map[string]any{"actual": (map[string]int)(nil)}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := kw.Execute(ctx, execCtx, tt.params)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.Success != tt.want {
				t.Errorf("expected success=%v, got %v", tt.want, result.Success)
			}
		})
	}
}

func TestIsNotNull(t *testing.T) {
	kw := IsNotNull()
	ctx := context.Background()
	execCtx := keyword.NewExecutionContext()

	tests := []struct {
		name   string
		params map[string]any
		want   bool
	}{
		{"non-nil string", map[string]any{"actual": "hello"}, true},
		{"non-nil int", map[string]any{"actual": 123}, true},
		{"empty string", map[string]any{"actual": ""}, true},
		{"zero int", map[string]any{"actual": 0}, true},
		{"nil value", map[string]any{"actual": nil}, false},
		{"non-empty slice", map[string]any{"actual": []int{1, 2, 3}}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := kw.Execute(ctx, execCtx, tt.params)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.Success != tt.want {
				t.Errorf("expected success=%v, got %v", tt.want, result.Success)
			}
		})
	}
}

func TestIsType(t *testing.T) {
	kw := IsType()
	ctx := context.Background()
	execCtx := keyword.NewExecutionContext()

	tests := []struct {
		name     string
		actual   any
		expected string
		want     bool
	}{
		{"string type", "hello", "string", true},
		{"string wrong type", "hello", "number", false},
		{"int type", 123, "number", true},
		{"float type", 1.5, "number", true},
		{"bool type", true, "boolean", true},
		{"slice type", []int{1, 2, 3}, "array", true},
		{"map type", map[string]int{"a": 1}, "object", true},
		{"nil type", nil, "null", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := kw.Execute(ctx, execCtx, map[string]any{
				"actual":   tt.actual,
				"expected": tt.expected,
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.Success != tt.want {
				t.Errorf("expected success=%v, got %v", tt.want, result.Success)
			}
		})
	}
}

func TestGetTypeName(t *testing.T) {
	tests := []struct {
		name     string
		value    any
		expected string
	}{
		{"string", "hello", "string"},
		{"int", 123, "number"},
		{"int64", int64(123), "number"},
		{"float64", 1.5, "number"},
		{"bool", true, "boolean"},
		{"slice", []int{1, 2, 3}, "array"},
		{"map", map[string]int{"a": 1}, "object"},
		{"nil", nil, "null"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getTypeName(tt.value)
			if result != tt.expected {
				t.Errorf("expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

func TestRegisterTypeAssertions(t *testing.T) {
	registry := keyword.NewRegistry()
	RegisterTypeAssertions(registry)

	expected := []string{"is_null", "is_not_null", "is_type"}
	for _, name := range expected {
		if !registry.Has(name) {
			t.Errorf("expected keyword '%s' to be registered", name)
		}
	}
}
