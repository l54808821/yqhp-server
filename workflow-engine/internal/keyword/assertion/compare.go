// Package assertion provides assertion keywords for workflow engine v2.
package assertion

import (
	"context"
	"fmt"
	"reflect"

	"github.com/grafana/k6/workflow-engine/internal/keyword"
)

// CompareFunc is a function that compares two values.
type CompareFunc func(actual, expected any) (bool, error)

// compareKeyword is a base implementation for comparison assertions.
type compareKeyword struct {
	keyword.BaseKeyword
	compare CompareFunc
}

func newCompareKeyword(name, description string, compare CompareFunc) *compareKeyword {
	return &compareKeyword{
		BaseKeyword: keyword.NewBaseKeyword(name, keyword.CategoryAssertion, description),
		compare:     compare,
	}
}

func (k *compareKeyword) Execute(ctx context.Context, execCtx *keyword.ExecutionContext, params map[string]any) (*keyword.Result, error) {
	actual, actualExists := params["actual"]
	if !actualExists {
		return nil, fmt.Errorf("required parameter 'actual' is missing")
	}

	expected, expectedExists := params["expected"]
	if !expectedExists {
		return nil, fmt.Errorf("required parameter 'expected' is missing")
	}

	message := keyword.OptionalParam(params, "message", "")

	result, err := k.compare(actual, expected)
	if err != nil {
		return keyword.NewFailureResult(fmt.Sprintf("comparison error: %v", err), err), nil
	}

	if result {
		return keyword.NewSuccessResult("assertion passed", nil), nil
	}

	failMsg := fmt.Sprintf("assertion '%s' failed: actual=%v, expected=%v", k.Name(), actual, expected)
	if message != "" {
		failMsg = fmt.Sprintf("%s - %s", message, failMsg)
	}
	return keyword.NewFailureResult(failMsg, nil), nil
}

func (k *compareKeyword) Validate(params map[string]any) error {
	if _, ok := params["actual"]; !ok {
		return fmt.Errorf("'actual' parameter is required")
	}
	if _, ok := params["expected"]; !ok {
		return fmt.Errorf("'expected' parameter is required")
	}
	return nil
}

// toFloat64 converts a numeric value to float64 for comparison.
func toFloat64(v any) (float64, bool) {
	switch n := v.(type) {
	case int:
		return float64(n), true
	case int8:
		return float64(n), true
	case int16:
		return float64(n), true
	case int32:
		return float64(n), true
	case int64:
		return float64(n), true
	case uint:
		return float64(n), true
	case uint8:
		return float64(n), true
	case uint16:
		return float64(n), true
	case uint32:
		return float64(n), true
	case uint64:
		return float64(n), true
	case float32:
		return float64(n), true
	case float64:
		return n, true
	default:
		return 0, false
	}
}

// Equals creates an equals assertion keyword.
func Equals() keyword.Keyword {
	return newCompareKeyword("equals", "Asserts that actual equals expected", func(actual, expected any) (bool, error) {
		return reflect.DeepEqual(actual, expected), nil
	})
}

// NotEquals creates a not_equals assertion keyword.
func NotEquals() keyword.Keyword {
	return newCompareKeyword("not_equals", "Asserts that actual does not equal expected", func(actual, expected any) (bool, error) {
		return !reflect.DeepEqual(actual, expected), nil
	})
}

// GreaterThan creates a greater_than assertion keyword.
func GreaterThan() keyword.Keyword {
	return newCompareKeyword("greater_than", "Asserts that actual is greater than expected", func(actual, expected any) (bool, error) {
		a, aOk := toFloat64(actual)
		e, eOk := toFloat64(expected)
		if !aOk || !eOk {
			return false, fmt.Errorf("both values must be numeric for greater_than comparison")
		}
		return a > e, nil
	})
}

// GreaterOrEqual creates a greater_or_equal assertion keyword.
func GreaterOrEqual() keyword.Keyword {
	return newCompareKeyword("greater_or_equal", "Asserts that actual is greater than or equal to expected", func(actual, expected any) (bool, error) {
		a, aOk := toFloat64(actual)
		e, eOk := toFloat64(expected)
		if !aOk || !eOk {
			return false, fmt.Errorf("both values must be numeric for greater_or_equal comparison")
		}
		return a >= e, nil
	})
}

// LessThan creates a less_than assertion keyword.
func LessThan() keyword.Keyword {
	return newCompareKeyword("less_than", "Asserts that actual is less than expected", func(actual, expected any) (bool, error) {
		a, aOk := toFloat64(actual)
		e, eOk := toFloat64(expected)
		if !aOk || !eOk {
			return false, fmt.Errorf("both values must be numeric for less_than comparison")
		}
		return a < e, nil
	})
}

// LessOrEqual creates a less_or_equal assertion keyword.
func LessOrEqual() keyword.Keyword {
	return newCompareKeyword("less_or_equal", "Asserts that actual is less than or equal to expected", func(actual, expected any) (bool, error) {
		a, aOk := toFloat64(actual)
		e, eOk := toFloat64(expected)
		if !aOk || !eOk {
			return false, fmt.Errorf("both values must be numeric for less_or_equal comparison")
		}
		return a <= e, nil
	})
}

// RegisterCompareAssertions registers all comparison assertion keywords.
func RegisterCompareAssertions(registry *keyword.Registry) {
	registry.MustRegister(Equals())
	registry.MustRegister(NotEquals())
	registry.MustRegister(GreaterThan())
	registry.MustRegister(GreaterOrEqual())
	registry.MustRegister(LessThan())
	registry.MustRegister(LessOrEqual())
}
