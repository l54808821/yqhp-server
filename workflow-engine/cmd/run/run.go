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
	vus := fs.Int("vus", 0, "虚拟用户数 (覆盖工作流配置)")
	duration := fs.Duration("duration", 0, "测试持续时间 (覆盖工作流配置)")
	iterations := fs.Int("iterations", 0, "迭代次数 (覆盖工作流配置)")
	mode := fs.String("mode", "", "执行模式 (constant-vus, ramping-vus 等)")

	// Output options
	quiet := fs.Bool("quiet", false, "静默模式，不输出进度")
	jsonOutput := fs.String("out-json", "", "输出 JSON 结果到文件")

	// Help
	help := fs.Bool("help", false, "显示帮助信息")

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
		return fmt.Errorf("需要指定工作流文件路径")
	}

	workflowPath := remainingArgs[0]

	// Parse workflow file
	p := parser.NewYAMLParser()
	workflow, err := p.ParseFile(workflowPath)
	if err != nil {
		return fmt.Errorf("解析工作流失败: %w", err)
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
		fmt.Println("\n正在中止测试...")
		cancel()
	}()

	// Print execution info
	if !*quiet {
		printExecutionInfo(workflow)
	}

	// Execute workflow
	result, err := executeWorkflow(ctx, workflow, *quiet)
	if err != nil {
		return fmt.Errorf("执行失败: %w", err)
	}

	// Print results
	if !*quiet {
		printResults(result)
	}

	// Write JSON output if requested
	if *jsonOutput != "" {
		if err := writeJSONOutput(*jsonOutput, result); err != nil {
			return fmt.Errorf("写入 JSON 输出失败: %w", err)
		}
		if !*quiet {
			fmt.Printf("\n结果已写入: %s\n", *jsonOutput)
		}
	}

	// Check thresholds
	if result.ThresholdsFailed > 0 {
		return fmt.Errorf("阈值检查失败: %d/%d", result.ThresholdsFailed, result.ThresholdsPassed+result.ThresholdsFailed)
	}

	return nil
}

func printUsage() {
	fmt.Println(`workflow-engine run - 独立模式执行工作流

用法:
  workflow-engine run [选项] <workflow.yaml>

选项:
  -vus int
        虚拟用户数 (覆盖工作流配置)
  -duration duration
        测试持续时间 (覆盖工作流配置，如 30s, 5m)
  -iterations int
        迭代次数 (覆盖工作流配置)
  -mode string
        执行模式 (constant-vus, ramping-vus, per-vu-iterations, shared-iterations)
  -quiet
        静默模式，不输出进度
  -out-json string
        输出 JSON 结果到文件
  -help
        显示帮助信息

示例:
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
	fmt.Printf("  执行模式: 独立模式\n")
	fmt.Printf("  工作流: %s\n", workflow.ID)
	if workflow.Description != "" {
		fmt.Printf("  描述: %s\n", workflow.Description)
	}
	fmt.Printf("  步骤数: %d\n", len(workflow.Steps))
	fmt.Println()
	fmt.Printf("  虚拟用户数: %d\n", workflow.Options.VUs)
	if workflow.Options.Duration > 0 {
		fmt.Printf("  持续时间: %s\n", workflow.Options.Duration)
	}
	if workflow.Options.Iterations > 0 {
		fmt.Printf("  迭代次数: %d\n", workflow.Options.Iterations)
	}
	if workflow.Options.ExecutionMode != "" {
		fmt.Printf("  执行模式: %s\n", workflow.Options.ExecutionMode)
	}
	fmt.Println()
	fmt.Println("执行中...")
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
	RPS              float64
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
		Status:       "已完成",
		TotalVUs:     workflow.Options.VUs,
		Errors:       []string{},
	}

	// Create master in standalone mode
	masterCfg := &master.Config{
		StandaloneMode:          true,
		MaxConcurrentExecutions: 1,
		HealthCheckInterval:     10 * time.Second,
		HeartbeatTimeout:        30 * time.Second,
	}

	registry := master.NewInMemorySlaveRegistry()
	scheduler := master.NewWorkflowScheduler(registry)
	aggregator := master.NewDefaultMetricsAggregator()

	m := master.NewWorkflowMaster(masterCfg, registry, scheduler, aggregator)

	// Start master
	if err := m.Start(ctx); err != nil {
		return nil, fmt.Errorf("启动执行引擎失败: %w", err)
	}
	defer m.Stop(context.Background())

	// Submit workflow
	executionID, err := m.SubmitWorkflow(ctx, workflow)
	if err != nil {
		return nil, fmt.Errorf("提交工作流失败: %w", err)
	}

	// Monitor execution
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	lastProgress := float64(0)

	for {
		select {
		case <-ctx.Done():
			result.Status = "已中止"
			result.Duration = time.Since(startTime)
			return result, nil

		case <-ticker.C:
			status, err := m.GetExecutionStatus(ctx, executionID)
			if err != nil {
				continue
			}

			// Print progress
			if !quiet && status.Progress != lastProgress {
				fmt.Printf("\r  进度: %.1f%%", status.Progress*100)
				lastProgress = status.Progress
			}

			// Check if completed
			switch status.Status {
			case types.ExecutionStatusCompleted:
				if !quiet {
					fmt.Printf("\r  进度: 100.0%%\n")
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

				// Calculate RPS
				if result.Duration.Seconds() > 0 {
					result.RPS = float64(result.TotalRequests) / result.Duration.Seconds()
				}

				return result, nil

			case types.ExecutionStatusFailed:
				if !quiet {
					fmt.Println()
				}
				result.Status = "失败"
				result.Duration = time.Since(startTime)
				for _, execErr := range status.Errors {
					result.Errors = append(result.Errors, execErr.Message)
				}
				return result, nil

			case types.ExecutionStatusAborted:
				if !quiet {
					fmt.Println()
				}
				result.Status = "已中止"
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
	fmt.Println("     测试结果:")
	fmt.Println()
	fmt.Printf("     状态...............: %s\n", result.Status)
	fmt.Printf("     总耗时.............: %s\n", result.Duration.Round(time.Millisecond))
	fmt.Printf("     虚拟用户数.........: %d\n", result.TotalVUs)
	fmt.Printf("     总迭代次数.........: %d\n", result.TotalIterations)
	fmt.Printf("     总请求数...........: %d\n", result.TotalRequests)
	if result.TotalRequests > 0 {
		fmt.Printf("     每秒请求数(RPS)....: %.2f\n", result.RPS)
		fmt.Printf("     成功率.............: %.2f%%\n", result.SuccessRate*100)
		fmt.Printf("     失败率.............: %.2f%%\n", result.ErrorRate*100)
		fmt.Printf("     平均响应时间.......: %s\n", result.AvgDuration.Round(time.Microsecond))
		if result.P95Duration > 0 {
			fmt.Printf("     P95 响应时间.......: %s\n", result.P95Duration.Round(time.Microsecond))
		}
		if result.P99Duration > 0 {
			fmt.Printf("     P99 响应时间.......: %s\n", result.P99Duration.Round(time.Microsecond))
		}
	}

	if result.ThresholdsPassed > 0 || result.ThresholdsFailed > 0 {
		fmt.Println()
		fmt.Printf("     阈值检查...........: %d 通过, %d 失败\n", result.ThresholdsPassed, result.ThresholdsFailed)
	}

	if len(result.Errors) > 0 {
		fmt.Println()
		fmt.Println("     错误信息:")
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
  "rps": %.2f,
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
		result.RPS,
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
