// Package reporter provides the reporting framework for the workflow execution engine.
// Requirements: 9.1.1
package reporter

import (
	"context"
	"fmt"
	"sync"

	"github.com/grafana/k6/workflow-engine/pkg/types"
)

// Reporter defines the interface for report output.
// Requirements: 9.1.1
type Reporter interface {
	// Name returns the reporter name.
	Name() string

	// Init initializes the reporter with configuration.
	Init(ctx context.Context, config map[string]any) error

	// Report sends a metrics report.
	Report(ctx context.Context, metrics *types.Metrics) error

	// Flush flushes any buffered data.
	Flush(ctx context.Context) error

	// Close closes the reporter and releases resources.
	Close(ctx context.Context) error
}

// ReporterType defines the type of reporter.
type ReporterType string

const (
	// ReporterTypeConsole outputs to console.
	ReporterTypeConsole ReporterType = "console"
	// ReporterTypeJSON outputs to JSON file.
	ReporterTypeJSON ReporterType = "json"
	// ReporterTypeCSV outputs to CSV file.
	ReporterTypeCSV ReporterType = "csv"
	// ReporterTypePrometheus pushes to Prometheus.
	ReporterTypePrometheus ReporterType = "prometheus"
	// ReporterTypeInfluxDB writes to InfluxDB.
	ReporterTypeInfluxDB ReporterType = "influxdb"
	// ReporterTypeWebhook sends to webhook URL.
	ReporterTypeWebhook ReporterType = "webhook"
)

// ReporterConfig holds configuration for a reporter.
type ReporterConfig struct {
	Type    ReporterType   `yaml:"type"`
	Enabled bool           `yaml:"enabled"`
	Config  map[string]any `yaml:"config,omitempty"`
}

// ReporterFactory creates reporters of a specific type.
type ReporterFactory func(config map[string]any) (Reporter, error)

// Registry manages reporter registration and creation.
// Requirements: 9.1.1
type Registry struct {
	factories map[ReporterType]ReporterFactory
	mu        sync.RWMutex
}

// NewRegistry creates a new reporter registry.
func NewRegistry() *Registry {
	return &Registry{
		factories: make(map[ReporterType]ReporterFactory),
	}
}

// Register registers a reporter factory for a type.
func (r *Registry) Register(reporterType ReporterType, factory ReporterFactory) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.factories[reporterType]; exists {
		return fmt.Errorf("reporter type already registered: %s", reporterType)
	}

	r.factories[reporterType] = factory
	return nil
}

// Unregister removes a reporter factory.
func (r *Registry) Unregister(reporterType ReporterType) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.factories, reporterType)
}

// Create creates a reporter of the specified type.
func (r *Registry) Create(reporterType ReporterType, config map[string]any) (Reporter, error) {
	r.mu.RLock()
	factory, exists := r.factories[reporterType]
	r.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("unknown reporter type: %s", reporterType)
	}

	return factory(config)
}

// ListTypes returns all registered reporter types.
func (r *Registry) ListTypes() []ReporterType {
	r.mu.RLock()
	defer r.mu.RUnlock()

	types := make([]ReporterType, 0, len(r.factories))
	for t := range r.factories {
		types = append(types, t)
	}
	return types
}

// HasType checks if a reporter type is registered.
func (r *Registry) HasType(reporterType ReporterType) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, exists := r.factories[reporterType]
	return exists
}
