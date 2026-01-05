package assertion

import (
	"context"
	"fmt"
	"reflect"

	"yqhp/workflow-engine/internal/keyword"
)

// In creates an in assertion keyword.
// Checks if actual is in the expected list.
func In() keyword.Keyword {
	return &inKeyword{
		BaseKeyword: keyword.NewBaseKeyword("in", keyword.CategoryAssertion, "Asserts that actual is in expected list"),
	}
}

type inKeyword struct {
	keyword.BaseKeyword
}

func (k *inKeyword) Execute(ctx context.Context, execCtx *keyword.ExecutionContext, params map[string]any) (*keyword.Result, error) {
	actual, ok := params["actual"]
	if !ok {
		return nil, fmt.Errorf("required parameter 'actual' is missing")
	}

	expected, ok := params["expected"]
	if !ok {
		return nil, fmt.Errorf("required parameter 'expected' is missing")
	}

	message := keyword.OptionalParam(params, "message", "")

	// expected should be a slice/array
	expectedVal := reflect.ValueOf(expected)
	if expectedVal.Kind() != reflect.Slice && expectedVal.Kind() != reflect.Array {
		return keyword.NewFailureResult("expected must be a slice or array", nil), nil
	}

	found := false
	for i := 0; i < expectedVal.Len(); i++ {
		if reflect.DeepEqual(actual, expectedVal.Index(i).Interface()) {
			found = true
			break
		}
	}

	if found {
		return keyword.NewSuccessResult("assertion passed", nil), nil
	}

	failMsg := fmt.Sprintf("assertion 'in' failed: %v not in %v", actual, expected)
	if message != "" {
		failMsg = fmt.Sprintf("%s - %s", message, failMsg)
	}
	return keyword.NewFailureResult(failMsg, nil), nil
}

func (k *inKeyword) Validate(params map[string]any) error {
	if _, ok := params["actual"]; !ok {
		return fmt.Errorf("'actual' parameter is required")
	}
	if _, ok := params["expected"]; !ok {
		return fmt.Errorf("'expected' parameter is required")
	}
	return nil
}

// NotIn creates a not_in assertion keyword.
func NotIn() keyword.Keyword {
	return &notInKeyword{
		BaseKeyword: keyword.NewBaseKeyword("not_in", keyword.CategoryAssertion, "Asserts that actual is not in expected list"),
	}
}

type notInKeyword struct {
	keyword.BaseKeyword
}

func (k *notInKeyword) Execute(ctx context.Context, execCtx *keyword.ExecutionContext, params map[string]any) (*keyword.Result, error) {
	actual, ok := params["actual"]
	if !ok {
		return nil, fmt.Errorf("required parameter 'actual' is missing")
	}

	expected, ok := params["expected"]
	if !ok {
		return nil, fmt.Errorf("required parameter 'expected' is missing")
	}

	message := keyword.OptionalParam(params, "message", "")

	expectedVal := reflect.ValueOf(expected)
	if expectedVal.Kind() != reflect.Slice && expectedVal.Kind() != reflect.Array {
		return keyword.NewFailureResult("expected must be a slice or array", nil), nil
	}

	found := false
	for i := 0; i < expectedVal.Len(); i++ {
		if reflect.DeepEqual(actual, expectedVal.Index(i).Interface()) {
			found = true
			break
		}
	}

	if !found {
		return keyword.NewSuccessResult("assertion passed", nil), nil
	}

	failMsg := fmt.Sprintf("assertion 'not_in' failed: %v found in %v", actual, expected)
	if message != "" {
		failMsg = fmt.Sprintf("%s - %s", message, failMsg)
	}
	return keyword.NewFailureResult(failMsg, nil), nil
}

func (k *notInKeyword) Validate(params map[string]any) error {
	if _, ok := params["actual"]; !ok {
		return fmt.Errorf("'actual' parameter is required")
	}
	if _, ok := params["expected"]; !ok {
		return fmt.Errorf("'expected' parameter is required")
	}
	return nil
}

// IsEmpty creates an is_empty assertion keyword.
func IsEmpty() keyword.Keyword {
	return &isEmptyKeyword{
		BaseKeyword: keyword.NewBaseKeyword("is_empty", keyword.CategoryAssertion, "Asserts that actual is empty"),
	}
}

type isEmptyKeyword struct {
	keyword.BaseKeyword
}

func (k *isEmptyKeyword) Execute(ctx context.Context, execCtx *keyword.ExecutionContext, params map[string]any) (*keyword.Result, error) {
	actual, ok := params["actual"]
	if !ok {
		return nil, fmt.Errorf("required parameter 'actual' is missing")
	}

	message := keyword.OptionalParam(params, "message", "")

	isEmpty := isValueEmpty(actual)

	if isEmpty {
		return keyword.NewSuccessResult("assertion passed", nil), nil
	}

	failMsg := fmt.Sprintf("assertion 'is_empty' failed: %v is not empty", actual)
	if message != "" {
		failMsg = fmt.Sprintf("%s - %s", message, failMsg)
	}
	return keyword.NewFailureResult(failMsg, nil), nil
}

