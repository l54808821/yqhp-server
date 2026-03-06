package ai

import (
	"encoding/json"
	"regexp"
	"strings"

	"github.com/cloudwego/eino/schema"

	"yqhp/workflow-engine/pkg/logger"
)

// PlanStepDef 计划步骤定义
type PlanStepDef struct {
	Step int    `json:"step"`
	Task string `json:"task"`
}

// parsePlanFromToolCall 从 create_plan 工具调用中解析计划步骤
func parsePlanFromToolCall(toolCalls []schema.ToolCall) ([]PlanStepDef, bool) {
	for _, tc := range toolCalls {
		if tc.Function.Name != createPlanToolName {
			continue
		}
		var args struct {
			Steps []PlanStepDef `json:"steps"`
		}
		if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
			logger.Warn("[Plan] create_plan 工具参数解析失败: %v", err)
			continue
		}
		if len(args.Steps) > 0 {
			return args.Steps, true
		}
	}
	return nil, false
}

// parsePlanFromText 从 LLM 文本输出中解析计划步骤（多层回退）
func parsePlanFromText(content string) []PlanStepDef {
	trimmed := strings.TrimSpace(content)

	// 策略 1: 尝试提取 JSON 数组
	if steps := tryParseJSONArray(trimmed); len(steps) > 0 {
		return steps
	}

	// 策略 2: 尝试提取 JSON 代码块
	if steps := tryParseJSONCodeBlock(trimmed); len(steps) > 0 {
		return steps
	}

	// 策略 3: 解析编号列表（支持多种格式）
	if steps := parseNumberedListEnhanced(trimmed); len(steps) > 0 {
		logger.Warn("[Plan] Plan JSON 解析失败，回退到编号列表解析，提取到 %d 个步骤", len(steps))
		return steps
	}

	// 策略 4: 解析 markdown 列表
	if steps := parseMarkdownList(trimmed); len(steps) > 0 {
		logger.Warn("[Plan] Plan 回退到 markdown 列表解析，提取到 %d 个步骤", len(steps))
		return steps
	}

	// 最终兜底: 整段内容作为单一步骤
	logger.Warn("[Plan] Plan 步骤解析失败，将整段内容作为单一步骤执行")
	return []PlanStepDef{{Step: 1, Task: trimmed}}
}

func tryParseJSONArray(content string) []PlanStepDef {
	start := strings.Index(content, "[")
	end := strings.LastIndex(content, "]")
	if start < 0 || end <= start {
		return nil
	}
	jsonContent := content[start : end+1]

	var rawSteps []PlanStepDef
	if err := json.Unmarshal([]byte(jsonContent), &rawSteps); err != nil {
		return nil
	}

	var valid []PlanStepDef
	for i, s := range rawSteps {
		if s.Task != "" {
			if s.Step == 0 {
				s.Step = i + 1
			}
			valid = append(valid, s)
		}
	}
	return valid
}

var jsonCodeBlockRe = regexp.MustCompile("(?s)```(?:json)?\\s*\\n(\\[.*?\\])\\s*\\n```")

func tryParseJSONCodeBlock(content string) []PlanStepDef {
	matches := jsonCodeBlockRe.FindStringSubmatch(content)
	if len(matches) < 2 {
		return nil
	}
	return tryParseJSONArray(matches[1])
}

var numberedListRe = regexp.MustCompile(`^(\d+)\s*[.)、]\s*(.+)`)

func parseNumberedListEnhanced(content string) []PlanStepDef {
	var steps []PlanStepDef
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		matches := numberedListRe.FindStringSubmatch(line)
		if len(matches) >= 3 {
			task := strings.TrimSpace(matches[2])
			task = strings.TrimPrefix(task, "**")
			task = strings.TrimSuffix(task, "**")
			task = strings.TrimSpace(task)
			if task != "" {
				steps = append(steps, PlanStepDef{
					Step: len(steps) + 1,
					Task: task,
				})
			}
		}
	}
	return steps
}

var markdownListRe = regexp.MustCompile(`^[-*+]\s+(.+)`)

func parseMarkdownList(content string) []PlanStepDef {
	var steps []PlanStepDef
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		matches := markdownListRe.FindStringSubmatch(line)
		if len(matches) >= 2 {
			task := strings.TrimSpace(matches[1])
			if task != "" {
				steps = append(steps, PlanStepDef{
					Step: len(steps) + 1,
					Task: task,
				})
			}
		}
	}
	return steps
}

// stepsToTasks 将 PlanStepDef 列表转为任务字符串列表
func stepsToTasks(steps []PlanStepDef) []string {
	tasks := make([]string, len(steps))
	for i, s := range steps {
		tasks[i] = s.Task
	}
	return tasks
}
