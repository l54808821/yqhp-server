// Package hook provides the hook execution framework for workflow and step pre/post hooks.
//
// The hook system supports:
//   - Workflow-level pre-hooks: Executed before any workflow steps
//   - Workflow-level post-hooks: Executed after all workflow steps complete
//   - Step-level pre-hooks: Executed before each step
//   - Step-level post-hooks: Executed after each step
//
// Hook failure handling:
//   - Pre-hook failure: The associated step/workflow is skipped, error is recorded
//   - Post-hook: Always executed regardless of step/workflow success or failure
//
// Requirements covered:
//   - 4.1: Workflow pre-hook execution
//   - 4.2: Workflow post-hook execution
//   - 4.3: Step pre-hook execution
//   - 4.4: Step post-hook execution
//   - 4.5: Pre-hook failure handling (skip associated step/workflow)
//   - 4.6: Post-hook always executes regardless of success/failure
package hook
