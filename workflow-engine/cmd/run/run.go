// Package run provides CLI commands for executing workflows in standalone mode.
// Requirements: 5.7
package run

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"yqhp/workflow-engine/internal/master"
	"yqhp/workflow-engine/internal/parser"
	"yqhp/workflow-engine/pkg/types"
)

// Execute executes the run command with the given arguments.
// Requirements: 5.7
func Execute(args []string) error {
	fs := flag.NewFlagSet("run", flag.ExitOnError)

	// Execution options
	vus := fs.Int("vus", 0, "Number of virtual users (overrides workflow config)")
	duration := fs.Duration("duration", 0, "Test duration (overrides workflow config)")
	iterations := fs.Int("iterations", 0, "Number of iterations (overrides workflow config)")
	mode := fs.String("mode", "", "Execution mode (constant-vus, ramping-vus, etc.)")

	// Output options
	quiet := fs.Bool("quiet", false, "Suppress progress output")
	jsonOutput := fs.String("out-json", "", "Output results to JSON file")

	// Help
	help := fs.Bool("help", false, "Show help message")

	fs.Usage = func() {
		printUsage()
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	if *help {
		printUsage()
		return nil
	}

	// Get workflow file path
	remainingArgs := fs.Args()
	if len(remainingArgs) < 1 {
		printUsage()
		return fmt.Errorf("workflow file path is required")
	}

	workflowPath := remainingArgs[0]

	// Parse workflow file
	p := parser.NewYAMLParser()
	workflow, err := p.ParseFile(workflowPath)
	if err != nil {
		return fmt.Errorf("failed to parse workflow: %w", err)
	}

	// Apply command-line overrides
	if *vus > 0 {
		workflow.Options.VUs = *vus
	}
	if *duration > 0 {
		workflow.Options.Duration = *duration
	}
	if *iterations > 0 {
		workflow.Options.Iterations = *iterations
	}
	if *mode != "" {
		workflow.Options.ExecutionMode = types.ExecutionMode(*mode)
	}

	// Set defaults if not specified
	if workflow.Options.VUs <= 0 {
		workflow.Options.VUs = 1
	}
	if workflow.Options.Duration <= 0 && workflow.Options.Iterations <= 0 {
		workflow.Options.Iterations = 1
	}

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		fmt.Println("\nAborting test...")
		cancel()
	}()

	// Print execution info
	if !*quiet {
		printExecutionInfo(workflow)
	}

	// Execute workflow
	result, err := executeWorkflow(ctx, workflow, *quiet)
	if err != nil {
		return fmt.Errorf("execution failed: %w", err)
	}

	// Print results
	if !*quiet {
		printResults(result)
	}

	// Write JSON output if requested
	if *jsonOutput != "" {
		if err := writeJSONOutput(*jsonOutput, result); err != nil {
			return fmt.Errorf("failed to write JSON output: %w", err)
		}
		if !*quiet {
			fmt.Printf("\nResults written to: %s\n", *jsonOutput)
		}
	}

	// Check thresholds
	if result.ThresholdsFailed > 0 {
		return fmt.Errorf("thresholds failed: %d/%d", result.ThresholdsFailed, result.ThresholdsPassed+result.ThresholdsFailed)
	}

	return nil
}

func printUsage() {
	fmt.Println(`workflow-engine run - Execute a workflow in standalone mode

Usage:
  workflow-engine run [options] <workflow.yaml>

Options:
  -vus int
        Number of virtual users (overrides workflow config)
  -duration duration
        Test duration (overrides workflow config, e.g., 30s, 5m)
  -iterations int
        Number of iterations (overrides workflow config)
  -mode string
        Execution mode (constant-vus, ramping-vus, per-vu-iterations, shared-iterations)
  -quiet
        Suppress progress output
  -out-json string
        Output results to JSON file
  -help
        Show this help message

Examples:
  workflow-engine run workflow.yaml
  workflow-engine run -vus 10 -duration 30s workflow.yaml
  workflow-engine run -iterations 100 -mode shared-iterations workflow.yaml`)
}

