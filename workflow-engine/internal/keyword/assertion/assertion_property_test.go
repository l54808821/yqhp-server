package assertion

import (
	"context"
	"strings"
	"testing"

	"yqhp/workflow-engine/internal/keyword"
	"pgregory.net/rapid"
)

// TestProperty_AssertionKeywordCorrectness tests Property 2:
// For any assertion keyword and any value pair (actual, expected),
// the assertion result must be mathematically correct.
//
// **Property 2: 断言关键字正确性**
// **Validates: Requirements 2.1, 2.2, 2.3, 2.4**
func TestProperty_AssertionKeywordCorrectness(t *testing.T) {
	ctx := context.Background()
	execCtx := keyword.NewExecutionContext()

	t.Run("equals_correctness", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			kw := Equals()

			// Generate random values
			a := rapid.IntRange(-1000, 1000).Draw(t, "a")
			b := rapid.IntRange(-1000, 1000).Draw(t, "b")

			result, err := kw.Execute(ctx, execCtx, map[string]any{
				"actual":   a,
				"expected": b,
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Property: equals(a, b) returns true iff a == b
			expected := a == b
			if result.Success != expected {
				t.Errorf("equals(%d, %d) = %v, want %v", a, b, result.Success, expected)
			}
		})
	})

	t.Run("not_equals_correctness", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			kw := NotEquals()

			a := rapid.IntRange(-1000, 1000).Draw(t, "a")
			b := rapid.IntRange(-1000, 1000).Draw(t, "b")

			result, err := kw.Execute(ctx, execCtx, map[string]any{
				"actual":   a,
				"expected": b,
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Property: not_equals(a, b) returns true iff a != b
			expected := a != b
			if result.Success != expected {
				t.Errorf("not_equals(%d, %d) = %v, want %v", a, b, result.Success, expected)
			}
		})
	})

	t.Run("greater_than_correctness", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			kw := GreaterThan()

			a := rapid.Float64Range(-1000, 1000).Draw(t, "a")
			b := rapid.Float64Range(-1000, 1000).Draw(t, "b")

			result, err := kw.Execute(ctx, execCtx, map[string]any{
				"actual":   a,
				"expected": b,
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Property: greater_than(a, b) returns true iff a > b
			expected := a > b
			if result.Success != expected {
				t.Errorf("greater_than(%f, %f) = %v, want %v", a, b, result.Success, expected)
			}
		})
	})

	t.Run("greater_or_equal_correctness", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			kw := GreaterOrEqual()

			a := rapid.Float64Range(-1000, 1000).Draw(t, "a")
			b := rapid.Float64Range(-1000, 1000).Draw(t, "b")

			result, err := kw.Execute(ctx, execCtx, map[string]any{
				"actual":   a,
				"expected": b,
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Property: greater_or_equal(a, b) returns true iff a >= b
			expected := a >= b
			if result.Success != expected {
				t.Errorf("greater_or_equal(%f, %f) = %v, want %v", a, b, result.Success, expected)
			}
		})
	})

	t.Run("less_than_correctness", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			kw := LessThan()

			a := rapid.Float64Range(-1000, 1000).Draw(t, "a")
			b := rapid.Float64Range(-1000, 1000).Draw(t, "b")

			result, err := kw.Execute(ctx, execCtx, map[string]any{
				"actual":   a,
				"expected": b,
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Property: less_than(a, b) returns true iff a < b
			expected := a < b
			if result.Success != expected {
				t.Errorf("less_than(%f, %f) = %v, want %v", a, b, result.Success, expected)
			}
		})
	})

	t.Run("less_or_equal_correctness", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			kw := LessOrEqual()

			a := rapid.Float64Range(-1000, 1000).Draw(t, "a")
			b := rapid.Float64Range(-1000, 1000).Draw(t, "b")

			result, err := kw.Execute(ctx, execCtx, map[string]any{
				"actual":   a,
				"expected": b,
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Property: less_or_equal(a, b) returns true iff a <= b
			expected := a <= b
			if result.Success != expected {
				t.Errorf("less_or_equal(%f, %f) = %v, want %v", a, b, result.Success, expected)
			}
		})
	})

	t.Run("contains_correctness", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			kw := Contains()

			s := rapid.String().Draw(t, "s")
			sub := rapid.String().Draw(t, "sub")

			result, err := kw.Execute(ctx, execCtx, map[string]any{
				"actual":   s,
				"expected": sub,
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Property: contains(s, sub) returns true iff s contains sub
			expected := strings.Contains(s, sub)
			if result.Success != expected {
				t.Errorf("contains(%q, %q) = %v, want %v", s, sub, result.Success, expected)
			}
		})
	})

	t.Run("not_contains_correctness", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			kw := NotContains()

			s := rapid.String().Draw(t, "s")
			sub := rapid.String().Draw(t, "sub")

			result, err := kw.Execute(ctx, execCtx, map[string]any{
				"actual":   s,
				"expected": sub,
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Property: not_contains(s, sub) returns true iff s does not contain sub
			expected := !strings.Contains(s, sub)
			if result.Success != expected {
				t.Errorf("not_contains(%q, %q) = %v, want %v", s, sub, result.Success, expected)
			}
		})
	})

	t.Run("starts_with_correctness", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			kw := StartsWith()

			s := rapid.String().Draw(t, "s")
			prefix := rapid.String().Draw(t, "prefix")

			result, err := kw.Execute(ctx, execCtx, map[string]any{
				"actual":   s,
				"expected": prefix,
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Property: starts_with(s, prefix) returns true iff s starts with prefix
			expected := strings.HasPrefix(s, prefix)
			if result.Success != expected {
				t.Errorf("starts_with(%q, %q) = %v, want %v", s, prefix, result.Success, expected)
			}
		})
	})

	t.Run("ends_with_correctness", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			kw := EndsWith()

			s := rapid.String().Draw(t, "s")
			suffix := rapid.String().Draw(t, "suffix")

			result, err := kw.Execute(ctx, execCtx, map[string]any{
				"actual":   s,
				"expected": suffix,
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Property: ends_with(s, suffix) returns true iff s ends with suffix
			expected := strings.HasSuffix(s, suffix)
			if result.Success != expected {
				t.Errorf("ends_with(%q, %q) = %v, want %v", s, suffix, result.Success, expected)
			}
		})
	})

	t.Run("in_correctness", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			kw := In()

			// Generate a slice and a value
			slice := rapid.SliceOf(rapid.IntRange(-100, 100)).Draw(t, "slice")
			val := rapid.IntRange(-100, 100).Draw(t, "val")

			result, err := kw.Execute(ctx, execCtx, map[string]any{
				"actual":   val,
				"expected": slice,
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Property: in(val, slice) returns true iff val is in slice
			expected := false
			for _, v := range slice {
				if v == val {
					expected = true
					break
				}
			}
			if result.Success != expected {
				t.Errorf("in(%d, %v) = %v, want %v", val, slice, result.Success, expected)
			}
		})
	})

	t.Run("not_in_correctness", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			kw := NotIn()

			slice := rapid.SliceOf(rapid.IntRange(-100, 100)).Draw(t, "slice")
			val := rapid.IntRange(-100, 100).Draw(t, "val")

			result, err := kw.Execute(ctx, execCtx, map[string]any{
				"actual":   val,
				"expected": slice,
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Property: not_in(val, slice) returns true iff val is not in slice
			found := false
			for _, v := range slice {
				if v == val {
					found = true
					break
				}
			}
			expected := !found
			if result.Success != expected {
				t.Errorf("not_in(%d, %v) = %v, want %v", val, slice, result.Success, expected)
			}
		})
	})

	t.Run("is_empty_correctness", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			kw := IsEmpty()

			// Generate either empty or non-empty slice
			isEmpty := rapid.Bool().Draw(t, "isEmpty")
			var slice []int
			if !isEmpty {
				slice = rapid.SliceOfN(rapid.Int(), 1, 10).Draw(t, "slice")
			}

			result, err := kw.Execute(ctx, execCtx, map[string]any{
				"actual": slice,
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Property: is_empty(slice) returns true iff len(slice) == 0
			expected := len(slice) == 0
			if result.Success != expected {
				t.Errorf("is_empty(%v) = %v, want %v", slice, result.Success, expected)
			}
		})
	})

	t.Run("is_not_empty_correctness", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			kw := IsNotEmpty()

			isEmpty := rapid.Bool().Draw(t, "isEmpty")
			var slice []int
			if !isEmpty {
				slice = rapid.SliceOfN(rapid.Int(), 1, 10).Draw(t, "slice")
			}

			result, err := kw.Execute(ctx, execCtx, map[string]any{
				"actual": slice,
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Property: is_not_empty(slice) returns true iff len(slice) > 0
			expected := len(slice) > 0
			if result.Success != expected {
				t.Errorf("is_not_empty(%v) = %v, want %v", slice, result.Success, expected)
			}
		})
	})

	t.Run("length_equals_correctness", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			kw := LengthEquals()

			slice := rapid.SliceOf(rapid.Int()).Draw(t, "slice")
			expectedLen := rapid.IntRange(0, 20).Draw(t, "expectedLen")

			result, err := kw.Execute(ctx, execCtx, map[string]any{
				"actual":   slice,
				"expected": expectedLen,
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Property: length_equals(slice, n) returns true iff len(slice) == n
			expected := len(slice) == expectedLen
			if result.Success != expected {
				t.Errorf("length_equals(%v, %d) = %v, want %v", slice, expectedLen, result.Success, expected)
			}
		})
	})
}

