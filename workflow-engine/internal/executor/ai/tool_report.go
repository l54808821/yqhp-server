package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"yqhp/workflow-engine/internal/executor"
	"yqhp/workflow-engine/pkg/logger"
	"yqhp/workflow-engine/pkg/types"
)

var artifactSeq int64

func nextArtifactBlockID(stepID string) string {
	prefix := stepID
	if len(prefix) > 8 {
		prefix = prefix[:8]
	}
	return fmt.Sprintf("artifact_%s_%d", prefix, atomic.AddInt64(&artifactSeq, 1))
}

// ReportTool 报告生成工具，让 AI Agent 调用 LLM 生成 HTML/Markdown/PPT 格式报告
type ReportTool struct {
	config *AIConfig
	stepID string
	cb     types.AIStreamCallback
}

func NewReportTool(config *AIConfig) *ReportTool {
	return &ReportTool{config: config}
}

func (t *ReportTool) SetContext(stepID string, callback types.AIStreamCallback) {
	t.stepID = stepID
	t.cb = callback
}

func (t *ReportTool) Definition() *types.ToolDefinition {
	return &types.ToolDefinition{
		Name:        "generate_report",
		Description: "根据任务描述和上下文信息，生成专业的报告产物。支持三种输出格式：html（网页报告，含图表和交互），ppt（HTML 幻灯片演示），markdown（结构化文档）。工具会调用 LLM 生成完整内容并返回。当用户需要正式的报告、演示文稿或文档时使用此工具。",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"task": {
					"type": "string",
					"description": "报告的任务描述，明确说明报告主题、目标受众和关键要点"
				},
				"file_type": {
					"type": "string",
					"enum": ["html", "ppt", "markdown"],
					"description": "报告输出格式：html（网页报告，含 ECharts 图表），ppt（HTML 幻灯片），markdown（结构化文档）"
				},
				"context": {
					"type": "string",
					"description": "报告所需的参考资料和数据内容，将作为报告生成的知识库"
				}
			},
			"required": ["task", "file_type"]
		}`),
	}
}

func (t *ReportTool) Execute(ctx context.Context, arguments string, execCtx *executor.ExecutionContext) (*types.ToolResult, error) {
	var args struct {
		Task     string `json:"task"`
		FileType string `json:"file_type"`
		Context  string `json:"context"`
	}
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return types.NewErrorResult(fmt.Sprintf("参数解析失败: %v", err)), nil
	}
	if args.Task == "" {
		return types.NewErrorResult("缺少必填参数: task"), nil
	}
	if args.FileType == "" {
		args.FileType = "html"
	}
	if args.FileType != "html" && args.FileType != "ppt" && args.FileType != "markdown" {
		return types.NewErrorResult(fmt.Sprintf("不支持的报告格式: %s，可选值: html, ppt, markdown", args.FileType)), nil
	}

	logger.Debug("[ReportTool] 开始生成报告, type=%s, task=%s", args.FileType, truncateForLog(args.Task, 100))

	prompt := buildReportPrompt(args.Task, args.FileType, args.Context)

	chatModel, err := createChatModelFromConfig(ctx, t.config)
	if err != nil {
		return types.NewErrorResult(fmt.Sprintf("创建报告生成模型失败: %v", err)), nil
	}

	messages := []*schema.Message{
		schema.UserMessage(prompt),
	}

	artifactBlockID := nextArtifactBlockID(t.stepID)

	if t.cb != nil {
		t.cb.OnAIArtifactStart(ctx, t.stepID, artifactBlockID, args.FileType, args.Task)
	}

	var content string
	if t.cb != nil {
		content, err = t.streamGenerateWithArtifact(ctx, chatModel, messages, artifactBlockID)
	} else {
		resp, genErr := chatModel.Generate(ctx, messages)
		if genErr != nil {
			err = genErr
		} else {
			content = resp.Content
		}
	}
	if err != nil {
		return types.NewErrorResult(fmt.Sprintf("报告生成失败: %v", err)), nil
	}

	content = extractReportContent(content, args.FileType)

	if args.FileType == "html" || args.FileType == "ppt" {
		guluHost := getGuluHost(t.config)
		fileURL, uploadErr := uploadReportFile(ctx, guluHost, content, args.Task, args.FileType)
		if uploadErr != nil {
			logger.Warn("[ReportTool] 文件上传失败，返回内联内容: %v", uploadErr)
			if t.cb != nil {
				t.cb.OnAIArtifactComplete(ctx, t.stepID, artifactBlockID, "")
			}
			result := map[string]interface{}{
				"type":    args.FileType,
				"content": content,
				"inline":  true,
			}
			resultJSON, _ := json.Marshal(result)
			return types.NewUserResult(
				fmt.Sprintf("报告已生成（%s 格式），但文件上传失败，内容以内联方式返回。", args.FileType),
				string(resultJSON),
			), nil
		}

		if t.cb != nil {
			t.cb.OnAIArtifactComplete(ctx, t.stepID, artifactBlockID, fileURL)
		}

		result := map[string]interface{}{
			"type":     args.FileType,
			"url":      fileURL,
			"title":    args.Task,
			"artifact": true,
		}
		resultJSON, _ := json.Marshal(result)
		return types.NewUserResult(
			fmt.Sprintf("报告已生成（%s 格式），可通过链接查看: %s", args.FileType, fileURL),
			string(resultJSON),
		), nil
	}

	if t.cb != nil {
		t.cb.OnAIArtifactComplete(ctx, t.stepID, artifactBlockID, "")
	}

	result := map[string]interface{}{
		"type":     args.FileType,
		"content":  content,
		"artifact": true,
	}
	resultJSON, _ := json.Marshal(result)
	return types.NewUserResult(
		fmt.Sprintf("Markdown 报告已生成，共 %d 字。", len([]rune(content))),
		string(resultJSON),
	), nil
}

func (t *ReportTool) streamGenerateWithArtifact(ctx context.Context, chatModel model.ToolCallingChatModel, messages []*schema.Message, artifactBlockID string) (string, error) {
	stream, err := chatModel.Stream(ctx, messages)
	if err != nil {
		return "", err
	}
	defer stream.Close()

	var sb strings.Builder
	for {
		chunk, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
		if chunk.Content != "" {
			sb.WriteString(chunk.Content)
			t.cb.OnAIArtifactChunk(ctx, t.stepID, artifactBlockID, chunk.Content)
		}
	}
	return sb.String(), nil
}

func extractReportContent(content string, fileType string) string {
	if fileType == "html" || fileType == "ppt" {
		if idx := strings.Index(content, "<!DOCTYPE"); idx >= 0 {
			content = content[idx:]
		} else if idx := strings.Index(content, "<!doctype"); idx >= 0 {
			content = content[idx:]
		} else if idx := strings.Index(content, "<html"); idx >= 0 {
			content = content[idx:]
		}

		if idx := strings.LastIndex(content, "</html>"); idx >= 0 {
			content = content[:idx+len("</html>")]
		}

		content = strings.TrimPrefix(content, "```html\n")
		content = strings.TrimPrefix(content, "```html")
		content = strings.TrimSuffix(content, "\n```")
		content = strings.TrimSuffix(content, "```")
	}

	if fileType == "markdown" {
		content = strings.TrimPrefix(content, "```markdown\n")
		content = strings.TrimPrefix(content, "```markdown")
		content = strings.TrimSuffix(content, "\n```")
		content = strings.TrimSuffix(content, "```")
	}

	return strings.TrimSpace(content)
}

