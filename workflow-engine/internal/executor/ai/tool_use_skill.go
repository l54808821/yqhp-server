package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"yqhp/workflow-engine/internal/executor"
	"yqhp/workflow-engine/pkg/logger"
	"yqhp/workflow-engine/pkg/types"
)

// UseSkillTool 加载 Skill 的完整操作指令（替代旧的 read_skill）。
// 优先从预加载列表查找，未命中则动态调用 gulu API 获取。
type UseSkillTool struct {
	config    *AIConfig
	preloaded []*SkillInfo
}

func NewUseSkillTool(config *AIConfig) *UseSkillTool {
	return &UseSkillTool{
		config:    config,
		preloaded: config.Skills,
	}
}

func (t *UseSkillTool) Definition() *types.ToolDefinition {
	desc := "加载专业技能（Skill）的完整操作指令。加载后按指令使用现有工具执行任务。支持通过 Skill ID 或名称加载。"
	if len(t.preloaded) > 0 {
		desc += "\n\n已绑定的 Skills（可直接使用）：\n"
		for _, s := range t.preloaded {
			desc += fmt.Sprintf("- %s (id=%d): %s\n", s.Name, s.ID, s.Description)
		}
	}
	desc += "\n通过 find_skills 工具发现的 Skill 也可用此工具加载。"

	return &types.ToolDefinition{
		Name:        "use_skill",
		Description: desc,
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"skill_id": {
					"type": "integer",
					"description": "Skill ID（来自 find_skills 结果或已绑定列表）"
				},
				"name": {
					"type": "string",
					"description": "Skill 名称（当不知道 ID 时可按名称查找）"
				}
			}
		}`),
	}
}

func (t *UseSkillTool) Execute(ctx context.Context, arguments string, execCtx *executor.ExecutionContext) (*types.ToolResult, error) {
	var args struct {
		SkillID int64  `json:"skill_id"`
		Name    string `json:"name"`
	}
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return types.NewErrorResult(fmt.Sprintf("参数解析失败: %v", err)), nil
	}
	if args.SkillID == 0 && args.Name == "" {
		return types.NewErrorResult("请提供 skill_id 或 name 参数"), nil
	}

	// 优先从预加载列表查找（已绑定的 Skill 数据已在内存中）
	for _, s := range t.preloaded {
		if (args.SkillID > 0 && s.ID == args.SkillID) || (args.Name != "" && s.Name == args.Name) {
			if s.Body != "" {
				return types.NewSilentResult(s.Body), nil
			}
			break
		}
	}

	// 内存中未找到或没有 body -> 调用 gulu API 动态获取
	if args.SkillID > 0 {
		body, err := t.fetchSkillBody(ctx, args.SkillID)
		if err != nil {
			return types.NewErrorResult(fmt.Sprintf("获取 Skill 失败: %v", err)), nil
		}
		return types.NewSilentResult(body), nil
	}

	return types.NewErrorResult(fmt.Sprintf("Skill 未找到: %s。请先使用 find_skills 工具搜索可用的 Skill", args.Name)), nil
}

func (t *UseSkillTool) fetchSkillBody(ctx context.Context, skillID int64) (string, error) {
	guluHost := getGuluHost(t.config)
	reqURL := fmt.Sprintf("%s/api/internal/skills/%d/body", guluHost, skillID)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return "", err
	}

	resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(httpReq)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Code int `json:"code"`
		Data struct {
			Skill struct {
				Name        string `json:"name"`
				Description string `json:"description"`
			} `json:"skill"`
			Body string `json:"body"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("响应解析失败: %w", err)
	}

	if result.Data.Body == "" {
		logger.Warn("[UseSkill] Skill %d body 为空", skillID)
		return "此 Skill 没有操作指令（SKILL.md 为空）", nil
	}

	return result.Data.Body, nil
}
