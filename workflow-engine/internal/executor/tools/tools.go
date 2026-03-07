package tools

import (
	"context"
	"os/exec"
	"strings"

	"yqhp/workflow-engine/internal/executor"
)

// RegisterAll 将所有内置工具注册到指定的 ToolRegistry。
func RegisterAll(registry *executor.ToolRegistry) {
	allTools := []executor.Tool{
		// 基础工具
		&HTTPTool{},
		&VarReadTool{},
		&VarWriteTool{},
		&JSONParseTool{},
		// 联网工具
		&TavilySearchTool{},
		&JinaReaderTool{},
		// 代码执行
		&CodeExecuteTool{},
		// 命令行
		&ShellExecTool{},
		// 文件操作
		&ReadFileTool{},
		&WriteFileTool{},
		&EditFileTool{},
		&AppendFileTool{},
		&ListDirTool{},
	}
	for _, tool := range allTools {
		registry.Register(tool)
	}
}

// ExecCommandContext 封装 exec.CommandContext，供 code/shell 工具使用。
func ExecCommandContext(ctx context.Context, name string, args ...string) *exec.Cmd {
	return exec.CommandContext(ctx, name, args...)
}

// StripTags 移除 HTML 标签并解码常见 HTML 实体。
func StripTags(s string) string {
	var result strings.Builder
	inTag := false
	for _, ch := range s {
		if ch == '<' {
			inTag = true
		} else if ch == '>' {
			inTag = false
		} else if !inTag {
			result.WriteRune(ch)
		}
	}
	return DecodeHTMLEntities(strings.TrimSpace(result.String()))
}

// DecodeHTMLEntities 解码常见 HTML 实体。
func DecodeHTMLEntities(s string) string {
	r := strings.NewReplacer(
		"&nbsp;", " ",
		"&ensp;", " ",
		"&emsp;", " ",
		"&amp;", "&",
		"&lt;", "<",
		"&gt;", ">",
		"&quot;", "\"",
		"&#39;", "'",
		"&#0183;", "·",
		"&#183;", "·",
		"&middot;", "·",
		"&hellip;", "...",
		"&mdash;", "—",
		"&ndash;", "–",
		"&laquo;", "«",
		"&raquo;", "»",
	)
	return r.Replace(s)
}

// HTMLToText 将 HTML 转为纯文本：移除 script/style，去标签，解码实体。
func HTMLToText(html string) string {
	for _, tag := range []string{"script", "style", "noscript"} {
		for {
			startTag := strings.Index(strings.ToLower(html), "<"+tag)
			if startTag < 0 {
				break
			}
			endTag := strings.Index(strings.ToLower(html[startTag:]), "</"+tag+">")
			if endTag < 0 {
				html = html[:startTag]
				break
			}
			html = html[:startTag] + html[startTag+endTag+len("</"+tag+">"):]
		}
	}

	text := StripTags(html)

	lines := strings.Split(text, "\n")
	var cleaned []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			cleaned = append(cleaned, trimmed)
		}
	}
	return strings.Join(cleaned, "\n")
}

func init() {
	RegisterAll(executor.DefaultToolRegistry)
}

type searchResult struct {
	Title   string
	Snippet string
	URL     string
}
