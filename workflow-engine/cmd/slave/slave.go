// Package slave provides CLI commands for managing slave nodes.
// Requirements: 8.1
package slave

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/grafana/k6/workflow-engine/internal/config"
	"github.com/grafana/k6/workflow-engine/internal/executor"
	"github.com/grafana/k6/workflow-engine/internal/slave"
	"github.com/grafana/k6/workflow-engine/pkg/types"
)

// Execute executes the slave command with the given arguments.
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
		return fmt.Errorf("unknown slave subcommand: %s", subcommand)
	}
}

func printUsage() {
	fmt.Println(`workflow-engine slave - Manage slave nodes

Usage:
  workflow-engine slave <subcommand> [options]

Subcommands:
  start     Start a slave node
  status    Show slave node status

Use "workflow-engine slave <subcommand> --help" for more information.`)
}

// executeStart starts a slave node.
// Requirements: 8.1
func executeStart(args []string) error {
	fs := flag.NewFlagSet("slave start", flag.ExitOnError)

	// Configuration flags
	configPath := fs.String("config", "", "Path to configuration file")
	slaveID := fs.String("id", "", "Slave ID (auto-generated if not specified)")
	slaveType := fs.String("type", "worker", "Slave type (worker, gateway, aggregator)")
	address := fs.String("address", ":9091", "Slave listen address")
	masterAddr := fs.String("master", "localhost:9090", "Master node address")
	maxVUs := fs.Int("max-vus", 100, "Maximum virtual users")
	capabilities := fs.String("capabilities", "http_executor,script_executor", "Comma-separated list of capabilities")
	labels := fs.String("labels", "", "Comma-separated key=value labels (e.g., region=us-east,env=prod)")

	fs.Usage = func() {
		fmt.Println(`workflow-engine slave start - Start a slave node

Usage:
  workflow-engine slave start [options]

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

	// Generate slave ID if not specified
	id := *slaveID
	if id == "" {
		id = fmt.Sprintf("slave-%s", uuid.New().String()[:8])
	}

	// Parse slave type
	var sType types.SlaveType
	switch *slaveType {
	case "worker":
		sType = types.SlaveTypeWorker
	case "gateway":
		sType = types.SlaveTypeGateway
	case "aggregator":
		sType = types.SlaveTypeAggregator
	default:
		return fmt.Errorf("invalid slave type: %s", *slaveType)
	}

	// Parse capabilities
	caps := parseCommaSeparated(*capabilities)

	// Parse labels
	lbls := parseLabels(*labels)

	// Apply command-line overrides
	if *masterAddr != "localhost:9090" {
		cfg.Slave.MasterAddr = *masterAddr
	}
	if *maxVUs != 100 {
		cfg.Slave.MaxVUs = *maxVUs
	}

	// Create slave configuration
	slaveCfg := &slave.Config{
		ID:                id,
		Type:              sType,
		Address:           *address,
		MasterAddress:     cfg.Slave.MasterAddr,
		Capabilities:      caps,
		Labels:            lbls,
		HeartbeatInterval: 5 * time.Second,
		HeartbeatTimeout:  10 * time.Second,
		MaxVUs:            cfg.Slave.MaxVUs,
		CPUCores:          4,    // Could be detected from runtime
		MemoryMB:          4096, // Could be detected from runtime
	}

	// Create executor registry
	registry := executor.NewRegistry()

	// Create and start slave
	s := slave.NewWorkerSlave(slaveCfg, registry)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		fmt.Println("\nShutting down slave...")
		cancel()
	}()

	fmt.Printf("Starting slave node...\n")
	fmt.Printf("  ID: %s\n", id)
	fmt.Printf("  Type: %s\n", sType)
	fmt.Printf("  Address: %s\n", *address)
	fmt.Printf("  Master: %s\n", cfg.Slave.MasterAddr)
	fmt.Printf("  Max VUs: %d\n", cfg.Slave.MaxVUs)
	fmt.Printf("  Capabilities: %v\n", caps)
	if len(lbls) > 0 {
		fmt.Printf("  Labels: %v\n", lbls)
	}
	fmt.Println()

	if err := s.Start(ctx); err != nil {
		return fmt.Errorf("failed to start slave: %w", err)
	}

	// Connect to master
	fmt.Printf("Connecting to master at %s...\n", cfg.Slave.MasterAddr)
	if err := s.Connect(ctx, cfg.Slave.MasterAddr); err != nil {
		fmt.Printf("Warning: Failed to connect to master: %v\n", err)
		fmt.Println("Slave will continue running and retry connection...")
	} else {
		fmt.Println("Connected to master successfully.")
	}

	fmt.Println("Slave node started. Press Ctrl+C to stop.")

	// Wait for context cancellation
	<-ctx.Done()

	// Graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := s.Stop(shutdownCtx); err != nil {
		return fmt.Errorf("failed to stop slave: %w", err)
	}

	fmt.Println("Slave node stopped.")
	return nil
}

// executeStatus shows the slave node status.
// Requirements: 8.1
func executeStatus(args []string) error {
	fs := flag.NewFlagSet("slave status", flag.ExitOnError)

	address := fs.String("address", "http://localhost:9091", "Slave node address")

	fs.Usage = func() {
		fmt.Println(`workflow-engine slave status - Show slave node status

Usage:
  workflow-engine slave status [options]

Options:`)
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	fmt.Printf("Checking slave status at %s...\n", *address)

	// In a real implementation, this would make an HTTP request to the slave
	// For now, we'll just print a placeholder message
	fmt.Println()
	fmt.Println("Slave Status:")
	fmt.Println("  State: unknown (not connected)")
	fmt.Println()
	fmt.Println("Note: To get actual status, ensure the slave is running and accessible.")
	fmt.Printf("      Try: curl %s/status\n", *address)

	return nil
}

// parseCommaSeparated parses a comma-separated string into a slice.
func parseCommaSeparated(s string) []string {
	if s == "" {
		return []string{}
	}
	result := []string{}
	for _, item := range splitAndTrim(s, ",") {
		if item != "" {
			result = append(result, item)
		}
	}
	return result
}

// parseLabels parses a comma-separated key=value string into a map.
func parseLabels(s string) map[string]string {
	if s == "" {
		return map[string]string{}
	}
	result := make(map[string]string)
	for _, pair := range splitAndTrim(s, ",") {
		parts := splitAndTrim(pair, "=")
		if len(parts) == 2 {
			result[parts[0]] = parts[1]
		}
	}
	return result
}

// splitAndTrim splits a string and trims whitespace from each part.
func splitAndTrim(s, sep string) []string {
	parts := []string{}
	for _, part := range split(s, sep) {
		trimmed := trim(part)
		if trimmed != "" {
			parts = append(parts, trimmed)
		}
	}
	return parts
}

// split splits a string by separator.
func split(s, sep string) []string {
	result := []string{}
	start := 0
	for i := 0; i <= len(s)-len(sep); i++ {
		if s[i:i+len(sep)] == sep {
			result = append(result, s[start:i])
			start = i + len(sep)
			i += len(sep) - 1
		}
	}
	result = append(result, s[start:])
	return result
}

// trim trims whitespace from a string.
func trim(s string) string {
	start := 0
	end := len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t' || s[start] == '\n' || s[start] == '\r') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\n' || s[end-1] == '\r') {
		end--
	}
	return s[start:end]
}
