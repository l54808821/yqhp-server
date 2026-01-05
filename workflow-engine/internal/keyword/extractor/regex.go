package extractor

import (
	"context"
	"fmt"
	"regexp"

	"yqhp/workflow-engine/internal/keyword"
)

// Regex creates a regex extractor keyword.
func Regex() keyword.Keyword {
	return &regexKeyword{
		BaseKeyword: keyword.NewBaseKeyword("regex", keyword.CategoryExtractor, "Extracts value using regular expression"),
	}
}

type regexKeyword struct {
	keyword.BaseKeyword
}

func (k *regexKeyword) Execute(ctx context.Context, execCtx *keyword.ExecutionContext, params map[string]any) (*keyword.Result, error) {
	// Get the pattern
	pattern, err := keyword.RequiredParam[string](params, "expression")
	if err != nil {
		pattern, err = keyword.RequiredParam[string](params, "regex")
		if err != nil {
			pattern, err = keyword.RequiredParam[string](params, "pattern")
			if err != nil {
				return nil, fmt.Errorf("'expression', 'regex', or 'pattern' parameter is required")
			}
		}
	}

	// Get the target variable name
	to, err := keyword.RequiredParam[string](params, "to")
	if err != nil {
		return nil, err
	}

	// Get optional parameters
	defaultVal := params["default"]
	index := params["index"]
	group := keyword.OptionalParam(params, "group", 0)

	// Get source data
	source := keyword.OptionalParam(params, "source", "response.body")
	data, err := getSourceString(execCtx, source, params)
	if err != nil {
		if defaultVal != nil {
			execCtx.SetVariable(to, defaultVal)
			return keyword.NewSuccessResult(fmt.Sprintf("extraction failed, using default value: %v", defaultVal), defaultVal), nil
		}
		return keyword.NewFailureResult(fmt.Sprintf("failed to get source data: %v", err), err), nil
	}

	// Compile regex
	re, err := regexp.Compile(pattern)
	if err != nil {
		return keyword.NewFailureResult(fmt.Sprintf("invalid regex pattern '%s': %v", pattern, err), err), nil
	}

	// Find matches
	matches := re.FindAllStringSubmatch(data, -1)
	if len(matches) == 0 {
		if defaultVal != nil {
			execCtx.SetVariable(to, defaultVal)
			return keyword.NewSuccessResult(fmt.Sprintf("no match found, using default value: %v", defaultVal), defaultVal), nil
		}
		return keyword.NewFailureResult(fmt.Sprintf("regex '%s' returned no matches", pattern), nil), nil
	}

	// Handle index parameter (which match to use)
	matchIdx := 0
	if index != nil {
		idx, ok := toInt(index)
		if !ok {
			return keyword.NewFailureResult("index must be an integer", nil), nil
		}
		if idx < 0 || idx >= len(matches) {
			if defaultVal != nil {
				execCtx.SetVariable(to, defaultVal)
				return keyword.NewSuccessResult(fmt.Sprintf("index out of range, using default value: %v", defaultVal), defaultVal), nil
			}
			return keyword.NewFailureResult(fmt.Sprintf("index %d out of range (0-%d)", idx, len(matches)-1), nil), nil
		}
		matchIdx = idx
	}

	// Get the specific match
	match := matches[matchIdx]

	// Handle group parameter (which capture group to use)
	groupIdx, ok := toInt(group)
	if !ok {
		groupIdx = 0
	}
	if groupIdx < 0 || groupIdx >= len(match) {
		if defaultVal != nil {
			execCtx.SetVariable(to, defaultVal)
			return keyword.NewSuccessResult(fmt.Sprintf("group out of range, using default value: %v", defaultVal), defaultVal), nil
		}
		return keyword.NewFailureResult(fmt.Sprintf("group %d out of range (0-%d)", groupIdx, len(match)-1), nil), nil
	}

	result := match[groupIdx]
	execCtx.SetVariable(to, result)
	return keyword.NewSuccessResult(fmt.Sprintf("extracted value to '%s'", to), result), nil
}

func (k *regexKeyword) Validate(params map[string]any) error {
	_, hasExpr := params["expression"]
	_, hasRegex := params["regex"]
	_, hasPattern := params["pattern"]
	if !hasExpr && !hasRegex && !hasPattern {
		return fmt.Errorf("'expression', 'regex', or 'pattern' parameter is required")
	}
	if _, ok := params["to"]; !ok {
		return fmt.Errorf("'to' parameter is required")
	}
	return nil
}

// getSourceString retrieves string data from the execution context.
func getSourceString(execCtx *keyword.ExecutionContext, source string, params map[string]any) (string, error) {
	// If data is provided directly in params, use it
	if data, ok := params["data"]; ok {
		return fmt.Sprintf("%v", data), nil
	}

	// Get from response
	resp := execCtx.GetResponse()
	if resp == nil {
		return "", fmt.Errorf("no response data available")
	}

	switch source {
	case "response.body", "body":
		return resp.Body, nil
	default:
		// Try to get from variables
		if val, ok := execCtx.GetVariable(source); ok {
			return fmt.Sprintf("%v", val), nil
		}
		return "", fmt.Errorf("unknown source: %s", source)
	}
}

// RegisterRegexExtractor registers the regex extractor.
func RegisterRegexExtractor(registry *keyword.Registry) {
	registry.MustRegister(Regex())
}
