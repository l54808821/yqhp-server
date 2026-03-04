package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"yqhp/workflow-engine/internal/executor"
	"yqhp/workflow-engine/pkg/types"
)

const maxReadFileSize = 512 * 1024 // 512KB
const maxListDirEntries = 200

// ========== 读取文件工具 ==========

type ReadFileTool struct{}

func (t *ReadFileTool) Definition() *types.ToolDefinition {
	return &types.ToolDefinition{
		Name:        "read_file",
		Description: "读取指定路径的文件内容。支持文本文件，最大 512KB。可指定起始行和行数来读取部分内容。",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"path": {
					"type": "string",
					"description": "文件路径（绝对路径或相对于工作目录的路径）"
				},
				"start_line": {
					"type": "integer",
					"description": "起始行号（从 1 开始，可选）"
				},
				"num_lines": {
					"type": "integer",
					"description": "读取行数（可选，默认读取全部）"
				}
			},
			"required": ["path"]
		}`),
	}
}

func (t *ReadFileTool) Execute(ctx context.Context, arguments string, execCtx *executor.ExecutionContext) (*types.ToolResult, error) {
	var args struct {
		Path      string `json:"path"`
		StartLine int    `json:"start_line,omitempty"`
		NumLines  int    `json:"num_lines,omitempty"`
	}
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return types.NewErrorResult(fmt.Sprintf("参数解析失败: %v", err)), nil
	}
	if args.Path == "" {
		return types.NewErrorResult("缺少必填参数: path"), nil
	}

	absPath, err := resolveFilePath(args.Path)
	if err != nil {
		return types.NewErrorResult(fmt.Sprintf("路径解析失败: %v", err)), nil
	}

	info, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			return types.NewErrorResult(fmt.Sprintf("文件不存在: %s", args.Path)), nil
		}
		return types.NewErrorResult(fmt.Sprintf("文件状态获取失败: %v", err)), nil
	}
	if info.IsDir() {
		return types.NewErrorResult(fmt.Sprintf("%s 是目录，请使用 list_dir 工具", args.Path)), nil
	}
	if info.Size() > maxReadFileSize {
		return types.NewErrorResult(fmt.Sprintf("文件过大 (%d bytes)，超过 %d bytes 限制。请使用 start_line 和 num_lines 参数读取部分内容。", info.Size(), maxReadFileSize)), nil
	}

	data, err := os.ReadFile(absPath)
	if err != nil {
		return types.NewErrorResult(fmt.Sprintf("文件读取失败: %v", err)), nil
	}

	content := string(data)

	if args.StartLine > 0 || args.NumLines > 0 {
		lines := strings.Split(content, "\n")
		start := 0
		if args.StartLine > 0 {
			start = args.StartLine - 1
		}
		if start >= len(lines) {
			return types.NewErrorResult(fmt.Sprintf("起始行 %d 超出文件总行数 %d", args.StartLine, len(lines))), nil
		}
		end := len(lines)
		if args.NumLines > 0 && start+args.NumLines < end {
			end = start + args.NumLines
		}
		content = strings.Join(lines[start:end], "\n")
		content = fmt.Sprintf("(行 %d-%d，共 %d 行)\n%s", start+1, end, len(lines), content)
	}

	return types.NewToolResult(content), nil
}

// ========== 写入文件工具 ==========

type WriteFileTool struct{}

func (t *WriteFileTool) Definition() *types.ToolDefinition {
	return &types.ToolDefinition{
		Name:        "write_file",
		Description: "将内容写入指定路径的文件。如果文件不存在则创建，如果存在则覆盖。会自动创建不存在的父目录。",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"path": {
					"type": "string",
					"description": "文件路径"
				},
				"content": {
					"type": "string",
					"description": "要写入的文件内容"
				}
			},
			"required": ["path", "content"]
		}`),
	}
}

func (t *WriteFileTool) Execute(ctx context.Context, arguments string, execCtx *executor.ExecutionContext) (*types.ToolResult, error) {
	var args struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return types.NewErrorResult(fmt.Sprintf("参数解析失败: %v", err)), nil
	}
	if args.Path == "" {
		return types.NewErrorResult("缺少必填参数: path"), nil
	}

	absPath, err := resolveFilePath(args.Path)
	if err != nil {
		return types.NewErrorResult(fmt.Sprintf("路径解析失败: %v", err)), nil
	}

	if err := validateWritePath(absPath); err != nil {
		return types.NewErrorResult(err.Error()), nil
	}

	dir := filepath.Dir(absPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return types.NewErrorResult(fmt.Sprintf("创建目录失败: %v", err)), nil
	}

	if err := os.WriteFile(absPath, []byte(args.Content), 0644); err != nil {
		return types.NewErrorResult(fmt.Sprintf("写入文件失败: %v", err)), nil
	}

	return types.NewToolResult(fmt.Sprintf("文件已写入: %s (%d bytes)", args.Path, len(args.Content))), nil
}

