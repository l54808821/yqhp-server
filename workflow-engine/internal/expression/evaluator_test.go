package expression

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEvaluator_SimpleLiterals(t *testing.T) {
	evaluator := NewEvaluator()
	ctx := NewEvaluationContext()

	tests := []struct {
		expr     string
		expected bool
	}{
		{expr: "true", expected: true},
		{expr: "false", expected: false},
		{expr: "TRUE", expected: true},
		{expr: "FALSE", expected: false},
	}

	for _, tt := range tests {
		t.Run(tt.expr, func(t *testing.T) {
			result, err := evaluator.EvaluateString(tt.expr, ctx)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestEvaluator_Variables(t *testing.T) {
	evaluator := NewEvaluator()
	ctx := NewEvaluationContext().WithVariables(map[string]any{
		"flag":   true,
		"count":  10,
		"name":   "test",
		"active": false,
	})

	tests := []struct {
		expr     string
		expected bool
	}{
		{expr: "${flag}", expected: true},
		{expr: "${active}", expected: false},
	}

	for _, tt := range tests {
		t.Run(tt.expr, func(t *testing.T) {
			result, err := evaluator.EvaluateString(tt.expr, ctx)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestEvaluator_ComparisonOperators(t *testing.T) {
	evaluator := NewEvaluator()
	ctx := NewEvaluationContext().WithVariables(map[string]any{
		"a": 10,
		"b": 20,
		"c": 10,
		"s": "hello",
	})

	tests := []struct {
		expr     string
		expected bool
	}{
		// Equality
		{expr: "${a} == 10", expected: true},
		{expr: "${a} == 20", expected: false},
		{expr: "${a} == ${c}", expected: true},
		{expr: "${a} != ${b}", expected: true},
		{expr: "${a} != ${c}", expected: false},

		// Less than
		{expr: "${a} < ${b}", expected: true},
		{expr: "${b} < ${a}", expected: false},
		{expr: "${a} < ${c}", expected: false},

		// Greater than
		{expr: "${b} > ${a}", expected: true},
		{expr: "${a} > ${b}", expected: false},
		{expr: "${a} > ${c}", expected: false},

		// Less than or equal
		{expr: "${a} <= ${b}", expected: true},
		{expr: "${a} <= ${c}", expected: true},
		{expr: "${b} <= ${a}", expected: false},

		// Greater than or equal
		{expr: "${b} >= ${a}", expected: true},
		{expr: "${a} >= ${c}", expected: true},
		{expr: "${a} >= ${b}", expected: false},

		// String comparison
		{expr: `${s} == "hello"`, expected: true},
		{expr: `${s} != "world"`, expected: true},
	}

	for _, tt := range tests {
		t.Run(tt.expr, func(t *testing.T) {
			result, err := evaluator.EvaluateString(tt.expr, ctx)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestEvaluator_LogicalOperators(t *testing.T) {
	evaluator := NewEvaluator()
	ctx := NewEvaluationContext().WithVariables(map[string]any{
		"a": true,
		"b": false,
		"x": 10,
		"y": 20,
	})

	tests := []struct {
		expr     string
		expected bool
	}{
		// AND
		{expr: "${a} AND ${a}", expected: true},
		{expr: "${a} AND ${b}", expected: false},
		{expr: "${b} AND ${a}", expected: false},
		{expr: "${b} AND ${b}", expected: false},

		// OR
		{expr: "${a} OR ${a}", expected: true},
		{expr: "${a} OR ${b}", expected: true},
		{expr: "${b} OR ${a}", expected: true},
		{expr: "${b} OR ${b}", expected: false},

		// NOT
		{expr: "NOT ${a}", expected: false},
		{expr: "NOT ${b}", expected: true},

		// Combined
		{expr: "${x} == 10 AND ${y} == 20", expected: true},
		{expr: "${x} == 10 AND ${y} == 10", expected: false},
		{expr: "${x} == 10 OR ${y} == 10", expected: true},
		{expr: "${x} == 5 OR ${y} == 10", expected: false},
		{expr: "NOT ${x} == 5", expected: true},
		{expr: "NOT ${x} == 10", expected: false},
	}

	for _, tt := range tests {
		t.Run(tt.expr, func(t *testing.T) {
			result, err := evaluator.EvaluateString(tt.expr, ctx)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestEvaluator_PathVariables(t *testing.T) {
	evaluator := NewEvaluator()
	ctx := NewEvaluationContext().WithResults(map[string]any{
		"login": map[string]any{
			"status": 200,
			"body": map[string]any{
				"success": true,
				"token":   "abc123",
			},
		},
		"response": map[string]any{
			"code": 404,
		},
	})

	tests := []struct {
		expr     string
		expected bool
	}{
		{expr: "${login.status} == 200", expected: true},
		{expr: "${login.status} != 200", expected: false},
		{expr: "${login.body.success} == true", expected: true},
		{expr: "${response.code} == 404", expected: true},
		{expr: "${login.status} == 200 AND ${login.body.success} == true", expected: true},
	}

	for _, tt := range tests {
		t.Run(tt.expr, func(t *testing.T) {
			result, err := evaluator.EvaluateString(tt.expr, ctx)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestEvaluator_ComplexExpressions(t *testing.T) {
	evaluator := NewEvaluator()
	ctx := NewEvaluationContext().
		WithVariables(map[string]any{
			"env": "production",
		}).
		WithResults(map[string]any{
			"login": map[string]any{
				"status": 200,
				"body": map[string]any{
					"success": true,
				},
			},
		})

	tests := []struct {
		expr     string
		expected bool
	}{
		{
			expr:     "${login.status} == 200 AND ${login.body.success} == true",
			expected: true,
		},
		{
			expr:     "(${login.status} == 200 OR ${login.status} == 201) AND ${login.body.success} == true",
			expected: true,
		},
		{
			expr:     "NOT (${login.status} == 404)",
			expected: true,
		},
		{
			expr:     `${env} == "production" AND ${login.status} == 200`,
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.expr, func(t *testing.T) {
			result, err := evaluator.EvaluateString(tt.expr, ctx)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestEvaluator_ShortCircuit(t *testing.T) {
	evaluator := NewEvaluator()
	ctx := NewEvaluationContext().WithVariables(map[string]any{
		"a": true,
		"b": false,
	})

	// AND short-circuit: if left is false, right is not evaluated
	// This should not error even though "nonexistent" doesn't exist
	// because the left side is false
	result, err := evaluator.EvaluateString("${b} AND ${nonexistent}", ctx)
	require.NoError(t, err)
	assert.False(t, result)

	// OR short-circuit: if left is true, right is not evaluated
	result, err = evaluator.EvaluateString("${a} OR ${nonexistent}", ctx)
	require.NoError(t, err)
	assert.True(t, result)
}

func TestEvaluator_TypeConversion(t *testing.T) {
	evaluator := NewEvaluator()

	tests := []struct {
		name     string
		vars     map[string]any
		expr     string
		expected bool
	}{
		{
			name:     "int to bool (non-zero)",
			vars:     map[string]any{"x": 1},
			expr:     "${x}",
			expected: true,
		},
		{
			name:     "int to bool (zero)",
			vars:     map[string]any{"x": 0},
			expr:     "${x}",
			expected: false,
		},
		{
			name:     "string true",
			vars:     map[string]any{"x": "true"},
			expr:     "${x}",
			expected: true,
		},
		{
			name:     "string false",
			vars:     map[string]any{"x": "false"},
			expr:     "${x}",
			expected: false,
		},
		{
			name:     "float comparison",
			vars:     map[string]any{"x": 3.14},
			expr:     "${x} > 3",
			expected: true,
		},
		{
			name:     "string number comparison",
			vars:     map[string]any{"x": "100"},
			expr:     "${x} > 50",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := NewEvaluationContext().WithVariables(tt.vars)
			result, err := evaluator.EvaluateString(tt.expr, ctx)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestEvaluator_Errors(t *testing.T) {
	evaluator := NewEvaluator()
	ctx := NewEvaluationContext()

	tests := []struct {
		name string
		expr string
	}{
		{name: "undefined variable", expr: "${undefined}"},
		{name: "undefined path", expr: "${a.b.c}"},
		{name: "invalid expression", expr: "== 10"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := evaluator.EvaluateString(tt.expr, ctx)
			require.Error(t, err)
		})
	}
}

func TestEvaluator_NilContext(t *testing.T) {
	evaluator := NewEvaluator()

	// Should handle nil context gracefully
	_, err := evaluator.EvaluateString("${x}", nil)
	require.Error(t, err)
}

func TestEvaluator_NilAST(t *testing.T) {
	evaluator := NewEvaluator()
	ctx := NewEvaluationContext()

	_, err := evaluator.Evaluate(nil, ctx)
	require.Error(t, err)

	_, err = evaluator.Evaluate(&ExpressionAST{Root: nil}, ctx)
	require.Error(t, err)
}

func TestEvaluate_ConvenienceFunction(t *testing.T) {
	ctx := NewEvaluationContext().WithVariables(map[string]any{
		"x": 10,
	})

	result, err := Evaluate("${x} == 10", ctx)
	require.NoError(t, err)
	assert.True(t, result)
}

func TestEvaluationContext_Methods(t *testing.T) {
	ctx := NewEvaluationContext()

	// Test Set
	ctx.Set("foo", "bar")
	assert.Equal(t, "bar", ctx.Variables["foo"])

	// Test SetResult
	ctx.SetResult("step1", map[string]any{"status": 200})
	assert.Equal(t, map[string]any{"status": 200}, ctx.Results["step1"])
}
