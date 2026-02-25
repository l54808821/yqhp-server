package workflow

import (
	"encoding/json"
	"fmt"

	"yqhp/workflow-engine/pkg/types"
)

// WorkflowLoader 工作流加载接口，用于打破 workflow <-> logic 的循环依赖
type WorkflowLoader interface {
	LoadDefinition(id int64) (name string, definition string, err error)
}

// RefWorkflowResolver 引用工作流解析器
// 递归遍历步骤，将 ref_workflow 类型的步骤展开为完整的子工作流定义。
type RefWorkflowResolver struct {
	loader WorkflowLoader
}

func NewRefWorkflowResolver(loader WorkflowLoader) *RefWorkflowResolver {
	return &RefWorkflowResolver{loader: loader}
}

func (r *RefWorkflowResolver) Resolve(steps []types.Step) error {
	return r.resolveSteps(steps, make(map[int64]bool))
}

func (r *RefWorkflowResolver) resolveSteps(steps []types.Step, visited map[int64]bool) error {
	for i := range steps {
		step := &steps[i]

		if step.Type == "ref_workflow" {
			if err := r.resolveRefWorkflow(step, visited); err != nil {
				return fmt.Errorf("步骤 '%s'(%s) 引用工作流解析失败: %w", step.Name, step.ID, err)
			}
		}

		for j := range step.Branches {
			if err := r.resolveSteps(step.Branches[j].Steps, copyVisited(visited)); err != nil {
				return err
			}
		}

		if err := r.resolveSteps(step.Children, copyVisited(visited)); err != nil {
			return err
		}

		if step.Loop != nil {
			if err := r.resolveSteps(step.Loop.Steps, copyVisited(visited)); err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *RefWorkflowResolver) resolveRefWorkflow(step *types.Step, visited map[int64]bool) error {
	if step.Config == nil {
		return fmt.Errorf("缺少配置")
	}

	wfID, err := extractWorkflowID(step.Config)
	if err != nil {
		return err
	}

	if visited[wfID] {
		return fmt.Errorf("检测到循环引用: 工作流 ID %d", wfID)
	}
	visited[wfID] = true

	name, definition, err := r.loader.LoadDefinition(wfID)
	if err != nil {
		return fmt.Errorf("加载工作流 %d 失败: %w", wfID, err)
	}

	targetDef, err := ParseJSON(definition)
	if err != nil {
		return fmt.Errorf("解析工作流 %d 定义失败: %w", wfID, err)
	}

	if err := r.resolveSteps(targetDef.Steps, visited); err != nil {
		return fmt.Errorf("解析工作流 %d 的嵌套引用失败: %w", wfID, err)
	}

	wfDefMap := map[string]any{
		"steps": targetDef.Steps,
	}
	if len(targetDef.Variables) > 0 {
		wfDefMap["variables"] = targetDef.Variables
	}

	step.Config["workflow_definition"] = wfDefMap

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
