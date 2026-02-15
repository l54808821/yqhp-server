package workflow

import (
	"errors"
	"fmt"
	"strings"
)

// ValidNodeTypes workflow-engine 支持的节点类型
var ValidNodeTypes = map[string]bool{
	"http":      true, // HTTP 请求
	"script":    true, // 脚本执行
	"condition": true, // 条件判断
	"loop":      true, // 循环
	"database":  true, // 数据库操作
	"wait":      true, // 等待/延时
	"mq":        true, // MQ 操作
	"ai":        true, // AI 调用
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

	// 注意：definition 中的 name 是可选的，因为工作流名称已经在数据库表中存储
	// 不再验证 def.Name

	// 验证步骤（仅在执行时要求至少一个步骤）
	if requireSteps && len(def.Steps) == 0 {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Field:   "steps",
			Message: "工作流必须包含至少一个步骤",
		})
	}

	// 验证每个步骤
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

// validateStep 验证单个步骤
func validateStep(step *Step, index int, stepIDs map[string]bool) []ValidationError {
	var errs []ValidationError
	prefix := fmt.Sprintf("steps[%d]", index)

	// 验证 ID
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

	// 验证类型
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

	// 验证名称
	if strings.TrimSpace(step.Name) == "" {
		errs = append(errs, ValidationError{
			Field:   prefix + ".name",
			Message: "步骤名称不能为空",
		})
	}

	// 根据类型验证特定配置
	switch step.Type {
	case "http":
		errs = append(errs, validateHTTPStep(step, prefix)...)
	case "script":
		errs = append(errs, validateScriptStep(step, prefix)...)
	case "condition":
		errs = append(errs, validateConditionStep(step, prefix, stepIDs)...)
	case "loop":
		errs = append(errs, validateLoopStep(step, prefix)...)
	case "database":
		errs = append(errs, validateDatabaseStep(step, prefix)...)
	case "wait":
		errs = append(errs, validateWaitStep(step, prefix)...)
	case "mq":
		errs = append(errs, validateMQStep(step, prefix)...)
	case "ai":
		errs = append(errs, validateAIStep(step, prefix)...)
	}

	return errs
}

