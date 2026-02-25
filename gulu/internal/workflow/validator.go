package workflow

import (
	"errors"
	"fmt"
	"strings"

	"yqhp/workflow-engine/pkg/types"
)

// ValidNodeTypes workflow-engine 支持的节点类型
var ValidNodeTypes = map[string]bool{
	"http":         true,
	"script":       true,
	"condition":    true,
	"loop":         true,
	"database":     true,
	"db":           true,
	"wait":         true,
	"mq":           true,
	"ai":           true,
	"ref_workflow": true,
}

// ValidationError 验证错误
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// ValidationResult 验证结果
type ValidationResult struct {
	Valid  bool              `json:"valid"`
	Errors []ValidationError `json:"errors,omitempty"`
}

// Validate 验证工作流定义
func Validate(def *WorkflowDefinition) *ValidationResult {
	return ValidateWithOptions(def, false)
}

// ValidateForExecution 验证工作流定义（用于执行前验证，要求至少一个步骤）
func ValidateForExecution(def *WorkflowDefinition) *ValidationResult {
	return ValidateWithOptions(def, true)
}

// ValidateWithOptions 验证工作流定义（带选项）
func ValidateWithOptions(def *WorkflowDefinition, requireSteps bool) *ValidationResult {
	result := &ValidationResult{Valid: true, Errors: []ValidationError{}}

	if def == nil {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Field:   "definition",
			Message: "工作流定义不能为空",
		})
		return result
	}

	if requireSteps && len(def.Steps) == 0 {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Field:   "steps",
			Message: "工作流必须包含至少一个步骤",
		})
	}

	stepIDs := make(map[string]bool)
	for i, step := range def.Steps {
		stepErrors := validateStep(&step, i, stepIDs)
		if len(stepErrors) > 0 {
			result.Valid = false
			result.Errors = append(result.Errors, stepErrors...)
		}
	}

	return result
}

func validateStep(step *types.Step, index int, stepIDs map[string]bool) []ValidationError {
	var errs []ValidationError
	prefix := fmt.Sprintf("steps[%d]", index)

	if strings.TrimSpace(step.ID) == "" {
		errs = append(errs, ValidationError{
			Field:   prefix + ".id",
			Message: "步骤 ID 不能为空",
		})
	} else {
		if stepIDs[step.ID] {
			errs = append(errs, ValidationError{
				Field:   prefix + ".id",
				Message: fmt.Sprintf("步骤 ID '%s' 重复", step.ID),
			})
		}
		stepIDs[step.ID] = true
	}

	if strings.TrimSpace(step.Type) == "" {
		errs = append(errs, ValidationError{
			Field:   prefix + ".type",
			Message: "步骤类型不能为空",
		})
	} else if !IsValidNodeType(step.Type) {
		errs = append(errs, ValidationError{
			Field:   prefix + ".type",
			Message: fmt.Sprintf("无效的步骤类型 '%s'，支持的类型: %s", step.Type, GetValidNodeTypes()),
		})
	}

	if strings.TrimSpace(step.Name) == "" {
		errs = append(errs, ValidationError{
			Field:   prefix + ".name",
			Message: "步骤名称不能为空",
		})
	}

	switch step.Type {
	case "http":
		errs = append(errs, validateHTTPStep(step, prefix)...)
	case "script":
		errs = append(errs, validateScriptStep(step, prefix)...)
	case "condition":
		errs = append(errs, validateConditionStep(step, prefix, stepIDs)...)
	case "loop":
		errs = append(errs, validateLoopStep(step, prefix)...)
	case "database", "db":
		errs = append(errs, validateDatabaseStep(step, prefix)...)
	case "wait":
		errs = append(errs, validateWaitStep(step, prefix)...)
	case "mq":
		errs = append(errs, validateMQStep(step, prefix)...)
	case "ai":
		errs = append(errs, validateAIStep(step, prefix)...)
	case "ref_workflow":
		errs = append(errs, validateRefWorkflowStep(step, prefix)...)
	}

	return errs
}

func validateHTTPStep(step *types.Step, prefix string) []ValidationError {
	var errs []ValidationError

	if step.Config == nil {
		errs = append(errs, ValidationError{
			Field:   prefix + ".config",
			Message: "HTTP 步骤必须包含配置",
		})
		return errs
	}

	if _, ok := step.Config["method"]; !ok {
		errs = append(errs, ValidationError{
			Field:   prefix + ".config.method",
			Message: "HTTP 步骤必须指定请求方法",
		})
	}

	_, hasURL := step.Config["url"]
	_, hasDomain := step.Config["domain"]
	_, hasDomainCode := step.Config["domainCode"]
	if !hasURL && !hasDomain && !hasDomainCode {
		errs = append(errs, ValidationError{
			Field:   prefix + ".config.url",
			Message: "HTTP 步骤必须指定 URL 或 domain",
		})
	}

	return errs
}

