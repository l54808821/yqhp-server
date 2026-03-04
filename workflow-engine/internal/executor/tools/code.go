package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"yqhp/workflow-engine/internal/executor"
	"yqhp/workflow-engine/pkg/logger"
	"yqhp/workflow-engine/pkg/types"
)

type CodeExecuteTool struct{}

func (t *CodeExecuteTool) Definition() *types.ToolDefinition {
	return &types.ToolDefinition{
		Name:        "code_execute",
		Description: "执行代码片段。支持 Python 和 JavaScript (Node.js)。用于数据计算、格式转换、文本处理等场景。代码在沙箱中运行，有 30 秒超时限制。代码的最后一个表达式的值会自动作为结果返回（无需手动 print/console.log）。",
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
		cmdArgs = []string{"-c", wrapPythonCode(args.Code)}
	case "javascript":
		cmd = "node"
		cmdArgs = []string{"-e", wrapJavaScriptCode(args.Code)}
	default:
		return types.NewErrorResult(fmt.Sprintf("不支持的语言: %s，仅支持 python 和 javascript", args.Language)), nil
	}

	logger.Debug("[CodeExecute] 准备执行 %s 代码, 命令: %s, 代码内容:\n%s", args.Language, cmd, args.Code)

	execCtx2, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	execCmd := ExecCommandContext(execCtx2, cmd, cmdArgs...)
	output, err := execCmd.CombinedOutput()
	if err != nil {
		result := string(output)
		if result == "" {
			result = err.Error()
		}
		logger.Debug("[CodeExecute] %s 代码执行失败: %s", args.Language, result)
		return types.NewErrorResult(fmt.Sprintf("执行失败:\n%s", result)), nil
	}

	logger.Debug("[CodeExecute] %s 代码执行成功, 输出:\n%s", args.Language, string(output))
	return types.NewToolResult(string(output)), nil
}

// wrapPythonCode wraps user Python code so that the last expression's value
// is automatically printed (mimicking REPL behavior).
// Uses ast to detect if the last statement is an expression; if so, captures
// and prints its repr. All prior output (print calls etc.) still works normally.
func wrapPythonCode(code string) string {
	wrapper := `
import ast as _ast, sys as _sys

_code = %s

try:
    _tree = _ast.parse(_code)
except SyntaxError:
    exec(_code)
    _sys.exit(0)

if _tree.body and isinstance(_tree.body[-1], _ast.Expr):
    _last = _tree.body.pop()
    _last_expr = _ast.Expression(body=_last.value)
    _ast.fix_missing_locations(_last_expr)
    if _tree.body:
        exec(compile(_tree, '<code>', 'exec'))
    _result = eval(compile(_last_expr, '<code>', 'eval'))
    if _result is not None:
        print(repr(_result))
else:
    exec(compile(_tree, '<code>', 'exec'))
`
	jsonBytes, _ := json.Marshal(code)
	return fmt.Sprintf(wrapper, string(jsonBytes))
}

// wrapJavaScriptCode wraps user JavaScript code so that the last expression
// statement's value is automatically logged (mimicking Node REPL behavior).
// Uses a simple heuristic: if the last non-empty line is a pure expression
// (not a declaration/control flow), wrap it with console.log.
func wrapJavaScriptCode(code string) string {
	lines := strings.Split(code, "\n")

	lastIdx := -1
	for i := len(lines) - 1; i >= 0; i-- {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed != "" && trimmed != "}" && trimmed != ");" {
			lastIdx = i
			break
		}
	}

	if lastIdx < 0 {
		return code
	}

	lastLine := strings.TrimSpace(lines[lastIdx])

	declarationPrefixes := []string{
		"const ", "let ", "var ", "function ", "class ",
		"if ", "if(", "else ", "else{",
		"for ", "for(", "while ", "while(",
		"switch ", "switch(",
		"try ", "try{", "catch ", "catch(",
		"return ", "throw ", "import ", "export ",
	}
	for _, prefix := range declarationPrefixes {
		if strings.HasPrefix(lastLine, prefix) {
			return code
		}
	}

	if strings.HasSuffix(lastLine, ";") {
		lastLine = lastLine[:len(lastLine)-1]
	}

	lines[lastIdx] = fmt.Sprintf(`{const __r = (%s); if (__r !== undefined) console.log(typeof __r === 'object' ? JSON.stringify(__r, null, 2) : __r);}`, lastLine)
	return strings.Join(lines, "\n")
}
