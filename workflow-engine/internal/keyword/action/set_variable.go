// Package action provides action keywords for workflow engine v2.
package action

import (
	"context"
	"fmt"

	"yqhp/workflow-engine/internal/keyword"
)

// SetVariable creates a set_variable action keyword.
func SetVariable() keyword.Keyword {
	return &setVariableKeyword{
		BaseKeyword: keyword.NewBaseKeyword("set_variable", keyword.CategoryAction, "Sets a variable in the execution context"),
	}
}

type setVariableKeyword struct {
	keyword.BaseKeyword
}

func (k *setVariableKeyword) Execute(ctx context.Context, execCtx *keyword.ExecutionContext, params map[string]any) (*keyword.Result, error) {
	// Get variable name
	name, err := keyword.RequiredParam[string](params, "name")
	if err != nil {
		// Also support "variableName" as parameter name
		name, err = keyword.RequiredParam[string](params, "variableName")
		if err != nil {
			return nil, fmt.Errorf("'name' or 'variableName' parameter is required")
		}
	}

	// Get value
	value, ok := params["value"]
	if !ok {
		return nil, fmt.Errorf("'value' parameter is required")
	}

	// Get scope (temp or env)
	scope := keyword.OptionalParam(params, "scope", "temp")

	// Set the variable
	execCtx.SetVariable(name, value)

	// If scope is env, also mark it for environment persistence
	if scope == "env" {
		execCtx.SetMetadata(fmt.Sprintf("env_var_%s", name), value)
	}

	return keyword.NewSuccessResult(fmt.Sprintf("variable '%s' set to '%v' (scope: %s)", name, value, scope), value), nil
}

func (k *setVariableKeyword) Validate(params map[string]any) error {
	_, hasName := params["name"]
	_, hasVarName := params["variableName"]
	if !hasName && !hasVarName {
		return fmt.Errorf("'name' or 'variableName' parameter is required")
	}
	if _, ok := params["value"]; !ok {
		return fmt.Errorf("'value' parameter is required")
	}
	return nil
}

// RegisterSetVariable registers the set_variable keyword.
func RegisterSetVariable(registry *keyword.Registry) {
	registry.MustRegister(SetVariable())
}
