package executor

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grafana/k6/workflow-engine/pkg/types"
)

func TestScriptExecutor_Type(t *testing.T) {
	executor := NewScriptExecutor()
	assert.Equal(t, ScriptExecutorType, executor.Type())
}

func TestScriptExecutor_Init(t *testing.T) {
	executor := NewScriptExecutor()

	err := executor.Init(context.Background(), nil)

	assert.NoError(t, err)
	assert.NotEmpty(t, executor.shell)
	assert.NotEmpty(t, executor.shellArgs)
}

func TestScriptExecutor_Init_CustomShell(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping on Windows")
	}

	executor := NewScriptExecutor()

	err := executor.Init(context.Background(), map[string]any{
		"shell": "/bin/bash",
	})

	assert.NoError(t, err)
	assert.Equal(t, "/bin/bash", executor.shell)
	assert.Contains(t, executor.shellArgs, "-c")
}

func TestScriptExecutor_Execute_InlineScript(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping on Windows")
	}

	executor := NewScriptExecutor()
	require.NoError(t, executor.Init(context.Background(), nil))

	step := &types.Step{
		ID:   "test-script",
		Name: "Test Script",
		Type: ScriptExecutorType,
		Config: map[string]any{
			"inline": "echo 'Hello, World!'",
		},
	}

	result, err := executor.Execute(context.Background(), step, NewExecutionContext())

	require.NoError(t, err)
	assert.Equal(t, types.ResultStatusSuccess, result.Status)

	output, ok := result.Output.(*ScriptOutput)
	require.True(t, ok)
	assert.Contains(t, output.Stdout, "Hello, World!")
	assert.Equal(t, 0, output.ExitCode)
}

func TestScriptExecutor_Execute_WithVariables(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping on Windows")
	}

	executor := NewScriptExecutor()
	require.NoError(t, executor.Init(context.Background(), nil))

	execCtx := NewExecutionContext()
	execCtx.SetVariable("name", "TestUser")

	step := &types.Step{
		ID:   "test-vars",
		Name: "Test Variables",
		Type: ScriptExecutorType,
		Config: map[string]any{
			"inline": "echo 'Hello, ${name}!'",
		},
	}

	result, err := executor.Execute(context.Background(), step, execCtx)

	require.NoError(t, err)
	assert.Equal(t, types.ResultStatusSuccess, result.Status)

	output, ok := result.Output.(*ScriptOutput)
	require.True(t, ok)
	assert.Contains(t, output.Stdout, "Hello, TestUser!")
}

func TestScriptExecutor_Execute_WithEnvVars(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping on Windows")
	}

	executor := NewScriptExecutor()
	require.NoError(t, executor.Init(context.Background(), nil))

	step := &types.Step{
		ID:   "test-env",
		Name: "Test Env",
		Type: ScriptExecutorType,
		Config: map[string]any{
			"inline": "echo $MY_VAR",
			"env": map[string]any{
				"MY_VAR": "custom_value",
			},
		},
	}

	result, err := executor.Execute(context.Background(), step, NewExecutionContext())

	require.NoError(t, err)
	assert.Equal(t, types.ResultStatusSuccess, result.Status)

	output, ok := result.Output.(*ScriptOutput)
	require.True(t, ok)
	assert.Contains(t, output.Stdout, "custom_value")
}

func TestScriptExecutor_Execute_ScriptFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping on Windows")
	}

	// Create temporary script file
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "test.sh")
	err := os.WriteFile(scriptPath, []byte("#!/bin/sh\necho 'From file'"), 0755)
	require.NoError(t, err)

	executor := NewScriptExecutor()
	require.NoError(t, executor.Init(context.Background(), nil))

	step := &types.Step{
		ID:   "test-file",
		Name: "Test File",
		Type: ScriptExecutorType,
		Config: map[string]any{
			"file": scriptPath,
		},
	}

	result, err := executor.Execute(context.Background(), step, NewExecutionContext())

	require.NoError(t, err)
	assert.Equal(t, types.ResultStatusSuccess, result.Status)

	output, ok := result.Output.(*ScriptOutput)
	require.True(t, ok)
	assert.Contains(t, output.Stdout, "From file")
}

func TestScriptExecutor_Execute_Timeout(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping on Windows")
	}

	executor := NewScriptExecutor()
	require.NoError(t, executor.Init(context.Background(), nil))

	step := &types.Step{
		ID:      "test-timeout",
		Name:    "Test Timeout",
		Type:    ScriptExecutorType,
		Timeout: 100 * time.Millisecond,
		Config: map[string]any{
			"inline": "sleep 5",
		},
	}

	result, err := executor.Execute(context.Background(), step, NewExecutionContext())

	require.NoError(t, err)
	assert.Equal(t, types.ResultStatusTimeout, result.Status)
}

func TestScriptExecutor_Execute_NonZeroExit(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping on Windows")
	}

	executor := NewScriptExecutor()
	require.NoError(t, executor.Init(context.Background(), nil))

	step := &types.Step{
		ID:   "test-exit",
		Name: "Test Exit",
		Type: ScriptExecutorType,
		Config: map[string]any{
			"inline": "exit 42",
		},
	}

	result, err := executor.Execute(context.Background(), step, NewExecutionContext())

	require.NoError(t, err)
	assert.Equal(t, types.ResultStatusFailed, result.Status)

	output, ok := result.Output.(*ScriptOutput)
	require.True(t, ok)
	assert.Equal(t, 42, output.ExitCode)
}

func TestScriptExecutor_Execute_Stderr(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping on Windows")
	}

	executor := NewScriptExecutor()
	require.NoError(t, executor.Init(context.Background(), nil))

	step := &types.Step{
		ID:   "test-stderr",
		Name: "Test Stderr",
		Type: ScriptExecutorType,
		Config: map[string]any{
			"inline": "echo 'error message' >&2",
		},
	}

	result, err := executor.Execute(context.Background(), step, NewExecutionContext())

	require.NoError(t, err)
	assert.Equal(t, types.ResultStatusSuccess, result.Status)

	output, ok := result.Output.(*ScriptOutput)
	require.True(t, ok)
	assert.Contains(t, output.Stderr, "error message")
}

func TestScriptExecutor_Execute_MissingScript(t *testing.T) {
	executor := NewScriptExecutor()
	require.NoError(t, executor.Init(context.Background(), nil))

	step := &types.Step{
		ID:     "test-no-script",
		Name:   "Test No Script",
		Type:   ScriptExecutorType,
		Config: map[string]any{},
	}

	result, err := executor.Execute(context.Background(), step, NewExecutionContext())

	require.NoError(t, err)
	assert.Equal(t, types.ResultStatusFailed, result.Status)
	assert.NotNil(t, result.Error)
}

func TestScriptExecutor_Execute_MissingFile(t *testing.T) {
	executor := NewScriptExecutor()
	require.NoError(t, executor.Init(context.Background(), nil))

	step := &types.Step{
		ID:   "test-missing-file",
		Name: "Test Missing File",
		Type: ScriptExecutorType,
		Config: map[string]any{
			"file": "/nonexistent/script.sh",
		},
	}

	result, err := executor.Execute(context.Background(), step, NewExecutionContext())

	require.NoError(t, err)
	assert.Equal(t, types.ResultStatusFailed, result.Status)
}

func TestScriptExecutor_Cleanup(t *testing.T) {
	executor := NewScriptExecutor()
	require.NoError(t, executor.Init(context.Background(), nil))

	err := executor.Cleanup(context.Background())
	assert.NoError(t, err)
}
