package parser

import (
	"bytes"
	"fmt"
	"os"
	"strings"

	"github.com/grafana/k6/workflow-engine/pkg/types"
	"gopkg.in/yaml.v3"
)

// YAMLParser implements the Parser interface for YAML workflow definitions.
type YAMLParser struct {
	resolver *DefaultVariableResolver
}

// NewYAMLParser creates a new YAMLParser.
func NewYAMLParser() *YAMLParser {
	return &YAMLParser{
		resolver: NewDefaultVariableResolver(),
	}
}

// WithResolver sets a custom variable resolver.
func (p *YAMLParser) WithResolver(resolver *DefaultVariableResolver) *YAMLParser {
	p.resolver = resolver
	return p
}

// Parse parses a workflow definition from bytes.
func (p *YAMLParser) Parse(data []byte) (*types.Workflow, error) {
	var workflow types.Workflow

	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true) // Strict mode: error on unknown fields

	if err := decoder.Decode(&workflow); err != nil {
		return nil, p.wrapYAMLError(err, data)
	}

	// Set variables in resolver for later resolution
	if workflow.Variables != nil {
		p.resolver.WithVariables(workflow.Variables)
	}

	// Validate the parsed workflow
	if err := p.validate(&workflow); err != nil {
		return nil, err
	}

	return &workflow, nil
}

// ParseFile parses a workflow definition from a file.
func (p *YAMLParser) ParseFile(path string) (*types.Workflow, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, NewParseError(0, 0, fmt.Sprintf("failed to read file: %s", path), err)
	}
	return p.Parse(data)
}

// wrapYAMLError converts a YAML error to a ParseError with line information.
func (p *YAMLParser) wrapYAMLError(err error, data []byte) error {
	if err == nil {
		return nil
	}

	// Try to extract line information from YAML error
	errStr := err.Error()

	// YAML errors often contain "line X:" pattern
	line, column := extractLineColumn(errStr)

	// Create a more user-friendly message
	message := cleanYAMLErrorMessage(errStr)

	return NewParseError(line, column, message, err)
}

// extractLineColumn attempts to extract line and column from YAML error message.
func extractLineColumn(errStr string) (int, int) {
	var line, column int

	// Try to find "line X" pattern
	if idx := strings.Index(errStr, "line "); idx != -1 {
		fmt.Sscanf(errStr[idx:], "line %d", &line)
	}

	// Try to find "column X" pattern
	if idx := strings.Index(errStr, "column "); idx != -1 {
		fmt.Sscanf(errStr[idx:], "column %d", &column)
	}

	return line, column
}

// cleanYAMLErrorMessage creates a cleaner error message.
func cleanYAMLErrorMessage(errStr string) string {
	// Remove "yaml: " prefix if present
	errStr = strings.TrimPrefix(errStr, "yaml: ")

	// Capitalize first letter
	if len(errStr) > 0 {
		errStr = strings.ToUpper(errStr[:1]) + errStr[1:]
	}

	return errStr
}

// validate validates a parsed workflow.
func (p *YAMLParser) validate(workflow *types.Workflow) error {
	if workflow.ID == "" {
		return NewValidationError("id", "workflow ID is required")
	}

	if workflow.Name == "" {
		return NewValidationError("name", "workflow name is required")
	}

	if len(workflow.Steps) == 0 {
		return NewValidationError("steps", "workflow must have at least one step")
	}

	// Validate each step
	stepIDs := make(map[string]bool)
	for i, step := range workflow.Steps {
		if err := p.validateStep(&step, stepIDs, fmt.Sprintf("steps[%d]", i)); err != nil {
			return err
		}
	}

	// Validate hooks if present
	if workflow.PreHook != nil {
		if err := p.validateHook(workflow.PreHook, "pre_hook"); err != nil {
			return err
		}
	}
	if workflow.PostHook != nil {
		if err := p.validateHook(workflow.PostHook, "post_hook"); err != nil {
			return err
		}
	}

	return nil
}

// validateStep validates a single step.
func (p *YAMLParser) validateStep(step *types.Step, stepIDs map[string]bool, path string) error {
	if step.ID == "" {
		return NewValidationError(path+".id", "step ID is required")
	}

	if stepIDs[step.ID] {
		return NewValidationError(path+".id", fmt.Sprintf("duplicate step ID: %s", step.ID))
	}
	stepIDs[step.ID] = true

	if step.Name == "" {
		return NewValidationError(path+".name", "step name is required")
	}

	if step.Type == "" {
		return NewValidationError(path+".type", "step type is required")
	}

	// Validate step type
	validTypes := map[string]bool{
		"http":      true,
		"script":    true,
		"grpc":      true,
		"condition": true,
	}
	if !validTypes[step.Type] {
		return NewValidationError(path+".type", fmt.Sprintf("invalid step type: %s", step.Type))
	}

	// Validate condition if present
	if step.Condition != nil {
		if err := p.validateCondition(step.Condition, stepIDs, path+".condition"); err != nil {
			return err
		}
	}

	// Validate hooks if present
	if step.PreHook != nil {
		if err := p.validateHook(step.PreHook, path+".pre_hook"); err != nil {
			return err
		}
	}
	if step.PostHook != nil {
		if err := p.validateHook(step.PostHook, path+".post_hook"); err != nil {
			return err
		}
	}

	return nil
}

