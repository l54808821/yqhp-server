package extractor

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"yqhp/workflow-engine/internal/keyword"
	"pgregory.net/rapid"
)

// TestProperty_ExtractorVariableStorage tests Property 3:
// For any extractor operation with a valid expression, the extracted value
// must be stored to the specified variable name and accessible via ${variable}.
//
// **Property 3: 提取器变量存储**
// **Validates: Requirements 3.2, 3.3**
func TestProperty_ExtractorVariableStorage(t *testing.T) {
	ctx := context.Background()

	t.Run("jsonpath_stores_to_variable", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			kw := JSONPath()

			// Generate random variable name
			varName := rapid.StringMatching(`[a-z][a-z0-9_]{2,10}`).Draw(t, "varName")

			// Generate random value
			value := rapid.StringMatching(`[a-zA-Z0-9]{1,20}`).Draw(t, "value")

			// Create JSON with the value
			jsonData := map[string]any{"field": value}
			jsonBytes, _ := json.Marshal(jsonData)

			execCtx := keyword.NewExecutionContext()
			execCtx.SetResponse(&keyword.ResponseData{
				Body: string(jsonBytes),
			})

			result, err := kw.Execute(ctx, execCtx, map[string]any{
				"expression": "$.field",
				"to":         varName,
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if !result.Success {
				t.Fatalf("extraction failed: %s", result.Message)
			}

			// Property: extracted value must be stored to the specified variable
			storedVal, ok := execCtx.GetVariable(varName)
			if !ok {
				t.Errorf("variable '%s' was not stored", varName)
			}
			if storedVal != value {
				t.Errorf("stored value '%v' does not match extracted value '%v'", storedVal, value)
			}
		})
	})

	t.Run("regex_stores_to_variable", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			kw := Regex()

			varName := rapid.StringMatching(`[a-z][a-z0-9_]{2,10}`).Draw(t, "varName")

			// Generate a number to extract
			number := rapid.IntRange(0, 9999).Draw(t, "number")
			body := fmt.Sprintf("prefix %d suffix", number)

			execCtx := keyword.NewExecutionContext()
			execCtx.SetResponse(&keyword.ResponseData{
				Body: body,
			})

			result, err := kw.Execute(ctx, execCtx, map[string]any{
				"expression": `\d+`,
				"to":         varName,
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if !result.Success {
				t.Fatalf("extraction failed: %s", result.Message)
			}

			// Property: extracted value must be stored to the specified variable
			storedVal, ok := execCtx.GetVariable(varName)
			if !ok {
				t.Errorf("variable '%s' was not stored", varName)
			}
			if storedVal != fmt.Sprintf("%d", number) {
				t.Errorf("stored value '%v' does not match expected '%d'", storedVal, number)
			}
		})
	})

	t.Run("header_stores_to_variable", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			kw := Header()

			varName := rapid.StringMatching(`[a-z][a-z0-9_]{2,10}`).Draw(t, "varName")
			headerValue := rapid.StringMatching(`[a-zA-Z0-9/-]{1,30}`).Draw(t, "headerValue")

			execCtx := keyword.NewExecutionContext()
			execCtx.SetResponse(&keyword.ResponseData{
				Headers: map[string]string{"X-Custom": headerValue},
			})

			result, err := kw.Execute(ctx, execCtx, map[string]any{
				"name": "X-Custom",
				"to":   varName,
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if !result.Success {
				t.Fatalf("extraction failed: %s", result.Message)
			}

			// Property: extracted value must be stored to the specified variable
			storedVal, ok := execCtx.GetVariable(varName)
			if !ok {
				t.Errorf("variable '%s' was not stored", varName)
			}
			if storedVal != headerValue {
				t.Errorf("stored value '%v' does not match expected '%v'", storedVal, headerValue)
			}
		})
	})

	t.Run("status_stores_to_variable", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			kw := Status()

			varName := rapid.StringMatching(`[a-z][a-z0-9_]{2,10}`).Draw(t, "varName")
			statusCode := rapid.IntRange(100, 599).Draw(t, "statusCode")

			execCtx := keyword.NewExecutionContext()
			execCtx.SetResponse(&keyword.ResponseData{
				Status: statusCode,
			})

			result, err := kw.Execute(ctx, execCtx, map[string]any{
				"to": varName,
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if !result.Success {
				t.Fatalf("extraction failed: %s", result.Message)
			}

			// Property: extracted value must be stored to the specified variable
			storedVal, ok := execCtx.GetVariable(varName)
			if !ok {
				t.Errorf("variable '%s' was not stored", varName)
			}
			if storedVal != statusCode {
				t.Errorf("stored value '%v' does not match expected '%v'", storedVal, statusCode)
			}
		})
	})
}

