package execution

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"yqhp/workflow-engine/pkg/types"
)

func TestNewRegistry(t *testing.T) {
	r := NewRegistry()
	assert.NotNil(t, r)

	// Should have all default modes registered
	modes := r.List()
	assert.Contains(t, modes, types.ModeConstantVUs)
	assert.Contains(t, modes, types.ModeRampingVUs)
	assert.Contains(t, modes, types.ModeConstantArrivalRate)
	assert.Contains(t, modes, types.ModeRampingArrivalRate)
	assert.Contains(t, modes, types.ModePerVUIterations)
	assert.Contains(t, modes, types.ModeSharedIterations)
	assert.Contains(t, modes, types.ModeExternally)
}

func TestRegistry_Get(t *testing.T) {
	r := NewRegistry()

	tests := []struct {
		mode     types.ExecutionMode
		expected types.ExecutionMode
	}{
		{types.ModeConstantVUs, types.ModeConstantVUs},
		{types.ModeRampingVUs, types.ModeRampingVUs},
		{types.ModeConstantArrivalRate, types.ModeConstantArrivalRate},
		{types.ModeRampingArrivalRate, types.ModeRampingArrivalRate},
		{types.ModePerVUIterations, types.ModePerVUIterations},
		{types.ModeSharedIterations, types.ModeSharedIterations},
		{types.ModeExternally, types.ModeExternally},
	}

	for _, tt := range tests {
		t.Run(string(tt.mode), func(t *testing.T) {
			mode, err := r.Get(tt.mode)
			require.NoError(t, err)
			assert.NotNil(t, mode)
			assert.Equal(t, tt.expected, mode.Name())
		})
	}
}

func TestRegistry_Get_UnknownMode(t *testing.T) {
	r := NewRegistry()

	mode, err := r.Get("unknown-mode")
	assert.Error(t, err)
	assert.Nil(t, mode)
}

func TestRegistry_GetOrDefault(t *testing.T) {
	r := NewRegistry()

	// Empty mode should return constant-vus
	mode, err := r.GetOrDefault("")
	require.NoError(t, err)
	assert.Equal(t, types.ModeConstantVUs, mode.Name())

	// Specified mode should return that mode
	mode, err = r.GetOrDefault(types.ModeRampingVUs)
	require.NoError(t, err)
	assert.Equal(t, types.ModeRampingVUs, mode.Name())
}

func TestRegistry_Register_Custom(t *testing.T) {
	r := NewRegistry()

	// Register a custom mode
	customMode := types.ExecutionMode("custom-mode")
	r.Register(customMode, func() Mode {
		return NewConstantVUsMode() // Just reuse for testing
	})

	// Should be able to get it
	mode, err := r.Get(customMode)
	require.NoError(t, err)
	assert.NotNil(t, mode)
}

func TestDefaultRegistry(t *testing.T) {
	// Test the default registry
	mode, err := GetMode(types.ModeConstantVUs)
	require.NoError(t, err)
	assert.NotNil(t, mode)
	assert.Equal(t, types.ModeConstantVUs, mode.Name())
}

func TestGetModeOrDefault(t *testing.T) {
	// Empty mode should return constant-vus
	mode, err := GetModeOrDefault("")
	require.NoError(t, err)
	assert.Equal(t, types.ModeConstantVUs, mode.Name())
}
