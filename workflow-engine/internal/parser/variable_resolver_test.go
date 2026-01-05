package parser

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultVariableResolver_ResolveEnv(t *testing.T) {
	// Set environment variable for test
	os.Setenv("TEST_VAR", "test_value")
	defer os.Unsetenv("TEST_VAR")

	resolver := NewDefaultVariableResolver()
	value, err := resolver.Resolve("env:TEST_VAR")

	require.NoError(t, err)
	assert.Equal(t, "test_value", value)
}

func TestDefaultVariableResolver_ResolveEnv_NotFound(t *testing.T) {
	resolver := NewDefaultVariableResolver()
	_, err := resolver.Resolve("env:NONEXISTENT_VAR_12345")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestDefaultVariableResolver_ResolveSecret(t *testing.T) {
	resolver := NewDefaultVariableResolver().WithSecrets(map[string]string{
		"api_key": "secret123",
	})

	value, err := resolver.Resolve("secret:api_key")

	require.NoError(t, err)
	assert.Equal(t, "secret123", value)
}

func TestDefaultVariableResolver_ResolveSecret_NotFound(t *testing.T) {
	resolver := NewDefaultVariableResolver()
	_, err := resolver.Resolve("secret:nonexistent")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestDefaultVariableResolver_ResolveVar(t *testing.T) {
	resolver := NewDefaultVariableResolver().WithVariables(map[string]any{
		"base_url": "http://localhost:8080",
		"timeout":  30,
	})

	value, err := resolver.Resolve("var:base_url")
	require.NoError(t, err)
	assert.Equal(t, "http://localhost:8080", value)

	value, err = resolver.Resolve("var:timeout")
	require.NoError(t, err)
	assert.Equal(t, 30, value)
}

func TestDefaultVariableResolver_ResolveShorthand(t *testing.T) {
	resolver := NewDefaultVariableResolver().WithVariables(map[string]any{
		"name": "test",
	})

	value, err := resolver.Resolve("name")

	require.NoError(t, err)
	assert.Equal(t, "test", value)
}

func TestDefaultVariableResolver_ResolveUnknownPrefix(t *testing.T) {
	resolver := NewDefaultVariableResolver()
	_, err := resolver.Resolve("unknown:value")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown variable prefix")
}

func TestDefaultVariableResolver_ResolveString(t *testing.T) {
	os.Setenv("API_HOST", "api.example.com")
	defer os.Unsetenv("API_HOST")

	resolver := NewDefaultVariableResolver().WithVariables(map[string]any{
		"version": "v1",
	})

	result, err := resolver.ResolveString("https://${env:API_HOST}/${var:version}/users")

	require.NoError(t, err)
	assert.Equal(t, "https://api.example.com/v1/users", result)
}

func TestDefaultVariableResolver_ResolveString_NoVariables(t *testing.T) {
	resolver := NewDefaultVariableResolver()
	result, err := resolver.ResolveString("plain string without variables")

	require.NoError(t, err)
	assert.Equal(t, "plain string without variables", result)
}

func TestDefaultVariableResolver_ResolveString_Error(t *testing.T) {
	resolver := NewDefaultVariableResolver()
	_, err := resolver.ResolveString("${env:NONEXISTENT_VAR_12345}")

	require.Error(t, err)
}

func TestHasVariableReferences(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"${env:VAR}", true},
		{"${secret:KEY}", true},
		{"${var:name}", true},
		{"${name}", true},
		{"plain string", false},
		{"$not_a_var", false},
		{"{not_a_var}", false},
		{"prefix ${var:x} suffix", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := HasVariableReferences(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExtractVariableReferences(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{"${env:VAR}", []string{"env:VAR"}},
		{"${a} and ${b}", []string{"a", "b"}},
		{"no variables", []string{}},
		{"${env:HOST}:${var:PORT}", []string{"env:HOST", "var:PORT"}},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := ExtractVariableReferences(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
