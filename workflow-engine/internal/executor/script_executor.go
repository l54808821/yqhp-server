package executor

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"yqhp/workflow-engine/pkg/types"
)

const (
	// ScriptExecutorType 是脚本执行器的类型标识符。
	ScriptExecutorType = "script"

	// 脚本执行的默认超时时间。
	defaultScriptTimeout = 60 * time.Second
)

// ScriptExecutor 执行脚本步骤。
type ScriptExecutor struct {
	*BaseExecutor
	shell     string
	shellArgs []string
}

// NewScriptExecutor 创建一个新的脚本执行器。
func NewScriptExecutor() *ScriptExecutor {
	return &ScriptExecutor{
		BaseExecutor: NewBaseExecutor(ScriptExecutorType),
	}
}

// Init 使用配置初始化脚本执行器。
func (e *ScriptExecutor) Init(ctx context.Context, config map[string]any) error {
	if err := e.BaseExecutor.Init(ctx, config); err != nil {
		return err
	}

	// 根据操作系统确定 shell
	e.shell = e.GetConfigString("shell", "")
	if e.shell == "" {
		if runtime.GOOS == "windows" {
			e.shell = "cmd"
			e.shellArgs = []string{"/C"}
		} else {
			e.shell = "/bin/sh"
			e.shellArgs = []string{"-c"}
		}
	} else {
		// 从配置解析 shell 参数
		if args, ok := config["shell_args"].([]any); ok {
			for _, arg := range args {
				if s, ok := arg.(string); ok {
					e.shellArgs = append(e.shellArgs, s)
				}
			}
		} else {
			// 常见 shell 的默认参数
			switch {
			case strings.Contains(e.shell, "bash"):
				e.shellArgs = []string{"-c"}
			case strings.Contains(e.shell, "sh"):
				e.shellArgs = []string{"-c"}
			case strings.Contains(e.shell, "cmd"):
				e.shellArgs = []string{"/C"}
			case strings.Contains(e.shell, "powershell"):
				e.shellArgs = []string{"-Command"}
			default:
				e.shellArgs = []string{"-c"}
			}
		}
	}

	return nil
}

// Execute 执行脚本步骤。
func (e *ScriptExecutor) Execute(ctx context.Context, step *types.Step, execCtx *ExecutionContext) (*types.StepResult, error) {
	startTime := time.Now()

	// 解析步骤配置
	config, err := e.parseConfig(step.Config)
	if err != nil {
		return CreateFailedResult(step.ID, startTime, err), nil
	}

	// 解析脚本中的变量
	script := e.resolveVariables(config.Script, execCtx)

	// 如果指定了步骤超时则应用
	timeout := step.Timeout
	if timeout <= 0 {
		timeout = e.GetConfigDuration("timeout", defaultScriptTimeout)
	}

	// 创建带超时的命令上下文
	cmdCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// 构建命令
	args := append(e.shellArgs, script)
	cmd := exec.CommandContext(cmdCtx, e.shell, args...)

	// 设置环境变量
	cmd.Env = os.Environ()
	for k, v := range config.Env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}

	// 将执行上下文变量添加到环境变量
	if execCtx != nil {
		for k, v := range execCtx.Variables {
			cmd.Env = append(cmd.Env, fmt.Sprintf("WF_%s=%v", strings.ToUpper(k), v))
		}
	}

	// 设置工作目录
	if config.WorkDir != "" {
		cmd.Dir = config.WorkDir
	}

	// 捕获输出
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// 执行命令
	err = cmd.Run()

	// 构建输出
	output := &ScriptOutput{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: 0,
	}

	if err != nil {
		// 检查是否超时
		if cmdCtx.Err() == context.DeadlineExceeded {
			return CreateTimeoutResult(step.ID, startTime, timeout), nil
		}

		// 如果可用则获取退出码
		if exitErr, ok := err.(*exec.ExitError); ok {
			output.ExitCode = exitErr.ExitCode()
		} else {
			output.ExitCode = -1
		}

		result := CreateFailedResult(step.ID, startTime, NewExecutionError(step.ID, "script execution failed", err))
		result.Output = output
		return result, nil
	}

	// 创建成功结果
	result := CreateSuccessResult(step.ID, startTime, output)
	result.Metrics["script_exit_code"] = float64(output.ExitCode)

	return result, nil
}

// Cleanup 释放脚本执行器持有的资源。
func (e *ScriptExecutor) Cleanup(ctx context.Context) error {
	return nil
}

// ScriptConfig 表示脚本步骤的配置。
type ScriptConfig struct {
	Script  string            // 内联脚本内容
	File    string            // 脚本文件路径（内联的替代方式）
	Env     map[string]string // 环境变量
	WorkDir string            // 工作目录
}

// ScriptOutput 表示脚本执行的输出。
type ScriptOutput struct {
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	ExitCode int    `json:"exit_code"`
}

// parseConfig 将步骤配置解析为 ScriptConfig。
func (e *ScriptExecutor) parseConfig(config map[string]any) (*ScriptConfig, error) {
	scriptConfig := &ScriptConfig{
		Env: make(map[string]string),
	}

	// 获取内联脚本
	if inline, ok := config["inline"].(string); ok {
		scriptConfig.Script = inline
	}

	// 获取脚本文件
	if file, ok := config["file"].(string); ok {
		scriptConfig.File = file
	}

	// 如果指定了文件，读取其内容
	if scriptConfig.File != "" && scriptConfig.Script == "" {
		content, err := os.ReadFile(scriptConfig.File)
		if err != nil {
			return nil, NewConfigError(fmt.Sprintf("failed to read script file: %s", scriptConfig.File), err)
		}
		scriptConfig.Script = string(content)
	}

	// 验证是否有脚本
	if scriptConfig.Script == "" {
		return nil, NewConfigError("script step requires 'inline' or 'file' configuration", nil)
	}

	// 获取环境变量
	if env, ok := config["env"].(map[string]any); ok {
		for k, v := range env {
			if s, ok := v.(string); ok {
				scriptConfig.Env[k] = s
			}
		}
	}

	// 获取工作目录
	if workDir, ok := config["work_dir"].(string); ok {
		scriptConfig.WorkDir = workDir
	}

	return scriptConfig, nil
}

// resolveVariables 解析脚本中的变量引用。
func (e *ScriptExecutor) resolveVariables(script string, execCtx *ExecutionContext) string {
	if execCtx == nil {
		return script
	}

	result := script
	evalCtx := execCtx.ToEvaluationContext()

	for key, value := range evalCtx {
		placeholder := fmt.Sprintf("${%s}", key)
		if strings.Contains(result, placeholder) {
			result = strings.ReplaceAll(result, placeholder, fmt.Sprintf("%v", value))
		}

		// 处理嵌套访问
		if m, ok := value.(map[string]any); ok {
			for subKey, subValue := range m {
				nestedPlaceholder := fmt.Sprintf("${%s.%s}", key, subKey)
				if strings.Contains(result, nestedPlaceholder) {
					result = strings.ReplaceAll(result, nestedPlaceholder, fmt.Sprintf("%v", subValue))
				}
			}
		}
	}

	return result
}

// init 在默认注册表中注册脚本执行器。
func init() {
	MustRegister(NewScriptExecutor())
}
