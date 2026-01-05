package assertion

import (
	"context"
	"fmt"
	"reflect"

	"github.com/grafana/k6/workflow-engine/internal/keyword"
)

// IsNull creates an is_null assertion keyword.
func IsNull() keyword.Keyword {
	return &isNullKeyword{
		BaseKeyword: keyword.NewBaseKeyword("is_null", keyword.CategoryAssertion, "Asserts that actual is null/nil"),
	}
}

type isNullKeyword struct {
	keyword.BaseKeyword
}

func (k *isNullKeyword) Execute(ctx context.Context, execCtx *keyword.ExecutionContext, params map[string]any) (*keyword.Result, error) {
	actual, exists := params["actual"]
	message := keyword.OptionalParam(params, "message", "")

	isNull := !exists || actual == nil || isNilValue(actual)

	if isNull {
		return keyword.NewSuccessResult("assertion passed", nil), nil
	}

	failMsg := fmt.Sprintf("assertion 'is_null' failed: %v is not null", actual)
	if message != "" {
		failMsg = fmt.Sprintf("%s - %s", message, failMsg)
	}
	return keyword.NewFailureResult(failMsg, nil), nil
}

func (k *isNullKeyword) Validate(params map[string]any) error {
	// actual can be missing (which means null)
	return nil
}

// IsNotNull creates an is_not_null assertion keyword.
func IsNotNull() keyword.Keyword {
	return &isNotNullKeyword{
		BaseKeyword: keyword.NewBaseKeyword("is_not_null", keyword.CategoryAssertion, "Asserts that actual is not null/nil"),
	}
}

type isNotNullKeyword struct {
	keyword.BaseKeyword
}

func (k *isNotNullKeyword) Execute(ctx context.Context, execCtx *keyword.ExecutionContext, params map[string]any) (*keyword.Result, error) {
	actual, exists := params["actual"]
	message := keyword.OptionalParam(params, "message", "")

	isNull := !exists || actual == nil || isNilValue(actual)

	if !isNull {
		return keyword.NewSuccessResult("assertion passed", nil), nil
	}

	failMsg := "assertion 'is_not_null' failed: value is null"
	if message != "" {
		failMsg = fmt.Sprintf("%s - %s", message, failMsg)
	}
	return keyword.NewFailureResult(failMsg, nil), nil
}

func (k *isNotNullKeyword) Validate(params map[string]any) error {
	if _, ok := params["actual"]; !ok {
		return fmt.Errorf("'actual' parameter is required")
	}
	return nil
}

// IsType creates an is_type assertion keyword.
func IsType() keyword.Keyword {
	return &isTypeKeyword{
		BaseKeyword: keyword.NewBaseKeyword("is_type", keyword.CategoryAssertion, "Asserts that actual is of expected type"),
	}
}

type isTypeKeyword struct {
	keyword.BaseKeyword
}

func (k *isTypeKeyword) Execute(ctx context.Context, execCtx *keyword.ExecutionContext, params map[string]any) (*keyword.Result, error) {
	actual, ok := params["actual"]
	if !ok {
		return nil, fmt.Errorf("required parameter 'actual' is missing")
	}

	expectedType, ok := params["expected"].(string)
	if !ok {
		return nil, fmt.Errorf("'expected' must be a string type name")
	}

	message := keyword.OptionalParam(params, "message", "")

	actualType := getTypeName(actual)
	matches := actualType == expectedType

	if matches {
		return keyword.NewSuccessResult("assertion passed", nil), nil
	}

	failMsg := fmt.Sprintf("assertion 'is_type' failed: actual type='%s', expected='%s'", actualType, expectedType)
	if message != "" {
		failMsg = fmt.Sprintf("%s - %s", message, failMsg)
	}
	return keyword.NewFailureResult(failMsg, nil), nil
}

func (k *isTypeKeyword) Validate(params map[string]any) error {
	if _, ok := params["actual"]; !ok {
		return fmt.Errorf("'actual' parameter is required")
	}
	if _, ok := params["expected"]; !ok {
		return fmt.Errorf("'expected' parameter is required")
	}
	return nil
}

// isNilValue checks if a value is nil (for pointer, interface, etc.)
func isNilValue(v any) bool {
	if v == nil {
		return true
	}
	val := reflect.ValueOf(v)
	switch val.Kind() {
	case reflect.Ptr, reflect.Interface, reflect.Slice, reflect.Map, reflect.Chan, reflect.Func:
		return val.IsNil()
	default:
		return false
	}
}

// getTypeName returns a simplified type name for the value.
func getTypeName(v any) string {
	if v == nil {
		return "null"
	}

	val := reflect.ValueOf(v)
	kind := val.Kind()

	switch kind {
	case reflect.String:
		return "string"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		return "number"
	case reflect.Bool:
		return "boolean"
	case reflect.Slice, reflect.Array:
		return "array"
	case reflect.Map:
		return "object"
	case reflect.Struct:
		return "object"
	case reflect.Ptr, reflect.Interface:
		if val.IsNil() {
			return "null"
		}
		return getTypeName(val.Elem().Interface())
	default:
		return kind.String()
	}
}

// RegisterTypeAssertions registers all type assertion keywords.
func RegisterTypeAssertions(registry *keyword.Registry) {
	registry.MustRegister(IsNull())
	registry.MustRegister(IsNotNull())
	registry.MustRegister(IsType())
}
