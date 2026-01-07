// Package console provides a console reporter for the workflow execution engine.
// Requirements: 9.1.2
package console

import (
	"context"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"yqhp/workflow-engine/pkg/types"
)

// Config holds configuration for the console reporter.
type Config struct {
	// ShowProgress enables progress bar display.
	ShowProgress bool `yaml:"show_progress"`
	// ShowMetrics enables real-time metrics output.
	ShowMetrics bool `yaml:"show_metrics"`
	// ColorOutput enables colored output.
	ColorOutput bool `yaml:"color_output"`
	// RefreshInterval is the interval for updating the display.
	RefreshInterval time.Duration `yaml:"refresh_interval"`
	// Writer is the output writer (defaults to os.Stdout).
	Writer io.Writer `yaml:"-"`
}

// DefaultConfig returns the default console reporter configuration.
func DefaultConfig() *Config {
	return &Config{
		ShowProgress:    true,
		ShowMetrics:     true,
		ColorOutput:     true,
		RefreshInterval: time.Second,
		Writer:          os.Stdout,
	}
}

// Reporter implements the console reporter.
// Requirements: 9.1.2
type Reporter struct {
	config *Config
	writer io.Writer

	// Progress tracking
	startTime  time.Time
	totalVUs   int
	iterations int64
	duration   time.Duration

	// Metrics tracking
	lastMetrics *types.Metrics
	mu          sync.RWMutex

	// State
	initialized bool
}

// New creates a new console reporter.
func New(config *Config) *Reporter {
	if config == nil {
		config = DefaultConfig()
	}
	if config.Writer == nil {
		config.Writer = os.Stdout
	}
	return &Reporter{
		config:    config,
		writer:    config.Writer,
		startTime: time.Now(),
	}
}

// NewFactory returns a factory function for creating console reporters.
func NewFactory() func(config map[string]any) (interface{ Name() string }, error) {
	return func(config map[string]any) (interface{ Name() string }, error) {
		cfg := DefaultConfig()
		if config != nil {
			if v, ok := config["show_progress"].(bool); ok {
				cfg.ShowProgress = v
			}
			if v, ok := config["show_metrics"].(bool); ok {
				cfg.ShowMetrics = v
			}
			if v, ok := config["color_output"].(bool); ok {
				cfg.ColorOutput = v
			}
			if v, ok := config["refresh_interval"].(string); ok {
				if d, err := time.ParseDuration(v); err == nil {
					cfg.RefreshInterval = d
				}
			}
		}
		return New(cfg), nil
	}
}

// Name returns the reporter name.
func (r *Reporter) Name() string {
	return "console"
}

// Init initializes the reporter.
func (r *Reporter) Init(ctx context.Context, config map[string]any) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.initialized {
		return fmt.Errorf("报告器已初始化")
	}

	r.startTime = time.Now()
	r.initialized = true

	r.printHeader()
	return nil
}

// Report sends metrics to the console.
func (r *Reporter) Report(ctx context.Context, metrics *types.Metrics) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.initialized {
		return fmt.Errorf("报告器未初始化")
	}

	r.lastMetrics = metrics

	if r.config.ShowProgress {
		r.printProgress(metrics)
	}

	if r.config.ShowMetrics {
		r.printMetrics(metrics)
	}

	return nil
}

// Flush flushes any buffered output.
func (r *Reporter) Flush(ctx context.Context) error {
	// Console output is unbuffered, nothing to flush
	return nil
}

// Close closes the reporter.
func (r *Reporter) Close(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.initialized {
		return nil
	}

	r.printSummary()
	r.initialized = false
	return nil
}

// printHeader prints the report header.
func (r *Reporter) printHeader() {
	r.writeLine("")
	r.writeLine(r.colorize("=== Workflow Execution Started ===", colorCyan))
	r.writeLine(fmt.Sprintf("Start Time: %s", r.startTime.Format(time.RFC3339)))
	r.writeLine("")
}

// printProgress prints the progress bar.
func (r *Reporter) printProgress(metrics *types.Metrics) {
	elapsed := time.Since(r.startTime)

	// Calculate total iterations
	var totalCount int64
	for _, sm := range metrics.StepMetrics {
		totalCount += sm.Count
	}

	// Build progress line
	progressLine := fmt.Sprintf("\r[%s] Iterations: %d | Elapsed: %s",
		r.formatElapsed(elapsed),
		totalCount,
		r.formatDuration(elapsed),
	)

	r.write(progressLine)
}

