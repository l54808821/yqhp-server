// Package expression provides property-based tests for the expression evaluator.
// Requirements: 3.3, 3.4, 3.5 - Expression evaluation correctness
// Property 4: For any valid expression with comparison operators (==, !=, <, >, <=, >=)
// and logical operators (AND, OR, NOT), the evaluation result should match the expected boolean logic.
package expression

import (
	"fmt"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
	"github.com/stretchr/testify/assert"
)

// TestExpressionEvaluationProperty tests Property 4: Expression evaluation correctness.
// evaluate(expr) == expected_boolean_result
func TestExpressionEvaluationProperty(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	// Property: Integer comparison operators produce correct results
	properties.Property("integer comparison operators are correct", prop.ForAll(
		func(left, right int, op string) bool {
			expr := fmt.Sprintf("${a} %s ${b}", op)
			ctx := NewEvaluationContext().WithVariables(map[string]any{
				"a": left,
				"b": right,
			})

			result, err := Evaluate(expr, ctx)
			if err != nil {
				return false
			}

			expected := computeIntComparison(left, right, op)
			return result == expected
		},
		gen.Int(),
		gen.Int(),
		gen.OneConstOf("==", "!=", "<", ">", "<=", ">="),
	))

	// Property: Float comparison operators produce correct results
	properties.Property("float comparison operators are correct", prop.ForAll(
		func(left, right float64, op string) bool {
			expr := fmt.Sprintf("${a} %s ${b}", op)
			ctx := NewEvaluationContext().WithVariables(map[string]any{
				"a": left,
				"b": right,
			})

			result, err := Evaluate(expr, ctx)
			if err != nil {
				return false
			}

			expected := computeFloatComparison(left, right, op)
			return result == expected
		},
		gen.Float64Range(-1000, 1000),
		gen.Float64Range(-1000, 1000),
		gen.OneConstOf("==", "!=", "<", ">", "<=", ">="),
	))

	// Property: AND operator follows boolean logic
	properties.Property("AND operator follows boolean logic", prop.ForAll(
		func(left, right bool) bool {
			ctx := NewEvaluationContext().WithVariables(map[string]any{
				"a": left,
				"b": right,
			})

			result, err := Evaluate("${a} AND ${b}", ctx)
			if err != nil {
				return false
			}

			expected := left && right
			return result == expected
		},
		gen.Bool(),
		gen.Bool(),
	))

	// Property: OR operator follows boolean logic
	properties.Property("OR operator follows boolean logic", prop.ForAll(
		func(left, right bool) bool {
			ctx := NewEvaluationContext().WithVariables(map[string]any{
				"a": left,
				"b": right,
			})

			result, err := Evaluate("${a} OR ${b}", ctx)
			if err != nil {
				return false
			}

			expected := left || right
			return result == expected
		},
		gen.Bool(),
		gen.Bool(),
	))

	// Property: NOT operator follows boolean logic
	properties.Property("NOT operator follows boolean logic", prop.ForAll(
		func(value bool) bool {
			ctx := NewEvaluationContext().WithVariables(map[string]any{
				"a": value,
			})

			result, err := Evaluate("NOT ${a}", ctx)
			if err != nil {
				return false
			}

			expected := !value
			return result == expected
		},
		gen.Bool(),
	))

	properties.TestingRun(t)
}

// TestCompoundExpressionProperty tests compound expressions.
func TestCompoundExpressionProperty(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	// Property: Compound AND expressions are correct
	properties.Property("compound AND expressions are correct", prop.ForAll(
		func(a, b, c int) bool {
			ctx := NewEvaluationContext().WithVariables(map[string]any{
				"a": a,
				"b": b,
				"c": c,
			})

			result, err := Evaluate("${a} > ${b} AND ${b} > ${c}", ctx)
			if err != nil {
				return false
			}

			expected := (a > b) && (b > c)
			return result == expected
		},
		gen.IntRange(-100, 100),
		gen.IntRange(-100, 100),
		gen.IntRange(-100, 100),
	))

	// Property: Compound OR expressions are correct
	properties.Property("compound OR expressions are correct", prop.ForAll(
		func(a, b, c int) bool {
			ctx := NewEvaluationContext().WithVariables(map[string]any{
				"a": a,
				"b": b,
				"c": c,
			})

			result, err := Evaluate("${a} == ${b} OR ${b} == ${c}", ctx)
			if err != nil {
				return false
			}

			expected := (a == b) || (b == c)
			return result == expected
		},
		gen.IntRange(-100, 100),
		gen.IntRange(-100, 100),
		gen.IntRange(-100, 100),
	))

	// Property: Mixed AND/OR expressions are correct
	properties.Property("mixed AND/OR expressions are correct", prop.ForAll(
		func(a, b bool, x, y int) bool {
			ctx := NewEvaluationContext().WithVariables(map[string]any{
				"a": a,
				"b": b,
				"x": x,
				"y": y,
			})

			result, err := Evaluate("${a} AND ${b} OR ${x} > ${y}", ctx)
			if err != nil {
				return false
			}

			// AND has higher precedence than OR
			expected := (a && b) || (x > y)
			return result == expected
		},
		gen.Bool(),
		gen.Bool(),
		gen.IntRange(-100, 100),
		gen.IntRange(-100, 100),
	))

	properties.TestingRun(t)
}