func printExecutionInfo(workflow *types.Workflow) {
	fmt.Println()
	fmt.Printf("          /\\      |‾‾| Workflow Engine v0.1.0\n")
	fmt.Printf("     /\\  /  \\     |  |\n")
	fmt.Printf("    /  \\/    \\    |  |\n")
	fmt.Printf("   /          \\   |  |\n")
	fmt.Printf("  / __________ \\  |__|  %s\n", workflow.Name)
	fmt.Println()
	fmt.Printf("  execution: standalone\n")
	fmt.Printf("  workflow: %s\n", workflow.ID)
	if workflow.Description != "" {
		fmt.Printf("  description: %s\n", workflow.Description)
	}
	fmt.Printf("  steps: %d\n", len(workflow.Steps))
	fmt.Println()
	fmt.Printf("  vus: %d\n", workflow.Options.VUs)
	if workflow.Options.Duration > 0 {
		fmt.Printf("  duration: %s\n", workflow.Options.Duration)
	}
	if workflow.Options.Iterations > 0 {
		fmt.Printf("  iterations: %d\n", workflow.Options.Iterations)
	}
	if workflow.Options.ExecutionMode != "" {
		fmt.Printf("  mode: %s\n", workflow.Options.ExecutionMode)
	}
	fmt.Println()
	fmt.Println("running...")
	fmt.Println()
}

// ExecutionResult holds the results of a workflow execution.
type ExecutionResult struct {
	WorkflowID       string
	WorkflowName     string
	Status           string
	Duration         time.Duration
	TotalVUs         int
	TotalIterations  int64
	TotalRequests    int64
	SuccessRate      float64
	ErrorRate        float64
	AvgDuration      time.Duration
	P95Duration      time.Duration
	P99Duration      time.Duration
	ThresholdsPassed int
	ThresholdsFailed int
	Errors           []string
}

func executeWorkflow(ctx context.Context, workflow *types.Workflow, quiet bool) (*ExecutionResult, error) {
	startTime := time.Now()

	result := &ExecutionResult{
		WorkflowID:   workflow.ID,
		WorkflowName: workflow.Name,
		Status:       "completed",
		TotalVUs:     workflow.Options.VUs,
		Errors:       []string{},
	}

	// Create master in standalone mode
	masterCfg := &master.Config{
		StandaloneMode:          true,
		MaxConcurrentExecutions: 1,
	}

	registry := master.NewInMemorySlaveRegistry()
	scheduler := master.NewWorkflowScheduler(registry)
	aggregator := master.NewDefaultMetricsAggregator()

	m := master.NewWorkflowMaster(masterCfg, registry, scheduler, aggregator)

	// Start master
	if err := m.Start(ctx); err != nil {
		return nil, fmt.Errorf("failed to start execution engine: %w", err)
	}
	defer m.Stop(context.Background())

	// Submit workflow
	executionID, err := m.SubmitWorkflow(ctx, workflow)
	if err != nil {
		return nil, fmt.Errorf("failed to submit workflow: %w", err)
	}

	// Monitor execution
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	lastProgress := float64(0)

	for {
		select {
		case <-ctx.Done():
			result.Status = "aborted"
			result.Duration = time.Since(startTime)
			return result, nil

		case <-ticker.C:
			status, err := m.GetExecutionStatus(ctx, executionID)
			if err != nil {
				continue
			}

			// Print progress
			if !quiet && status.Progress != lastProgress {
				fmt.Printf("\r  progress: %.1f%%", status.Progress*100)
				lastProgress = status.Progress
			}

			// Check if completed
			switch status.Status {
			case types.ExecutionStatusCompleted:
				if !quiet {
					fmt.Printf("\r  progress: 100.0%%\n")
				}
				result.Duration = time.Since(startTime)
				result.TotalIterations = int64(status.Progress * float64(workflow.Options.Iterations))
				if result.TotalIterations == 0 {
					result.TotalIterations = 1
				}

				// Get metrics
				metrics, _ := m.GetMetrics(ctx, executionID)
				if metrics != nil {
					populateResultFromMetrics(result, metrics)
				}

				return result, nil

			case types.ExecutionStatusFailed:
				if !quiet {
					fmt.Println()
				}
				result.Status = "failed"
				result.Duration = time.Since(startTime)
				for _, execErr := range status.Errors {
					result.Errors = append(result.Errors, execErr.Message)
				}
				return result, nil

			case types.ExecutionStatusAborted:
				if !quiet {
					fmt.Println()
				}
				result.Status = "aborted"
				result.Duration = time.Since(startTime)
				return result, nil
			}
		}
	}
}