// printMetrics prints the current metrics.
func (r *Reporter) printMetrics(metrics *types.Metrics) {
	r.writeLine("")
	r.writeLine(r.colorize("--- Current Metrics ---", colorYellow))

	// Sort step IDs for consistent output
	stepIDs := make([]string, 0, len(metrics.StepMetrics))
	for stepID := range metrics.StepMetrics {
		stepIDs = append(stepIDs, stepID)
	}
	sort.Strings(stepIDs)

	for _, stepID := range stepIDs {
		sm := metrics.StepMetrics[stepID]
		r.printStepMetrics(stepID, sm)
	}

	// Print system metrics if available
	if metrics.SystemMetrics != nil {
		r.writeLine("")
		r.writeLine(r.colorize("System:", colorBlue))
		r.writeLine(fmt.Sprintf("  CPU: %.1f%% | Memory: %.1f%% | Goroutines: %d",
			metrics.SystemMetrics.CPUUsage,
			metrics.SystemMetrics.MemoryUsage,
			metrics.SystemMetrics.GoroutineCount,
		))
	}
}

// printStepMetrics prints metrics for a single step.
func (r *Reporter) printStepMetrics(stepID string, sm *types.StepMetrics) {
	successRate := float64(0)
	if sm.Count > 0 {
		successRate = float64(sm.SuccessCount) / float64(sm.Count) * 100
	}

	statusColor := colorGreen
	if successRate < 95 {
		statusColor = colorYellow
	}
	if successRate < 80 {
		statusColor = colorRed
	}

	r.writeLine(fmt.Sprintf("  %s:", r.colorize(stepID, colorWhite)))
	r.writeLine(fmt.Sprintf("    Count: %d | Success: %s | Failed: %d",
		sm.Count,
		r.colorize(fmt.Sprintf("%.1f%%", successRate), statusColor),
		sm.FailureCount,
	))

	if sm.Duration != nil {
		r.writeLine(fmt.Sprintf("    Duration: min=%s avg=%s max=%s",
			r.formatDuration(sm.Duration.Min),
			r.formatDuration(sm.Duration.Avg),
			r.formatDuration(sm.Duration.Max),
		))
		r.writeLine(fmt.Sprintf("    Percentiles: p50=%s p90=%s p95=%s p99=%s",
			r.formatDuration(sm.Duration.P50),
			r.formatDuration(sm.Duration.P90),
			r.formatDuration(sm.Duration.P95),
			r.formatDuration(sm.Duration.P99),
		))
	}
}

// printSummary prints the final summary.
func (r *Reporter) printSummary() {
	elapsed := time.Since(r.startTime)

	r.writeLine("")
	r.writeLine(r.colorize("=== Execution Summary ===", colorCyan))
	r.writeLine(fmt.Sprintf("Total Duration: %s", r.formatDuration(elapsed)))

	if r.lastMetrics != nil {
		var totalCount, totalSuccess, totalFailed int64
		for _, sm := range r.lastMetrics.StepMetrics {
			totalCount += sm.Count
			totalSuccess += sm.SuccessCount
			totalFailed += sm.FailureCount
		}

		successRate := float64(0)
		if totalCount > 0 {
			successRate = float64(totalSuccess) / float64(totalCount) * 100
		}

		r.writeLine(fmt.Sprintf("Total Iterations: %d", totalCount))
		r.writeLine(fmt.Sprintf("Success Rate: %.2f%%", successRate))
		r.writeLine(fmt.Sprintf("Failed: %d", totalFailed))
	}

	r.writeLine(r.colorize("=========================", colorCyan))
	r.writeLine("")
}

// Helper methods

func (r *Reporter) write(s string) {
	fmt.Fprint(r.writer, s)
}

func (r *Reporter) writeLine(s string) {
	fmt.Fprintln(r.writer, s)
}

func (r *Reporter) formatElapsed(d time.Duration) string {
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60
	return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
}

func (r *Reporter) formatDuration(d time.Duration) string {
	if d < time.Millisecond {
		return fmt.Sprintf("%dµs", d.Microseconds())
	}
	if d < time.Second {
		return fmt.Sprintf("%.2fms", float64(d.Microseconds())/1000)
	}
	return fmt.Sprintf("%.2fs", d.Seconds())
}

// Color codes
const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorCyan   = "\033[36m"
	colorWhite  = "\033[37m"
)

func (r *Reporter) colorize(s string, color string) string {
	if !r.config.ColorOutput {
		return s
	}
	return color + s + colorReset
}

// ProgressBar represents a simple progress bar.
type ProgressBar struct {
	total   int
	current int
	width   int
}

// NewProgressBar creates a new progress bar.
func NewProgressBar(total, width int) *ProgressBar {
	return &ProgressBar{
		total: total,
		width: width,
	}
}

// Update updates the progress bar.
func (p *ProgressBar) Update(current int) {
	p.current = current
}

// String returns the progress bar as a string.
func (p *ProgressBar) String() string {
	if p.total == 0 {
		return "[" + strings.Repeat("-", p.width) + "]"
	}

	progress := float64(p.current) / float64(p.total)
	filled := int(progress * float64(p.width))
	if filled > p.width {
		filled = p.width
	}

	bar := strings.Repeat("█", filled) + strings.Repeat("░", p.width-filled)
	percentage := progress * 100

	return fmt.Sprintf("[%s] %.1f%%", bar, percentage)
}
