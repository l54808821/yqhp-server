// Package runner provides a unified execution entry point for all workflow
// execution modes (debug, perf test, CLI), following k6's cmd/run.go pattern.
//
// Pipeline: Submit → Engine → TaskEngine → samplesChan → OutputManager
//
//	→ [Outputs + MetricsEngine + Summary]
package runner

import (
	"context"
	"fmt"
	"time"

	"yqhp/workflow-engine/pkg/types"
)

// WorkflowEngine is the interface required by Run. Both engine.Engine and
// gulu's workflow.Engine satisfy it, so callers can pass either one.
type WorkflowEngine interface {
	SubmitWorkflow(ctx context.Context, wf *types.Workflow) (string, error)
	GetExecutionStatus(ctx context.Context, execID string) (*types.ExecutionState, error)
	StopExecution(ctx context.Context, execID string) error
	GetMetrics(ctx context.Context, execID string) (*types.AggregatedMetrics, error)
	GetPerformanceReport(ctx context.Context, execID string) (*types.PerformanceTestReport, error)
}

// RunOptions configures a workflow execution.
type RunOptions struct {
	// Workflow to execute (required).
	Workflow *types.Workflow

	// Engine to use (required).
	Engine WorkflowEngine

	// OnProgress is called periodically with execution state and metrics.
	OnProgress func(state *types.ExecutionState, metrics *types.AggregatedMetrics)

	// PollInterval controls how often to poll execution status.
	// Defaults to 200ms.
	PollInterval time.Duration
}

// RunResult contains the result of a workflow execution.
type RunResult struct {
	ExecutionID string
	Status      types.ExecutionStatus
	Duration    time.Duration
	Iterations  int64
	Report      *types.PerformanceTestReport
	Variables   map[string]any
	EnvVars     map[string]any
	Errors      []types.ExecutionError
}

// Run executes a workflow through the unified pipeline. All execution paths
// (debug, perf, CLI) converge here.
func Run(ctx context.Context, opts RunOptions) (*RunResult, error) {
	if opts.Workflow == nil {
		return nil, fmt.Errorf("workflow is required")
	}
	if opts.Engine == nil {
		return nil, fmt.Errorf("engine is required")
	}

	startTime := time.Now()

	execID, err := opts.Engine.SubmitWorkflow(ctx, opts.Workflow)
	if err != nil {
		return nil, fmt.Errorf("submit workflow failed: %w", err)
	}

	result := &RunResult{ExecutionID: execID}

	pollInterval := opts.PollInterval
	if pollInterval <= 0 {
		pollInterval = 200 * time.Millisecond
	}

	execErr := waitForCompletion(ctx, opts.Engine, execID, pollInterval, opts.OnProgress)

	result.Duration = time.Since(startTime)

	if execErr != nil {
		result.Status = types.ExecutionStatusFailed
		result.Errors = append(result.Errors, types.ExecutionError{
			Code:      types.ErrCodeExecution,
			Message:   execErr.Error(),
			Timestamp: time.Now(),
		})
	} else {
		result.Status = types.ExecutionStatusCompleted
	}

	if report, err := opts.Engine.GetPerformanceReport(ctx, execID); err == nil && report != nil {
		result.Report = report
	}

	if m, err := opts.Engine.GetMetrics(ctx, execID); err == nil && m != nil {
		result.Iterations = m.TotalIterations
	}

	result.Variables = opts.Workflow.FinalVariables
	result.EnvVars = opts.Workflow.EnvVariables

	return result, execErr
}

func waitForCompletion(
	ctx context.Context,
	eng WorkflowEngine,
	execID string,
	pollInterval time.Duration,
	onProgress func(*types.ExecutionState, *types.AggregatedMetrics),
) error {
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			_ = eng.StopExecution(context.Background(), execID)
			return ctx.Err()

		case <-ticker.C:
			state, err := eng.GetExecutionStatus(ctx, execID)
			if err != nil {
				continue
			}

			if onProgress != nil {
				m, _ := eng.GetMetrics(ctx, execID)
				onProgress(state, m)
			}

			switch state.Status {
			case types.ExecutionStatusCompleted:
				return nil
			case types.ExecutionStatusFailed:
				if len(state.Errors) > 0 {
					return fmt.Errorf("%s", state.Errors[0].Message)
				}
				return fmt.Errorf("execution failed")
			case types.ExecutionStatusAborted:
				return fmt.Errorf("execution aborted")
			}
		}
	}
}