func (k *isEmptyKeyword) Validate(params map[string]any) error {
	if _, ok := params["actual"]; !ok {
		return fmt.Errorf("'actual' parameter is required")
	}
	return nil
}

// IsNotEmpty creates an is_not_empty assertion keyword.
func IsNotEmpty() keyword.Keyword {
	return &isNotEmptyKeyword{
		BaseKeyword: keyword.NewBaseKeyword("is_not_empty", keyword.CategoryAssertion, "Asserts that actual is not empty"),
	}
}

type isNotEmptyKeyword struct {
	keyword.BaseKeyword
}

func (k *isNotEmptyKeyword) Execute(ctx context.Context, execCtx *keyword.ExecutionContext, params map[string]any) (*keyword.Result, error) {
	actual, ok := params["actual"]
	if !ok {
		return nil, fmt.Errorf("required parameter 'actual' is missing")
	}

	message := keyword.OptionalParam(params, "message", "")

	isEmpty := isValueEmpty(actual)

	if !isEmpty {
		return keyword.NewSuccessResult("assertion passed", nil), nil
	}

	failMsg := fmt.Sprintf("assertion 'is_not_empty' failed: value is empty")
	if message != "" {
		failMsg = fmt.Sprintf("%s - %s", message, failMsg)
	}
	return keyword.NewFailureResult(failMsg, nil), nil
}

func (k *isNotEmptyKeyword) Validate(params map[string]any) error {
	if _, ok := params["actual"]; !ok {
		return fmt.Errorf("'actual' parameter is required")
	}
	return nil
}

// LengthEquals creates a length_equals assertion keyword.
func LengthEquals() keyword.Keyword {
	return &lengthEqualsKeyword{
		BaseKeyword: keyword.NewBaseKeyword("length_equals", keyword.CategoryAssertion, "Asserts that actual has expected length"),
	}
}

type lengthEqualsKeyword struct {
	keyword.BaseKeyword
}

func (k *lengthEqualsKeyword) Execute(ctx context.Context, execCtx *keyword.ExecutionContext, params map[string]any) (*keyword.Result, error) {
	actual, ok := params["actual"]
	if !ok {
		return nil, fmt.Errorf("required parameter 'actual' is missing")
	}

	expectedRaw, ok := params["expected"]
	if !ok {
		return nil, fmt.Errorf("required parameter 'expected' is missing")
	}

	expected, ok := toInt(expectedRaw)
	if !ok {
		return nil, fmt.Errorf("expected must be an integer")
	}

	message := keyword.OptionalParam(params, "message", "")

	actualLen, err := getLength(actual)
	if err != nil {
		return keyword.NewFailureResult(fmt.Sprintf("cannot get length: %v", err), err), nil
	}

	if actualLen == expected {
		return keyword.NewSuccessResult("assertion passed", nil), nil
	}

	failMsg := fmt.Sprintf("assertion 'length_equals' failed: actual length=%d, expected=%d", actualLen, expected)
	if message != "" {
		failMsg = fmt.Sprintf("%s - %s", message, failMsg)
	}
	return keyword.NewFailureResult(failMsg, nil), nil
}

func (k *lengthEqualsKeyword) Validate(params map[string]any) error {
	if _, ok := params["actual"]; !ok {
		return fmt.Errorf("'actual' parameter is required")
	}
	if _, ok := params["expected"]; !ok {
		return fmt.Errorf("'expected' parameter is required")
	}
	return nil
}

// isValueEmpty checks if a value is empty.
func isValueEmpty(v any) bool {
	if v == nil {
		return true
	}

	val := reflect.ValueOf(v)
	switch val.Kind() {
	case reflect.String:
		return val.Len() == 0
	case reflect.Slice, reflect.Array, reflect.Map:
		return val.Len() == 0
	case reflect.Ptr, reflect.Interface:
		return val.IsNil()
	default:
		return false
	}
}

// getLength returns the length of a value.
func getLength(v any) (int, error) {
	if v == nil {
		return 0, nil
	}

	val := reflect.ValueOf(v)
	switch val.Kind() {
	case reflect.String, reflect.Slice, reflect.Array, reflect.Map:
		return val.Len(), nil
	default:
		return 0, fmt.Errorf("cannot get length of %T", v)
	}
}

// toInt converts a value to int.
func toInt(v any) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case int8:
		return int(n), true
	case int16:
		return int(n), true
	case int32:
		return int(n), true
	case int64:
		return int(n), true
	case uint:
		return int(n), true
	case uint8:
		return int(n), true
	case uint16:
		return int(n), true
	case uint32:
		return int(n), true
	case uint64:
		return int(n), true
	case float32:
		return int(n), true
	case float64:
		return int(n), true
	default:
		return 0, false
	}
}

// RegisterCollectionAssertions registers all collection assertion keywords.
func RegisterCollectionAssertions(registry *keyword.Registry) {
	registry.MustRegister(In())
	registry.MustRegister(NotIn())
	registry.MustRegister(IsEmpty())
	registry.MustRegister(IsNotEmpty())
	registry.MustRegister(LengthEquals())
}
