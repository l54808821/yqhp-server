package assertion

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/grafana/k6/workflow-engine/internal/keyword"
)

// stringKeyword is a base implementation for string assertions.
type stringKeyword struct {
	keyword.BaseKeyword
	check func(actual, expected string) (bool, error)
}

func newStringKeyword(name, description string, check func(actual, expected string) (bool, error)) *stringKeyword {
	return &stringKeyword{
		BaseKeyword: keyword.NewBaseKeyword(name, keyword.CategoryAssertion, description),
		check:       check,
	}
}

func (k *stringKeyword) Execute(ctx context.Context, execCtx *keyword.ExecutionContext, params map[string]any) (*keyword.Result, error) {
	actualRaw, ok := params["actual"]
	if !ok {
		return nil, fmt.Errorf("required parameter 'actual' is missing")
	}
	actual := fmt.Sprintf("%v", actualRaw)

	expectedRaw, ok := params["expected"]
	if !ok {
		return nil, fmt.Errorf("required parameter 'expected' is missing")
	}
	expected := fmt.Sprintf("%v", expectedRaw)

	message := keyword.OptionalParam(params, "message", "")

	result, err := k.check(actual, expected)
	if err != nil {
		return keyword.NewFailureResult(fmt.Sprintf("check error: %v", err), err), nil
	}

	if result {
		return keyword.NewSuccessResult("assertion passed", nil), nil
	}

	failMsg := fmt.Sprintf("assertion '%s' failed: actual='%s', expected='%s'", k.Name(), actual, expected)
	if message != "" {
		failMsg = fmt.Sprintf("%s - %s", message, failMsg)
	}
	return keyword.NewFailureResult(failMsg, nil), nil
}

func (k *stringKeyword) Validate(params map[string]any) error {
	if _, ok := params["actual"]; !ok {
		return fmt.Errorf("'actual' parameter is required")
	}
	if _, ok := params["expected"]; !ok {
		return fmt.Errorf("'expected' parameter is required")
	}
	return nil
}

// Contains creates a contains assertion keyword.
func Contains() keyword.Keyword {
	return newStringKeyword("contains", "Asserts that actual contains expected substring", func(actual, expected string) (bool, error) {
		return strings.Contains(actual, expected), nil
	})
}

// NotContains creates a not_contains assertion keyword.
func NotContains() keyword.Keyword {
	return newStringKeyword("not_contains", "Asserts that actual does not contain expected substring", func(actual, expected string) (bool, error) {
		return !strings.Contains(actual, expected), nil
	})
}

// StartsWith creates a starts_with assertion keyword.
func StartsWith() keyword.Keyword {
	return newStringKeyword("starts_with", "Asserts that actual starts with expected prefix", func(actual, expected string) (bool, error) {
		return strings.HasPrefix(actual, expected), nil
	})
}

// EndsWith creates an ends_with assertion keyword.
func EndsWith() keyword.Keyword {
	return newStringKeyword("ends_with", "Asserts that actual ends with expected suffix", func(actual, expected string) (bool, error) {
		return strings.HasSuffix(actual, expected), nil
	})
}

// Matches creates a matches assertion keyword for regex matching.
func Matches() keyword.Keyword {
	return newStringKeyword("matches", "Asserts that actual matches expected regex pattern", func(actual, expected string) (bool, error) {
		re, err := regexp.Compile(expected)
		if err != nil {
			return false, fmt.Errorf("invalid regex pattern '%s': %w", expected, err)
		}
		return re.MatchString(actual), nil
	})
}

// RegisterStringAssertions registers all string assertion keywords.
func RegisterStringAssertions(registry *keyword.Registry) {
	registry.MustRegister(Contains())
	registry.MustRegister(NotContains())
	registry.MustRegister(StartsWith())
	registry.MustRegister(EndsWith())
	registry.MustRegister(Matches())
}