func validateScriptStep(step *types.Step, prefix string) []ValidationError {
	var errs []ValidationError

	if step.Config == nil {
		errs = append(errs, ValidationError{
			Field:   prefix + ".config",
			Message: "脚本步骤必须包含配置",
		})
		return errs
	}

	if _, ok := step.Config["script"]; !ok {
		errs = append(errs, ValidationError{
			Field:   prefix + ".config.script",
			Message: "脚本步骤必须指定脚本内容",
		})
	}

	return errs
}

func validateConditionStep(step *types.Step, prefix string, stepIDs map[string]bool) []ValidationError {
	var errs []ValidationError

	if len(step.Branches) == 0 {
		errs = append(errs, ValidationError{
			Field:   prefix + ".branches",
			Message: "条件步骤必须包含 branches（if/else_if/else）",
		})
		return errs
	}

	seenBranchIDs := map[string]bool{}
	for i, br := range step.Branches {
		bp := fmt.Sprintf("%s.branches[%d]", prefix, i)

		if strings.TrimSpace(br.ID) == "" {
			errs = append(errs, ValidationError{
				Field:   bp + ".id",
				Message: "分支 ID 不能为空",
			})
		} else {
			if seenBranchIDs[br.ID] {
				errs = append(errs, ValidationError{
					Field:   bp + ".id",
					Message: fmt.Sprintf("分支 ID '%s' 重复", br.ID),
				})
			}
			seenBranchIDs[br.ID] = true
		}

		kind := strings.TrimSpace(string(br.Kind))
		if kind == "" {
			kind = "if"
		}
		if kind != "if" && kind != "else_if" && kind != "else" {
			errs = append(errs, ValidationError{
				Field:   bp + ".kind",
				Message: "分支 kind 必须为 if / else_if / else",
			})
		}

		if kind != "else" {
			if strings.TrimSpace(br.Expression) == "" {
				errs = append(errs, ValidationError{
					Field:   bp + ".expression",
					Message: "条件表达式不能为空",
				})
			}
		}

		for j, child := range br.Steps {
			childErrs := validateStep(&child, j, stepIDs)
			for _, e := range childErrs {
				e.Field = bp + ".steps." + e.Field
				errs = append(errs, e)
			}
		}
	}

	return errs
}

func validateLoopStep(step *types.Step, prefix string) []ValidationError {
	var errs []ValidationError

	if step.Loop == nil {
		errs = append(errs, ValidationError{
			Field:   prefix + ".loop",
			Message: "循环步骤必须包含循环配置",
		})
		return errs
	}

	if step.Loop.Count <= 0 && step.Loop.Items == "" && step.Loop.Condition == "" {
		errs = append(errs, ValidationError{
			Field:   prefix + ".loop",
			Message: "循环步骤必须指定 count、items 或 condition",
		})
	}

	return errs
}

func validateDatabaseStep(step *types.Step, prefix string) []ValidationError {
	var errs []ValidationError

	if step.Config == nil {
		errs = append(errs, ValidationError{
			Field:   prefix + ".config",
			Message: "数据库步骤必须包含配置",
		})
		return errs
	}

	if _, ok := step.Config["database_config"]; !ok {
		if _, ok := step.Config["datasourceCode"]; !ok {
			errs = append(errs, ValidationError{
				Field:   prefix + ".config.database_config",
				Message: "数据库步骤必须指定数据库配置",
			})
		}
	}

	if _, ok := step.Config["sql"]; !ok {
		errs = append(errs, ValidationError{
			Field:   prefix + ".config.sql",
			Message: "数据库步骤必须指定查询语句",
		})
	}

	return errs
}

func validateWaitStep(step *types.Step, prefix string) []ValidationError {
	var errs []ValidationError

	if step.Config == nil {
		errs = append(errs, ValidationError{
			Field:   prefix + ".config",
			Message: "等待步骤必须包含配置",
		})
		return errs
	}

	if _, ok := step.Config["duration"]; !ok {
		errs = append(errs, ValidationError{
			Field:   prefix + ".config.duration",
			Message: "等待步骤必须指定等待时间",
		})
	}

	return errs
}

func validateMQStep(step *types.Step, prefix string) []ValidationError {
	var errs []ValidationError

	if step.Config == nil {
		errs = append(errs, ValidationError{
			Field:   prefix + ".config",
			Message: "MQ 步骤必须包含配置",
		})
		return errs
	}

	if _, ok := step.Config["mq_config"]; !ok {
		errs = append(errs, ValidationError{
			Field:   prefix + ".config.mq_config",
			Message: "MQ 步骤必须指定 MQ 配置",
		})
	}

	if _, ok := step.Config["action"]; !ok {
		errs = append(errs, ValidationError{
			Field:   prefix + ".config.action",
			Message: "MQ 步骤必须指定操作类型 (send/receive)",
		})
	}

	return errs
}

