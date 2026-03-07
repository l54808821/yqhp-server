package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"yqhp/workflow-engine/internal/executor"
	"yqhp/workflow-engine/pkg/types"
)

// TodoTool 任务规划工具，让 LLM 自主管理复杂任务的分步执行
type TodoTool struct {
	mu    sync.Mutex
	todos []TodoItem
}

type TodoItem struct {
	ID      flexString `json:"id"`
	Content string     `json:"content"`
	Status  string     `json:"status"`
}

type flexString string

func (f *flexString) UnmarshalJSON(data []byte) error {
	if len(data) > 0 && data[0] == '"' {
		var s string
		if err := json.Unmarshal(data, &s); err != nil {
			return err
		}
		*f = flexString(s)
		return nil
	}
	var n json.Number
	if err := json.Unmarshal(data, &n); err != nil {
		return err
	}
	*f = flexString(n.String())
	return nil
}

func NewTodoTool() *TodoTool {
	return &TodoTool{}
}

func (t *TodoTool) Definition() *types.ToolDefinition {
	return &types.ToolDefinition{
		Name: "todo_write",
		Description: `创建和管理任务列表，用于跟踪复杂多步任务的进度。
当任务涉及 3 个以上步骤时使用此工具进行规划。
每完成一个任务后更新状态为 completed。
简单任务不需要使用此工具。

使用方式：
- 传入 todos 数组，每个 todo 包含 id、content、status
- status 可选值：pending（待执行）、in_progress（执行中）、completed（已完成）
- merge 为 true 时合并更新已有任务，为 false 时替换全部任务`,
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"todos": {
					"type": "array",
					"description": "任务列表",
					"items": {
						"type": "object",
						"properties": {
							"id": {
								"type": "string",
								"description": "任务唯一标识,建议使用简短且有意义的字符串"
							},
							"content": {
								"type": "string",
								"description": "任务描述"
							},
							"status": {
								"type": "string",
								"enum": ["pending", "in_progress", "completed"],
								"description": "任务状态"
							}
						},
						"required": ["id", "content", "status"]
					}
				},
				"merge": {
					"type": "boolean",
					"description": "是否合并更新（true=合并，false=替换全部），默认 false"
				}
			},
			"required": ["todos"]
		}`),
	}
}

func (t *TodoTool) Execute(ctx context.Context, arguments string, execCtx *executor.ExecutionContext) (*types.ToolResult, error) {
	var args struct {
		Todos []TodoItem `json:"todos"`
		Merge bool       `json:"merge"`
	}
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return types.NewErrorResult(fmt.Sprintf("参数解析失败: %v", err)), nil
	}
	if len(args.Todos) == 0 {
		return types.NewErrorResult("todos 不能为空"), nil
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	if args.Merge {
		existingMap := make(map[flexString]int)
		for i, todo := range t.todos {
			existingMap[todo.ID] = i
		}
		for _, newTodo := range args.Todos {
			if idx, ok := existingMap[newTodo.ID]; ok {
				if newTodo.Content != "" {
					t.todos[idx].Content = newTodo.Content
				}
				if newTodo.Status != "" {
					t.todos[idx].Status = newTodo.Status
				}
			} else {
				t.todos = append(t.todos, newTodo)
			}
		}
	} else {
		t.todos = args.Todos
	}

	var sb strings.Builder
	sb.WriteString("任务列表已更新：\n")
	for _, todo := range t.todos {
		statusIcon := "○"
		switch todo.Status {
		case "in_progress":
			statusIcon = "◔"
		case "completed":
			statusIcon = "●"
		}
		sb.WriteString(fmt.Sprintf("%s [%s] %s\n", statusIcon, todo.ID, todo.Content))
	}

	return types.NewSilentResult(sb.String()), nil
}
