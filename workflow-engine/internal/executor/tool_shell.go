package executor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"yqhp/workflow-engine/pkg/types"
)

const (
	shellTimeout        = 60 * time.Second
	shellMaxOutputBytes = 128 * 1024 // 128KB
)

// ShellExecTool 在受控环境中执行 Shell 命令，支持 bash/sh。
// 参考 https://github.com/cloudwego/eino-ext/components/tool/commandline
type ShellExecTool struct{}

func (t *ShellExecTool) Definition() *types.ToolDefinition {
	return &types.ToolDefinition{
		Name:        "shell_exec",
		Description: "在服务器上执行 Shell 命令（bash）。适用于：系统信息查询、文件操作、网络诊断、文本处理（grep/awk/sed）、调用 CLI 工具等。命令有 60 秒超时限制，输出上限 128KB。",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"command": {
					"type": "string",
					"description": "要执行的 Shell 命令"
				},
				"working_dir": {
					"type": "string",
					"description": "工作目录（可选，默认为系统临时目录）"
				},
				"timeout": {
					"type": "integer",
					"description": "超时秒数（可选，默认 60，最大 300）"
				}
			},
			"required": ["command"]
		}`),
	}
}

var shellDangerousPatterns = []string{
	"rm -rf /",
	"mkfs.",
	"dd if=",
	":(){:|:&};:",
	"> /dev/sda",
	"chmod -R 777 /",
	"shutdown",
	"reboot",
	"init 0",
	"init 6",
}

func (t *ShellExecTool) Execute(ctx context.Context, arguments string, execCtx *ExecutionContext) (*types.ToolResult, error) {
	var args struct {
		Command    string `json:"command"`
		WorkingDir string `json:"working_dir"`
		Timeout    int    `json:"timeout"`
	}
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return types.NewErrorResult(fmt.Sprintf("参数解析失败: %v", err)), nil
	}
	if strings.TrimSpace(args.Command) == "" {
		return types.NewErrorResult("缺少必填参数: command"), nil
	}

	cmdLower := strings.ToLower(args.Command)
	for _, pattern := range shellDangerousPatterns {
		if strings.Contains(cmdLower, pattern) {
			return types.NewErrorResult(fmt.Sprintf("安全限制：命令包含危险操作 %q，已拒绝执行", pattern)), nil
		}
	}

	timeout := shellTimeout
	if args.Timeout > 0 {
		if args.Timeout > 300 {
			args.Timeout = 300
		}
		timeout = time.Duration(args.Timeout) * time.Second
	}

	execCtx2, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := execCommandContext(execCtx2, "bash", "-c", args.Command)

	if args.WorkingDir != "" {
		absDir, err := filepath.Abs(args.WorkingDir)
		if err == nil {
			if info, statErr := os.Stat(absDir); statErr == nil && info.IsDir() {
				cmd.Dir = absDir
			}
		}
	}
	if cmd.Dir == "" {
		cmd.Dir = os.TempDir()
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	startTime := time.Now()
	err := cmd.Run()
	elapsed := time.Since(startTime)

	var result strings.Builder
	result.WriteString(fmt.Sprintf("耗时: %s\n", elapsed.Round(time.Millisecond)))

	if err != nil {
		result.WriteString(fmt.Sprintf("退出状态: 错误 (%v)\n", err))
	} else {
		result.WriteString("退出状态: 成功\n")
	}

	stdoutStr := stdout.String()
	stderrStr := stderr.String()

	if len(stdoutStr) > shellMaxOutputBytes {
		stdoutStr = stdoutStr[:shellMaxOutputBytes] + "\n...(stdout 已截断)"
	}
	if len(stderrStr) > shellMaxOutputBytes {
		stderrStr = stderrStr[:shellMaxOutputBytes] + "\n...(stderr 已截断)"
	}

	if stdoutStr != "" {
		result.WriteString("\n--- stdout ---\n")
		result.WriteString(stdoutStr)
	}
	if stderrStr != "" {
		result.WriteString("\n--- stderr ---\n")
		result.WriteString(stderrStr)
	}

	if stdoutStr == "" && stderrStr == "" {
		result.WriteString("\n(无输出)")
	}

	return types.NewToolResult(result.String()), nil
}

func init() {
	RegisterTool(&ShellExecTool{})
}