// validateCondition validates a condition block.
func (p *YAMLParser) validateCondition(cond *types.Condition, stepIDs map[string]bool, path string) error {
	if cond.Expression == "" {
		return NewValidationError(path+".expression", "condition expression is required")
	}

	if len(cond.Then) == 0 {
		return NewValidationError(path+".then", "condition must have at least one 'then' step")
	}

	// Validate then steps
	for i, step := range cond.Then {
		if err := p.validateStep(&step, stepIDs, fmt.Sprintf("%s.then[%d]", path, i)); err != nil {
			return err
		}
	}

	// Validate else steps if present
	for i, step := range cond.Else {
		if err := p.validateStep(&step, stepIDs, fmt.Sprintf("%s.else[%d]", path, i)); err != nil {
			return err
		}
	}

	return nil
}

// validateHook validates a hook definition.
func (p *YAMLParser) validateHook(hook *types.Hook, path string) error {
	if hook.Type == "" {
		return NewValidationError(path+".type", "hook type is required")
	}

	validTypes := map[string]bool{
		"script": true,
		"http":   true,
	}
	if !validTypes[hook.Type] {
		return NewValidationError(path+".type", fmt.Sprintf("invalid hook type: %s", hook.Type))
	}

	return nil
}

// ResolveVariables resolves all variable references in a workflow.
// This modifies the workflow in place.
func (p *YAMLParser) ResolveVariables(workflow *types.Workflow) error {
	// Set workflow variables in resolver
	if workflow.Variables != nil {
		p.resolver.WithVariables(workflow.Variables)
	}

	// Resolve variables in steps
	for i := range workflow.Steps {
		if err := p.resolveStepVariables(&workflow.Steps[i]); err != nil {
			return err
		}
	}

	return nil
}

// resolveStepVariables resolves variables in a step's config.
func (p *YAMLParser) resolveStepVariables(step *types.Step) error {
	if step.Config != nil {
		resolved, err := p.resolveMapVariables(step.Config)
		if err != nil {
			return err
		}
		step.Config = resolved
	}

	// Resolve in condition steps
	if step.Condition != nil {
		// Resolve expression
		if HasVariableReferences(step.Condition.Expression) {
			resolved, err := p.resolver.ResolveString(step.Condition.Expression)
			if err != nil {
				return err
			}
			step.Condition.Expression = resolved
		}

		// Resolve then steps
		for i := range step.Condition.Then {
			if err := p.resolveStepVariables(&step.Condition.Then[i]); err != nil {
				return err
			}
		}

		// Resolve else steps
		for i := range step.Condition.Else {
			if err := p.resolveStepVariables(&step.Condition.Else[i]); err != nil {
				return err
			}
		}
	}

	// Resolve in hooks
	if step.PreHook != nil && step.PreHook.Config != nil {
		resolved, err := p.resolveMapVariables(step.PreHook.Config)
		if err != nil {
			return err
		}
		step.PreHook.Config = resolved
	}
	if step.PostHook != nil && step.PostHook.Config != nil {
		resolved, err := p.resolveMapVariables(step.PostHook.Config)
		if err != nil {
			return err
		}
		step.PostHook.Config = resolved
	}

	return nil
}

// resolveMapVariables resolves variables in a map recursively.
func (p *YAMLParser) resolveMapVariables(m map[string]any) (map[string]any, error) {
	result := make(map[string]any)

	for k, v := range m {
		resolved, err := p.resolveValue(v)
		if err != nil {
			return nil, err
		}
		result[k] = resolved
	}

	return result, nil
}

// resolveValue resolves variables in a value recursively.
func (p *YAMLParser) resolveValue(v any) (any, error) {
	switch val := v.(type) {
	case string:
		if HasVariableReferences(val) {
			return p.resolver.ResolveString(val)
		}
		return val, nil

	case map[string]any:
		return p.resolveMapVariables(val)

	case []any:
		result := make([]any, len(val))
		for i, item := range val {
			resolved, err := p.resolveValue(item)
			if err != nil {
				return nil, err
			}
			result[i] = resolved
		}
		return result, nil

	default:
		return v, nil
	}
}