func uploadReportFile(ctx context.Context, guluHost, content, title, fileType string) (string, error) {
	ext := ".html"
	if fileType == "markdown" {
		ext = ".md"
	}

	fileName := sanitizeFileName(title) + ext
	if len(fileName) > 100 {
		fileName = fileName[:96] + ext
	}

	reqBody := map[string]interface{}{
		"content":   content,
		"file_name": fileName,
		"file_type": fileType,
	}
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("请求序列化失败: %w", err)
	}

	url := fmt.Sprintf("%s/api/report-files", guluHost)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", fmt.Errorf("创建请求失败: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("上传请求失败: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("上传返回错误 (HTTP %d): %s", resp.StatusCode, string(body))
	}

	// gulu 的 response 格式: {"code":0,"data":{"url":"...","fileName":"...","fileType":"..."}}
	var result struct {
		Code int `json:"code"`
		Data struct {
			URL string `json:"url"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("响应解析失败: %w", err)
	}
	if result.Data.URL == "" {
		return "", fmt.Errorf("上传成功但未返回 URL, response: %s", string(body))
	}

	// url 是相对路径如 /api/attachments/files/...，需要拼成完整 URL
	fileURL := guluHost + result.Data.URL
	return fileURL, nil
}

func sanitizeFileName(name string) string {
	replacer := strings.NewReplacer(
		"/", "_", "\\", "_", ":", "_", "*", "_",
		"?", "_", "\"", "_", "<", "_", ">", "_", "|", "_",
		"\n", "_", "\r", "_", "\t", "_",
	)
	result := replacer.Replace(strings.TrimSpace(name))
	if result == "" {
		result = "report"
	}
	return result
}

var _ executor.ContextualTool = (*ReportTool)(nil)
