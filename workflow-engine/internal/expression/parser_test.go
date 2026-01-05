package expression

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParser_SimpleLiterals(t *testing.T) {
	tests := []struct {
		input    string
		expected any
	}{
		{input: "123", expected: int64(123)},
		{input: "-456", expected: int64(-456)},
		{input: "3.14", expected: 3.14},
		{input: `"hello"`, expected: "hello"},
		{input: "true", expected: true},
		{input: "false", expected: false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			ast, err := ParseExpression(tt.input)
			require.NoError(t, err)
			require.NotNil(t, ast)

			lit, ok := ast.Root.(*LiteralNode)
			require.True(t, ok, "expected LiteralNode")
			assert.Equal(t, tt.expected, lit.Value)
		})
	}
}

func TestParser_VariableReferences(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{input: "${a}", expected: "a"},
		{input: "${login.status}", expected: "login.status"},
		{input: "${env:API_URL}", expected: "env:API_URL"},
		{input: "foo", expected: "foo"}, // bare identifier
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			ast, err := ParseExpression(tt.input)
			require.NoError(t, err)
			require.NotNil(t, ast)

			varNode, ok := ast.Root.(*VariableNode)
			require.True(t, ok, "expected VariableNode")
			assert.Equal(t, tt.expected, varNode.Name)
		})
	}
}

func TestParser_ComparisonExpressions(t *testing.T) {
	tests := []struct {
		input string
		left  any
		op    string
		right any
	}{
		{input: "${a} == 100", left: "a", op: "==", right: int64(100)},
		{input: "${b} != 200", left: "b", op: "!=", right: int64(200)},
		{input: "${c} < 50", left: "c", op: "<", right: int64(50)},
		{input: "${d} > 10", left: "d", op: ">", right: int64(10)},
		{input: "${e} <= 100", left: "e", op: "<=", right: int64(100)},
		{input: "${f} >= 0", left: "f", op: ">=", right: int64(0)},
		{input: `${name} == "test"`, left: "name", op: "==", right: "test"},
		{input: "${flag} == true", left: "flag", op: "==", right: true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			ast, err := ParseExpression(tt.input)
			require.NoError(t, err)
			require.NotNil(t, ast)

			comp, ok := ast.Root.(*ComparisonNode)
			require.True(t, ok, "expected ComparisonNode")
			assert.Equal(t, tt.op, comp.Operator)

			leftVar, ok := comp.Left.(*VariableNode)
			require.True(t, ok, "expected left to be VariableNode")
			assert.Equal(t, tt.left, leftVar.Name)

			rightLit, ok := comp.Right.(*LiteralNode)
			require.True(t, ok, "expected right to be LiteralNode")
			assert.Equal(t, tt.right, rightLit.Value)
		})
	}
}

func TestParser_LogicalExpressions(t *testing.T) {
	t.Run("AND expression", func(t *testing.T) {
		ast, err := ParseExpression("${a} == 1 AND ${b} == 2")
		require.NoError(t, err)
		require.NotNil(t, ast)

		logical, ok := ast.Root.(*LogicalNode)
		require.True(t, ok, "expected LogicalNode")
		assert.Equal(t, "AND", logical.Operator)

		_, ok = logical.Left.(*ComparisonNode)
		require.True(t, ok, "expected left to be ComparisonNode")

		_, ok = logical.Right.(*ComparisonNode)
		require.True(t, ok, "expected right to be ComparisonNode")
	})

	t.Run("OR expression", func(t *testing.T) {
		ast, err := ParseExpression("${a} == 1 OR ${b} == 2")
		require.NoError(t, err)
		require.NotNil(t, ast)

		logical, ok := ast.Root.(*LogicalNode)
		require.True(t, ok, "expected LogicalNode")
		assert.Equal(t, "OR", logical.Operator)
	})

	t.Run("NOT expression", func(t *testing.T) {
		ast, err := ParseExpression("NOT ${flag}")
		require.NoError(t, err)
		require.NotNil(t, ast)

		notNode, ok := ast.Root.(*NotNode)
		require.True(t, ok, "expected NotNode")

		_, ok = notNode.Operand.(*VariableNode)
		require.True(t, ok, "expected operand to be VariableNode")
	})

	t.Run("NOT with comparison", func(t *testing.T) {
		ast, err := ParseExpression("NOT ${a} == 1")
		require.NoError(t, err)
		require.NotNil(t, ast)

		notNode, ok := ast.Root.(*NotNode)
		require.True(t, ok, "expected NotNode")

		_, ok = notNode.Operand.(*ComparisonNode)
		require.True(t, ok, "expected operand to be ComparisonNode")
	})
}

