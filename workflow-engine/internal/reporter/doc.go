// Package reporter provides the reporting framework for the workflow execution engine.
//
// The reporter package implements a pluggable reporting system that supports
// multiple concurrent reporters for outputting execution metrics to various
// destinations such as console, files, Prometheus, InfluxDB, and webhooks.
//
// # Architecture
//
// The package consists of three main components:
//
//   - Reporter: The interface that all reporters must implement
//   - Registry: Manages reporter type registration and factory functions
//   - Manager: Coordinates multiple reporters for an execution
//
// # Usage
//
// To use the reporter system:
//
//  1. Create a Registry and register reporter factories
//  2. Create a Manager with the registry
//  3. Add reporters via configuration or directly
//  4. Call Report() to send metrics to all reporters
//  5. Call Close() when done to release resources
//
// Example:
//
//	registry := reporter.NewRegistry()
//	registry.Register(reporter.ReporterTypeConsole, console.NewFactory())
//
//	manager := reporter.NewManager(registry)
//	manager.AddReporterFromConfig(ctx, &reporter.ReporterConfig{
//	    Type:    reporter.ReporterTypeConsole,
//	    Enabled: true,
//	})
//
//	manager.Start(ctx)
//	manager.Report(ctx, metrics)
//	manager.Close(ctx)
//
// Requirements: 9.1.1
package reporter
