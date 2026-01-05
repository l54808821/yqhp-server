package extractor

import (
	"context"
	"fmt"

	"yqhp/workflow-engine/internal/keyword"
)

// Extract creates a generic extract keyword that delegates to specific extractors.
func Extract() keyword.Keyword {
	return &extractKeyword{
		BaseKeyword: keyword.NewBaseKeyword("extract", keyword.CategoryExtractor, "Generic extractor that delegates based on parameters"),
	}
}

type extractKeyword struct {
	keyword.BaseKeyword
}

func (k *extractKeyword) Execute(ctx context.Context, execCtx *keyword.ExecutionContext, params map[string]any) (*keyword.Result, error) {
	// Determine which extractor to use based on parameters
	if _, ok := params["json_path"]; ok {
		return JSONPath().Execute(ctx, execCtx, params)
	}
	if _, ok := params["regex"]; ok {
		return Regex().Execute(ctx, execCtx, params)
	}
	if _, ok := params["pattern"]; ok {
		return Regex().Execute(ctx, execCtx, params)
	}
	if _, ok := params["header"]; ok {
		return Header().Execute(ctx, execCtx, params)
	}
	if _, ok := params["cookie"]; ok {
		return Cookie().Execute(ctx, execCtx, params)
	}

	// Check for expression parameter and try to determine type
	if expr, ok := params["expression"].(string); ok {
		// If expression starts with $, assume JSONPath
		if len(expr) > 0 && expr[0] == '$' {
			return JSONPath().Execute(ctx, execCtx, params)
		}
		// Otherwise assume regex
		return Regex().Execute(ctx, execCtx, params)
	}

	return nil, fmt.Errorf("cannot determine extractor type from parameters")
}

func (k *extractKeyword) Validate(params map[string]any) error {
	if _, ok := params["to"]; !ok {
		return fmt.Errorf("'to' parameter is required")
	}

	// Check for at least one extraction method
	_, hasJP := params["json_path"]
	_, hasRegex := params["regex"]
	_, hasPattern := params["pattern"]
	_, hasHeader := params["header"]
	_, hasCookie := params["cookie"]
	_, hasExpr := params["expression"]

	if !hasJP && !hasRegex && !hasPattern && !hasHeader && !hasCookie && !hasExpr {
		return fmt.Errorf("extraction method required (json_path, regex, pattern, header, cookie, or expression)")
	}

	return nil
}

// RegisterAllExtractors registers all extractor keywords.
func RegisterAllExtractors(registry *keyword.Registry) {
	registry.MustRegister(Extract())
	RegisterJSONPathExtractor(registry)
	RegisterRegexExtractor(registry)
	RegisterHeaderExtractors(registry)
}