// TestProperty_ExtractorDefaultValue tests that default values are used when extraction fails.
// **Validates: Requirements 3.3**
func TestProperty_ExtractorDefaultValue(t *testing.T) {
	ctx := context.Background()

	t.Run("jsonpath_uses_default_on_no_match", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			kw := JSONPath()

			varName := rapid.StringMatching(`[a-z][a-z0-9_]{2,10}`).Draw(t, "varName")
			defaultVal := rapid.StringMatching(`[a-zA-Z0-9]{1,20}`).Draw(t, "defaultVal")

			execCtx := keyword.NewExecutionContext()
			execCtx.SetResponse(&keyword.ResponseData{
				Body: `{"other": "value"}`,
			})

			result, err := kw.Execute(ctx, execCtx, map[string]any{
				"expression": "$.nonexistent",
				"to":         varName,
				"default":    defaultVal,
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Property: when extraction fails, default value must be used
			if !result.Success {
				t.Errorf("expected success with default value")
			}

			storedVal, ok := execCtx.GetVariable(varName)
			if !ok {
				t.Errorf("variable '%s' was not stored", varName)
			}
			if storedVal != defaultVal {
				t.Errorf("stored value '%v' does not match default '%v'", storedVal, defaultVal)
			}
		})
	})

	t.Run("regex_uses_default_on_no_match", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			kw := Regex()

			varName := rapid.StringMatching(`[a-z][a-z0-9_]{2,10}`).Draw(t, "varName")
			defaultVal := rapid.StringMatching(`[a-zA-Z0-9]{1,20}`).Draw(t, "defaultVal")

			execCtx := keyword.NewExecutionContext()
			execCtx.SetResponse(&keyword.ResponseData{
				Body: "no numbers here",
			})

			result, err := kw.Execute(ctx, execCtx, map[string]any{
				"expression": `\d+`,
				"to":         varName,
				"default":    defaultVal,
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Property: when extraction fails, default value must be used
			if !result.Success {
				t.Errorf("expected success with default value")
			}

			storedVal, ok := execCtx.GetVariable(varName)
			if !ok {
				t.Errorf("variable '%s' was not stored", varName)
			}
			if storedVal != defaultVal {
				t.Errorf("stored value '%v' does not match default '%v'", storedVal, defaultVal)
			}
		})
	})
}

// TestProperty_ExtractorVariableAccessibility tests that extracted variables
// can be accessed in subsequent operations.
// **Validates: Requirements 3.2**
func TestProperty_ExtractorVariableAccessibility(t *testing.T) {
	ctx := context.Background()

	rapid.Check(t, func(t *rapid.T) {
		kw := JSONPath()

		varName := rapid.StringMatching(`[a-z][a-z0-9_]{2,10}`).Draw(t, "varName")
		value := rapid.StringMatching(`[a-zA-Z0-9]{1,20}`).Draw(t, "value")

		jsonData := map[string]any{"field": value}
		jsonBytes, _ := json.Marshal(jsonData)

		execCtx := keyword.NewExecutionContext()
		execCtx.SetResponse(&keyword.ResponseData{
			Body: string(jsonBytes),
		})

		// First extraction
		_, err := kw.Execute(ctx, execCtx, map[string]any{
			"expression": "$.field",
			"to":         varName,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Property: variable should be accessible via GetVariable
		storedVal, ok := execCtx.GetVariable(varName)
		if !ok {
			t.Errorf("variable '%s' not accessible", varName)
		}
		if storedVal != value {
			t.Errorf("accessible value '%v' does not match '%v'", storedVal, value)
		}

		// Property: variable should be in GetVariables map
		allVars := execCtx.GetVariables()
		if v, exists := allVars[varName]; !exists || v != value {
			t.Errorf("variable '%s' not in GetVariables or has wrong value", varName)
		}
	})
}
