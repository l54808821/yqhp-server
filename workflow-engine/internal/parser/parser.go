// Package parser 提供工作流解析和序列化功能。
package parser

import (
	"yqhp/workflow-engine/pkg/types"
)

// Parser 定义了解析工作流定义的接口。
type Parser interface {
	// Parse 从字节数据解析工作流定义。
	Parse(data []byte) (*types.Workflow, error)

	// ParseFile 从文件解析工作流定义。
	ParseFile(path string) (*types.Workflow, error)
}

// Printer 定义了序列化工作流定义的接口。
type Printer interface {
	// Print 将工作流序列化为字节数据。
	Print(workflow *types.Workflow) ([]byte, error)

	// PrintToFile 将工作流序列化到文件。
	PrintToFile(workflow *types.Workflow, path string) error
}

// VariableResolver 定义了解析变量引用的接口。
type VariableResolver interface {
	// Resolve 解析变量引用并返回其值。
	Resolve(ref string) (any, error)
}
