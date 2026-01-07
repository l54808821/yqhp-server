package expression

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

// EvaluationContext holds the context for expression evaluation.
type EvaluationContext struct {
	// Variables holds variable values accessible in expressions.
	Variables map[string]any
	// Results holds step execution results (e.g., login.status).
	Results map[string]any
}

// NewEvaluationContext creates a new EvaluationContext.
func NewEvaluationContext() *EvaluationContext {
	return &EvaluationContext{
		Variables: make(map[string]any),
		Results:   make(map[string]any),
	}
}

// WithVariables sets the variables map.
func (c *EvaluationContext) WithVariables(vars map[string]any) *EvaluationContext {
	c.Variables = vars
	return c
}

// WithResults sets the results map.
func (c *EvaluationContext) WithResults(results map[string]any) *EvaluationContext {
	c.Results = results
	return c
}

// Set sets a variable value.
func (c *EvaluationContext) Set(name string, value any) {
	c.Variables[name] = value
}

// SetResult sets a step result.
func (c *EvaluationContext) SetResult(stepID string, result any) {
	c.Results[stepID] = result
}

// ExpressionEvaluator evaluates condition expressions.
type ExpressionEvaluator interface {
	// Parse parses an expression string into an AST.
	Parse(expr string) (*ExpressionAST, error)

	// Evaluate evaluates an AST with the given context.
	Evaluate(ast *ExpressionAST, ctx *EvaluationContext) (bool, error)

	// EvaluateString parses and evaluates an expression string.
	EvaluateString(expr string, ctx *EvaluationContext) (bool, error)
}

// DefaultEvaluator is the default implementation of ExpressionEvaluator.
type DefaultEvaluator struct{}

// NewEvaluator creates a new DefaultEvaluator.
func NewEvaluator() *DefaultEvaluator {
	return &DefaultEvaluator{}
}

// Parse parses an expression string into an AST.
func (e *DefaultEvaluator) Parse(expr string) (*ExpressionAST, error) {
	return ParseExpression(expr)
}

// Evaluate evaluates an AST with the given context.
func (e *DefaultEvaluator) Evaluate(ast *ExpressionAST, ctx *EvaluationContext) (bool, error) {
	if ast == nil || ast.Root == nil {
		return false, NewEvaluationError("nil AST", nil)
	}

	result, err := e.evaluateNode(ast.Root, ctx)
	if err != nil {
		return false, err
	}

	return toBool(result)
}

// EvaluateString parses and evaluates an expression string.
func (e *DefaultEvaluator) EvaluateString(expr string, ctx *EvaluationContext) (bool, error) {
	ast, err := e.Parse(expr)
	if err != nil {
		return false, err
	}
	return e.Evaluate(ast, ctx)
}

// evaluateNode evaluates a single AST node.
func (e *DefaultEvaluator) evaluateNode(node Node, ctx *EvaluationContext) (any, error) {
	switch n := node.(type) {
	case *LiteralNode:
		return n.Value, nil

	case *VariableNode:
		return e.resolveVariable(n.Name, ctx)

	case *ComparisonNode:
		return e.evaluateComparison(n, ctx)

	case *LogicalNode:
		return e.evaluateLogical(n, ctx)

	case *NotNode:
		return e.evaluateNot(n, ctx)

	default:
		return nil, NewEvaluationError(fmt.Sprintf("unknown node type: %T", node), nil)
	}
}

// resolveVariable resolves a variable reference.
func (e *DefaultEvaluator) resolveVariable(name string, ctx *EvaluationContext) (any, error) {
	if ctx == nil {
		return nil, NewVariableNotFoundError(name)
	}

	// Check for path notation (e.g., "login.status", "response.body.success")
	if strings.Contains(name, ".") {
		return e.resolvePathVariable(name, ctx)
	}

	// Check variables first
	if val, ok := ctx.Variables[name]; ok {
		return val, nil
	}

	// Check results
	if val, ok := ctx.Results[name]; ok {
		return val, nil
	}

	return nil, NewVariableNotFoundError(name)
}

// resolvePathVariable resolves a dotted path variable (e.g., "login.status").
func (e *DefaultEvaluator) resolvePathVariable(path string, ctx *EvaluationContext) (any, error) {
	parts := strings.Split(path, ".")
	if len(parts) < 2 {
		return nil, NewVariableNotFoundError(path)
	}

	// First part is the root variable/result name
	root := parts[0]
	var current any
	var found bool

	// Check results first (for step results like login.status)
	if current, found = ctx.Results[root]; !found {
		// Check variables
		if current, found = ctx.Variables[root]; !found {
			return nil, NewVariableNotFoundError(path)
		}
	}

	// Navigate the path
	for i := 1; i < len(parts); i++ {
		part := parts[i]
		var err error
		current, err = getField(current, part)
		if err != nil {
			return nil, NewEvaluationError(fmt.Sprintf("cannot resolve path '%s': %v", path, err), err)
		}
	}

	return current, nil
}

// getField gets a field from a value (map or struct).
func getField(v any, field string) (any, error) {
	if v == nil {
		return nil, fmt.Errorf("无法从 nil 获取字段 '%s'", field)
	}

	// Handle map
	if m, ok := v.(map[string]any); ok {
		if val, exists := m[field]; exists {
			return val, nil
		}
		return nil, fmt.Errorf("在 map 中未找到字段 '%s'", field)
	}

	// Handle struct via reflection
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Ptr {
		rv = rv.Elem()
	}

	if rv.Kind() == reflect.Struct {
		fv := rv.FieldByName(field)
		if fv.IsValid() {
			return fv.Interface(), nil
		}
		// Try case-insensitive match
		for i := 0; i < rv.NumField(); i++ {
			if strings.EqualFold(rv.Type().Field(i).Name, field) {
				return rv.Field(i).Interface(), nil
			}
		}
		return nil, fmt.Errorf("在结构体中未找到字段 '%s'", field)
	}

	return nil, fmt.Errorf("无法从类型 %T 获取字段 '%s'", field, v)
}

