package extractor

import (
	"context"
	"fmt"
	"strings"

	"github.com/grafana/k6/workflow-engine/internal/keyword"
)

// Header creates a header extractor keyword.
func Header() keyword.Keyword {
	return &headerKeyword{
		BaseKeyword: keyword.NewBaseKeyword("header", keyword.CategoryExtractor, "Extracts response header value"),
	}
}

type headerKeyword struct {
	keyword.BaseKeyword
}

func (k *headerKeyword) Execute(ctx context.Context, execCtx *keyword.ExecutionContext, params map[string]any) (*keyword.Result, error) {
	// Get the header name
	headerName, err := keyword.RequiredParam[string](params, "name")
	if err != nil {
		headerName, err = keyword.RequiredParam[string](params, "header")
		if err != nil {
			return nil, fmt.Errorf("'name' or 'header' parameter is required")
		}
	}

	// Get the target variable name
	to, err := keyword.RequiredParam[string](params, "to")
	if err != nil {
		return nil, err
	}

	defaultVal := params["default"]

	// Get response
	resp := execCtx.GetResponse()
	if resp == nil {
		if defaultVal != nil {
			execCtx.SetVariable(to, defaultVal)
			return keyword.NewSuccessResult(fmt.Sprintf("no response, using default value: %v", defaultVal), defaultVal), nil
		}
		return keyword.NewFailureResult("no response data available", nil), nil
	}

	// Find header (case-insensitive)
	var value string
	found := false
	for k, v := range resp.Headers {
		if strings.EqualFold(k, headerName) {
			value = v
			found = true
			break
		}
	}

	if !found {
		if defaultVal != nil {
			execCtx.SetVariable(to, defaultVal)
			return keyword.NewSuccessResult(fmt.Sprintf("header '%s' not found, using default value: %v", headerName, defaultVal), defaultVal), nil
		}
		return keyword.NewFailureResult(fmt.Sprintf("header '%s' not found", headerName), nil), nil
	}

	execCtx.SetVariable(to, value)
	return keyword.NewSuccessResult(fmt.Sprintf("extracted header '%s' to '%s'", headerName, to), value), nil
}

func (k *headerKeyword) Validate(params map[string]any) error {
	_, hasName := params["name"]
	_, hasHeader := params["header"]
	if !hasName && !hasHeader {
		return fmt.Errorf("'name' or 'header' parameter is required")
	}
	if _, ok := params["to"]; !ok {
		return fmt.Errorf("'to' parameter is required")
	}
	return nil
}

// Cookie creates a cookie extractor keyword.
func Cookie() keyword.Keyword {
	return &cookieKeyword{
		BaseKeyword: keyword.NewBaseKeyword("cookie", keyword.CategoryExtractor, "Extracts cookie value from Set-Cookie header"),
	}
}

type cookieKeyword struct {
	keyword.BaseKeyword
}

func (k *cookieKeyword) Execute(ctx context.Context, execCtx *keyword.ExecutionContext, params map[string]any) (*keyword.Result, error) {
	// Get the cookie name
	cookieName, err := keyword.RequiredParam[string](params, "name")
	if err != nil {
		cookieName, err = keyword.RequiredParam[string](params, "cookie")
		if err != nil {
			return nil, fmt.Errorf("'name' or 'cookie' parameter is required")
		}
	}

	// Get the target variable name
	to, err := keyword.RequiredParam[string](params, "to")
	if err != nil {
		return nil, err
	}

	defaultVal := params["default"]

	// Get response
	resp := execCtx.GetResponse()
	if resp == nil {
		if defaultVal != nil {
			execCtx.SetVariable(to, defaultVal)
			return keyword.NewSuccessResult(fmt.Sprintf("no response, using default value: %v", defaultVal), defaultVal), nil
		}
		return keyword.NewFailureResult("no response data available", nil), nil
	}

	// Find Set-Cookie header
	var setCookie string
	for k, v := range resp.Headers {
		if strings.EqualFold(k, "Set-Cookie") {
			setCookie = v
			break
		}
	}

	if setCookie == "" {
		if defaultVal != nil {
			execCtx.SetVariable(to, defaultVal)
			return keyword.NewSuccessResult(fmt.Sprintf("no Set-Cookie header, using default value: %v", defaultVal), defaultVal), nil
		}
		return keyword.NewFailureResult("no Set-Cookie header found", nil), nil
	}

	// Parse cookie value
	value := parseCookieValue(setCookie, cookieName)
	if value == "" {
		if defaultVal != nil {
			execCtx.SetVariable(to, defaultVal)
			return keyword.NewSuccessResult(fmt.Sprintf("cookie '%s' not found, using default value: %v", cookieName, defaultVal), defaultVal), nil
		}
		return keyword.NewFailureResult(fmt.Sprintf("cookie '%s' not found", cookieName), nil), nil
	}

	execCtx.SetVariable(to, value)
	return keyword.NewSuccessResult(fmt.Sprintf("extracted cookie '%s' to '%s'", cookieName, to), value), nil
}

