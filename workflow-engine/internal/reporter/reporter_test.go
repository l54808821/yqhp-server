package reporter

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"yqhp/workflow-engine/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockReporter is a test implementation of Reporter.
type mockReporter struct {
	name        string
	initCalled  atomic.Bool
	reportCount atomic.Int32
	flushCalled atomic.Bool
	closeCalled atomic.Bool
	initErr     error
	reportErr   error
	flushErr    error
	closeErr    error
	lastMetrics *types.Metrics
}

func newMockReporter(name string) *mockReporter {
	return &mockReporter{name: name}
}

func (m *mockReporter) Name() string {
	return m.name
}

func (m *mockReporter) Init(ctx context.Context, config map[string]any) error {
	m.initCalled.Store(true)
	return m.initErr
}

func (m *mockReporter) Report(ctx context.Context, metrics *types.Metrics) error {
	m.reportCount.Add(1)
	m.lastMetrics = metrics
	return m.reportErr
}

func (m *mockReporter) Flush(ctx context.Context) error {
	m.flushCalled.Store(true)
	return m.flushErr
}

func (m *mockReporter) Close(ctx context.Context) error {
	m.closeCalled.Store(true)
	return m.closeErr
}

func TestRegistry_Register(t *testing.T) {
	registry := NewRegistry()

	factory := func(config map[string]any) (Reporter, error) {
		return newMockReporter("test"), nil
	}

	// Register should succeed
	err := registry.Register(ReporterTypeConsole, factory)
	assert.NoError(t, err)

	// Duplicate registration should fail
	err = registry.Register(ReporterTypeConsole, factory)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already registered")
}

func TestRegistry_Create(t *testing.T) {
	registry := NewRegistry()

	factory := func(config map[string]any) (Reporter, error) {
		return newMockReporter("test"), nil
	}

	err := registry.Register(ReporterTypeConsole, factory)
	require.NoError(t, err)

	// Create should succeed for registered type
	reporter, err := registry.Create(ReporterTypeConsole, nil)
	assert.NoError(t, err)
	assert.NotNil(t, reporter)
	assert.Equal(t, "test", reporter.Name())

	// Create should fail for unregistered type
	_, err = registry.Create(ReporterTypeJSON, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown reporter type")
}

func TestRegistry_Unregister(t *testing.T) {
	registry := NewRegistry()

	factory := func(config map[string]any) (Reporter, error) {
		return newMockReporter("test"), nil
	}

	err := registry.Register(ReporterTypeConsole, factory)
	require.NoError(t, err)

	assert.True(t, registry.HasType(ReporterTypeConsole))

	registry.Unregister(ReporterTypeConsole)

	assert.False(t, registry.HasType(ReporterTypeConsole))
}

func TestRegistry_ListTypes(t *testing.T) {
	registry := NewRegistry()

	factory := func(config map[string]any) (Reporter, error) {
		return newMockReporter("test"), nil
	}

	registry.Register(ReporterTypeConsole, factory)
	registry.Register(ReporterTypeJSON, factory)

	types := registry.ListTypes()
	assert.Len(t, types, 2)
}

func TestManager_AddReporter(t *testing.T) {
	manager := NewManager(nil)

	reporter := newMockReporter("test")
	err := manager.AddReporter(reporter)
	assert.NoError(t, err)
	assert.Equal(t, 1, manager.GetReporterCount())

	// Add another reporter
	reporter2 := newMockReporter("test2")
	err = manager.AddReporter(reporter2)
	assert.NoError(t, err)
	assert.Equal(t, 2, manager.GetReporterCount())
}

func TestManager_AddReporterAfterStart(t *testing.T) {
	manager := NewManager(nil)
	ctx := context.Background()

	err := manager.Start(ctx)
	require.NoError(t, err)

	reporter := newMockReporter("test")
	err = manager.AddReporter(reporter)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot add reporter after manager has started")
}

func TestManager_Report(t *testing.T) {
	manager := NewManager(nil)
	ctx := context.Background()

	reporter1 := newMockReporter("reporter1")
	reporter2 := newMockReporter("reporter2")

	manager.AddReporter(reporter1)
	manager.AddReporter(reporter2)
	manager.Start(ctx)

	metrics := &types.Metrics{
		Timestamp: time.Now(),
		StepMetrics: map[string]*types.StepMetrics{
			"step1": {StepID: "step1", Count: 10},
		},
	}

	err := manager.Report(ctx, metrics)
	assert.NoError(t, err)

	assert.Equal(t, int32(1), reporter1.reportCount.Load())
	assert.Equal(t, int32(1), reporter2.reportCount.Load())
	assert.Equal(t, metrics, reporter1.lastMetrics)
	assert.Equal(t, metrics, reporter2.lastMetrics)
}

func TestManager_ReportWithError(t *testing.T) {
	manager := NewManager(nil)
	ctx := context.Background()

	reporter1 := newMockReporter("reporter1")
	reporter1.reportErr = errors.New("report failed")

	reporter2 := newMockReporter("reporter2")

	manager.AddReporter(reporter1)
	manager.AddReporter(reporter2)
	manager.Start(ctx)

	metrics := &types.Metrics{Timestamp: time.Now()}

	err := manager.Report(ctx, metrics)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "report errors")

	// Both reporters should still be called
	assert.Equal(t, int32(1), reporter1.reportCount.Load())
	assert.Equal(t, int32(1), reporter2.reportCount.Load())
}

