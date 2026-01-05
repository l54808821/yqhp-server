// Package extractor provides extractor keywords for workflow engine v2.
package extractor

import (
	"context"
	"encoding/json"
	"fmt"

	"yqhp/workflow-engine/internal/keyword"
	"github.com/ohler55/ojg/jp"
)

// JSONPath creates a json_path extractor keyword.
func JSONPath() keyword.Keyword {
	return &jsonPathKeyword{
		BaseKeyword: keyword.NewBaseKeyword("json_path", keyword.CategoryExtractor, "Extracts value using JSONPath expression"),
	}
}

type jsonPathKeyword struct {
	keyword.BaseKeyword
}

func (k *jsonPathKeyword) Execute(ctx context.Context, execCtx *keyword.ExecutionContext, params map[string]any) (*keyword.Result, error) {
	// Get the expression
	expression, err := keyword.RequiredParam[string](params, "expression")
	if err != nil {
		// Also support "json_path" as the expression parameter name
		expression, err = keyword.RequiredParam[string](params, "json_path")
		if err != nil {
			return nil, fmt.Errorf("'expression' or 'json_path' parameter is required")
		}
	}

	// Get the target variable name
	to, err := keyword.RequiredParam[string](params, "to")
	if err != nil {
		return nil, err
	}

	// Get the source data
	source := keyword.OptionalParam(params, "source", "response.body")
	defaultVal := params["default"]
	index := params["index"]

	// Get data from context
	data, err := getSourceData(execCtx, source, params)
	if err != nil {
		if defaultVal != nil {
			execCtx.SetVariable(to, defaultVal)
			return keyword.NewSuccessResult(fmt.Sprintf("extraction failed, using default value: %v", defaultVal), defaultVal), nil
		}
		return keyword.NewFailureResult(fmt.Sprintf("failed to get source data: %v", err), err), nil
	}

	// Parse JSONPath expression
	path, err := jp.ParseString(expression)
	if err != nil {
		return keyword.NewFailureResult(fmt.Sprintf("invalid JSONPath expression '%s': %v", expression, err), err), nil
	}

	// Execute JSONPath
	results := path.Get(data)
	if len(results) == 0 {
		if defaultVal != nil {
			execCtx.SetVariable(to, defaultVal)
			return keyword.NewSuccessResult(fmt.Sprintf("no match found, using default value: %v", defaultVal), defaultVal), nil
		}
		return keyword.NewFailureResult(fmt.Sprintf("JSONPath '%s' returned no results", expression), nil), nil
	}

	// Handle index parameter
	var result any
	if index != nil {
		idx, ok := toInt(index)
		if !ok {
			return keyword.NewFailureResult("index must be an integer", nil), nil
		}
		if idx < 0 || idx >= len(results) {
			if defaultVal != nil {
				execCtx.SetVariable(to, defaultVal)
				return keyword.NewSuccessResult(fmt.Sprintf("index out of range, using default value: %v", defaultVal), defaultVal), nil
			}
			return keyword.NewFailureResult(fmt.Sprintf("index %d out of range (0-%d)", idx, len(results)-1), nil), nil
		}
		result = results[idx]
	} else if len(results) == 1 {
		result = results[0]
	} else {
		result = results
	}

	// Store the result
	execCtx.SetVariable(to, result)
	return keyword.NewSuccessResult(fmt.Sprintf("extracted value to '%s'", to), result), nil
}

func (k *jsonPathKeyword) Validate(params map[string]any) error {
	_, hasExpr := params["expression"]
	_, hasJP := params["json_path"]
	if !hasExpr && !hasJP {
		return fmt.Errorf("'expression' or 'json_path' parameter is required")
	}
	if _, ok := params["to"]; !ok {
		return fmt.Errorf("'to' parameter is required")
	}
	return nil
}

// getSourceData retrieves data from the execution context based on source.
func getSourceData(execCtx *keyword.ExecutionContext, source string, params map[string]any) (any, error) {
	// If data is provided directly in params, use it
	if data, ok := params["data"]; ok {
		return parseData(data)
	}

	// Get from response
	resp := execCtx.GetResponse()
	if resp == nil {
		return nil, fmt.Errorf("no response data available")
	}

	switch source {
	case "response.body", "body":
		return parseData(resp.Body)
	case "response.data", "data":
		if resp.Data != nil {
			return resp.Data, nil
		}
		return parseData(resp.Body)
	case "response.headers", "headers":
		return resp.Headers, nil
	default:
		// Try to get from variables
		if val, ok := execCtx.GetVariable(source); ok {
			return parseData(val)
		}
		return nil, fmt.Errorf("unknown source: %s", source)
	}
}

// parseData parses data into a Go value suitable for JSONPath.
func parseData(data any) (any, error) {
	switch v := data.(type) {
	case string:
		var result any
		if err := json.Unmarshal([]byte(v), &result); err != nil {
			return nil, fmt.Errorf("failed to parse JSON: %w", err)
		}
		return result, nil
	case []byte:
		var result any
		if err := json.Unmarshal(v, &result); err != nil {
			return nil, fmt.Errorf("failed to parse JSON: %w", err)
		}
		return result, nil
	default:
		return data, nil
	}
}

// toInt converts a value to int.
func toInt(v any) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case int64:
		return int(n), true
	case float64:
		return int(n), true
	default:
		return 0, false
	}
}

// RegisterJSONPathExtractor registers the JSONPath extractor.
func RegisterJSONPathExtractor(registry *keyword.Registry) {
	registry.MustRegister(JSONPath())
}
