// Package executor 提供工作流步骤执行的执行器框架。
package executor

import (
	"fmt"
	"regexp"
	"strings"
	"sync"
)

// VariableResolver 变量解析器
// 使用正则表达式一次性提取所有变量引用，并支持嵌套路径访问。
// 线程安全，可在多个执行器中共享使用。
type VariableResolver struct {
	// 正则表达式用于匹配 ${variable} 或 ${object.path} 格式
	pattern *regexp.Regexp
}

var (
	// 全局变量解析器实例
	defaultResolver *VariableResolver
	resolverOnce    sync.Once
)

// GetVariableResolver 获取全局变量解析器实例
func GetVariableResolver() *VariableResolver {
	resolverOnce.Do(func() {
		defaultResolver = NewVariableResolver()
	})
	return defaultResolver
}

// NewVariableResolver 创建新的变量解析器
func NewVariableResolver() *VariableResolver {
	// 匹配 ${...} 格式的变量引用
	// 支持的格式：${var}, ${obj.field}, ${obj.nested.field}
	pattern := regexp.MustCompile(`\$\{([^}]+)\}`)
	return &VariableResolver{
		pattern: pattern,
	}
}

// ResolveString 解析字符串中的所有变量引用
// 使用正则表达式一次性找出所有变量引用，避免多次遍历
func (r *VariableResolver) ResolveString(s string, ctx map[string]any) string {
	if s == "" || ctx == nil || len(ctx) == 0 {
		return s
	}

	// 如果字符串中不包含 ${ 则直接返回
	if !strings.Contains(s, "${") {
		return s
	}

	// 使用 ReplaceAllStringFunc 一次性替换所有匹配项
	result := r.pattern.ReplaceAllStringFunc(s, func(match string) string {
		// 提取变量路径（去掉 ${ 和 }）
		path := match[2 : len(match)-1]

		// 解析变量值
		value := r.resolveVariablePath(path, ctx)
		if value == nil {
			// 未找到变量，保持原样
			return match
		}

		return fmt.Sprintf("%v", value)
	})

	return result
}

// ResolveMap 解析 map 中所有字符串值的变量引用
func (r *VariableResolver) ResolveMap(m map[string]string, ctx map[string]any) map[string]string {
	if m == nil || len(m) == 0 {
		return m
	}

	result := make(map[string]string, len(m))
	for k, v := range m {
		result[k] = r.ResolveString(v, ctx)
	}
	return result
}

// resolveVariablePath 解析变量路径（支持嵌套访问如 obj.field.subfield）
func (r *VariableResolver) resolveVariablePath(path string, ctx map[string]any) any {
	// 分割路径
	parts := strings.Split(path, ".")
	if len(parts) == 0 {
		return nil
	}

	// 获取根变量
	current, exists := ctx[parts[0]]
	if !exists {
		return nil
	}

	// 如果只有一个部分，直接返回
	if len(parts) == 1 {
		return current
	}

	// 遍历路径的其余部分
	for i := 1; i < len(parts); i++ {
		if current == nil {
			return nil
		}

		// 尝试作为 map 访问
		switch m := current.(type) {
		case map[string]any:
			current = m[parts[i]]
		case map[string]interface{}:
			current = m[parts[i]]
		case map[interface{}]interface{}:
			current = m[parts[i]]
		default:
			// 不支持的类型
			return nil
		}
	}

	return current
}

// ExtractVariables 提取字符串中的所有变量名
func (r *VariableResolver) ExtractVariables(s string) []string {
	if s == "" || !strings.Contains(s, "${") {
		return nil
	}

	matches := r.pattern.FindAllStringSubmatch(s, -1)
	if len(matches) == 0 {
		return nil
	}

	result := make([]string, 0, len(matches))
	for _, match := range matches {
		if len(match) > 1 {
			result = append(result, match[1])
		}
	}
	return result
}

// HasVariables 检查字符串是否包含变量引用
func (r *VariableResolver) HasVariables(s string) bool {
	if s == "" {
		return false
	}
	return strings.Contains(s, "${")
}

// ========== 便捷函数 ==========

// ResolveVariables 解析字符串中的变量（使用全局解析器）
func ResolveVariables(s string, ctx map[string]any) string {
	return GetVariableResolver().ResolveString(s, ctx)
}

// ResolveVariablesInMap 解析 map 中的变量（使用全局解析器）
func ResolveVariablesInMap(m map[string]string, ctx map[string]any) map[string]string {
	return GetVariableResolver().ResolveMap(m, ctx)
}
