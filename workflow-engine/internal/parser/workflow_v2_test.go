package parser

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseString_BasicWorkflow(t *testing.T) {
	yaml := `
id: test-workflow
name: Test Workflow
description: A test workflow
version: "1.0"
variables:
  base_url: "https://api.example.com"
steps:
  - id: step1
    name: First Step
    type: http
    config:
      method: GET
      url: "${base_url}/users"
`

	workflow, err := ParseString(yaml)
	require.NoError(t, err)

	assert.Equal(t, "test-workflow", workflow.ID)
	assert.Equal(t, "Test Workflow", workflow.Name)
	assert.Equal(t, "A test workflow", workflow.Description)
	assert.Equal(t, "1.0", workflow.Version)
	assert.Equal(t, "https://api.example.com", workflow.Variables["base_url"])
	assert.Len(t, workflow.Steps, 1)
	assert.Equal(t, "step1", workflow.Steps[0].ID)
	assert.Equal(t, "http", workflow.Steps[0].Type)
}

func TestParseString_WithScripts(t *testing.T) {
	yaml := `
id: test-workflow
name: Test Workflow
scripts:
  setup:
    name: setup
    description: Setup script
    params:
      - name: value
        type: number
        default: 10
    steps:
      - id: step1
        type: script
        config:
          script: "return value * 2"
    returns:
      - name: result
        value: "${_result}"
steps:
  - id: step1
    type: call
    script: setup
    args:
      value: 5
`

	workflow, err := ParseString(yaml)
	require.NoError(t, err)

	assert.Len(t, workflow.Scripts, 1)
	assert.NotNil(t, workflow.Scripts["setup"])
	assert.Equal(t, "Setup script", workflow.Scripts["setup"].Description)
}

func TestParseString_WithConfig(t *testing.T) {
	yaml := `
id: test-workflow
name: Test Workflow
config:
  http:
    default_domain: api
    domains:
      api:
        base_url: "https://api.example.com"
    timeout:
      connect: 5s
      total: 30s
    headers:
      Authorization: "Bearer token"
steps:
  - id: step1
    type: http
    config:
      method: GET
      url: "/users"
`

	workflow, err := ParseString(yaml)
	require.NoError(t, err)

	require.NotNil(t, workflow.Config)
	require.NotNil(t, workflow.Config.HTTP)
	assert.Equal(t, "api", workflow.Config.HTTP.DefaultDomain)
	assert.Equal(t, "https://api.example.com", workflow.Config.HTTP.Domains["api"].BaseURL)
	assert.Equal(t, 5*time.Second, workflow.Config.HTTP.Timeout.Connect)
	assert.Equal(t, "Bearer token", workflow.Config.HTTP.Headers["Authorization"])
}

func TestParseString_WithPrePostScripts(t *testing.T) {
	yaml := `
id: test-workflow
name: Test Workflow
steps:
  - id: step1
    type: http
    config:
      method: GET
      url: "/users"
    pre_scripts:
      - name: setup
        script: "ctx.SetVariable('start_time', time.Now())"
    post_scripts:
      - name: cleanup
        script: "ctx.SetVariable('end_time', time.Now())"
        on_error: continue
`

	workflow, err := ParseString(yaml)
	require.NoError(t, err)

	step := workflow.Steps[0]
	assert.Len(t, step.PreScripts, 1)
	assert.Len(t, step.PostScripts, 1)
	assert.Equal(t, "setup", step.PreScripts[0].Name)
	assert.Equal(t, "cleanup", step.PostScripts[0].Name)
}

func TestParseString_IfStep(t *testing.T) {
	yaml := `
id: test-workflow
name: Test Workflow
steps:
  - id: check_status
    type: if
    condition: "${response.status_code} == 200"
    then:
      - id: success_step
        type: script
        config:
          script: "log('success')"
    else_if:
      - condition: "${response.status_code} == 404"
        steps:
          - id: not_found_step
            type: script
            config:
              script: "log('not found')"
    else:
      - id: error_step
        type: script
        config:
          script: "log('error')"
`

	workflow, err := ParseString(yaml)
	require.NoError(t, err)

	step := workflow.Steps[0]
	assert.Equal(t, "if", step.Type)
	assert.Equal(t, "${response.status_code} == 200", step.Condition)
	assert.Len(t, step.Then, 1)
	assert.Len(t, step.ElseIf, 1)
	assert.Len(t, step.Else, 1)
}