func populateResultFromMetrics(result *ExecutionResult, metrics *types.AggregatedMetrics) {
	result.TotalIterations = metrics.TotalIterations
	result.TotalVUs = metrics.TotalVUs

	var totalRequests, totalSuccess, totalFailure int64
	var totalDuration time.Duration
	var p95Max, p99Max time.Duration

	for _, stepMetrics := range metrics.StepMetrics {
		totalRequests += stepMetrics.Count
		totalSuccess += stepMetrics.SuccessCount
		totalFailure += stepMetrics.FailureCount

		if stepMetrics.Duration != nil {
			totalDuration += stepMetrics.Duration.Avg * time.Duration(stepMetrics.Count)
			if stepMetrics.Duration.P95 > p95Max {
				p95Max = stepMetrics.Duration.P95
			}
			if stepMetrics.Duration.P99 > p99Max {
				p99Max = stepMetrics.Duration.P99
			}
		}
	}

	result.TotalRequests = totalRequests
	if totalRequests > 0 {
		result.SuccessRate = float64(totalSuccess) / float64(totalRequests)
		result.ErrorRate = float64(totalFailure) / float64(totalRequests)
		result.AvgDuration = totalDuration / time.Duration(totalRequests)
	}
	result.P95Duration = p95Max
	result.P99Duration = p99Max

	for _, threshold := range metrics.Thresholds {
		if threshold.Passed {
			result.ThresholdsPassed++
		} else {
			result.ThresholdsFailed++
		}
	}
}

func printResults(result *ExecutionResult) {
	fmt.Println()
	fmt.Println("     results:")
	fmt.Println()
	fmt.Printf("     status............: %s\n", result.Status)
	fmt.Printf("     duration..........: %s\n", result.Duration.Round(time.Millisecond))
	fmt.Printf("     vus...............: %d\n", result.TotalVUs)
	fmt.Printf("     iterations........: %d\n", result.TotalIterations)
	fmt.Printf("     requests..........: %d\n", result.TotalRequests)
	if result.TotalRequests > 0 {
		fmt.Printf("     success_rate......: %.2f%%\n", result.SuccessRate*100)
		fmt.Printf("     error_rate........: %.2f%%\n", result.ErrorRate*100)
		fmt.Printf("     avg_duration......: %s\n", result.AvgDuration.Round(time.Microsecond))
		if result.P95Duration > 0 {
			fmt.Printf("     p95_duration......: %s\n", result.P95Duration.Round(time.Microsecond))
		}
		if result.P99Duration > 0 {
			fmt.Printf("     p99_duration......: %s\n", result.P99Duration.Round(time.Microsecond))
		}
	}

	if result.ThresholdsPassed > 0 || result.ThresholdsFailed > 0 {
		fmt.Println()
		fmt.Printf("     thresholds........: %d passed, %d failed\n", result.ThresholdsPassed, result.ThresholdsFailed)
	}

	if len(result.Errors) > 0 {
		fmt.Println()
		fmt.Println("     errors:")
		for _, err := range result.Errors {
			fmt.Printf("       - %s\n", err)
		}
	}

	fmt.Println()
}

func writeJSONOutput(path string, result *ExecutionResult) error {
	// Simple JSON serialization without external dependencies
	content := fmt.Sprintf(`{
  "workflow_id": "%s",
  "workflow_name": "%s",
  "status": "%s",
  "duration_ms": %d,
  "total_vus": %d,
  "total_iterations": %d,
  "total_requests": %d,
  "success_rate": %.4f,
  "error_rate": %.4f,
  "avg_duration_ms": %d,
  "p95_duration_ms": %d,
  "p99_duration_ms": %d,
  "thresholds_passed": %d,
  "thresholds_failed": %d
}`,
		result.WorkflowID,
		result.WorkflowName,
		result.Status,
		result.Duration.Milliseconds(),
		result.TotalVUs,
		result.TotalIterations,
		result.TotalRequests,
		result.SuccessRate,
		result.ErrorRate,
		result.AvgDuration.Milliseconds(),
		result.P95Duration.Milliseconds(),
		result.P99Duration.Milliseconds(),
		result.ThresholdsPassed,
		result.ThresholdsFailed,
	)

	return os.WriteFile(path, []byte(content), 0644)
}
