package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"yqhp/workflow-engine/internal/executor"
	"yqhp/workflow-engine/pkg/types"
)

// InstallSkillTool 从远程技能市场安装 Skill 到系统
type InstallSkillTool struct {
	config *AIConfig
}

func NewInstallSkillTool(config *AIConfig) *InstallSkillTool {
	return &InstallSkillTool{config: config}
}

func (t *InstallSkillTool) Definition() *types.ToolDefinition {
	return &types.ToolDefinition{
		Name: "install_skill",
		Description: `从远程技能市场（skills.sh / GitHub）安装 Skill 到系统。
安装后可用 use_skill 工具加载其操作指令。
需要提供 find_skills 搜索结果中的安装路径（path 格式为 owner/repo/skill-name）。`,
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"path": {
					"type": "string",
					"description": "Skill 的安装路径，格式为 owner/repo/skill-name（来自 find_skills 远程搜索结果）"
				}
			},
			"required": ["path"]
		}`),
	}
}

func (t *InstallSkillTool) Execute(ctx context.Context, arguments string, execCtx *executor.ExecutionContext) (*types.ToolResult, error) {
	var args struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return types.NewErrorResult(fmt.Sprintf("参数解析失败: %v", err)), nil
	}
	if args.Path == "" {
		return types.NewErrorResult("缺少 path 参数"), nil
	}

	guluHost := getGuluHost(t.config)
	reqBody, _ := json.Marshal(map[string]string{"path": args.Path})

	reqURL := fmt.Sprintf("%s/api/internal/skillshub/install", guluHost)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, bytes.NewReader(reqBody))
	if err != nil {
		return types.NewErrorResult(fmt.Sprintf("创建请求失败: %v", err)), nil
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := (&http.Client{Timeout: 60 * time.Second}).Do(httpReq)
	if err != nil {
		return types.NewErrorResult(fmt.Sprintf("安装请求失败: %v", err)), nil
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return types.NewErrorResult(fmt.Sprintf("安装失败 (HTTP %d): %s", resp.StatusCode, string(body))), nil
	}

	var result struct {
		Code int             `json:"code"`
		Data json.RawMessage `json:"data"`
		Msg  string          `json:"msg"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return types.NewErrorResult(fmt.Sprintf("响应解析失败: %s", string(body))), nil
	}
	if result.Code != 0 {
		errMsg := result.Msg
		if errMsg == "" {
			errMsg = string(body)
		}
		return types.NewErrorResult(fmt.Sprintf("安装失败: %s", errMsg)), nil
	}

	var skillData struct {
		ID   int64  `json:"id"`
		Name string `json:"name"`
		Slug string `json:"slug"`
	}
	if err := json.Unmarshal(result.Data, &skillData); err != nil {
		return types.NewSilentResult(fmt.Sprintf("Skill 安装成功！响应: %s\n\n请使用 find_skills 搜索刚安装的 Skill。", string(result.Data))), nil
	}

	return types.NewSilentResult(fmt.Sprintf(
		"Skill 安装成功！\n- 名称: %s\n- ID: %d\n\n现在可以使用 `use_skill` 工具加载此 Skill 的操作指令（skill_id=%d）。",
		skillData.Name, skillData.ID, skillData.ID,
	)), nil
}
