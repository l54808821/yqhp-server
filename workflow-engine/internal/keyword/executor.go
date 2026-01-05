package keyword

import (
	"context"
	"fmt"
)

// Action represents a keyword action to be executed.
type Action struct {
	Keyword string         `yaml:"keyword" json:"keyword"` // Keyword name
	Params  map[string]any `yaml:",inline" json:"params"`  // Keyword parameters
}

// ScriptExecutor executes keyword scripts (pre_scripts and post_scripts).
type ScriptExecutor struct {
	registry *Registry
}

// NewScriptExecutor creates a new script executor.
func NewScriptExecutor(registry *Registry) *ScriptExecutor {
	if registry == nil {
		registry = DefaultRegistry
	}
	return &ScriptExecutor{registry: registry}
}

// ExecutionRecord records the execution of a keyword for testing/debugging.
type ExecutionRecord struct {
	Keyword string
	Order   int
	Phase   string // "pre", "main", "post"
	Success bool
	Error   error
}

// ExecuteScripts executes a list of keyword actions in order.
// Returns the execution records and any error encountered.
func (e *ScriptExecutor) ExecuteScripts(
	ctx context.Context,
	execCtx *ExecutionContext,
	actions []Action,
	phase string,
) ([]ExecutionRecord, error) {
	records := make([]ExecutionRecord, 0, len(actions))

	for i, action := range actions {
		record := ExecutionRecord{
			Keyword: action.Keyword,
			Order:   i,
			Phase:   phase,
		}

		kw, err := e.registry.Get(action.Keyword)
		if err != nil {
			record.Success = false
			record.Error = fmt.Errorf("keyword '%s' not found: %w", action.Keyword, err)
			records = append(records, record)
			return records, record.Error
		}

		// Validate parameters
		if err := kw.Validate(action.Params); err != nil {
			record.Success = false
			record.Error = fmt.Errorf("validation failed for keyword '%s': %w", action.Keyword, err)
			records = append(records, record)
			return records, record.Error
		}

		// Execute keyword
		result, err := kw.Execute(ctx, execCtx, action.Params)
		if err != nil {
			record.Success = false
			record.Error = err
			records = append(records, record)
			return records, err
		}

		if !result.Success {
			record.Success = false
			record.Error = result.Error
			if record.Error == nil {
				record.Error = fmt.Errorf("keyword '%s' failed: %s", action.Keyword, result.Message)
			}
			records = append(records, record)
			return records, record.Error
		}

		record.Success = true
		records = append(records, record)
	}

	return records, nil
}

// StepExecution represents the execution of a complete step with pre/post scripts.
type StepExecution struct {
	PreRecords  []ExecutionRecord
	MainRecord  *ExecutionRecord
	PostRecords []ExecutionRecord
}

// ExecuteStep executes a step with pre_scripts, main action, and post_scripts.
// The execution order is: pre_scripts -> main -> post_scripts
func (e *ScriptExecutor) ExecuteStep(
	ctx context.Context,
	execCtx *ExecutionContext,
	preScripts []Action,
	mainAction func() error,
	postScripts []Action,
) (*StepExecution, error) {
	execution := &StepExecution{}

	// Execute pre_scripts
	preRecords, err := e.ExecuteScripts(ctx, execCtx, preScripts, "pre")
	execution.PreRecords = preRecords
	if err != nil {
		return execution, fmt.Errorf("pre_scripts failed: %w", err)
	}

	// Execute main action
	mainRecord := &ExecutionRecord{
		Keyword: "main",
		Order:   0,
		Phase:   "main",
	}
	if mainAction != nil {
		if err := mainAction(); err != nil {
			mainRecord.Success = false
			mainRecord.Error = err
			execution.MainRecord = mainRecord
			return execution, fmt.Errorf("main action failed: %w", err)
		}
	}
	mainRecord.Success = true
	execution.MainRecord = mainRecord

	// Execute post_scripts
	postRecords, err := e.ExecuteScripts(ctx, execCtx, postScripts, "post")
	execution.PostRecords = postRecords
	if err != nil {
		return execution, fmt.Errorf("post_scripts failed: %w", err)
	}

	return execution, nil
}

// GetExecutionOrder returns the execution order from a StepExecution.
// Returns a slice of phase names in execution order.
func (s *StepExecution) GetExecutionOrder() []string {
	var order []string

	for _, r := range s.PreRecords {
		order = append(order, fmt.Sprintf("pre:%s:%d", r.Keyword, r.Order))
	}

	if s.MainRecord != nil {
		order = append(order, "main")
	}

	for _, r := range s.PostRecords {
		order = append(order, fmt.Sprintf("post:%s:%d", r.Keyword, r.Order))
	}

	return order
}
