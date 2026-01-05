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

	"github.com/grafana/k6/workflow-engine/pkg/types"
)

const (
	// ScriptExecutorType is the type identifier for script executor.
	ScriptExecutorType = "script"

	// Default timeout for script execution.
	defaultScriptTimeout = 60 * time.Second
)

// ScriptExecutor executes script steps.
type ScriptExecutor struct {
	*BaseExecutor
	shell     string
	shellArgs []string
}

// NewScriptExecutor creates a new script executor.
func NewScriptExecutor() *ScriptExecutor {
	return &ScriptExecutor{
		BaseExecutor: NewBaseExecutor(ScriptExecutorType),
	}
}

// Init initializes the script executor with configuration.
func (e *ScriptExecutor) Init(ctx context.Context, config map[string]any) error {
	if err := e.BaseExecutor.Init(ctx, config); err != nil {
		return err
	}

	// Determine shell based on OS
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
		// Parse shell args from config
		if args, ok := config["shell_args"].([]any); ok {
			for _, arg := range args {
				if s, ok := arg.(string); ok {
					e.shellArgs = append(e.shellArgs, s)
				}
			}
		} else {
			// Default args for common shells
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

// Execute executes a script step.
func (e *ScriptExecutor) Execute(ctx context.Context, step *types.Step, execCtx *ExecutionContext) (*types.StepResult, error) {
	startTime := time.Now()

	// Parse step configuration
	config, err := e.parseConfig(step.Config)
	if err != nil {
		return CreateFailedResult(step.ID, startTime, err), nil
	}

	// Resolve variables in script
	script := e.resolveVariables(config.Script, execCtx)

	// Apply step timeout if specified
	timeout := step.Timeout
	if timeout <= 0 {
		timeout = e.GetConfigDuration("timeout", defaultScriptTimeout)
	}

	// Create command context with timeout
	cmdCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Build command
	args := append(e.shellArgs, script)
	cmd := exec.CommandContext(cmdCtx, e.shell, args...)

	// Set environment variables
	cmd.Env = os.Environ()
	for k, v := range config.Env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}

	// Add execution context variables to environment
	if execCtx != nil {
		for k, v := range execCtx.Variables {
			cmd.Env = append(cmd.Env, fmt.Sprintf("WF_%s=%v", strings.ToUpper(k), v))
		}
	}

	// Set working directory
	if config.WorkDir != "" {
		cmd.Dir = config.WorkDir
	}

	// Capture output
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Execute command
	err = cmd.Run()

	// Build output
	output := &ScriptOutput{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: 0,
	}

	if err != nil {
		// Check if it was a timeout
		if cmdCtx.Err() == context.DeadlineExceeded {
			return CreateTimeoutResult(step.ID, startTime, timeout), nil
		}

		// Get exit code if available
		if exitErr, ok := err.(*exec.ExitError); ok {
			output.ExitCode = exitErr.ExitCode()
		} else {
			output.ExitCode = -1
		}

		result := CreateFailedResult(step.ID, startTime, NewExecutionError(step.ID, "script execution failed", err))
		result.Output = output
		return result, nil
	}

	// Create success result
	result := CreateSuccessResult(step.ID, startTime, output)
	result.Metrics["script_exit_code"] = float64(output.ExitCode)

	return result, nil
}

// Cleanup releases resources held by the script executor.
func (e *ScriptExecutor) Cleanup(ctx context.Context) error {
	return nil
}

// ScriptConfig represents the configuration for a script step.
type ScriptConfig struct {
	Script  string            // Inline script content
	File    string            // Script file path (alternative to inline)
	Env     map[string]string // Environment variables
	WorkDir string            // Working directory
}

// ScriptOutput represents the output of a script execution.
type ScriptOutput struct {
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	ExitCode int    `json:"exit_code"`
}

// parseConfig parses the step configuration into ScriptConfig.
func (e *ScriptExecutor) parseConfig(config map[string]any) (*ScriptConfig, error) {
	scriptConfig := &ScriptConfig{
		Env: make(map[string]string),
	}

	// Get inline script
	if inline, ok := config["inline"].(string); ok {
		scriptConfig.Script = inline
	}

	// Get script file
	if file, ok := config["file"].(string); ok {
		scriptConfig.File = file
	}

	// If file is specified, read its content
	if scriptConfig.File != "" && scriptConfig.Script == "" {
		content, err := os.ReadFile(scriptConfig.File)
		if err != nil {
			return nil, NewConfigError(fmt.Sprintf("failed to read script file: %s", scriptConfig.File), err)
		}
		scriptConfig.Script = string(content)
	}

	// Validate that we have a script
	if scriptConfig.Script == "" {
		return nil, NewConfigError("script step requires 'inline' or 'file' configuration", nil)
	}

	// Get environment variables
	if env, ok := config["env"].(map[string]any); ok {
		for k, v := range env {
			if s, ok := v.(string); ok {
				scriptConfig.Env[k] = s
			}
		}
	}

	// Get working directory
	if workDir, ok := config["work_dir"].(string); ok {
		scriptConfig.WorkDir = workDir
	}

	return scriptConfig, nil
}

// resolveVariables resolves variable references in the script.
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

		// Handle nested access
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

// init registers the script executor with the default registry.
func init() {
	MustRegister(NewScriptExecutor())
}
