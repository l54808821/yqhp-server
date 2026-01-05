package parser

import (
	"bytes"
	"os"

	"yqhp/workflow-engine/pkg/types"
	"gopkg.in/yaml.v3"
)

// YAMLPrinter implements the Printer interface for YAML workflow definitions.
type YAMLPrinter struct {
	indent int // Number of spaces for indentation
}

// NewYAMLPrinter creates a new YAMLPrinter with default settings.
func NewYAMLPrinter() *YAMLPrinter {
	return &YAMLPrinter{
		indent: 2, // Default 2-space indentation
	}
}

// WithIndent sets the indentation level.
func (p *YAMLPrinter) WithIndent(spaces int) *YAMLPrinter {
	p.indent = spaces
	return p
}

// Print serializes a workflow to YAML bytes.
func (p *YAMLPrinter) Print(workflow *types.Workflow) ([]byte, error) {
	var buf bytes.Buffer

	encoder := yaml.NewEncoder(&buf)
	encoder.SetIndent(p.indent)

	if err := encoder.Encode(workflow); err != nil {
		return nil, NewParseError(0, 0, "failed to encode workflow to YAML", err)
	}

	if err := encoder.Close(); err != nil {
		return nil, NewParseError(0, 0, "failed to close YAML encoder", err)
	}

	return buf.Bytes(), nil
}

// PrintToFile serializes a workflow to a YAML file.
func (p *YAMLPrinter) PrintToFile(workflow *types.Workflow, path string) error {
	data, err := p.Print(workflow)
	if err != nil {
		return err
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return NewParseError(0, 0, "failed to write file: "+path, err)
	}

	return nil
}

// PrintPretty serializes a workflow to a formatted YAML string.
// This is an alias for Print that returns a string.
func (p *YAMLPrinter) PrintPretty(workflow *types.Workflow) (string, error) {
	data, err := p.Print(workflow)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