func TestManager_Flush(t *testing.T) {
	manager := NewManager(nil)
	ctx := context.Background()

	reporter1 := newMockReporter("reporter1")
	reporter2 := newMockReporter("reporter2")

	manager.AddReporter(reporter1)
	manager.AddReporter(reporter2)
	manager.Start(ctx)

	err := manager.Flush(ctx)
	assert.NoError(t, err)

	assert.True(t, reporter1.flushCalled.Load())
	assert.True(t, reporter2.flushCalled.Load())
}

func TestManager_Close(t *testing.T) {
	manager := NewManager(nil)
	ctx := context.Background()

	reporter1 := newMockReporter("reporter1")
	reporter2 := newMockReporter("reporter2")

	manager.AddReporter(reporter1)
	manager.AddReporter(reporter2)
	manager.Start(ctx)

	err := manager.Close(ctx)
	assert.NoError(t, err)

	assert.True(t, reporter1.closeCalled.Load())
	assert.True(t, reporter2.closeCalled.Load())
	assert.Equal(t, 0, manager.GetReporterCount())
	assert.False(t, manager.IsStarted())
}

func TestManager_AddReporterFromConfig(t *testing.T) {
	registry := NewRegistry()
	registry.Register(ReporterTypeConsole, func(config map[string]any) (Reporter, error) {
		return newMockReporter("console"), nil
	})

	manager := NewManager(registry)
	ctx := context.Background()

	config := &ReporterConfig{
		Type:    ReporterTypeConsole,
		Enabled: true,
		Config:  map[string]any{"key": "value"},
	}

	err := manager.AddReporterFromConfig(ctx, config)
	assert.NoError(t, err)
	assert.Equal(t, 1, manager.GetReporterCount())
}

func TestManager_AddReporterFromConfig_Disabled(t *testing.T) {
	registry := NewRegistry()
	manager := NewManager(registry)
	ctx := context.Background()

	config := &ReporterConfig{
		Type:    ReporterTypeConsole,
		Enabled: false,
	}

	err := manager.AddReporterFromConfig(ctx, config)
	assert.NoError(t, err)
	assert.Equal(t, 0, manager.GetReporterCount())
}

func TestManager_GetReporters(t *testing.T) {
	manager := NewManager(nil)

	reporter1 := newMockReporter("reporter1")
	reporter2 := newMockReporter("reporter2")

	manager.AddReporter(reporter1)
	manager.AddReporter(reporter2)

	reporters := manager.GetReporters()
	assert.Len(t, reporters, 2)
}
