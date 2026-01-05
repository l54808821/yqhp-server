package slave

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExecute_Help(t *testing.T) {
	// Test help command
	err := Execute([]string{"help"})
	assert.NoError(t, err)

	err = Execute([]string{"-h"})
	assert.NoError(t, err)

	err = Execute([]string{"--help"})
	assert.NoError(t, err)
}

func TestExecute_NoArgs(t *testing.T) {
	// Test with no arguments - should print usage
	err := Execute([]string{})
	assert.NoError(t, err)
}

func TestExecute_UnknownSubcommand(t *testing.T) {
	err := Execute([]string{"unknown"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown slave subcommand")
}

func TestExecuteStatus_Default(t *testing.T) {
	// Test status command with default address
	err := executeStatus([]string{})
	assert.NoError(t, err)
}

func TestExecuteStatus_CustomAddress(t *testing.T) {
	// Test status command with custom address
	err := executeStatus([]string{"-address", "http://custom:9091"})
	assert.NoError(t, err)
}

func TestParseCommaSeparated(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{"", []string{}},
		{"a", []string{"a"}},
		{"a,b,c", []string{"a", "b", "c"}},
		{"a, b, c", []string{"a", "b", "c"}},
		{" a , b , c ", []string{"a", "b", "c"}},
	}

	for _, tt := range tests {
		result := parseCommaSeparated(tt.input)
		assert.Equal(t, tt.expected, result, "input: %q", tt.input)
	}
}

func TestParseLabels(t *testing.T) {
	tests := []struct {
		input    string
		expected map[string]string
	}{
		{"", map[string]string{}},
		{"key=value", map[string]string{"key": "value"}},
		{"a=1,b=2", map[string]string{"a": "1", "b": "2"}},
		{"region=us-east, env=prod", map[string]string{"region": "us-east", "env": "prod"}},
	}

	for _, tt := range tests {
		result := parseLabels(tt.input)
		assert.Equal(t, tt.expected, result, "input: %q", tt.input)
	}
}
