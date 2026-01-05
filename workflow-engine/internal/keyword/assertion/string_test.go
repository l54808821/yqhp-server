package assertion

import (
	"context"
	"testing"

	"github.com/grafana/k6/workflow-engine/internal/keyword"
)

func TestContains(t *testing.T) {
	kw := Contains()
	ctx := context.Background()
	execCtx := keyword.NewExecutionContext()

	tests := []struct {
		name     string
		actual   any
		expected any
		want     bool
	}{
		{"contains substring", "hello world", "world", true},
		{"does not contain", "hello world", "foo", false},
		{"empty substring", "hello", "", true},
		{"exact match", "hello", "hello", true},
		{"case sensitive", "Hello", "hello", false},
		{"number to string", 12345, "234", true},
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

func TestNotContains(t *testing.T) {
	kw := NotContains()
	ctx := context.Background()
	execCtx := keyword.NewExecutionContext()

	tests := []struct {
		name     string
		actual   any
		expected any
		want     bool
	}{
		{"does not contain", "hello world", "foo", true},
		{"contains substring", "hello world", "world", false},
		{"case sensitive", "Hello", "hello", true},
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

func TestStartsWith(t *testing.T) {
	kw := StartsWith()
	ctx := context.Background()
	execCtx := keyword.NewExecutionContext()

	tests := []struct {
		name     string
		actual   any
		expected any
		want     bool
	}{
		{"starts with prefix", "hello world", "hello", true},
		{"does not start with", "hello world", "world", false},
		{"empty prefix", "hello", "", true},
		{"exact match", "hello", "hello", true},
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

func TestEndsWith(t *testing.T) {
	kw := EndsWith()
	ctx := context.Background()
	execCtx := keyword.NewExecutionContext()

	tests := []struct {
		name     string
		actual   any
		expected any
		want     bool
	}{
		{"ends with suffix", "hello world", "world", true},
		{"does not end with", "hello world", "hello", false},
		{"empty suffix", "hello", "", true},
		{"exact match", "hello", "hello", true},
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

func TestMatches(t *testing.T) {
	kw := Matches()
	ctx := context.Background()
	execCtx := keyword.NewExecutionContext()

	tests := []struct {
		name     string
		actual   any
		expected any
		want     bool
		wantErr  bool
	}{
		{"simple match", "hello123", `\d+`, true, false},
		{"no match", "hello", `\d+`, false, false},
		{"full match", "hello", `^hello$`, true, false},
		{"partial match", "hello world", `world`, true, false},
		{"email pattern", "test@example.com", `^[\w.-]+@[\w.-]+\.\w+$`, true, false},
		{"invalid regex", "hello", `[`, false, true},
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
					t.Error("expected failure for invalid regex")
				}
				return
			}
			if result.Success != tt.want {
				t.Errorf("expected success=%v, got %v", tt.want, result.Success)
			}
		})
	}
}

func TestRegisterStringAssertions(t *testing.T) {
	registry := keyword.NewRegistry()
	RegisterStringAssertions(registry)

	expected := []string{"contains", "not_contains", "starts_with", "ends_with", "matches"}
	for _, name := range expected {
		if !registry.Has(name) {
			t.Errorf("expected keyword '%s' to be registered", name)
		}
	}
}
