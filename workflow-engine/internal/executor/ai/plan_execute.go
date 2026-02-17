package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"yqhp/workflow-engine/internal/executor"
	"yqhp/workflow-engine/pkg/types"
)

// executePlanAndExecute 执行 Plan-and-Execute 模式
// 流程：规划 → 逐步执行（可使用工具）→ 汇总
func (e *AIExecutor) executePlanAndExecute(
	ctx context.Context,
	chatModel model.ChatModel,
	config *AIConfig,
	stepID string,
	execCtx *executor.ExecutionContext,
	aiCallback types.AICallback,
) (*AIOutput, error) {
	output := &AIOutput{
		Model: config.Model,
		AgentTrace: &AgentTrace{
			Mode:           "plan_and_execute",
			PlanAndExecute: &PlanExecTrace{},
		},
	}

	trace := output.AgentTrace.PlanAndExecute

	// ========== Phase 1: 规划 ==========
	planMessages := []*schema.Message{
		schema.SystemMessage(config.SystemPrompt),
		schema.UserMessage(config.Prompt + "\n\n" + planningInstruction),
	}

	planResp, err := chatModel.Generate(ctx, planMessages)
	if err != nil {
		return nil, fmt.Errorf("规划阶段失败: %w", err)
	}

	e.accumulateTokens(output, planResp)
	trace.Plan = planResp.Content

	// 通知前端：规划完成
	if aiCallback != nil {
		if tc, ok := aiCallback.(types.AIThinkingCallback); ok {
			tc.OnAIThinking(ctx, stepID, 0, "[Plan]\n"+planResp.Content)
		}
	}

	// 解析计划步骤
	planSteps, err := parsePlanSteps(planResp.Content)
	if err != nil {
		log.Printf("[WARN] 解析计划步骤失败，尝试直接使用原始计划: %v", err)
		planSteps = []parsedStep{{Step: 1, Task: config.Prompt}}
	}

	// 限制最大步骤数
	if len(planSteps) > defaultMaxPlanSteps {
		planSteps = planSteps[:defaultMaxPlanSteps]
	}

	// 初始化步骤记录
	for _, ps := range planSteps {
		trace.Steps = append(trace.Steps, PlanStep{
			Index:  ps.Step,
			Task:   ps.Task,
			Status: "pending",
		})
	}

	// ========== Phase 2: 逐步执行 ==========
	var previousResults []string

	for i, ps := range planSteps {
		// 构建上下文信息
		contextInfo := ""
		if len(previousResults) > 0 {
			contextInfo = "已完成步骤的结果：\n"
			for j, r := range previousResults {
				contextInfo += fmt.Sprintf("第 %d 步结果：%s\n", j+1, r)
			}
		}

		stepPrompt := fmt.Sprintf(planStepExecutionInstruction, trace.Plan, ps.Step, ps.Task, contextInfo)
		stepMessages := []*schema.Message{
			schema.SystemMessage(config.SystemPrompt),
			schema.UserMessage(stepPrompt),
		}

		trace.Steps[i].Status = "running"

		// 通知前端：步骤开始
		if aiCallback != nil {
			if tc, ok := aiCallback.(types.AIThinkingCallback); ok {
				tc.OnAIThinking(ctx, stepID, ps.Step, fmt.Sprintf("[Step %d] %s", ps.Step, ps.Task))
			}
		}

		// 执行步骤（如果有工具则使用工具）
		var stepContent string
		var stepToolCalls []ToolCallRecord

		if e.hasTools(config) {
			stepOutput, stepErr := e.executeWithTools(ctx, chatModel, stepMessages, config, stepID, execCtx, aiCallback)
			if stepErr != nil {
				trace.Steps[i].Status = "failed"
				trace.Steps[i].Result = fmt.Sprintf("执行失败: %v", stepErr)
				previousResults = append(previousResults, trace.Steps[i].Result)
				continue
			}
			stepContent = stepOutput.Content
			stepToolCalls = stepOutput.ToolCalls
			e.accumulateTokensFromOutput(output, stepOutput)
		} else {
			stepResp, stepErr := chatModel.Generate(ctx, stepMessages)
			if stepErr != nil {
				trace.Steps[i].Status = "failed"
				trace.Steps[i].Result = fmt.Sprintf("执行失败: %v", stepErr)
				previousResults = append(previousResults, trace.Steps[i].Result)
				continue
			}
			stepContent = stepResp.Content
			e.accumulateTokens(output, stepResp)
		}

		trace.Steps[i].Status = "completed"
		trace.Steps[i].Result = stepContent
		trace.Steps[i].ToolCalls = stepToolCalls
		previousResults = append(previousResults, stepContent)
	}

	// ========== Phase 3: 汇总 ==========
	allResults := ""
	for i, r := range previousResults {
		allResults += fmt.Sprintf("第 %d 步：%s\n\n", i+1, r)
	}

	synthesisPrompt := fmt.Sprintf(planSynthesisInstruction, config.Prompt, allResults)
	synthesisMessages := []*schema.Message{
		schema.SystemMessage(config.SystemPrompt),
		schema.UserMessage(synthesisPrompt),
	}

	// 通知前端：汇总阶段
	if aiCallback != nil {
		if tc, ok := aiCallback.(types.AIThinkingCallback); ok {
			tc.OnAIThinking(ctx, stepID, len(planSteps)+1, "[Synthesis] 汇总所有步骤结果...")
		}
	}

	if config.Streaming && aiCallback != nil {
		synthesisOutput, synthesisErr := e.executeStream(ctx, chatModel, synthesisMessages, stepID, config, aiCallback)
		if synthesisErr != nil {
			return nil, fmt.Errorf("汇总阶段失败: %w", synthesisErr)
		}
		output.Content = synthesisOutput.Content
		e.accumulateTokensFromOutput(output, synthesisOutput)
	} else {
		synthesisResp, synthesisErr := chatModel.Generate(ctx, synthesisMessages)
		if synthesisErr != nil {
			return nil, fmt.Errorf("汇总阶段失败: %w", synthesisErr)
		}
		e.accumulateTokens(output, synthesisResp)
		output.Content = synthesisResp.Content
	}

	trace.Synthesis = output.Content
	return output, nil
}

