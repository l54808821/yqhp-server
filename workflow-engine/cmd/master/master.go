// Package master provides CLI commands for managing the master node.
// Requirements: 5.1
package master

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"yqhp/workflow-engine/internal/config"
	"yqhp/workflow-engine/internal/master"
)

// Execute executes the master command with the given arguments.
func Execute(args []string) error {
	if len(args) < 1 {
		printUsage()
		return nil
	}

	subcommand := args[0]
	subArgs := args[1:]

	switch subcommand {
	case "start":
		return executeStart(subArgs)
	case "status":
		return executeStatus(subArgs)
	case "help", "-h", "--help":
		printUsage()
		return nil
	default:
		return fmt.Errorf("unknown master subcommand: %s", subcommand)
	}
}

func printUsage() {
	fmt.Println(`workflow-engine master - Manage the master node

Usage:
  workflow-engine master <subcommand> [options]

Subcommands:
  start     Start the master node
  status    Show master node status

Use "workflow-engine master <subcommand> --help" for more information.`)
}

// executeStart starts the master node.
// Requirements: 5.1
func executeStart(args []string) error {
	fs := flag.NewFlagSet("master start", flag.ExitOnError)

	// Configuration flags
	configPath := fs.String("config", "", "Path to configuration file")
	address := fs.String("address", ":8080", "HTTP server address")
	grpcAddress := fs.String("grpc-address", ":9090", "gRPC server address")
	standalone := fs.Bool("standalone", false, "Run in standalone mode without slaves")
	heartbeatTimeout := fs.Duration("heartbeat-timeout", 30*time.Second, "Slave heartbeat timeout")
	maxExecutions := fs.Int("max-executions", 100, "Maximum concurrent executions")

	fs.Usage = func() {
		fmt.Println(`workflow-engine master start - Start the master node

Usage:
  workflow-engine master start [options]

Options:`)
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	// Load configuration
	loader := config.NewLoader()
	if *configPath != "" {
		loader = loader.WithConfigPath(*configPath)
	}

	cfg, err := loader.Load()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Apply command-line overrides
	if *address != ":8080" {
		cfg.Server.Address = *address
	}
	if *grpcAddress != ":9090" {
		cfg.GRPC.Address = *grpcAddress
	}
	if *heartbeatTimeout != 30*time.Second {
		cfg.Master.HeartbeatTimeout = *heartbeatTimeout
	}

	// Create master configuration
	masterCfg := &master.Config{
		Address:                 cfg.Server.Address,
		HeartbeatTimeout:        cfg.Master.HeartbeatTimeout,
		HealthCheckInterval:     cfg.Master.HeartbeatInterval,
		StandaloneMode:          *standalone,
		MaxConcurrentExecutions: *maxExecutions,
	}

	// Create registry, scheduler, and aggregator
	registry := master.NewInMemorySlaveRegistry()
	scheduler := master.NewWorkflowScheduler(registry)
	aggregator := master.NewDefaultMetricsAggregator()

	// Create and start master
	m := master.NewWorkflowMaster(masterCfg, registry, scheduler, aggregator)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		fmt.Println("\nShutting down master...")
		cancel()
	}()

	fmt.Printf("Starting master node...\n")
	fmt.Printf("  HTTP Address: %s\n", cfg.Server.Address)
	fmt.Printf("  gRPC Address: %s\n", cfg.GRPC.Address)
	fmt.Printf("  Standalone Mode: %v\n", *standalone)
	fmt.Printf("  Max Concurrent Executions: %d\n", *maxExecutions)
	fmt.Println()

	if err := m.Start(ctx); err != nil {
		return fmt.Errorf("failed to start master: %w", err)
	}

	fmt.Println("Master node started successfully. Press Ctrl+C to stop.")

	// Wait for context cancellation
	<-ctx.Done()

	// Graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := m.Stop(shutdownCtx); err != nil {
		return fmt.Errorf("failed to stop master: %w", err)
	}

	fmt.Println("Master node stopped.")
	return nil
}

// executeStatus shows the master node status.
// Requirements: 5.1
func executeStatus(args []string) error {
	fs := flag.NewFlagSet("master status", flag.ExitOnError)

	address := fs.String("address", "http://localhost:8080", "Master node address")

	fs.Usage = func() {
		fmt.Println(`workflow-engine master status - Show master node status

Usage:
  workflow-engine master status [options]

Options:`)
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	fmt.Printf("Checking master status at %s...\n", *address)

	// In a real implementation, this would make an HTTP request to the master
	// For now, we'll just print a placeholder message
	fmt.Println()
	fmt.Println("Master Status:")
	fmt.Println("  State: unknown (not connected)")
	fmt.Println()
	fmt.Println("Note: To get actual status, ensure the master is running and accessible.")
	fmt.Printf("      Try: curl %s/api/v1/health\n", *address)

	return nil
}
