package executor

import (
	"context"
	"testing"

	"yqhp/workflow-engine/pkg/script"
	"yqhp/workflow-engine/pkg/types"
)

func TestCallExecutor_Execute(t *testing.T) {
	tests := []struct {
		name       string
		fragment   *script.Fragment
		stepConfig map[string]any
		execCtx    *ExecutionContext
		wantErr    bool
		checkFunc  func(t *testing.T, result *types.StepResult, execCtx *ExecutionContext)
	}{
		{
			name: "basic script call",
			fragment: &script.Fragment{
				Name:   "test_script",
				Params: []script.Param{},
				Steps:  []any{},
				Returns: []script.Return{
					{Name: "result", Value: "${_result}"},
				},
			},
			stepConfig: map[string]any{
				"script": "test_script",
				"results": map[string]any{
					"result": "output_var",
				},
			},
			execCtx: func() *ExecutionContext {
				ctx := NewExecutionContext()
				ctx.SetVariable("_result", "test_value")
				return ctx
			}(),
			wantErr: false,
			checkFunc: func(t *testing.T, result *types.StepResult, execCtx *ExecutionContext) {
				if result.Status != types.ResultStatusSuccess {
					t.Errorf("expected success status, got %s", result.Status)
				}
				// 检查返回值是否被映射
				if val, ok := execCtx.GetVariable("output_var"); !ok || val != "test_value" {
					t.Errorf("expected output_var to be 'test_value', got %v", val)
				}
			},
		},
		{
			name: "script with default params",
			fragment: &script.Fragment{
				Name: "script_with_defaults",
				Params: []script.Param{
					{Name: "param1", Type: script.ParamTypeString, Default: "default_value"},
					{Name: "param2", Type: script.ParamTypeNumber, Required: false},
				},
				Steps:   []any{},
				Returns: []script.Return{},
			},
			stepConfig: map[string]any{
				"script": "script_with_defaults",
				"params": map[string]any{},
			},
			execCtx: NewExecutionContext(),
			wantErr: false,
			checkFunc: func(t *testing.T, result *types.StepResult, execCtx *ExecutionContext) {
				if result.Status != types.ResultStatusSuccess {
					t.Errorf("expected success status, got %s", result.Status)
				}
			},
		},
		{
			name: "script with required param missing",
			fragment: &script.Fragment{
				Name: "script_required",
				Params: []script.Param{
					{Name: "required_param", Type: script.ParamTypeString, Required: true},
				},
				Steps:   []any{},
				Returns: []script.Return{},
			},
			stepConfig: map[string]any{
				"script": "script_required",
				"params": map[string]any{},
			},
			execCtx: NewExecutionContext(),
			wantErr: false, // 错误在结果中，不是返回的 error
			checkFunc: func(t *testing.T, result *types.StepResult, execCtx *ExecutionContext) {
				if result.Status != types.ResultStatusFailed {
					t.Errorf("expected failed status for missing required param, got %s", result.Status)
				}
			},
		},
		{
			name: "script not found",
			fragment: &script.Fragment{
				Name:    "existing_script",
				Params:  []script.Param{},
				Steps:   []any{},
				Returns: []script.Return{},
			},
			stepConfig: map[string]any{
				"script": "non_existent_script",
			},
			execCtx: NewExecutionContext(),
			wantErr: false,
			checkFunc: func(t *testing.T, result *types.StepResult, execCtx *ExecutionContext) {
				if result.Status != types.ResultStatusFailed {
					t.Errorf("expected failed status for non-existent script, got %s", result.Status)
				}
			},
		},
		{
			name: "script with multiple returns",
			fragment: &script.Fragment{
				Name:   "multi_return_script",
				Params: []script.Param{},
				Steps:  []any{},
				Returns: []script.Return{
					{Name: "token", Value: "${_token}"},
					{Name: "user_id", Value: "${_user_id}"},
					{Name: "role", Value: "${_role}"},
				},
			},
			stepConfig: map[string]any{
				"script": "multi_return_script",
				"results": map[string]any{
					"token":   "auth_token",
					"user_id": "current_user",
					// role 不映射，应该被忽略
				},
			},
			execCtx: func() *ExecutionContext {
				ctx := NewExecutionContext()
				ctx.SetVariable("_token", "abc123")
				ctx.SetVariable("_user_id", 42)
				ctx.SetVariable("_role", "admin")
				return ctx
			}(),
			wantErr: false,
			checkFunc: func(t *testing.T, result *types.StepResult, execCtx *ExecutionContext) {
				if result.Status != types.ResultStatusSuccess {
					t.Errorf("expected success status, got %s", result.Status)
				}
				// 检查映射的返回值
				if val, ok := execCtx.GetVariable("auth_token"); !ok || val != "abc123" {
					t.Errorf("expected auth_token to be 'abc123', got %v", val)
				}
				if val, ok := execCtx.GetVariable("current_user"); !ok || val != 42 {
					t.Errorf("expected current_user to be 42, got %v", val)
				}
				// role 不应该被映射
				if _, ok := execCtx.GetVariable("role"); ok {
					t.Error("role should not be mapped")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			executor := NewCallExecutor()

			// 注册脚本
			if tt.fragment != nil {
				_ = executor.registry.Register(tt.fragment)
			}

			step := &types.Step{
				ID:     "test_step",
				Type:   "call",
				Config: tt.stepConfig,
			}

			result, err := executor.Execute(context.Background(), step, tt.execCtx)
			if (err != nil) != tt.wantErr {
				t.Errorf("Execute() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.checkFunc != nil {
				tt.checkFunc(t, result, tt.execCtx)
			}
		})
	}
}

func TestCallExecutor_CircularCallDetection(t *testing.T) {
	executor := NewCallExecutor()

	// 注册脚本 A
	scriptA := &script.Fragment{
		Name:   "script_a",
		Params: []script.Param{},
		Steps:  []any{},
	}
	_ = executor.registry.Register(scriptA)

	// 模拟调用栈中已有 script_a
	_ = executor.callStack.Push("script_a")

	step := &types.Step{
		ID:   "test_step",
		Type: "call",
		Config: map[string]any{
			"script": "script_a",
		},
	}

	result, err := executor.Execute(context.Background(), step, NewExecutionContext())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Status != types.ResultStatusFailed {
		t.Errorf("expected failed status for circular call, got %s", result.Status)
	}
}

func TestCallExecutor_VariableResolution(t *testing.T) {
	executor := NewCallExecutor()

	fragment := &script.Fragment{
		Name: "var_script",
		Params: []script.Param{
			{Name: "input", Type: script.ParamTypeString},
		},
		Steps:   []any{},
		Returns: []script.Return{},
	}
	_ = executor.registry.Register(fragment)

	execCtx := NewExecutionContext()
	execCtx.SetVariable("dynamic_value", "resolved_input")

	step := &types.Step{
		ID:   "test_step",
		Type: "call",
		Config: map[string]any{
			"script": "var_script",
			"params": map[string]any{
				"input": "${dynamic_value}",
			},
		},
	}

	result, err := executor.Execute(context.Background(), step, execCtx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Status != types.ResultStatusSuccess {
		t.Errorf("expected success status, got %s", result.Status)
	}
}