func (k *cookieKeyword) Validate(params map[string]any) error {
	_, hasName := params["name"]
	_, hasCookie := params["cookie"]
	if !hasName && !hasCookie {
		return fmt.Errorf("'name' or 'cookie' parameter is required")
	}
	if _, ok := params["to"]; !ok {
		return fmt.Errorf("'to' parameter is required")
	}
	return nil
}

// parseCookieValue extracts a cookie value from Set-Cookie header.
func parseCookieValue(setCookie, name string) string {
	// Set-Cookie format: name=value; attr1; attr2=val2
	parts := strings.Split(setCookie, ";")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if idx := strings.Index(part, "="); idx > 0 {
			cookieName := strings.TrimSpace(part[:idx])
			if strings.EqualFold(cookieName, name) {
				return strings.TrimSpace(part[idx+1:])
			}
		}
	}
	return ""
}

// Status creates a status extractor keyword.
func Status() keyword.Keyword {
	return &statusKeyword{
		BaseKeyword: keyword.NewBaseKeyword("status", keyword.CategoryExtractor, "Extracts response status code"),
	}
}

type statusKeyword struct {
	keyword.BaseKeyword
}

func (k *statusKeyword) Execute(ctx context.Context, execCtx *keyword.ExecutionContext, params map[string]any) (*keyword.Result, error) {
	// Get the target variable name
	to, err := keyword.RequiredParam[string](params, "to")
	if err != nil {
		return nil, err
	}

	defaultVal := params["default"]

	// Get response
	resp := execCtx.GetResponse()
	if resp == nil {
		if defaultVal != nil {
			execCtx.SetVariable(to, defaultVal)
			return keyword.NewSuccessResult(fmt.Sprintf("no response, using default value: %v", defaultVal), defaultVal), nil
		}
		return keyword.NewFailureResult("no response data available", nil), nil
	}

	execCtx.SetVariable(to, resp.Status)
	return keyword.NewSuccessResult(fmt.Sprintf("extracted status %d to '%s'", resp.Status, to), resp.Status), nil
}

func (k *statusKeyword) Validate(params map[string]any) error {
	if _, ok := params["to"]; !ok {
		return fmt.Errorf("'to' parameter is required")
	}
	return nil
}

// Body creates a body extractor keyword.
func Body() keyword.Keyword {
	return &bodyKeyword{
		BaseKeyword: keyword.NewBaseKeyword("body", keyword.CategoryExtractor, "Extracts entire response body"),
	}
}

type bodyKeyword struct {
	keyword.BaseKeyword
}

func (k *bodyKeyword) Execute(ctx context.Context, execCtx *keyword.ExecutionContext, params map[string]any) (*keyword.Result, error) {
	// Get the target variable name
	to, err := keyword.RequiredParam[string](params, "to")
	if err != nil {
		return nil, err
	}

	defaultVal := params["default"]

	// Get response
	resp := execCtx.GetResponse()
	if resp == nil {
		if defaultVal != nil {
			execCtx.SetVariable(to, defaultVal)
			return keyword.NewSuccessResult(fmt.Sprintf("no response, using default value: %v", defaultVal), defaultVal), nil
		}
		return keyword.NewFailureResult("no response data available", nil), nil
	}

	execCtx.SetVariable(to, resp.Body)
	return keyword.NewSuccessResult(fmt.Sprintf("extracted body to '%s'", to), resp.Body), nil
}

func (k *bodyKeyword) Validate(params map[string]any) error {
	if _, ok := params["to"]; !ok {
		return fmt.Errorf("'to' parameter is required")
	}
	return nil
}

// RegisterHeaderExtractors registers header-related extractors.
func RegisterHeaderExtractors(registry *keyword.Registry) {
	registry.MustRegister(Header())
	registry.MustRegister(Cookie())
	registry.MustRegister(Status())
	registry.MustRegister(Body())
}
