package action

import (
	"context"
	"fmt"
	"time"

	"yqhp/workflow-engine/internal/keyword"
)

// Wait creates a wait action keyword.
func Wait() keyword.Keyword {
	return &waitKeyword{
		BaseKeyword: keyword.NewBaseKeyword("wait", keyword.CategoryAction, "Waits for a specified duration"),
	}
}

type waitKeyword struct {
	keyword.BaseKeyword
}

func (k *waitKeyword) Execute(ctx context.Context, execCtx *keyword.ExecutionContext, params map[string]any) (*keyword.Result, error) {
	// Get duration in milliseconds
	duration, err := getDuration(params)
	if err != nil {
		return nil, err
	}

	// Create a timer
	timer := time.NewTimer(time.Duration(duration) * time.Millisecond)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return keyword.NewFailureResult("wait cancelled", ctx.Err()), nil
	case <-timer.C:
		return keyword.NewSuccessResult(fmt.Sprintf("waited %d ms", duration), nil), nil
	}
}

func (k *waitKeyword) Validate(params map[string]any) error {
	_, err := getDuration(params)
	return err
}

// getDuration extracts duration from params.
func getDuration(params map[string]any) (int64, error) {
	// Try "duration" first
	if d, ok := params["duration"]; ok {
		return toInt64(d)
	}
	// Try "ms" as alias
	if d, ok := params["ms"]; ok {
		return toInt64(d)
	}
	// Try "seconds" and convert to ms
	if d, ok := params["seconds"]; ok {
		sec, err := toInt64(d)
		if err != nil {
			return 0, err
		}
		return sec * 1000, nil
	}
	return 0, fmt.Errorf("'duration' or 'ms' or 'seconds' parameter is required")
}

// toInt64 converts a value to int64.
func toInt64(v any) (int64, error) {
	switch n := v.(type) {
	case int:
		return int64(n), nil
	case int32:
		return int64(n), nil
	case int64:
		return n, nil
	case float64:
		return int64(n), nil
	case float32:
		return int64(n), nil
	default:
		return 0, fmt.Errorf("duration must be a number, got %T", v)
	}
}

// RegisterWait registers the wait keyword.
func RegisterWait(registry *keyword.Registry) {
	registry.MustRegister(Wait())
}
