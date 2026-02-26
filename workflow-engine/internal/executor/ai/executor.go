// Package ai 实现 AI 节点执行器，支持多种 LLM 提供商、流式输出、
// 工具调用（内置工具 + MCP + Skill）、人机交互等能力。
package ai

import (
	"context"
	"time"

	"yqhp/workflow-engine/internal/executor"
	"yqhp/workflow-engine/pkg/types"
)

// AIExecutor AI 节点执行器
type AIExecutor struct {
	*executor.BaseExecutor
}

// NewAIExecutor 创建 AI 执行器
func NewAIExecutor() *AIExecutor {
	return &AIExecutor{
		BaseExecutor: executor.NewBaseExecutor(AIExecutorType),
	}
}

// Init 初始化 AI 执行器
func (e *AIExecutor) Init(ctx context.Context, config map[string]any) error {
	return e.BaseExecutor.Init(ctx, config)
}

// Execute 执行 AI 节点
func (e *AIExecutor) Execute(ctx context.Context, step *types.Step, execCtx *executor.ExecutionContext) (*types.StepResult, error) {
	startTime := time.Now()

	config, err := e.parseConfig(step.Config)
	if err != nil {
		return executor.CreateFailedResult(step.ID, startTime, err), nil
	}

	config = e.resolveVariables(config, execCtx)

	chatModel, err := e.createChatModel(ctx, config)
	if err != nil {
		return executor.CreateFailedResult(step.ID, startTime, executor.NewExecutionError(step.ID, "创建 AI 模型失败", err)), nil
	}

	// 从执行上下文中提取对话历史（AI 工作流多轮记忆）
	var chatHistoryMaps []map[string]any
	if execCtx != nil && execCtx.Variables != nil {
		if history, ok := execCtx.Variables["__chat_history__"]; ok {
			if historySlice, ok := history.([]interface{}); ok {
				for _, item := range historySlice {
					if m, ok := item.(map[string]interface{}); ok {
						chatHistoryMaps = append(chatHistoryMaps, m)
					}
				}
			}
		}
		// 如果有 __user_message__，用它覆盖 config.Prompt
		if userMsg, ok := execCtx.Variables["__user_message__"].(string); ok && userMsg != "" {
			config.Prompt = userMsg
		}
	}

	messages := e.buildMessages(config, chatHistoryMaps...)

	timeout := step.Timeout
	if timeout <= 0 && config.Timeout > 0 {
		timeout = time.Duration(config.Timeout) * time.Second
	}
	if timeout <= 0 {
		timeout = defaultAITimeout
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var output *AIOutput
	var aiCallback types.AICallback
	if execCtx.Callback != nil {
		if cb, ok := execCtx.Callback.(types.AICallback); ok {
			aiCallback = cb
		}
	}

	// 根据 agent_mode 分发执行流程
	switch config.AgentMode {
	case "plan_and_execute":
		output, err = e.executePlanAndExecute(ctx, chatModel, config, step.ID, execCtx, aiCallback)
		if err == nil && output != nil && aiCallback != nil && config.Streaming {
			aiCallback.OnAIComplete(ctx, step.ID, &types.AIResult{
				Content:          output.Content,
				PromptTokens:     output.PromptTokens,
				CompletionTokens: output.CompletionTokens,
				TotalTokens:      output.TotalTokens,
			})
		}

	case "reflection":
		output, err = e.executeReflection(ctx, chatModel, messages, config, step.ID, execCtx, aiCallback)
		if err == nil && output != nil && aiCallback != nil && config.Streaming {
			aiCallback.OnAIComplete(ctx, step.ID, &types.AIResult{
				Content:          output.Content,
				PromptTokens:     output.PromptTokens,
				CompletionTokens: output.CompletionTokens,
				TotalTokens:      output.TotalTokens,
			})
		}

	default:
		// 默认模式（含 react）：使用原有的工具调用 / 无工具模式
		if e.hasTools(config) {
			output, err = e.executeWithTools(ctx, chatModel, messages, config, step.ID, execCtx, aiCallback)
			if err == nil && output != nil && aiCallback != nil && config.Streaming {
				aiCallback.OnAIComplete(ctx, step.ID, &types.AIResult{
					Content:          output.Content,
					PromptTokens:     output.PromptTokens,
					CompletionTokens: output.CompletionTokens,
					TotalTokens:      output.TotalTokens,
				})
			}
		} else {
			if config.Streaming && aiCallback != nil {
				output, err = e.executeStream(ctx, chatModel, messages, step.ID, config, aiCallback)
			} else {
				output, err = e.executeNonStream(ctx, chatModel, messages, config)
			}
		}
	}

	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return executor.CreateTimeoutResult(step.ID, startTime, timeout), nil
		}
		if aiCallback != nil {
			aiCallback.OnAIError(ctx, step.ID, err)
		}
		return executor.CreateFailedResult(step.ID, startTime, executor.NewExecutionError(step.ID, "AI 调用失败", err)), nil
	}

	// 将解析后的 prompt 写入输出，方便调试时查看实际输入
	output.SystemPrompt = config.SystemPrompt
	output.Prompt = config.Prompt

	// 执行后置处理器（extract_param、js_script、assertion 等）
	e.executePostProcessors(ctx, step, execCtx, output, startTime)

	result := executor.CreateSuccessResult(step.ID, startTime, output)
	result.Metrics["ai_prompt_tokens"] = float64(output.PromptTokens)
	result.Metrics["ai_completion_tokens"] = float64(output.CompletionTokens)
	result.Metrics["ai_total_tokens"] = float64(output.TotalTokens)

	return result, nil
}

// Cleanup 清理资源
func (e *AIExecutor) Cleanup(ctx context.Context) error {
	return nil
}

func init() {
	executor.MustRegister(NewAIExecutor())
}
