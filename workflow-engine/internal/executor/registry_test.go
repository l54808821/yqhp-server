package executor

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grafana/k6/workflow-engine/pkg/types"
)

// mockExecutor is a test executor implementation.
type mockExecutor struct {
	*BaseExecutor
	initCalled    bool
	cleanupCalled bool
	initError     error
	cleanupError  error
}

func newMockExecutor(execType string) *mockExecutor {
	return &mockExecutor{
		BaseExecutor: NewBaseExecutor(execType),
	}
}

func (m *mockExecutor) Init(ctx context.Context, config map[string]any) error {
	m.initCalled = true
	if m.initError != nil {
		return m.initError
	}
	return m.BaseExecutor.Init(ctx, config)
}

func (m *mockExecutor) Execute(ctx context.Context, step *types.Step, execCtx *ExecutionContext) (*types.StepResult, error) {
	return CreateSuccessResult(step.ID, time.Now(), nil), nil
}

func (m *mockExecutor) Cleanup(ctx context.Context) error {
	m.cleanupCalled = true
	return m.cleanupError
}

func TestNewRegistry(t *testing.T) {
	registry := NewRegistry()

	assert.NotNil(t, registry)
	assert.Equal(t, 0, registry.Count())
}

func TestRegistry_Register(t *testing.T) {
	registry := NewRegistry()
	executor := newMockExecutor("http")

	err := registry.Register(executor)
	assert.NoError(t, err)
	assert.Equal(t, 1, registry.Count())
	assert.True(t, registry.Has("http"))
}

func TestRegistry_Register_Duplicate(t *testing.T) {
	registry := NewRegistry()
	executor1 := newMockExecutor("http")
	executor2 := newMockExecutor("http")

	err := registry.Register(executor1)
	assert.NoError(t, err)

	err = registry.Register(executor2)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already registered")
}

func TestRegistry_Register_NilExecutor(t *testing.T) {
	registry := NewRegistry()

	err := registry.Register(nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "nil executor")
}

func TestRegistry_MustRegister_Panics(t *testing.T) {
	registry := NewRegistry()
	executor1 := newMockExecutor("http")
	executor2 := newMockExecutor("http")

	registry.MustRegister(executor1)

	assert.Panics(t, func() {
		registry.MustRegister(executor2)
	})
}

func TestRegistry_Unregister(t *testing.T) {
	registry := NewRegistry()
	executor := newMockExecutor("http")

	registry.MustRegister(executor)
	assert.True(t, registry.Has("http"))

	registry.Unregister("http")
	assert.False(t, registry.Has("http"))
}

func TestRegistry_Get(t *testing.T) {
	registry := NewRegistry()
	executor := newMockExecutor("http")
	registry.MustRegister(executor)

	retrieved := registry.Get("http")
	assert.Equal(t, executor, retrieved)

	notFound := registry.Get("nonexistent")
	assert.Nil(t, notFound)
}

func TestRegistry_GetOrError(t *testing.T) {
	registry := NewRegistry()
	executor := newMockExecutor("http")
	registry.MustRegister(executor)

	retrieved, err := registry.GetOrError("http")
	assert.NoError(t, err)
	assert.Equal(t, executor, retrieved)

	_, err = registry.GetOrError("nonexistent")
	assert.Error(t, err)
	assert.True(t, IsNotFoundError(err))
}

func TestRegistry_Types(t *testing.T) {
	registry := NewRegistry()
	registry.MustRegister(newMockExecutor("http"))
	registry.MustRegister(newMockExecutor("script"))
	registry.MustRegister(newMockExecutor("grpc"))

	types := registry.Types()
	assert.Len(t, types, 3)
	assert.Contains(t, types, "http")
	assert.Contains(t, types, "script")
	assert.Contains(t, types, "grpc")
}

func TestRegistry_InitAll(t *testing.T) {
	registry := NewRegistry()
	httpExec := newMockExecutor("http")
	scriptExec := newMockExecutor("script")

	registry.MustRegister(httpExec)
	registry.MustRegister(scriptExec)

	configs := map[string]map[string]any{
		"http":   {"timeout": 30},
		"script": {"shell": "/bin/bash"},
	}

	err := registry.InitAll(context.Background(), configs)
	require.NoError(t, err)

	assert.True(t, httpExec.initCalled)
	assert.True(t, scriptExec.initCalled)
}

func TestRegistry_CleanupAll(t *testing.T) {
	registry := NewRegistry()
	httpExec := newMockExecutor("http")
	scriptExec := newMockExecutor("script")

	registry.MustRegister(httpExec)
	registry.MustRegister(scriptExec)

	err := registry.CleanupAll(context.Background())
	require.NoError(t, err)

	assert.True(t, httpExec.cleanupCalled)
	assert.True(t, scriptExec.cleanupCalled)
}

func TestDefaultRegistry(t *testing.T) {
	// Reset default registry for test isolation
	originalRegistry := DefaultRegistry
	DefaultRegistry = NewRegistry()
	defer func() { DefaultRegistry = originalRegistry }()

	executor := newMockExecutor("test")

	err := Register(executor)
	assert.NoError(t, err)

	retrieved := Get("test")
	assert.Equal(t, executor, retrieved)

	retrieved2, err := GetOrError("test")
	assert.NoError(t, err)
	assert.Equal(t, executor, retrieved2)
}
