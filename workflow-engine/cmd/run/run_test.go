package run

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExecute_NoArgs(t *testing.T) {
	// Test with no arguments - should return error
	err := Execute([]string{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "workflow file path is required")
}

func TestExecute_NonExistentFile(t *testing.T) {
	err := Execute([]string{"nonexistent.yaml"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse workflow")
}

func TestExecute_InvalidWorkflow(t *testing.T) {
	// Create a temporary invalid workflow file
	tmpDir := t.TempDir()
	workflowPath := filepath.Join(tmpDir, "invalid.yaml")
	err := os.WriteFile(workflowPath, []byte("invalid: yaml: content"), 0644)
	require.NoError(t, err)

	err = Execute([]string{workflowPath})
	assert.Error(t, err)
}

func TestPrintResults(t *testing.T) {
	result := &ExecutionResult{
		WorkflowID:       "test-workflow",
		WorkflowName:     "Test Workflow",
		Status:           "completed",
		Duration:         5 * time.Second,
		TotalVUs:         10,
		TotalIterations:  100,
		TotalRequests:    100,
		SuccessRate:      0.95,
		ErrorRate:        0.05,
		AvgDuration:      50 * time.Millisecond,
		P95Duration:      100 * time.Millisecond,
		P99Duration:      150 * time.Millisecond,
		ThresholdsPassed: 2,
		ThresholdsFailed: 1,
		Errors:           []string{"test error"},
	}

	// Just verify it doesn't panic
	printResults(result)
}

func TestWriteJSONOutput(t *testing.T) {
	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "output.json")

	result := &ExecutionResult{
		WorkflowID:       "test-workflow",
		WorkflowName:     "Test Workflow",
		Status:           "completed",
		Duration:         5 * time.Second,
		TotalVUs:         10,
		TotalIterations:  100,
		TotalRequests:    100,
		SuccessRate:      0.95,
		ErrorRate:        0.05,
		AvgDuration:      50 * time.Millisecond,
		P95Duration:      100 * time.Millisecond,
		P99Duration:      150 * time.Millisecond,
		ThresholdsPassed: 2,
		ThresholdsFailed: 1,
	}

	err := writeJSONOutput(outputPath, result)
	assert.NoError(t, err)

	// Verify file was created
	_, err = os.Stat(outputPath)
	assert.NoError(t, err)

	// Verify content
	content, err := os.ReadFile(outputPath)
	assert.NoError(t, err)
	assert.Contains(t, string(content), "test-workflow")
	assert.Contains(t, string(content), "completed")
}