// ========== 编辑文件工具 ==========

type EditFileTool struct{}

func (t *EditFileTool) Definition() *types.ToolDefinition {
	return &types.ToolDefinition{
		Name:        "edit_file",
		Description: "编辑文件：查找并替换指定的文本内容。old_text 必须是文件中唯一存在的精确文本。",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"path": {
					"type": "string",
					"description": "文件路径"
				},
				"old_text": {
					"type": "string",
					"description": "要替换的原始文本（必须精确匹配文件中的内容）"
				},
				"new_text": {
					"type": "string",
					"description": "替换后的新文本"
				}
			},
			"required": ["path", "old_text", "new_text"]
		}`),
	}
}

func (t *EditFileTool) Execute(ctx context.Context, arguments string, execCtx *executor.ExecutionContext) (*types.ToolResult, error) {
	var args struct {
		Path    string `json:"path"`
		OldText string `json:"old_text"`
		NewText string `json:"new_text"`
	}
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return types.NewErrorResult(fmt.Sprintf("参数解析失败: %v", err)), nil
	}
	if args.Path == "" || args.OldText == "" {
		return types.NewErrorResult("缺少必填参数: path, old_text"), nil
	}

	absPath, err := resolveFilePath(args.Path)
	if err != nil {
		return types.NewErrorResult(fmt.Sprintf("路径解析失败: %v", err)), nil
	}

	data, err := os.ReadFile(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			return types.NewErrorResult(fmt.Sprintf("文件不存在: %s", args.Path)), nil
		}
		return types.NewErrorResult(fmt.Sprintf("读取文件失败: %v", err)), nil
	}

	content := string(data)
	count := strings.Count(content, args.OldText)
	if count == 0 {
		return types.NewErrorResult("未找到要替换的文本。请确保 old_text 与文件内容精确匹配（包括空格和换行）。"), nil
	}
	if count > 1 {
		return types.NewErrorResult(fmt.Sprintf("old_text 在文件中出现了 %d 次，请提供更多上下文使其唯一。", count)), nil
	}

	newContent := strings.Replace(content, args.OldText, args.NewText, 1)
	if err := os.WriteFile(absPath, []byte(newContent), 0644); err != nil {
		return types.NewErrorResult(fmt.Sprintf("写入文件失败: %v", err)), nil
	}

	return types.NewToolResult(fmt.Sprintf("文件已编辑: %s", args.Path)), nil
}

// ========== 追加文件工具 ==========

type AppendFileTool struct{}

func (t *AppendFileTool) Definition() *types.ToolDefinition {
	return &types.ToolDefinition{
		Name:        "append_file",
		Description: "在文件末尾追加内容。如果文件不存在则创建。",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"path": {
					"type": "string",
					"description": "文件路径"
				},
				"content": {
					"type": "string",
					"description": "要追加的内容"
				}
			},
			"required": ["path", "content"]
		}`),
	}
}

func (t *AppendFileTool) Execute(ctx context.Context, arguments string, execCtx *executor.ExecutionContext) (*types.ToolResult, error) {
	var args struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return types.NewErrorResult(fmt.Sprintf("参数解析失败: %v", err)), nil
	}
	if args.Path == "" || args.Content == "" {
		return types.NewErrorResult("缺少必填参数: path, content"), nil
	}

	absPath, err := resolveFilePath(args.Path)
	if err != nil {
		return types.NewErrorResult(fmt.Sprintf("路径解析失败: %v", err)), nil
	}

	if err := validateWritePath(absPath); err != nil {
		return types.NewErrorResult(err.Error()), nil
	}

	dir := filepath.Dir(absPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return types.NewErrorResult(fmt.Sprintf("创建目录失败: %v", err)), nil
	}

	f, err := os.OpenFile(absPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return types.NewErrorResult(fmt.Sprintf("打开文件失败: %v", err)), nil
	}
	defer f.Close()

	if _, err := f.WriteString(args.Content); err != nil {
		return types.NewErrorResult(fmt.Sprintf("追加内容失败: %v", err)), nil
	}

	return types.NewToolResult(fmt.Sprintf("内容已追加到: %s (%d bytes)", args.Path, len(args.Content))), nil
}

