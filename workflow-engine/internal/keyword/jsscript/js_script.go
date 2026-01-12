// Package jsscript provides JavaScript script execution for workflow engine v2.
// It uses the Goja JavaScript engine for executing user scripts.
package jsscript

import (
	"context"
	"fmt"

	"yqhp/workflow-engine/internal/keyword"

	"github.com/dop251/goja"
)

// JsScript creates a js_script keyword.
func JsScript() keyword.Keyword {
	return &jsScriptKeyword{
		BaseKeyword: keyword.NewBaseKeyword("js_script", keyword.CategoryAction, "Executes JavaScript code"),
	}
}

type jsScriptKeyword struct {
	keyword.BaseKeyword
}

func (k *jsScriptKeyword) Execute(ctx context.Context, execCtx *keyword.ExecutionContext, params map[string]any) (*keyword.Result, error) {
	// Get script
	script, err := keyword.RequiredParam[string](params, "script")
	if err != nil {
		return nil, err
	}

	// Get stopOnError option
	stopOnError := keyword.OptionalParam(params, "stopOnError", true)

	// Create Goja runtime
	vm := goja.New()

	// Create console logs collector
	consoleLogs := make([]string, 0)

	// Setup execution environment
	if err := setupEnvironment(vm, execCtx, &consoleLogs); err != nil {
		return nil, fmt.Errorf("failed to setup JS environment: %w", err)
	}

	// Execute script
	_, err = vm.RunString(script)
	if err != nil {
		errMsg := fmt.Sprintf("JS script error: %v", err)
		consoleLogs = append(consoleLogs, fmt.Sprintf("[ERROR] %s", errMsg))

		// Store console logs
		execCtx.SetMetadata("console_logs", consoleLogs)

		if stopOnError {
			return keyword.NewFailureResult(errMsg, err), nil
		}
		// Continue even on error
		return keyword.NewSuccessResult(fmt.Sprintf("script completed with error: %v", err), map[string]any{
			"error":   err.Error(),
			"console": consoleLogs,
		}), nil
	}

	// Store console logs
	execCtx.SetMetadata("console_logs", consoleLogs)

	return keyword.NewSuccessResult("script executed successfully", map[string]any{
		"console": consoleLogs,
	}), nil
}

func (k *jsScriptKeyword) Validate(params map[string]any) error {
	if _, ok := params["script"]; !ok {
		return fmt.Errorf("'script' parameter is required")
	}
	return nil
}