func TestParseString_ForEachStep(t *testing.T) {
	yaml := `
id: test-workflow
name: Test Workflow
steps:
  - id: process_items
    type: foreach
    items: "${users}"
    item_var: user
    index_var: idx
    steps:
      - id: process_user
        type: http
        config:
          method: GET
          url: "/users/${user.id}"
`

	workflow, err := ParseString(yaml)
	require.NoError(t, err)

	step := workflow.Steps[0]
	assert.Equal(t, "foreach", step.Type)
	assert.Equal(t, "${users}", step.Items)
	assert.Equal(t, "user", step.ItemVar)
	assert.Equal(t, "idx", step.IndexVar)
	assert.Len(t, step.Steps, 1)
}

func TestParseString_ParallelStep(t *testing.T) {
	yaml := `
id: test-workflow
name: Test Workflow
steps:
  - id: parallel_requests
    type: parallel
    max_concurrent: 5
    fail_fast: true
    steps:
      - id: req1
        type: http
        config:
          method: GET
          url: "/api/1"
      - id: req2
        type: http
        config:
          method: GET
          url: "/api/2"
`

	workflow, err := ParseString(yaml)
	require.NoError(t, err)

	step := workflow.Steps[0]
	assert.Equal(t, "parallel", step.Type)
	assert.Equal(t, 5, step.MaxConcurrent)
	assert.True(t, step.FailFast)
	assert.Len(t, step.Steps, 2)
}

func TestParseString_RetryStep(t *testing.T) {
	yaml := `
id: test-workflow
name: Test Workflow
steps:
  - id: retry_request
    type: retry
    retry:
      max_attempts: 3
      delay: 1s
      backoff: exponential
      max_delay: 10s
    steps:
      - id: http_request
        type: http
        config:
          method: GET
          url: "/api/unstable"
`

	workflow, err := ParseString(yaml)
	require.NoError(t, err)

	step := workflow.Steps[0]
	assert.Equal(t, "retry", step.Type)
	require.NotNil(t, step.Retry)
	assert.Equal(t, 3, step.Retry.MaxAttempts)
	assert.Equal(t, time.Second, step.Retry.Delay)
	assert.Equal(t, "exponential", step.Retry.Backoff)
	assert.Equal(t, 10*time.Second, step.Retry.MaxDelay)
}

func TestParseString_ValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		wantErr string
	}{
		{
			name:    "missing ID",
			yaml:    "name: Test\nsteps:\n  - id: s1\n    type: http",
			wantErr: "workflow ID is required",
		},
		{
			name:    "no steps",
			yaml:    "id: test\nname: Test\nsteps: []",
			wantErr: "workflow must have at least one step",
		},
		{
			name:    "missing step ID",
			yaml:    "id: test\nname: Test\nsteps:\n  - type: http",
			wantErr: "step ID is required",
		},
		{
			name:    "missing step type",
			yaml:    "id: test\nname: Test\nsteps:\n  - id: s1",
			wantErr: "step type is required",
		},
		{
			name:    "invalid step type",
			yaml:    "id: test\nname: Test\nsteps:\n  - id: s1\n    type: invalid",
			wantErr: "invalid step type",
		},
		{
			name:    "duplicate step ID",
			yaml:    "id: test\nname: Test\nsteps:\n  - id: s1\n    type: http\n  - id: s1\n    type: http",
			wantErr: "duplicate step ID",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseString(tt.yaml)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestParseString_ExecutionOptions(t *testing.T) {
	yaml := `
id: test-workflow
name: Test Workflow
options:
  concurrency: 10
  iterations: 100
  duration: 5m
  timeout: 10m
  failure_threshold: 0.1
steps:
  - id: step1
    type: http
    config:
      method: GET
      url: "/api"
`

	workflow, err := ParseString(yaml)
	require.NoError(t, err)

	require.NotNil(t, workflow.Options)
	assert.Equal(t, 10, workflow.Options.Concurrency)
	assert.Equal(t, 100, workflow.Options.Iterations)
	assert.Equal(t, 5*time.Minute, workflow.Options.Duration)
	assert.Equal(t, 10*time.Minute, workflow.Options.Timeout)
	assert.Equal(t, 0.1, workflow.Options.FailureThreshold)
}

func TestParseString_AllStepTypes(t *testing.T) {
	stepTypes := []string{
		"http", "script", "call", "socket", "mq", "db",
		"if", "while", "for", "foreach", "parallel",
		"sleep", "wait_until", "retry", "break", "continue",
	}

	for _, stepType := range stepTypes {
		t.Run(stepType, func(t *testing.T) {
			yaml := `
id: test-workflow
name: Test Workflow
steps:
  - id: step1
    type: ` + stepType

			workflow, err := ParseString(yaml)
			require.NoError(t, err)
			assert.Equal(t, stepType, workflow.Steps[0].Type)
		})
	}
}