// ========== 列出目录工具 ==========

type ListDirTool struct{}

func (t *ListDirTool) Definition() *types.ToolDefinition {
	return &types.ToolDefinition{
		Name:        "list_dir",
		Description: "列出指定目录下的文件和子目录。返回名称、类型（文件/目录）和大小。",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"path": {
					"type": "string",
					"description": "目录路径（默认为当前工作目录）"
				},
				"recursive": {
					"type": "boolean",
					"description": "是否递归列出子目录（默认 false）"
				}
			}
		}`),
	}
}

func (t *ListDirTool) Execute(ctx context.Context, arguments string, execCtx *executor.ExecutionContext) (*types.ToolResult, error) {
	var args struct {
		Path      string `json:"path"`
		Recursive bool   `json:"recursive"`
	}
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return types.NewErrorResult(fmt.Sprintf("参数解析失败: %v", err)), nil
	}

	dirPath := args.Path
	if dirPath == "" {
		dirPath = "."
	}

	absPath, err := resolveFilePath(dirPath)
	if err != nil {
		return types.NewErrorResult(fmt.Sprintf("路径解析失败: %v", err)), nil
	}

	info, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			return types.NewErrorResult(fmt.Sprintf("目录不存在: %s", dirPath)), nil
		}
		return types.NewErrorResult(fmt.Sprintf("获取目录信息失败: %v", err)), nil
	}
	if !info.IsDir() {
		return types.NewErrorResult(fmt.Sprintf("%s 不是目录", dirPath)), nil
	}

	var entries []map[string]any
	if args.Recursive {
		entries, err = listDirRecursive(absPath, absPath, 0)
	} else {
		entries, err = listDirFlat(absPath)
	}
	if err != nil {
		return types.NewErrorResult(fmt.Sprintf("列出目录失败: %v", err)), nil
	}

	if len(entries) > maxListDirEntries {
		entries = entries[:maxListDirEntries]
	}

	result, _ := json.Marshal(map[string]any{
		"path":    dirPath,
		"count":   len(entries),
		"entries": entries,
	})
	return types.NewToolResult(string(result)), nil
}

func listDirFlat(dirPath string) ([]map[string]any, error) {
	dirEntries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, err
	}

	var entries []map[string]any
	for _, de := range dirEntries {
		entry := map[string]any{
			"name": de.Name(),
			"type": entryType(de),
		}
		if info, err := de.Info(); err == nil && !de.IsDir() {
			entry["size"] = info.Size()
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

func listDirRecursive(basePath, currentPath string, depth int) ([]map[string]any, error) {
	if depth > 5 {
		return nil, nil
	}

	dirEntries, err := os.ReadDir(currentPath)
	if err != nil {
		return nil, err
	}

	var entries []map[string]any
	for _, de := range dirEntries {
		if len(entries) >= maxListDirEntries {
			break
		}

		relPath, _ := filepath.Rel(basePath, filepath.Join(currentPath, de.Name()))
		entry := map[string]any{
			"name": relPath,
			"type": entryType(de),
		}
		if info, err := de.Info(); err == nil && !de.IsDir() {
			entry["size"] = info.Size()
		}
		entries = append(entries, entry)

		if de.IsDir() {
			subEntries, err := listDirRecursive(basePath, filepath.Join(currentPath, de.Name()), depth+1)
			if err == nil {
				entries = append(entries, subEntries...)
			}
		}
	}
	return entries, nil
}

func entryType(de os.DirEntry) string {
	if de.IsDir() {
		return "dir"
	}
	return "file"
}

// --- 路径安全辅助 ---

func resolveFilePath(path string) (string, error) {
	if filepath.IsAbs(path) {
		return filepath.Clean(path), nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("获取工作目录失败: %v", err)
	}
	return filepath.Clean(filepath.Join(cwd, path)), nil
}

var dangerousPaths = []string{"/etc/passwd", "/etc/shadow", "/proc", "/sys"}

func validateWritePath(absPath string) error {
	for _, dp := range dangerousPaths {
		if strings.HasPrefix(absPath, dp) {
			return fmt.Errorf("拒绝写入敏感路径: %s", absPath)
		}
	}
	return nil
}
