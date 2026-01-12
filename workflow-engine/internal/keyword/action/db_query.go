package action

import (
	"context"
	"fmt"

	"yqhp/workflow-engine/internal/keyword"
)

// DBQuery creates a db_query action keyword.
func DBQuery() keyword.Keyword {
	return &dbQueryKeyword{
		BaseKeyword: keyword.NewBaseKeyword("db_query", keyword.CategoryAction, "Executes a database query"),
	}
}

type dbQueryKeyword struct {
	keyword.BaseKeyword
}

func (k *dbQueryKeyword) Execute(ctx context.Context, execCtx *keyword.ExecutionContext, params map[string]any) (*keyword.Result, error) {
	// Get datasource ID
	datasourceID, err := keyword.RequiredParam[float64](params, "datasourceId")
	if err != nil {
		// Try integer
		if id, ok := params["datasourceId"].(int); ok {
			datasourceID = float64(id)
		} else {
			return nil, fmt.Errorf("'datasourceId' parameter is required")
		}
	}

	// Get SQL
	sql, err := keyword.RequiredParam[string](params, "sql")
	if err != nil {
		return nil, err
	}

	// Get variable name for result
	variableName := keyword.OptionalParam(params, "variableName", "")

	// TODO: Execute the actual database query
	// This requires database connection management which should be injected
	// For now, we'll return a placeholder result

	result := map[string]any{
		"datasourceId": int(datasourceID),
		"sql":          sql,
		"rows":         []map[string]any{},
		"rowCount":     0,
	}

	// Store result if variable name is provided
	if variableName != "" {
		execCtx.SetVariable(variableName, result["rows"])
	}

	return keyword.NewSuccessResult(fmt.Sprintf("executed SQL on datasource %d", int(datasourceID)), result), nil
}

func (k *dbQueryKeyword) Validate(params map[string]any) error {
	if _, ok := params["datasourceId"]; !ok {
		return fmt.Errorf("'datasourceId' parameter is required")
	}
	if _, ok := params["sql"]; !ok {
		return fmt.Errorf("'sql' parameter is required")
	}
	return nil
}

// RegisterDBQuery registers the db_query keyword.
func RegisterDBQuery(registry *keyword.Registry) {
	registry.MustRegister(DBQuery())
}