// evaluateComparison evaluates a comparison expression.
func (e *DefaultEvaluator) evaluateComparison(node *ComparisonNode, ctx *EvaluationContext) (bool, error) {
	left, err := e.evaluateNode(node.Left, ctx)
	if err != nil {
		return false, err
	}

	right, err := e.evaluateNode(node.Right, ctx)
	if err != nil {
		return false, err
	}

	return compare(left, right, node.Operator)
}

// evaluateLogical evaluates a logical expression (AND, OR).
func (e *DefaultEvaluator) evaluateLogical(node *LogicalNode, ctx *EvaluationContext) (bool, error) {
	leftVal, err := e.evaluateNode(node.Left, ctx)
	if err != nil {
		return false, err
	}

	leftBool, err := toBool(leftVal)
	if err != nil {
		return false, err
	}

	// Short-circuit evaluation
	switch node.Operator {
	case "AND":
		if !leftBool {
			return false, nil
		}
	case "OR":
		if leftBool {
			return true, nil
		}
	}

	rightVal, err := e.evaluateNode(node.Right, ctx)
	if err != nil {
		return false, err
	}

	rightBool, err := toBool(rightVal)
	if err != nil {
		return false, err
	}

	switch node.Operator {
	case "AND":
		return leftBool && rightBool, nil
	case "OR":
		return leftBool || rightBool, nil
	default:
		return false, NewEvaluationError(fmt.Sprintf("unknown logical operator: %s", node.Operator), nil)
	}
}

// evaluateNot evaluates a NOT expression.
func (e *DefaultEvaluator) evaluateNot(node *NotNode, ctx *EvaluationContext) (bool, error) {
	val, err := e.evaluateNode(node.Operand, ctx)
	if err != nil {
		return false, err
	}

	boolVal, err := toBool(val)
	if err != nil {
		return false, err
	}

	return !boolVal, nil
}

// compare compares two values with the given operator.
func compare(left, right any, op string) (bool, error) {
	// Handle nil comparisons
	if left == nil || right == nil {
		switch op {
		case "==":
			return left == right, nil
		case "!=":
			return left != right, nil
		default:
			return false, NewEvaluationError(fmt.Sprintf("cannot compare nil with operator %s", op), nil)
		}
	}

	// Try numeric comparison first
	leftNum, leftIsNum := toFloat64(left)
	rightNum, rightIsNum := toFloat64(right)

	if leftIsNum && rightIsNum {
		return compareNumbers(leftNum, rightNum, op)
	}

	// String comparison
	leftStr := fmt.Sprintf("%v", left)
	rightStr := fmt.Sprintf("%v", right)

	switch op {
	case "==":
		return leftStr == rightStr, nil
	case "!=":
		return leftStr != rightStr, nil
	case "<":
		return leftStr < rightStr, nil
	case ">":
		return leftStr > rightStr, nil
	case "<=":
		return leftStr <= rightStr, nil
	case ">=":
		return leftStr >= rightStr, nil
	default:
		return false, NewEvaluationError(fmt.Sprintf("unknown comparison operator: %s", op), nil)
	}
}

// compareNumbers compares two numbers.
func compareNumbers(left, right float64, op string) (bool, error) {
	switch op {
	case "==":
		return left == right, nil
	case "!=":
		return left != right, nil
	case "<":
		return left < right, nil
	case ">":
		return left > right, nil
	case "<=":
		return left <= right, nil
	case ">=":
		return left >= right, nil
	default:
		return false, NewEvaluationError(fmt.Sprintf("unknown comparison operator: %s", op), nil)
	}
}

// toFloat64 converts a value to float64 if possible.
func toFloat64(v any) (float64, bool) {
	switch val := v.(type) {
	case int:
		return float64(val), true
	case int8:
		return float64(val), true
	case int16:
		return float64(val), true
	case int32:
		return float64(val), true
	case int64:
		return float64(val), true
	case uint:
		return float64(val), true
	case uint8:
		return float64(val), true
	case uint16:
		return float64(val), true
	case uint32:
		return float64(val), true
	case uint64:
		return float64(val), true
	case float32:
		return float64(val), true
	case float64:
		return val, true
	case string:
		if f, err := strconv.ParseFloat(val, 64); err == nil {
			return f, true
		}
		return 0, false
	default:
		return 0, false
	}
}

// toBool converts a value to bool.
func toBool(v any) (bool, error) {
	switch val := v.(type) {
	case bool:
		return val, nil
	case int, int8, int16, int32, int64:
		return reflect.ValueOf(val).Int() != 0, nil
	case uint, uint8, uint16, uint32, uint64:
		return reflect.ValueOf(val).Uint() != 0, nil
	case float32, float64:
		return reflect.ValueOf(val).Float() != 0, nil
	case string:
		lower := strings.ToLower(val)
		if lower == "true" || lower == "1" {
			return true, nil
		}
		if lower == "false" || lower == "0" || lower == "" {
			return false, nil
		}
		return false, NewTypeMismatchError("bool", "string", val)
	case nil:
		return false, nil
	default:
		return false, NewTypeMismatchError("bool", fmt.Sprintf("%T", v), v)
	}
}

// Evaluate is a convenience function to evaluate an expression string.
func Evaluate(expr string, ctx *EvaluationContext) (bool, error) {
	evaluator := NewEvaluator()
	return evaluator.EvaluateString(expr, ctx)
}
