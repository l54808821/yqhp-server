package reporter

import (
	"yqhp/workflow-engine/internal/reporter/console"
	"yqhp/workflow-engine/internal/reporter/file"
	"yqhp/workflow-engine/internal/reporter/influxdb"
	"yqhp/workflow-engine/internal/reporter/prometheus"
	"yqhp/workflow-engine/internal/reporter/webhook"
)

// RegisterBuiltinReporters registers all built-in reporters with the registry.
func RegisterBuiltinReporters(registry *Registry) error {
	// Console reporter
	if err := registry.Register(ReporterTypeConsole, func(config map[string]any) (Reporter, error) {
		factory := console.NewFactory()
		r, err := factory(config)
		if err != nil {
			return nil, err
		}
		return r.(*console.Reporter), nil
	}); err != nil {
		return err
	}

	// JSON reporter
	if err := registry.Register(ReporterTypeJSON, func(config map[string]any) (Reporter, error) {
		factory := file.NewJSONFactory()
		r, err := factory(config)
		if err != nil {
			return nil, err
		}
		return r.(*file.JSONReporter), nil
	}); err != nil {
		return err
	}

	// CSV reporter
	if err := registry.Register(ReporterTypeCSV, func(config map[string]any) (Reporter, error) {
		factory := file.NewCSVFactory()
		r, err := factory(config)
		if err != nil {
			return nil, err
		}
		return r.(*file.CSVReporter), nil
	}); err != nil {
		return err
	}

	// Prometheus reporter
	if err := registry.Register(ReporterTypePrometheus, func(config map[string]any) (Reporter, error) {
		factory := prometheus.NewFactory()
		r, err := factory(config)
		if err != nil {
			return nil, err
		}
		return r.(*prometheus.Reporter), nil
	}); err != nil {
		return err
	}

	// InfluxDB reporter
	if err := registry.Register(ReporterTypeInfluxDB, func(config map[string]any) (Reporter, error) {
		factory := influxdb.NewFactory()
		r, err := factory(config)
		if err != nil {
			return nil, err
		}
		return r.(*influxdb.Reporter), nil
	}); err != nil {
		return err
	}

	// Webhook reporter
	if err := registry.Register(ReporterTypeWebhook, func(config map[string]any) (Reporter, error) {
		factory := webhook.NewFactory()
		r, err := factory(config)
		if err != nil {
			return nil, err
		}
		return r.(*webhook.Reporter), nil
	}); err != nil {
		return err
	}

	return nil
}

// NewDefaultRegistry creates a new registry with all built-in reporters registered.
func NewDefaultRegistry() (*Registry, error) {
	registry := NewRegistry()
	if err := RegisterBuiltinReporters(registry); err != nil {
		return nil, err
	}
	return registry, nil
}
