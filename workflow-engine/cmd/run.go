package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"yqhp/workflow-engine/internal/master"
	"yqhp/workflow-engine/internal/parser"
	"yqhp/workflow-engine/pkg/logger"
	"yqhp/workflow-engine/pkg/types"
)

var (
	// run 命令的 flags
	runVUs        int
	runDuration   time.Duration
	runIterations int
	runMode       string
	runJSONOutput string
	runOutputs    []string
)

// runCmd 是 run 子命令
var runCmd = &cobra.Command{
	Use:   "run <workflow.yaml>",
	Short: "独立模式执行工作流",
	Long: `在独立模式下执行工作流文件。

支持多种执行模式：
  - constant-vus: 固定虚拟用户数
  - ramping-vus: 渐进式虚拟用户数
  - per-vu-iterations: 每个 VU 执行固定迭代次数
  - shared-iterations: 所有 VU 共享迭代次数`,
	Example: `  # 基本执行
  workflow-engine run workflow.yaml

  # 指定 VU 数和持续时间
  workflow-engine run -u 10 -d 30s workflow.yaml

  # 指定迭代次数
  workflow-engine run -i 100 --mode shared-iterations workflow.yaml

  # 输出指标到文件
  workflow-engine run --out json=metrics.json workflow.yaml

  # 多个输出目标
  workflow-engine run --out json=metrics.json --out console workflow.yaml`,
	Args: cobra.ExactArgs(1),
	RunE: runWorkflow,
}

func init() {
	rootCmd.AddCommand(runCmd)

	// run 命令的 flags
	runCmd.Flags().IntVarP(&runVUs, "vus", "u", 0, "虚拟用户数 (覆盖工作流配置)")
	runCmd.Flags().DurationVarP(&runDuration, "duration", "d", 0, "测试持续时间 (覆盖工作流配置)")
	runCmd.Flags().IntVarP(&runIterations, "iterations", "i", 0, "迭代次数 (覆盖工作流配置)")
	runCmd.Flags().StringVar(&runMode, "mode", "", "执行模式 (constant-vus, ramping-vus, per-vu-iterations, shared-iterations)")
	runCmd.Flags().StringVar(&runJSONOutput, "out-json", "", "输出 JSON 结果到文件")
	runCmd.Flags().StringArrayVarP(&runOutputs, "out", "o", nil, "指标输出目标 (可多次指定)，格式: type=config")
}

func runWorkflow(cmd *cobra.Command, args []string) error {
	workflowPath := args[0]

	// 启用调试日志
	if debug {
		logger.EnableDebug()
	}

	// 解析工作流文件
	p := parser.NewYAMLParser()
	workflow, err := p.ParseFile(workflowPath)
	if err != nil {
		return fmt.Errorf("解析工作流失败: %w", err)
	}

	// 应用命令行参数覆盖
	if runVUs > 0 {
		workflow.Options.VUs = runVUs
	}
	if runDuration > 0 {
		workflow.Options.Duration = runDuration
	}
	if runIterations > 0 {
		workflow.Options.Iterations = runIterations
	}
	if runMode != "" {
		workflow.Options.ExecutionMode = types.ExecutionMode(runMode)
	}

	// 设置默认值
	if workflow.Options.VUs <= 0 {
		workflow.Options.VUs = 1
	}
	if workflow.Options.Duration <= 0 && workflow.Options.Iterations <= 0 {
		workflow.Options.Iterations = 1
	}

	// 解析输出配置
	for _, out := range runOutputs {
		parts := strings.SplitN(out, "=", 2)
		cfg := types.OutputConfig{Type: parts[0]}
		if len(parts) > 1 {
			cfg.URL = parts[1]
		}
		workflow.Options.Outputs = append(workflow.Options.Outputs, cfg)
	}

	// 创建可取消的上下文
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 处理关闭信号
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		fmt.Println("\n正在中止测试...")
		cancel()
	}()

	// 打印执行信息
	if !quiet {
		printRunInfo(workflow)
	}

	// 执行工作流
	result, err := executeWorkflowRun(ctx, workflow, quiet)
	if err != nil {
		return fmt.Errorf("执行失败: %w", err)
	}

	// 打印结果
	if !quiet {
		printRunResults(result)
	}

	// 写入 JSON 输出
	if runJSONOutput != "" {
		if err := writeRunJSONOutput(runJSONOutput, result); err != nil {
			return fmt.Errorf("写入 JSON 输出失败: %w", err)
		}
		if !quiet {
			fmt.Printf("\n结果已写入: %s\n", runJSONOutput)
		}
	}

	// 检查阈值
	if result.ThresholdsFailed > 0 {
		return fmt.Errorf("阈值检查失败: %d/%d", result.ThresholdsFailed, result.ThresholdsPassed+result.ThresholdsFailed)
	}

	return nil
}

