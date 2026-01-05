package assertion

import (
	"context"
	"testing"

	"github.com/grafana/k6/workflow-engine/internal/keyword"
)

func TestEquals(t *testing.T) {
	kw := Equals()
	ctx := context.Background()
	execCtx := keyword.NewExecutionContext()

	tests := []struct {
		name     string
		actual   any
		expected any
		want     bool
	}{
		{"int equal", 1, 1, true},
		{"int not equal", 1, 2, false},
		{"string equal", "hello", "hello", true},
		{"string not equal", "hello", "world", false},
		{"float equal", 1.5, 1.5, true},
		{"slice equal", []int{1, 2, 3}, []int{1, 2, 3}, true},
		{"slice not equal", []int{1, 2, 3}, []int{1, 2, 4}, false},
		{"map equal", map[string]int{"a": 1}, map[string]int{"a": 1}, true},
		{"nil equal", nil, nil, true},
		{"bool equal", true, true, true},
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

func TestNotEquals(t *testing.T) {
	kw := NotEquals()
	ctx := context.Background()
	execCtx := keyword.NewExecutionContext()

	tests := []struct {
		name     string
		actual   any
		expected any
		want     bool
	}{
		{"int not equal", 1, 2, true},
		{"int equal", 1, 1, false},
		{"string not equal", "hello", "world", true},
		{"string equal", "hello", "hello", false},
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

func TestGreaterThan(t *testing.T) {
	kw := GreaterThan()
	ctx := context.Background()
	execCtx := keyword.NewExecutionContext()

	tests := []struct {
		name     string
		actual   any
		expected any
		want     bool
		wantErr  bool
	}{
		{"int greater", 5, 3, true, false},
		{"int equal", 3, 3, false, false},
		{"int less", 2, 3, false, false},
		{"float greater", 5.5, 3.3, true, false},
		{"mixed types", 5, 3.0, true, false},
		{"non-numeric", "a", "b", false, true},
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
			if tt.wantErr {
				if result.Success {
					t.Error("expected failure for non-numeric comparison")
				}
				return
			}
			if result.Success != tt.want {
				t.Errorf("expected success=%v, got %v", tt.want, result.Success)
			}
		})
	}
}

func TestGreaterOrEqual(t *testing.T) {
	kw := GreaterOrEqual()
	ctx := context.Background()
	execCtx := keyword.NewExecutionContext()

	tests := []struct {
		name     string
		actual   any
		expected any
		want     bool
	}{
		{"int greater", 5, 3, true},
		{"int equal", 3, 3, true},
		{"int less", 2, 3, false},
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

func TestLessThan(t *testing.T) {
	kw := LessThan()
	ctx := context.Background()
	execCtx := keyword.NewExecutionContext()

	tests := []struct {
		name     string
		actual   any
		expected any
		want     bool
	}{
		{"int less", 2, 3, true},
		{"int equal", 3, 3, false},
		{"int greater", 5, 3, false},
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

func TestLessOrEqual(t *testing.T) {
	kw := LessOrEqual()
	ctx := context.Background()
	execCtx := keyword.NewExecutionContext()

	tests := []struct {
		name     string
		actual   any
		expected any
		want     bool
	}{
		{"int less", 2, 3, true},
		{"int equal", 3, 3, true},
		{"int greater", 5, 3, false},
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

func TestCompareKeyword_Validate(t *testing.T) {
	kw := Equals()

	// Missing actual
	err := kw.Validate(map[string]any{"expected": 1})
	if err == nil {
		t.Error("expected error for missing actual")
	}

	// Missing expected
	err = kw.Validate(map[string]any{"actual": 1})
	if err == nil {
		t.Error("expected error for missing expected")
	}

	// Valid
	err = kw.Validate(map[string]any{"actual": 1, "expected": 1})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRegisterCompareAssertions(t *testing.T) {
	registry := keyword.NewRegistry()
	RegisterCompareAssertions(registry)

	expected := []string{"equals", "not_equals", "greater_than", "greater_or_equal", "less_than", "less_or_equal"}
	for _, name := range expected {
		if !registry.Has(name) {
			t.Errorf("expected keyword '%s' to be registered", name)
		}
	}
}
