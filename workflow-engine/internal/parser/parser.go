// Package parser provides workflow parsing and serialization functionality.
package parser

import (
	"yqhp/workflow-engine/pkg/types"
)

// Parser defines the interface for parsing workflow definitions.
type Parser interface {
	// Parse parses a workflow definition from bytes.
	Parse(data []byte) (*types.Workflow, error)

	// ParseFile parses a workflow definition from a file.
	ParseFile(path string) (*types.Workflow, error)
}

// Printer defines the interface for serializing workflow definitions.
type Printer interface {
	// Print serializes a workflow to bytes.
	Print(workflow *types.Workflow) ([]byte, error)

	// PrintToFile serializes a workflow to a file.
	PrintToFile(workflow *types.Workflow, path string) error
}

// VariableResolver defines the interface for resolving variable references.
type VariableResolver interface {
	// Resolve resolves a variable reference and returns its value.
	Resolve(ref string) (any, error)
}