func printRunInfo(workflow *types.Workflow) {
	fmt.Printf(Banner, Version)
	fmt.Printf("  %s\n", workflow.Name)
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

// RunResult 保存工作流执行结果
type RunResult struct {
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

func executeWorkflowRun(ctx context.Context, workflow *types.Workflow, quietMode bool) (*RunResult, error) {
	startTime := time.Now()

	result := &RunResult{
		WorkflowID:   workflow.ID,
		WorkflowName: workflow.Name,
		Status:       "已完成",
		TotalVUs:     workflow.Options.VUs,
		Errors:       []string{},
	}

	// 创建独立模式的 master
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

	// 启动 master
	if err := m.Start(ctx); err != nil {
		return nil, fmt.Errorf("启动执行引擎失败: %w", err)
	}
	defer m.Stop(context.Background())

	// 提交工作流
	executionID, err := m.SubmitWorkflow(ctx, workflow)
	if err != nil {
		return nil, fmt.Errorf("提交工作流失败: %w", err)
	}

	// 监控执行状态
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	// 实时进度显示
	printer := newRunProgressPrinter(workflow, quietMode)

	for {
		select {
		case <-ctx.Done():
			printer.clear()
			result.Status = "已中止"
			result.Duration = time.Since(startTime)
			return result, nil

		case <-ticker.C:
			state, err := m.GetExecutionStatus(ctx, executionID)
			if err != nil {
				continue
			}

			metrics, _ := m.GetMetrics(ctx, executionID)
			printer.update(state, metrics, time.Since(startTime))

			switch state.Status {
			case types.ExecutionStatusCompleted:
				printer.clear()
				result.Duration = time.Since(startTime)
				result.TotalIterations = int64(state.Progress * float64(workflow.Options.Iterations))
				if result.TotalIterations == 0 {
					result.TotalIterations = 1
				}

				if metrics != nil {
					populateRunResult(result, metrics)
				}

				if result.Duration.Seconds() > 0 {
					result.RPS = float64(result.TotalRequests) / result.Duration.Seconds()
				}

				return result, nil

			case types.ExecutionStatusFailed:
				printer.clear()
				result.Status = "失败"
				result.Duration = time.Since(startTime)
				for _, execErr := range state.Errors {
					result.Errors = append(result.Errors, execErr.Message)
				}
				return result, nil

			case types.ExecutionStatusAborted:
				printer.clear()
				result.Status = "已中止"
				result.Duration = time.Since(startTime)
				return result, nil
			}
		}
	}
}

type runProgressPrinter struct {
	workflow   *types.Workflow
	quiet      bool
	lastLines  int
	lastUpdate time.Time
}

func newRunProgressPrinter(workflow *types.Workflow, quiet bool) *runProgressPrinter {
	return &runProgressPrinter{workflow: workflow, quiet: quiet}
}

func (p *runProgressPrinter) update(state *types.ExecutionState, metrics *types.AggregatedMetrics, elapsed time.Duration) {
	if p.quiet {
		return
	}

	if time.Since(p.lastUpdate) < 200*time.Millisecond {
		return
	}
	p.lastUpdate = time.Now()

	p.clear()

	progress := state.Progress
	barWidth := 30
	filled := int(progress * float64(barWidth))
	bar := ""
	for i := 0; i < barWidth; i++ {
		if i < filled {
			bar += "█"
		} else {
			bar += "░"
		}
	}

	var eta string
	if progress > 0 && progress < 1 {
		remaining := time.Duration(float64(elapsed) / progress * (1 - progress))
		eta = fmt.Sprintf("剩余 %s", remaining.Round(time.Second))
	}

	fmt.Printf("  [%s] %.1f%% %s\n", bar, progress*100, eta)
	p.lastLines = 1

	if metrics != nil {
		var totalRequests, totalSuccess, totalFailure int64
		var totalDuration time.Duration

		for _, stepMetrics := range metrics.StepMetrics {
			totalRequests += stepMetrics.Count
			totalSuccess += stepMetrics.SuccessCount
			totalFailure += stepMetrics.FailureCount
			if stepMetrics.Duration != nil && stepMetrics.Count > 0 {
				totalDuration += stepMetrics.Duration.Avg * time.Duration(stepMetrics.Count)
			}
		}

		var avgDuration time.Duration
		if totalRequests > 0 {
			avgDuration = totalDuration / time.Duration(totalRequests)
		}

		rps := float64(0)
		if elapsed.Seconds() > 0 {
			rps = float64(totalRequests) / elapsed.Seconds()
		}

		successRate := float64(0)
		if totalRequests > 0 {
			successRate = float64(totalSuccess) / float64(totalRequests) * 100
		}

		fmt.Println()
		fmt.Printf("  运行时间: %-12s  迭代: %-8d  VUs: %d\n",
			elapsed.Round(time.Second), metrics.TotalIterations, metrics.TotalVUs)
		fmt.Printf("  请求数:   %-12d  RPS:  %-8.1f  成功率: %.1f%%\n",
			totalRequests, rps, successRate)
		fmt.Printf("  平均延迟: %-12s  失败: %d\n",
			avgDuration.Round(time.Microsecond), totalFailure)
		p.lastLines += 4
	}

	fmt.Print("\033[?25l")
}

func (p *runProgressPrinter) clear() {
	if p.quiet || p.lastLines == 0 {
		return
	}

	for i := 0; i < p.lastLines; i++ {
		fmt.Print("\033[A\033[K")
	}
	fmt.Print("\033[?25h")
	p.lastLines = 0
}

func populateRunResult(result *RunResult, metrics *types.AggregatedMetrics) {
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

func printRunResults(result *RunResult) {
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

func writeRunJSONOutput(path string, result *RunResult) error {
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