func validateAIStep(step *types.Step, prefix string) []ValidationError {
	var errs []ValidationError

	if step.Config == nil {
		errs = append(errs, ValidationError{
			Field:   prefix + ".config",
			Message: "AI 步骤必须包含配置",
		})
		return errs
	}

	if _, ok := step.Config["prompt"]; !ok {
		errs = append(errs, ValidationError{
			Field:   prefix + ".config.prompt",
			Message: "AI 步骤必须指定 prompt",
		})
	}

	if tools, ok := step.Config["tools"].([]any); ok {
		knownTools := map[string]bool{
			"http_request":      true,
			"var_read":          true,
			"var_write":         true,
			"json_parse":        true,
			"human_interaction": true,
		}
		for i, t := range tools {
			if name, ok := t.(string); ok {
				if !knownTools[name] {
					errs = append(errs, ValidationError{
						Field:   fmt.Sprintf("%s.config.tools[%d]", prefix, i),
						Message: fmt.Sprintf("未知的内置工具名称: %s", name),
					})
				}
			}
		}
	}

	if mcpServerIDs, ok := step.Config["mcp_server_ids"].([]any); ok {
		for i, id := range mcpServerIDs {
			if f, ok := id.(float64); ok {
				if f <= 0 || f != float64(int64(f)) {
					errs = append(errs, ValidationError{
						Field:   fmt.Sprintf("%s.config.mcp_server_ids[%d]", prefix, i),
						Message: "MCP 服务器 ID 必须为有效的正整数",
					})
				}
			} else {
				errs = append(errs, ValidationError{
					Field:   fmt.Sprintf("%s.config.mcp_server_ids[%d]", prefix, i),
					Message: "MCP 服务器 ID 必须为数字",
				})
			}
		}
	}

	if maxRounds, ok := step.Config["max_tool_rounds"].(float64); ok {
		if maxRounds < 1 || maxRounds > 50 {
			errs = append(errs, ValidationError{
				Field:   prefix + ".config.max_tool_rounds",
				Message: "max_tool_rounds 必须在 1 到 50 之间",
			})
		}
	}

	return errs
}

func validateRefWorkflowStep(step *types.Step, prefix string) []ValidationError {
	var errs []ValidationError

	if step.Config == nil {
		errs = append(errs, ValidationError{
			Field:   prefix + ".config",
			Message: "引用工作流步骤必须包含配置",
		})
		return errs
	}

	wfID, ok := step.Config["workflow_id"]
	if !ok || wfID == nil {
		errs = append(errs, ValidationError{
			Field:   prefix + ".config.workflow_id",
			Message: "引用工作流步骤必须指定目标工作流 ID",
		})
	} else if f, ok := wfID.(float64); ok {
		if f <= 0 || f != float64(int64(f)) {
			errs = append(errs, ValidationError{
				Field:   prefix + ".config.workflow_id",
				Message: "工作流 ID 必须为有效的正整数",
			})
		}
	}

	return errs
}

// IsValidNodeType 检查节点类型是否有效
func IsValidNodeType(nodeType string) bool {
	return ValidNodeTypes[nodeType]
}

// GetValidNodeTypes 获取所有有效的节点类型
func GetValidNodeTypes() string {
	nodeTypes := make([]string, 0, len(ValidNodeTypes))
	for t := range ValidNodeTypes {
		nodeTypes = append(nodeTypes, t)
	}
	return strings.Join(nodeTypes, ", ")
}

// ValidateJSON 验证 JSON 格式的工作流定义
func ValidateJSON(jsonContent string) (*ValidationResult, error) {
	def, err := ParseJSON(jsonContent)
	if err != nil {
		return nil, err
	}
	return Validate(def), nil
}

// ValidateYAML 验证 YAML 格式的工作流定义
func ValidateYAML(yamlContent string) (*ValidationResult, error) {
	def, err := ParseYAML(yamlContent)
	if err != nil {
		return nil, err
	}
	return Validate(def), nil
}

// ValidateDefinition 验证工作流定义（返回 error）
func ValidateDefinition(def *WorkflowDefinition) error {
	result := Validate(def)
	if !result.Valid {
		var msgs []string
		for _, e := range result.Errors {
			msgs = append(msgs, fmt.Sprintf("%s: %s", e.Field, e.Message))
		}
		return errors.New(strings.Join(msgs, "; "))
	}
	return nil
}
