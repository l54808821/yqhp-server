package parser

import (
	"os"
	"testing"
	"time"

	"yqhp/workflow-engine/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestYAMLParser_Parse_ValidWorkflow(t *testing.T) {
	yamlContent := `
id: test-workflow
name: Test Workflow
description: A test workflow
variables:
  base_url: http://localhost:8080
  timeout: 30
steps:
  - id: step1
    name: First Step
    type: http
    config:
      method: GET
      url: ${var:base_url}/api/test
    timeout: 10s
`
	parser := NewYAMLParser()
	workflow, err := parser.Parse([]byte(yamlContent))

	require.NoError(t, err)
	assert.Equal(t, "test-workflow", workflow.ID)
	assert.Equal(t, "Test Workflow", workflow.Name)
	assert.Equal(t, "A test workflow", workflow.Description)
	assert.Len(t, workflow.Steps, 1)
	assert.Equal(t, "step1", workflow.Steps[0].ID)
	assert.Equal(t, "http", workflow.Steps[0].Type)
	assert.Equal(t, 10*time.Second, workflow.Steps[0].Timeout)
}

func TestYAMLParser_Parse_WithCondition(t *testing.T) {
	yamlContent := `
id: conditional-workflow
name: Conditional Workflow
steps:
  - id: check-status
    name: Check Status
    type: condition
    config: {}
    condition:
      expression: "${status} == 200"
      then:
        - id: success-step
          name: Success
          type: script
          config:
            inline: echo "success"
      else:
        - id: failure-step
          name: Failure
          type: script
          config:
            inline: echo "failure"
`
	parser := NewYAMLParser()
	workflow, err := parser.Parse([]byte(yamlContent))

	require.NoError(t, err)
	assert.Len(t, workflow.Steps, 1)
	assert.NotNil(t, workflow.Steps[0].Condition)
	assert.Equal(t, "${status} == 200", workflow.Steps[0].Condition.Expression)
	assert.Len(t, workflow.Steps[0].Condition.Then, 1)
	assert.Len(t, workflow.Steps[0].Condition.Else, 1)
}

func TestYAMLParser_Parse_WithHooks(t *testing.T) {
	yamlContent := `
id: hook-workflow
name: Workflow with Hooks
pre_hook:
  type: script
  config:
    inline: echo "pre-workflow"
post_hook:
  type: script
  config:
    inline: echo "post-workflow"
steps:
  - id: main-step
    name: Main Step
    type: http
    config:
      method: GET
      url: http://example.com
    pre_hook:
      type: script
      config:
        inline: echo "pre-step"
    post_hook:
      type: script
      config:
        inline: echo "post-step"
`
	parser := NewYAMLParser()
	workflow, err := parser.Parse([]byte(yamlContent))

	require.NoError(t, err)
	assert.NotNil(t, workflow.PreHook)
	assert.Equal(t, "script", workflow.PreHook.Type)
	assert.NotNil(t, workflow.PostHook)
	assert.Equal(t, "script", workflow.PostHook.Type)
	assert.NotNil(t, workflow.Steps[0].PreHook)
	assert.NotNil(t, workflow.Steps[0].PostHook)
}

func TestYAMLParser_Parse_WithExecutionOptions(t *testing.T) {
	yamlContent := `
id: load-test
name: Load Test
steps:
  - id: request
    name: HTTP Request
    type: http
    config:
      method: GET
      url: http://example.com
options:
  vus: 100
  duration: 5m
  mode: ramping-vus
  stages:
    - duration: 1m
      target: 50
    - duration: 3m
      target: 100
    - duration: 1m
      target: 0
  thresholds:
    - metric: http_req_duration
      condition: p95 < 500
`
	parser := NewYAMLParser()
	workflow, err := parser.Parse([]byte(yamlContent))

	require.NoError(t, err)
	assert.Equal(t, 100, workflow.Options.VUs)
	assert.Equal(t, 5*time.Minute, workflow.Options.Duration)
	assert.Equal(t, types.ModeRampingVUs, workflow.Options.ExecutionMode)
	assert.Len(t, workflow.Options.Stages, 3)
	assert.Len(t, workflow.Options.Thresholds, 1)
}

func TestYAMLParser_Parse_MissingID(t *testing.T) {
	yamlContent := `
name: Test Workflow
steps:
  - id: step1
    name: Step 1
    type: http
    config: {}
`
	parser := NewYAMLParser()
	_, err := parser.Parse([]byte(yamlContent))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "workflow ID is required")
}

func TestYAMLParser_Parse_MissingName(t *testing.T) {
	yamlContent := `
id: test-workflow
steps:
  - id: step1
    name: Step 1
    type: http
    config: {}
`
	parser := NewYAMLParser()
	_, err := parser.Parse([]byte(yamlContent))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "workflow name is required")
}

func TestYAMLParser_Parse_NoSteps(t *testing.T) {
	yamlContent := `
id: test-workflow
name: Test Workflow
steps: []
`
	parser := NewYAMLParser()
	_, err := parser.Parse([]byte(yamlContent))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "at least one step")
}

func TestYAMLParser_Parse_DuplicateStepID(t *testing.T) {
	yamlContent := `
id: test-workflow
name: Test Workflow
steps:
  - id: step1
    name: Step 1
    type: http
    config: {}
  - id: step1
    name: Step 2
    type: http
    config: {}
`
	parser := NewYAMLParser()
	_, err := parser.Parse([]byte(yamlContent))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate step ID")
}

func TestYAMLParser_Parse_InvalidStepType(t *testing.T) {
	yamlContent := `
id: test-workflow
name: Test Workflow
steps:
  - id: step1
    name: Step 1
    type: invalid_type
    config: {}
`
	parser := NewYAMLParser()
	_, err := parser.Parse([]byte(yamlContent))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid step type")
}

func TestYAMLParser_Parse_InvalidYAML(t *testing.T) {
	yamlContent := `
id: test-workflow
name: Test Workflow
steps:
  - id: step1
    name: Step 1
    type: http
    config:
      invalid yaml here
        - not valid
`
	parser := NewYAMLParser()
	_, err := parser.Parse([]byte(yamlContent))

	require.Error(t, err)
	_, ok := err.(*ParseError)
	assert.True(t, ok, "expected ParseError")
}

func TestYAMLParser_ParseFile(t *testing.T) {
	// Create a temporary file
	content := `
id: file-workflow
name: File Workflow
steps:
  - id: step1
    name: Step 1
    type: http
    config:
      method: GET
      url: http://example.com
`
	tmpFile, err := os.CreateTemp("", "workflow-*.yaml")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	_, err = tmpFile.WriteString(content)
	require.NoError(t, err)
	tmpFile.Close()

	parser := NewYAMLParser()
	workflow, err := parser.ParseFile(tmpFile.Name())

	require.NoError(t, err)
	assert.Equal(t, "file-workflow", workflow.ID)
}

func TestYAMLParser_ParseFile_NotFound(t *testing.T) {
	parser := NewYAMLParser()
	_, err := parser.ParseFile("/nonexistent/path/workflow.yaml")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read file")
}
