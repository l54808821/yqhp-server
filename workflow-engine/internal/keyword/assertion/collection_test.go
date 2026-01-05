package assertion

import (
	"context"
	"testing"

	"yqhp/workflow-engine/internal/keyword"
)

func TestIn(t *testing.T) {
	kw := In()
	ctx := context.Background()
	execCtx := keyword.NewExecutionContext()

	tests := []struct {
		name     string
		actual   any
		expected any
		want     bool
	}{
		{"int in slice", 2, []int{1, 2, 3}, true},
		{"int not in slice", 4, []int{1, 2, 3}, false},
		{"string in slice", "b", []string{"a", "b", "c"}, true},
		{"string not in slice", "d", []string{"a", "b", "c"}, false},
		{"any in interface slice", "hello", []any{"hello", 123, true}, true},
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

func TestNotIn(t *testing.T) {
	kw := NotIn()
	ctx := context.Background()
	execCtx := keyword.NewExecutionContext()

	tests := []struct {
		name     string
		actual   any
		expected any
		want     bool
	}{
		{"int not in slice", 4, []int{1, 2, 3}, true},
		{"int in slice", 2, []int{1, 2, 3}, false},
		{"string not in slice", "d", []string{"a", "b", "c"}, true},
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

func TestIsEmpty(t *testing.T) {
	kw := IsEmpty()
	ctx := context.Background()
	execCtx := keyword.NewExecutionContext()

	tests := []struct {
		name   string
		actual any
		want   bool
	}{
		{"empty string", "", true},
		{"non-empty string", "hello", false},
		{"empty slice", []int{}, true},
		{"non-empty slice", []int{1, 2, 3}, false},
		{"empty map", map[string]int{}, true},
		{"non-empty map", map[string]int{"a": 1}, false},
		{"nil", nil, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := kw.Execute(ctx, execCtx, map[string]any{
				"actual": tt.actual,
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

func TestIsNotEmpty(t *testing.T) {
	kw := IsNotEmpty()
	ctx := context.Background()
	execCtx := keyword.NewExecutionContext()

	tests := []struct {
		name   string
		actual any
		want   bool
	}{
		{"non-empty string", "hello", true},
		{"empty string", "", false},
		{"non-empty slice", []int{1, 2, 3}, true},
		{"empty slice", []int{}, false},
		{"nil", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := kw.Execute(ctx, execCtx, map[string]any{
				"actual": tt.actual,
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

func TestLengthEquals(t *testing.T) {
	kw := LengthEquals()
	ctx := context.Background()
	execCtx := keyword.NewExecutionContext()

	tests := []struct {
		name     string
		actual   any
		expected any
		want     bool
	}{
		{"string length", "hello", 5, true},
		{"string wrong length", "hello", 3, false},
		{"slice length", []int{1, 2, 3}, 3, true},
		{"slice wrong length", []int{1, 2, 3}, 5, false},
		{"map length", map[string]int{"a": 1, "b": 2}, 2, true},
		{"empty slice", []int{}, 0, true},
		{"float expected", []int{1, 2, 3}, 3.0, true},
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

func TestRegisterCollectionAssertions(t *testing.T) {
	registry := keyword.NewRegistry()
	RegisterCollectionAssertions(registry)

	expected := []string{"in", "not_in", "is_empty", "is_not_empty", "length_equals"}
	for _, name := range expected {
		if !registry.Has(name) {
			t.Errorf("expected keyword '%s' to be registered", name)
		}
	}
}