func TestParser_Precedence(t *testing.T) {
	t.Run("AND has higher precedence than OR", func(t *testing.T) {
		// a OR b AND c should be parsed as a OR (b AND c)
		ast, err := ParseExpression("${a} == 1 OR ${b} == 2 AND ${c} == 3")
		require.NoError(t, err)
		require.NotNil(t, ast)

		or, ok := ast.Root.(*LogicalNode)
		require.True(t, ok, "expected LogicalNode")
		assert.Equal(t, "OR", or.Operator)

		// Right side should be AND
		and, ok := or.Right.(*LogicalNode)
		require.True(t, ok, "expected right to be LogicalNode")
		assert.Equal(t, "AND", and.Operator)
	})

	t.Run("NOT has highest precedence", func(t *testing.T) {
		// NOT a AND b should be parsed as (NOT a) AND b
		ast, err := ParseExpression("NOT ${a} AND ${b}")
		require.NoError(t, err)
		require.NotNil(t, ast)

		and, ok := ast.Root.(*LogicalNode)
		require.True(t, ok, "expected LogicalNode")
		assert.Equal(t, "AND", and.Operator)

		// Left side should be NOT
		_, ok = and.Left.(*NotNode)
		require.True(t, ok, "expected left to be NotNode")
	})
}

func TestParser_Parentheses(t *testing.T) {
	t.Run("parentheses override precedence", func(t *testing.T) {
		// (a OR b) AND c
		ast, err := ParseExpression("(${a} == 1 OR ${b} == 2) AND ${c} == 3")
		require.NoError(t, err)
		require.NotNil(t, ast)

		and, ok := ast.Root.(*LogicalNode)
		require.True(t, ok, "expected LogicalNode")
		assert.Equal(t, "AND", and.Operator)

		// Left side should be OR (due to parentheses)
		or, ok := and.Left.(*LogicalNode)
		require.True(t, ok, "expected left to be LogicalNode")
		assert.Equal(t, "OR", or.Operator)
	})

	t.Run("nested parentheses", func(t *testing.T) {
		ast, err := ParseExpression("((${a} == 1))")
		require.NoError(t, err)
		require.NotNil(t, ast)

		_, ok := ast.Root.(*ComparisonNode)
		require.True(t, ok, "expected ComparisonNode")
	})
}

func TestParser_ComplexExpressions(t *testing.T) {
	tests := []string{
		"${login.status} == 200 AND ${response.body.success} == true",
		"${a} > 10 AND ${b} < 20 OR ${c} == 0",
		"NOT ${error} AND ${status} == 200",
		"(${a} == 1 OR ${b} == 2) AND (${c} == 3 OR ${d} == 4)",
		"NOT (${a} == 1 AND ${b} == 2)",
	}

	for _, input := range tests {
		t.Run(input, func(t *testing.T) {
			ast, err := ParseExpression(input)
			require.NoError(t, err, "failed to parse: %s", input)
			require.NotNil(t, ast)
		})
	}
}

func TestParser_Errors(t *testing.T) {
	tests := []struct {
		input string
		desc  string
	}{
		{input: "", desc: "empty expression"},
		{input: "==", desc: "operator without operands"},
		{input: "${a} ==", desc: "missing right operand"},
		{input: "(${a} == 1", desc: "unclosed parenthesis"},
		{input: "${a} == 1 AND", desc: "trailing AND"},
		{input: "${a} == 1 OR", desc: "trailing OR"},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			_, err := ParseExpression(tt.input)
			require.Error(t, err, "expected error for: %s", tt.input)
		})
	}
}

func TestNodeType_String(t *testing.T) {
	tests := []struct {
		nodeType NodeType
		expected string
	}{
		{NodeTypeLiteral, "Literal"},
		{NodeTypeVariable, "Variable"},
		{NodeTypeComparison, "Comparison"},
		{NodeTypeLogical, "Logical"},
		{NodeTypeNot, "Not"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.nodeType.String())
		})
	}
}
