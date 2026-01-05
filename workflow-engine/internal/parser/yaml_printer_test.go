package parser

import (
	"os"
	"testing"
	"time"

	"yqhp/workflow-engine/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestYAMLPrinter_Print_BasicWorkflow(t *testing.T) {
	workflow := &types.Workflow{
		ID:          "test-workflow",
		Name:        "Test Workflow",
		Description: "A test workflow",
		Steps: []types.Step{
			{
				ID:   "step1",
				Name: "First Step",
				Type: "http",
				Config: map[string]any{
					"method": "GET",
					"url":    "http://example.com",
				},
			},
		},
	}

	printer := NewYAMLPrinter()
	data, err := printer.Print(workflow)

	require.NoError(t, err)
	assert.Contains(t, string(data), "id: test-workflow")
	assert.Contains(t, string(data), "name: Test Workflow")
	assert.Contains(t, string(data), "description: A test workflow")
	assert.Contains(t, string(data), "step1")
}

func TestYAMLPrinter_Print_WithOptions(t *testing.T) {
	workflow := &types.Workflow{
		ID:   "load-test",
		Name: "Load Test",
		Steps: []types.Step{
			{
				ID:     "request",
				Name:   "HTTP Request",
				Type:   "http",
				Config: map[string]any{},
			},
		},
		Options: types.ExecutionOptions{
			VUs:           100,
			Duration:      5 * time.Minute,
			ExecutionMode: types.ModeRampingVUs,
		},
	}

	printer := NewYAMLPrinter()
	data, err := printer.Print(workflow)

	require.NoError(t, err)
	assert.Contains(t, string(data), "vus: 100")
	assert.Contains(t, string(data), "mode: ramping-vus")
}

func TestYAMLPrinter_Print_WithHooks(t *testing.T) {
	workflow := &types.Workflow{
		ID:   "hook-workflow",
		Name: "Workflow with Hooks",
		PreHook: &types.Hook{
			Type: "script",
			Config: map[string]any{
				"inline": "echo pre",
			},
		},
		PostHook: &types.Hook{
			Type: "script",
			Config: map[string]any{
				"inline": "echo post",
			},
		},
		Steps: []types.Step{
			{
				ID:     "step1",
				Name:   "Step 1",
				Type:   "http",
				Config: map[string]any{},
			},
		},
	}

	printer := NewYAMLPrinter()
	data, err := printer.Print(workflow)

	require.NoError(t, err)
	assert.Contains(t, string(data), "pre_hook:")
	assert.Contains(t, string(data), "post_hook:")
}

func TestYAMLPrinter_Print_WithCondition(t *testing.T) {
	workflow := &types.Workflow{
		ID:   "conditional-workflow",
		Name: "Conditional Workflow",
		Steps: []types.Step{
			{
				ID:     "check",
				Name:   "Check",
				Type:   "condition",
				Config: map[string]any{},
				Condition: &types.Condition{
					Expression: "${status} == 200",
					Then: []types.Step{
						{
							ID:     "success",
							Name:   "Success",
							Type:   "script",
							Config: map[string]any{},
						},
					},
					Else: []types.Step{
						{
							ID:     "failure",
							Name:   "Failure",
							Type:   "script",
							Config: map[string]any{},
						},
					},
				},
			},
		},
	}

	printer := NewYAMLPrinter()
	data, err := printer.Print(workflow)

	require.NoError(t, err)
	assert.Contains(t, string(data), "condition:")
	assert.Contains(t, string(data), "expression:")
	assert.Contains(t, string(data), "then:")
	assert.Contains(t, string(data), "else:")
}

func TestYAMLPrinter_PrintToFile(t *testing.T) {
	workflow := &types.Workflow{
		ID:   "file-workflow",
		Name: "File Workflow",
		Steps: []types.Step{
			{
				ID:     "step1",
				Name:   "Step 1",
				Type:   "http",
				Config: map[string]any{},
			},
		},
	}

	tmpFile, err := os.CreateTemp("", "workflow-*.yaml")
	require.NoError(t, err)
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	printer := NewYAMLPrinter()
	err = printer.PrintToFile(workflow, tmpFile.Name())

	require.NoError(t, err)

	// Verify file contents
	data, err := os.ReadFile(tmpFile.Name())
	require.NoError(t, err)
	assert.Contains(t, string(data), "id: file-workflow")
}

func TestYAMLPrinter_PrintPretty(t *testing.T) {
	workflow := &types.Workflow{
		ID:   "pretty-workflow",
		Name: "Pretty Workflow",
		Steps: []types.Step{
			{
				ID:     "step1",
				Name:   "Step 1",
				Type:   "http",
				Config: map[string]any{},
			},
		},
	}

	printer := NewYAMLPrinter()
	result, err := printer.PrintPretty(workflow)

	require.NoError(t, err)
	assert.Contains(t, result, "id: pretty-workflow")
}

func TestYAMLPrinter_WithIndent(t *testing.T) {
	workflow := &types.Workflow{
		ID:   "indent-workflow",
		Name: "Indent Workflow",
		Steps: []types.Step{
			{
				ID:     "step1",
				Name:   "Step 1",
				Type:   "http",
				Config: map[string]any{},
			},
		},
	}

	printer := NewYAMLPrinter().WithIndent(4)
	data, err := printer.Print(workflow)

	require.NoError(t, err)
	// The output should have 4-space indentation
	assert.NotEmpty(t, data)
}
