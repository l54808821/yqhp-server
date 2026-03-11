package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"yqhp/workflow-engine/internal/executor"
	"yqhp/workflow-engine/pkg/logger"
	"yqhp/workflow-engine/pkg/types"
)

type FindSkillsTool struct {
	config *AIConfig
}

func NewFindSkillsTool(config *AIConfig) *FindSkillsTool {
	return &FindSkillsTool{config: config}
}

func (t *FindSkillsTool) Definition() *types.ToolDefinition {
	return &types.ToolDefinition{
		Name: "find_skills",
		Description: `搜索可用的专业技能（Skill）。根据关键词在本地系统和远程技能市场（skills.sh）中查找匹配的 Skill。
找到本地 Skill 后，用 use_skill 工具加载其完整操作指令。
找到远程 Skill 后，用 install_skill 工具安装到系统，再用 use_skill 加载。`,
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"query": {
					"type": "string",
					"description": "搜索关键词，描述你需要的能力（如「代码审查」「PPT生成」「数据分析」）"
				},
				"source": {
					"type": "string",
					"enum": ["local", "remote", "all"],
					"description": "搜索范围：local=仅本地已安装, remote=仅远程市场, all=全部。默认 all"
				}
			},
			"required": ["query"]
		}`),
	}
}

type localSkillResult struct {
	ID          int64    `json:"id"`
	Name        string   `json:"name"`
	Slug        string   `json:"slug"`
	Description string   `json:"description"`
	Category    string   `json:"category"`
	Tags        []string `json:"tags"`
	Type        int32    `json:"type"`
	Author      string   `json:"author"`
}

type remoteSkillResult struct {
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	Description string `json:"description"`
	Repository  string `json:"repository"`
	Owner       string `json:"owner"`
	Stars       int    `json:"stars"`
	SkillPath   string `json:"skill_path"`
}

func (t *FindSkillsTool) Execute(ctx context.Context, arguments string, execCtx *executor.ExecutionContext) (*types.ToolResult, error) {
	var args struct {
		Query  string `json:"query"`
		Source string `json:"source"`
	}
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return types.NewErrorResult(fmt.Sprintf("参数解析失败: %v", err)), nil
	}
	if args.Query == "" {
		return types.NewErrorResult("缺少 query 参数"), nil
	}
	if args.Source == "" {
		args.Source = "all"
	}

	guluHost := getGuluHost(t.config)
	var sb strings.Builder

	if args.Source == "local" || args.Source == "all" {
		localResults, err := t.searchLocal(ctx, guluHost, args.Query)
		if err != nil {
			logger.Warn("[FindSkills] 本地搜索失败: %v", err)
			sb.WriteString(fmt.Sprintf("本地搜索出错: %v\n\n", err))
		} else if len(localResults) > 0 {
			sb.WriteString(fmt.Sprintf("## 本地已安装 Skill（%d 个）\n\n", len(localResults)))
			for i, r := range localResults {
				sb.WriteString(fmt.Sprintf("%d. **%s** (id=%d)", i+1, r.Name, r.ID))
				if r.Category != "" {
					sb.WriteString(fmt.Sprintf(" [%s]", r.Category))
				}
				sb.WriteString("\n")
				if r.Description != "" {
					sb.WriteString(fmt.Sprintf("   %s\n", r.Description))
				}
				sb.WriteString("\n")
			}
			sb.WriteString("使用 `use_skill` 工具加载任一 Skill 的完整操作指令。\n\n")
		} else {
			sb.WriteString("本地未找到匹配的 Skill。\n\n")
		}
	}

	if args.Source == "remote" || args.Source == "all" {
		remoteResults, err := t.searchRemote(ctx, guluHost, args.Query)
		if err != nil {
			logger.Warn("[FindSkills] 远程搜索失败: %v", err)
			sb.WriteString(fmt.Sprintf("远程市场搜索出错: %v\n\n", err))
		} else if len(remoteResults) > 0 {
			sb.WriteString(fmt.Sprintf("## 远程技能市场（%d 个）\n\n", len(remoteResults)))
			for i, r := range remoteResults {
				sb.WriteString(fmt.Sprintf("%d. **%s**", i+1, r.Name))
				if r.Owner != "" {
					sb.WriteString(fmt.Sprintf(" by %s", r.Owner))
				}
				if r.Stars > 0 {
					sb.WriteString(fmt.Sprintf(" (%d stars)", r.Stars))
				}
				sb.WriteString("\n")
				if r.Description != "" {
					sb.WriteString(fmt.Sprintf("   %s\n", r.Description))
				}
				if r.SkillPath != "" {
					sb.WriteString(fmt.Sprintf("   安装路径: %s\n", r.SkillPath))
				}
				sb.WriteString("\n")
			}
			sb.WriteString("使用 `install_skill` 工具安装远程 Skill 后，再用 `use_skill` 加载。\n\n")
		} else {
			sb.WriteString("远程市场未找到匹配的 Skill。\n\n")
		}
	}

	result := sb.String()
	if result == "" {
		result = fmt.Sprintf("未找到与 %q 相关的 Skill", args.Query)
	}
	return types.NewSilentResult(result), nil
}

func (t *FindSkillsTool) searchLocal(ctx context.Context, guluHost, query string) ([]localSkillResult, error) {
	reqURL := fmt.Sprintf("%s/api/internal/skills/search?q=%s&limit=10", guluHost, url.QueryEscape(query))
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Code int                `json:"code"`
		Data []localSkillResult `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("响应解析失败: %w", err)
	}
	return result.Data, nil
}

func (t *FindSkillsTool) searchRemote(ctx context.Context, guluHost, query string) ([]remoteSkillResult, error) {
	reqURL := fmt.Sprintf("%s/api/internal/skillshub/search?q=%s", guluHost, url.QueryEscape(query))
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Code int                 `json:"code"`
		Data []remoteSkillResult `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("响应解析失败: %w", err)
	}
	return result.Data, nil
}