// validateHTTPStep 验证 HTTP 步骤
func validateHTTPStep(step *Step, prefix string) []ValidationError {
	var errs []ValidationError

	if step.Config == nil {
		errs = append(errs, ValidationError{
			Field:   prefix + ".config",
			Message: "HTTP 步骤必须包含配置",
		})
		return errs
	}

	// 验证 method
	if _, ok := step.Config["method"]; !ok {
		errs = append(errs, ValidationError{
			Field:   prefix + ".config.method",
			Message: "HTTP 步骤必须指定请求方法",
		})
	}

	// 验证 url 或 domain（兼容前端 domainCode 字段）
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

// validateScriptStep 验证脚本步骤
func validateScriptStep(step *Step, prefix string) []ValidationError {
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

// validateConditionStep 验证条件步骤
func validateConditionStep(step *Step, prefix string, stepIDs map[string]bool) []ValidationError {
	var errs []ValidationError

	// 新结构：branches（不再强制依赖 config.type/expression）
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

		kind := strings.TrimSpace(br.Kind)
		if kind == "" {
			kind = "if"
		}
		if kind != "if" && kind != "else_if" && kind != "else" {
			errs = append(errs, ValidationError{
				Field:   bp + ".kind",
				Message: "分支 kind 必须为 if / else_if / else",
			})
		}

		// if/else_if 需要 expression，else 不需要
		if kind != "else" {
			if strings.TrimSpace(br.Expression) == "" {
				errs = append(errs, ValidationError{
					Field:   bp + ".expression",
					Message: "条件表达式不能为空",
				})
			}
		}

		// 验证分支步骤
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

// validateLoopStep 验证循环步骤
func validateLoopStep(step *Step, prefix string) []ValidationError {
	var errs []ValidationError

	if step.Loop == nil {
		errs = append(errs, ValidationError{
			Field:   prefix + ".loop",
			Message: "循环步骤必须包含循环配置",
		})
		return errs
	}

	// 必须指定 count、items 或 condition 之一
	if step.Loop.Count <= 0 && step.Loop.Items == "" && step.Loop.Condition == "" {
		errs = append(errs, ValidationError{
			Field:   prefix + ".loop",
			Message: "循环步骤必须指定 count、items 或 condition",
		})
	}

	return errs
}

// validateDatabaseStep 验证数据库步骤
func validateDatabaseStep(step *Step, prefix string) []ValidationError {
	var errs []ValidationError

	if step.Config == nil {
		errs = append(errs, ValidationError{
			Field:   prefix + ".config",
			Message: "数据库步骤必须包含配置",
		})
		return errs
	}

	// 验证 database_config
	if _, ok := step.Config["database_config"]; !ok {
		errs = append(errs, ValidationError{
			Field:   prefix + ".config.database_config",
			Message: "数据库步骤必须指定数据库配置",
		})
	}

	// 验证 query
	if _, ok := step.Config["query"]; !ok {
		errs = append(errs, ValidationError{
			Field:   prefix + ".config.query",
			Message: "数据库步骤必须指定查询语句",
		})
	}

	return errs
}

// validateWaitStep 验证等待步骤
func validateWaitStep(step *Step, prefix string) []ValidationError {
	var errs []ValidationError

	if step.Config == nil {
		errs = append(errs, ValidationError{
			Field:   prefix + ".config",
			Message: "等待步骤必须包含配置",
		})
		return errs
	}

	// 验证 duration
	if _, ok := step.Config["duration"]; !ok {
		errs = append(errs, ValidationError{
			Field:   prefix + ".config.duration",
			Message: "等待步骤必须指定等待时间",
		})
	}

	return errs
}

// validateMQStep 验证 MQ 步骤
func validateMQStep(step *Step, prefix string) []ValidationError {
	var errs []ValidationError

	if step.Config == nil {
		errs = append(errs, ValidationError{
			Field:   prefix + ".config",
			Message: "MQ 步骤必须包含配置",
		})
		return errs
	}

	// 验证 mq_config
	if _, ok := step.Config["mq_config"]; !ok {
		errs = append(errs, ValidationError{
			Field:   prefix + ".config.mq_config",
			Message: "MQ 步骤必须指定 MQ 配置",
		})
	}

	// 验证 action
	if _, ok := step.Config["action"]; !ok {
		errs = append(errs, ValidationError{
			Field:   prefix + ".config.action",
			Message: "MQ 步骤必须指定操作类型 (send/receive)",
		})
	}

	return errs
}

// validateAIStep 验证 AI 步骤
func validateAIStep(step *Step, prefix string) []ValidationError {
	var errs []ValidationError

	if step.Config == nil {
		errs = append(errs, ValidationError{
			Field:   prefix + ".config",
			Message: "AI 步骤必须包含配置",
		})
		return errs
	}

	// 验证 prompt（AI 调用必须有提示词）
	if _, ok := step.Config["prompt"]; !ok {
		errs = append(errs, ValidationError{
			Field:   prefix + ".config.prompt",
			Message: "AI 步骤必须指定 prompt",
		})
	}

	// 验证内置工具名称
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

	// 验证 MCP 服务器 ID
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

	// 验证最大工具调用轮次
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

// IsValidNodeType 检查节点类型是否有效
func IsValidNodeType(nodeType string) bool {
	return ValidNodeTypes[nodeType]
}

// GetValidNodeTypes 获取所有有效的节点类型
func GetValidNodeTypes() string {
	types := make([]string, 0, len(ValidNodeTypes))
	for t := range ValidNodeTypes {
		types = append(types, t)
	}
	return strings.Join(types, ", ")
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
