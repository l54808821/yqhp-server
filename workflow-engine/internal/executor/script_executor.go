package executor

import (
	"context"
	"fmt"
	"strings"
	"time"

	"yqhp/workflow-engine/pkg/script"
	"yqhp/workflow-engine/pkg/types"
)

const (
	// ScriptExecutorType 是脚本执行器的类型标识符。
	ScriptExecutorType = "script"

	// 脚本执行的默认超时时间。
	defaultScriptTimeout = 60 * time.Second

	// 支持的脚本语言
	LanguageJavaScript = "javascript"
)

// ScriptExecutor 执行脚本步骤。
type ScriptExecutor struct {
	*BaseExecutor
}

// NewScriptExecutor 创建一个新的脚本执行器。
func NewScriptExecutor() *ScriptExecutor {
	return &ScriptExecutor{
		BaseExecutor: NewBaseExecutor(ScriptExecutorType),
	}
}

// Init 使用配置初始化脚本执行器。
func (e *ScriptExecutor) Init(ctx context.Context, config map[string]any) error {
	return e.BaseExecutor.Init(ctx, config)
}

// Execute 执行脚本步骤。
func (e *ScriptExecutor) Execute(ctx context.Context, step *types.Step, execCtx *ExecutionContext) (*types.StepResult, error) {
	startTime := time.Now()

	// 解析步骤配置
	config, err := e.parseConfig(step.Config)
	if err != nil {
		return CreateFailedResult(step.ID, startTime, err), nil
	}

	// 根据语言选择执行方式
	switch config.Language {
	case LanguageJavaScript, "js", "":
		return e.executeJavaScript(ctx, step, execCtx, config, startTime)
	default:
		return CreateFailedResult(step.ID, startTime,
			NewConfigError(fmt.Sprintf("不支持的脚本语言: %s，当前仅支持 javascript", config.Language), nil)), nil
	}
}

// executeJavaScript 执行 JavaScript 脚本
func (e *ScriptExecutor) executeJavaScript(ctx context.Context, step *types.Step, execCtx *ExecutionContext, config *ScriptConfig, startTime time.Time) (*types.StepResult, error) {
	// 准备运行时配置
	rtConfig := &script.JSRuntimeConfig{}

	// orderedLogs 按执行时序收集所有日志（console.log + 变量变更），确保顺序正确
	var orderedLogs []types.ConsoleLogEntry

	if execCtx != nil {
		rtConfig.Variables = make(map[string]interface{}, len(execCtx.Variables))
		for k, v := range execCtx.Variables {
			rtConfig.Variables[k] = v
		}

		rtConfig.OnGetVariable = func(key string) (interface{}, bool) {
			return execCtx.GetVariable(key)
		}
		rtConfig.OnSetVariable = func(key string, value interface{}) {
			scope := "temp"
			if strings.HasPrefix(key, "env.") {
				scope = "env"
			}
			oldValue, _ := execCtx.GetVariable(key)
			execCtx.SetVariable(key, value)
			orderedLogs = append(orderedLogs, types.NewVariableChangeEntry(types.VariableChangeInfo{
				Name:     key,
				OldValue: oldValue,
				NewValue: value,
				Scope:    scope,
				Source:   "js_script",
			}))
		}
		rtConfig.OnDelVariable = func(key string) {
			execCtx.SetVariable(key, nil)
		}
		rtConfig.OnLog = func(level, message string) {
			switch level {
			case "warn":
				orderedLogs = append(orderedLogs, types.NewWarnEntry(message))
			case "error":
				orderedLogs = append(orderedLogs, types.NewErrorEntry(message))
			default:
				orderedLogs = append(orderedLogs, types.NewLogEntry(message))
			}
		}

		if len(execCtx.Results) > 0 {
			for _, result := range execCtx.Results {
				if result.Output != nil {
					rtConfig.Response = result.Output
					rtConfig.PrevResult = result.Output
				}
			}
		}
	}

	// 创建 JS 运行时
	runtime := script.NewJSRuntime(rtConfig)

	// 确定超时时间
	timeout := step.Timeout
	if timeout <= 0 {
		if config.Timeout > 0 {
			timeout = time.Duration(config.Timeout) * time.Second
		} else {
			timeout = e.GetConfigDuration("timeout", defaultScriptTimeout)
		}
	}

	result, err := runtime.Execute(config.Script, timeout)

	// 使用 orderedLogs（通过 OnLog/OnSetVariable 回调按时序收集的日志）
	// 如果没有使用回调（execCtx 为 nil），则从 result.ConsoleLogs 兜底
	var consoleLogs []types.ConsoleLogEntry
	if len(orderedLogs) > 0 {
		consoleLogs = orderedLogs
	} else {
		consoleLogs = make([]types.ConsoleLogEntry, 0, len(result.ConsoleLogs))
		for _, log := range result.ConsoleLogs {
			consoleLogs = append(consoleLogs, types.NewLogEntry(log))
		}
	}

	output := &types.ScriptResponseData{
		Script:      config.Script,
		Language:    config.Language,
		ConsoleLogs: consoleLogs,
		Variables:   result.Variables,
		DurationMs:  time.Since(startTime).Milliseconds(),
	}

	if err != nil {
		output.Error = err.Error()
		stepResult := CreateFailedResult(step.ID, startTime, NewExecutionError(step.ID, "脚本执行失败", err))
		stepResult.Output = output
		if execCtx != nil {
			execCtx.CreateVariableSnapshot()
		}
		return stepResult, nil
	}

	output.Result = result.Value

	if execCtx != nil {
		execCtx.CreateVariableSnapshot()
	}

	// 创建成功结果
	stepResult := CreateSuccessResult(step.ID, startTime, output)
	stepResult.Metrics["script_duration_ms"] = float64(output.DurationMs)

	return stepResult, nil
}

// Cleanup 释放脚本执行器持有的资源。
func (e *ScriptExecutor) Cleanup(ctx context.Context) error {
	return nil
}

// ScriptConfig 表示脚本步骤的配置。
type ScriptConfig struct {
	Language string `json:"language"` // 脚本语言: javascript
	Script   string `json:"script"`   // 脚本代码
	Timeout  int    `json:"timeout"`  // 超时时间（秒）
}

// parseConfig 将步骤配置解析为 ScriptConfig。
func (e *ScriptExecutor) parseConfig(config map[string]any) (*ScriptConfig, error) {
	scriptConfig := &ScriptConfig{
		Language: LanguageJavaScript, // 默认 JavaScript
	}

	// 获取脚本语言
	if lang, ok := config["language"].(string); ok && lang != "" {
		scriptConfig.Language = lang
	}

	// 获取脚本内容
	if scriptStr, ok := config["script"].(string); ok {
		scriptConfig.Script = scriptStr
	}

	// 验证是否有脚本
	if scriptConfig.Script == "" {
		return nil, NewConfigError("脚本步骤需要配置 'script'（脚本内容）", nil)
	}

	// 获取超时时间
	if timeout, ok := config["timeout"].(int); ok {
		scriptConfig.Timeout = timeout
	} else if timeout, ok := config["timeout"].(float64); ok {
		scriptConfig.Timeout = int(timeout)
	}

	return scriptConfig, nil
}

// init 在默认注册表中注册脚本执行器。
func init() {
	MustRegister(NewScriptExecutor())
}