// TestStringComparisonProperty tests string comparison expressions.
func TestStringComparisonProperty(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	// Property: String equality comparison is correct
	properties.Property("string equality comparison is correct", prop.ForAll(
		func(s1, s2 string) bool {
			ctx := NewEvaluationContext().WithVariables(map[string]any{
				"a": s1,
				"b": s2,
			})

			result, err := Evaluate(`${a} == ${b}`, ctx)
			if err != nil {
				return false
			}

			expected := s1 == s2
			return result == expected
		},
		gen.AlphaString(),
		gen.AlphaString(),
	))

	// Property: String inequality comparison is correct
	properties.Property("string inequality comparison is correct", prop.ForAll(
		func(s1, s2 string) bool {
			ctx := NewEvaluationContext().WithVariables(map[string]any{
				"a": s1,
				"b": s2,
			})

			result, err := Evaluate(`${a} != ${b}`, ctx)
			if err != nil {
				return false
			}

			expected := s1 != s2
			return result == expected
		},
		gen.AlphaString(),
		gen.AlphaString(),
	))

	properties.TestingRun(t)
}

// TestExpressionIdempotency tests that evaluating the same expression twice gives the same result.
func TestExpressionIdempotency(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	properties.Property("expression evaluation is idempotent", prop.ForAll(
		func(a, b int, op string) bool {
			expr := fmt.Sprintf("${a} %s ${b}", op)
			ctx := NewEvaluationContext().WithVariables(map[string]any{
				"a": a,
				"b": b,
			})

			result1, err1 := Evaluate(expr, ctx)
			result2, err2 := Evaluate(expr, ctx)

			if err1 != nil || err2 != nil {
				return err1 != nil && err2 != nil
			}

			return result1 == result2
		},
		gen.Int(),
		gen.Int(),
		gen.OneConstOf("==", "!=", "<", ">", "<=", ">="),
	))

	properties.TestingRun(t)
}

// TestDeMorganLaws tests De Morgan's laws.
func TestDeMorganLaws(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	// Property: NOT (A AND B) == (NOT A) OR (NOT B)
	properties.Property("De Morgan's law for AND", prop.ForAll(
		func(a, b bool) bool {
			ctx := NewEvaluationContext().WithVariables(map[string]any{
				"a": a,
				"b": b,
			})

			// NOT (A AND B)
			result1, err1 := Evaluate("NOT (${a} AND ${b})", ctx)
			if err1 != nil {
				return false
			}

			// (NOT A) OR (NOT B)
			expected := (!a) || (!b)
			return result1 == expected
		},
		gen.Bool(),
		gen.Bool(),
	))

	// Property: NOT (A OR B) == (NOT A) AND (NOT B)
	properties.Property("De Morgan's law for OR", prop.ForAll(
		func(a, b bool) bool {
			ctx := NewEvaluationContext().WithVariables(map[string]any{
				"a": a,
				"b": b,
			})

			// NOT (A OR B)
			result1, err1 := Evaluate("NOT (${a} OR ${b})", ctx)
			if err1 != nil {
				return false
			}

			// (NOT A) AND (NOT B)
			expected := (!a) && (!b)
			return result1 == expected
		},
		gen.Bool(),
		gen.Bool(),
	))

	properties.TestingRun(t)
}

// Helper functions

// computeIntComparison computes the expected result of an integer comparison.
func computeIntComparison(left, right int, op string) bool {
	switch op {
	case "==":
		return left == right
	case "!=":
		return left != right
	case "<":
		return left < right
	case ">":
		return left > right
	case "<=":
		return left <= right
	case ">=":
		return left >= right
	default:
		return false
	}
}

// computeFloatComparison computes the expected result of a float comparison.
func computeFloatComparison(left, right float64, op string) bool {
	switch op {
	case "==":
		return left == right
	case "!=":
		return left != right
	case "<":
		return left < right
	case ">":
		return left > right
	case "<=":
		return left <= right
	case ">=":
		return left >= right
	default:
		return false
	}
}

// BenchmarkExpressionEvaluation benchmarks expression evaluation.
func BenchmarkExpressionEvaluation(b *testing.B) {
	ctx := NewEvaluationContext().WithVariables(map[string]any{
		"a": 10,
		"b": 20,
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Evaluate("${a} < ${b} AND ${a} > 0", ctx)
	}
}

// TestExpressionEvaluationSpecificCases tests specific edge cases.
func TestExpressionEvaluationSpecificCases(t *testing.T) {
	testCases := []struct {
		name     string
		expr     string
		vars     map[string]any
		expected bool
	}{
		{
			name:     "zero comparison",
			expr:     "${a} == 0",
			vars:     map[string]any{"a": 0},
			expected: true,
		},
		{
			name:     "negative comparison",
			expr:     "${a} < 0",
			vars:     map[string]any{"a": -1},
			expected: true,
		},
		{
			name:     "large numbers",
			expr:     "${a} > ${b}",
			vars:     map[string]any{"a": 1000000, "b": 999999},
			expected: true,
		},
		{
			name:     "boolean true",
			expr:     "${flag}",
			vars:     map[string]any{"flag": true},
			expected: true,
		},
		{
			name:     "boolean false",
			expr:     "${flag}",
			vars:     map[string]any{"flag": false},
			expected: false,
		},
		{
			name:     "double negation",
			expr:     "NOT NOT ${a}",
			vars:     map[string]any{"a": true},
			expected: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := NewEvaluationContext().WithVariables(tc.vars)
			result, err := Evaluate(tc.expr, ctx)
			assert.NoError(t, err)
			assert.Equal(t, tc.expected, result)
		})
	}
}