// accumulateTokens 从 schema.Message 累加 token 使用
func (e *AIExecutor) accumulateTokens(output *AIOutput, resp *schema.Message) {
	if resp.ResponseMeta != nil {
		if resp.ResponseMeta.Usage != nil {
			output.PromptTokens += resp.ResponseMeta.Usage.PromptTokens
			output.CompletionTokens += resp.ResponseMeta.Usage.CompletionTokens
			output.TotalTokens += resp.ResponseMeta.Usage.TotalTokens
		}
		if resp.ResponseMeta.FinishReason != "" {
			output.FinishReason = string(resp.ResponseMeta.FinishReason)
		}
	}
}

// accumulateTokensFromOutput 从另一个 AIOutput 累加 token 使用
func (e *AIExecutor) accumulateTokensFromOutput(target *AIOutput, source *AIOutput) {
	target.PromptTokens += source.PromptTokens
	target.CompletionTokens += source.CompletionTokens
	target.TotalTokens += source.TotalTokens
}

// parsedStep 解析后的计划步骤
type parsedStep struct {
	Step int    `json:"step"`
	Task string `json:"task"`
}

// parsePlanSteps 解析 LLM 输出的计划步骤 JSON
func parsePlanSteps(content string) ([]parsedStep, error) {
	content = strings.TrimSpace(content)

	// 尝试提取 JSON 数组（可能被 markdown 代码块包裹）
	if idx := strings.Index(content, "["); idx >= 0 {
		if endIdx := strings.LastIndex(content, "]"); endIdx > idx {
			content = content[idx : endIdx+1]
		}
	}

	var steps []parsedStep
	if err := json.Unmarshal([]byte(content), &steps); err != nil {
		return nil, fmt.Errorf("JSON 解析失败: %w", err)
	}

	if len(steps) == 0 {
		return nil, fmt.Errorf("计划步骤为空")
	}

	return steps, nil
}
