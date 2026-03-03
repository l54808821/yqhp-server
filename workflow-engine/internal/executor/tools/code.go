package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"yqhp/workflow-engine/internal/executor"
	"yqhp/workflow-engine/pkg/types"
)

type CodeExecuteTool struct{}

func (t *CodeExecuteTool) Definition() *types.ToolDefinition {
	return &types.ToolDefinition{
		Name:        "code_execute",
		Description: "执行代码片段。支持 Python 和 JavaScript (Node.js)。用于数据计算、格式转换、文本处理等场景。代码在沙箱中运行，有 30 秒超时限制。",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"language": {
					"type": "string",
					"enum": ["python", "javascript"],
					"description": "编程语言"
				},
				"code": {
					"type": "string",
					"description": "要执行的代码"
				}
			},
			"required": ["language", "code"]
		}`),
	}
}

func (t *CodeExecuteTool) Execute(ctx context.Context, arguments string, execCtx *executor.ExecutionContext) (*types.ToolResult, error) {
	var args struct {
		Language string `json:"language"`
		Code     string `json:"code"`
	}
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return types.NewErrorResult(fmt.Sprintf("参数解析失败: %v", err)), nil
	}
	if args.Language == "" {
		return types.NewErrorResult("缺少必填参数: language"), nil
	}
	if args.Code == "" {
		return types.NewErrorResult("缺少必填参数: code"), nil
	}

	var cmd string
	var cmdArgs []string
	switch args.Language {
	case "python":
		cmd = "python3"
		cmdArgs = []string{"-c", args.Code}
	case "javascript":
		cmd = "node"
		cmdArgs = []string{"-e", args.Code}
	default:
		return types.NewErrorResult(fmt.Sprintf("不支持的语言: %s，仅支持 python 和 javascript", args.Language)), nil
	}

	execCtx2, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	execCmd := ExecCommandContext(execCtx2, cmd, cmdArgs...)
	output, err := execCmd.CombinedOutput()
	if err != nil {
		result := string(output)
		if result == "" {
			result = err.Error()
		}
		return types.NewErrorResult(fmt.Sprintf("执行失败:\n%s", result)), nil
	}

	return types.NewToolResult(string(output)), nil
}
