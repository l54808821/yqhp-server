package workflow

import (
	"encoding/json"
	"fmt"
)

// WorkflowLoader 工作流加载接口，用于打破 workflow <-> logic 的循环依赖
type WorkflowLoader interface {
	// LoadDefinition 根据工作流 ID 加载其名称和 definition JSON
	LoadDefinition(id int64) (name string, definition string, err error)
}

// RefWorkflowResolver 引用工作流解析器
// 递归遍历步骤，将 ref_workflow 类型的步骤展开为完整的子工作流定义。
type RefWorkflowResolver struct {
	loader WorkflowLoader
}

// NewRefWorkflowResolver 创建引用工作流解析器
func NewRefWorkflowResolver(loader WorkflowLoader) *RefWorkflowResolver {
	return &RefWorkflowResolver{loader: loader}
}

// Resolve 解析步骤列表中的所有 ref_workflow 引用
func (r *RefWorkflowResolver) Resolve(steps []Step) error {
	return r.resolveSteps(steps, make(map[int64]bool))
}

func (r *RefWorkflowResolver) resolveSteps(steps []Step, visited map[int64]bool) error {
	for i := range steps {
		step := &steps[i]

		if step.Type == "ref_workflow" {
			if err := r.resolveRefWorkflow(step, visited); err != nil {
				return fmt.Errorf("步骤 '%s'(%s) 引用工作流解析失败: %w", step.Name, step.ID, err)
			}
		}

		// 递归处理 condition 分支内的步骤
		for j := range step.Branches {
			if err := r.resolveSteps(step.Branches[j].Steps, copyVisited(visited)); err != nil {
				return err
			}
		}

		// 递归处理 loop / 其他容器的子步骤
		if err := r.resolveSteps(step.Children, copyVisited(visited)); err != nil {
			return err
		}

		// 递归处理 loop.steps
		if step.Loop != nil {
			if err := r.resolveSteps(step.Loop.Steps, copyVisited(visited)); err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *RefWorkflowResolver) resolveRefWorkflow(step *Step, visited map[int64]bool) error {
	if step.Config == nil {
		return fmt.Errorf("缺少配置")
	}

	// 提取 workflow_id
	wfID, err := extractWorkflowID(step.Config)
	if err != nil {
		return err
	}

	// 循环引用检测
	if visited[wfID] {
		return fmt.Errorf("检测到循环引用: 工作流 ID %d", wfID)
	}
	visited[wfID] = true

	// 通过接口加载目标工作流
	name, definition, err := r.loader.LoadDefinition(wfID)
	if err != nil {
		return fmt.Errorf("加载工作流 %d 失败: %w", wfID, err)
	}

	// 解析目标工作流定义
	targetDef, err := ParseJSON(definition)
	if err != nil {
		return fmt.Errorf("解析工作流 %d 定义失败: %w", wfID, err)
	}

	// 递归解析子工作流内部可能嵌套的 ref_workflow
	if err := r.resolveSteps(targetDef.Steps, visited); err != nil {
		return fmt.Errorf("解析工作流 %d 的嵌套引用失败: %w", wfID, err)
	}

	// 构建子工作流定义（仅保留引擎需要的字段）
	wfDefMap := map[string]any{
		"steps": targetDef.Steps,
	}
	if len(targetDef.Variables) > 0 {
		wfDefMap["variables"] = targetDef.Variables
	}

	// 将完整定义塞入 config
	step.Config["workflow_definition"] = wfDefMap

	// 回填名称方便调试
	if name != "" {
		step.Config["workflow_name"] = name
	}

	return nil
}

func extractWorkflowID(config map[string]any) (int64, error) {
	raw, ok := config["workflow_id"]
	if !ok || raw == nil {
		return 0, fmt.Errorf("缺少 workflow_id")
	}

	switch v := raw.(type) {
	case float64:
		return int64(v), nil
	case json.Number:
		n, err := v.Int64()
		if err != nil {
			return 0, fmt.Errorf("workflow_id 格式无效: %w", err)
		}
		return n, nil
	case int64:
		return v, nil
	case int:
		return int64(v), nil
	default:
		return 0, fmt.Errorf("workflow_id 类型无效: %T", raw)
	}
}

func copyVisited(visited map[int64]bool) map[int64]bool {
	cp := make(map[int64]bool, len(visited))
	for k, v := range visited {
		cp[k] = v
	}
	return cp
}