// TestProperty_ComparisonSymmetry tests that comparison operators have correct symmetry.
func TestProperty_ComparisonSymmetry(t *testing.T) {
	ctx := context.Background()
	execCtx := keyword.NewExecutionContext()

	t.Run("equals_symmetry", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			kw := Equals()

			a := rapid.IntRange(-1000, 1000).Draw(t, "a")
			b := rapid.IntRange(-1000, 1000).Draw(t, "b")

			result1, _ := kw.Execute(ctx, execCtx, map[string]any{"actual": a, "expected": b})
			result2, _ := kw.Execute(ctx, execCtx, map[string]any{"actual": b, "expected": a})

			// Property: equals(a, b) == equals(b, a)
			if result1.Success != result2.Success {
				t.Errorf("equals symmetry violated: equals(%d, %d) = %v, equals(%d, %d) = %v",
					a, b, result1.Success, b, a, result2.Success)
			}
		})
	})

	t.Run("greater_less_inverse", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			gtKw := GreaterThan()
			ltKw := LessThan()

			a := rapid.Float64Range(-1000, 1000).Draw(t, "a")
			b := rapid.Float64Range(-1000, 1000).Draw(t, "b")

			gtResult, _ := gtKw.Execute(ctx, execCtx, map[string]any{"actual": a, "expected": b})
			ltResult, _ := ltKw.Execute(ctx, execCtx, map[string]any{"actual": b, "expected": a})

			// Property: greater_than(a, b) == less_than(b, a)
			if gtResult.Success != ltResult.Success {
				t.Errorf("greater/less inverse violated: gt(%f, %f) = %v, lt(%f, %f) = %v",
					a, b, gtResult.Success, b, a, ltResult.Success)
			}
		})
	})
}
