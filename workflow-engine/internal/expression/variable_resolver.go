package expression

import (
	"fmt"
	"os"
	"reflect"
	"regexp"
	"strconv"
	"strings"
)

// VariableResolver 变量解析器
type VariableResolver struct {
	ctx *EvaluationContext
}

// NewVariableResolver 创建新的变量解析器
func NewVariableResolver(ctx *EvaluationContext) *VariableResolver {
	return &VariableResolver{ctx: ctx}
}

// 变量引用正则表达式
var (
	// ${variable} 或 ${variable.path} 或 ${variable[0].field}
	varRefPattern = regexp.MustCompile(`\$\{([^}]+)\}`)
	// ${env.VAR_NAME}
	envVarPattern = regexp.MustCompile(`^env\.(.+)$`)
	// ${file:path/to/file}
	fileRefPattern = regexp.MustCompile(`^file:(.+)$`)
	// 数组索引 [0], [1], etc.
	arrayIndexPattern = regexp.MustCompile(`\[(\d+)\]`)
)

// ResolveString 解析字符串中的所有变量引用
func (r *VariableResolver) ResolveString(s string) (string, error) {
	result := varRefPattern.ReplaceAllStringFunc(s, func(match string) string {
		// 提取变量表达式
		expr := match[2 : len(match)-1] // 去掉 ${ 和 }

		// 解析变量
		val, err := r.ResolveExpression(expr)
		if err != nil {
			return match // 保持原样
		}

		return fmt.Sprintf("%v", val)
	})

	return result, nil
}

// ResolveExpression 解析单个变量表达式
func (r *VariableResolver) ResolveExpression(expr string) (any, error) {
	// 检查环境变量引用
	if matches := envVarPattern.FindStringSubmatch(expr); len(matches) == 2 {
		return r.resolveEnvVar(matches[1])
	}

	// 检查文件引用
	if matches := fileRefPattern.FindStringSubmatch(expr); len(matches) == 2 {
		return r.resolveFileRef(matches[1])
	}

	// 普通变量引用
	return r.resolveVariable(expr)
}

// resolveEnvVar 解析环境变量
func (r *VariableResolver) resolveEnvVar(name string) (string, error) {
	val := os.Getenv(name)
	return val, nil
}

// resolveFileRef 解析文件引用
func (r *VariableResolver) resolveFileRef(path string) (string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read file '%s': %w", path, err)
	}
	return string(content), nil
}

// resolveVariable 解析普通变量
func (r *VariableResolver) resolveVariable(expr string) (any, error) {
	if r.ctx == nil {
		return nil, fmt.Errorf("no context available")
	}

	// 解析路径（支持 . 和 [] 访问）
	return r.resolvePath(expr)
}

// resolvePath 解析变量路径
func (r *VariableResolver) resolvePath(path string) (any, error) {
	// 预处理：将 [n] 转换为 .n 以便统一处理
	normalizedPath := arrayIndexPattern.ReplaceAllString(path, ".$1")
	parts := strings.Split(normalizedPath, ".")

	if len(parts) == 0 {
		return nil, fmt.Errorf("empty path")
	}

	// 获取根变量
	root := parts[0]
	var current any
	var found bool

	// 先检查变量
	if current, found = r.ctx.Variables[root]; !found {
		// 再检查结果
		if current, found = r.ctx.Results[root]; !found {
			return nil, fmt.Errorf("variable '%s' not found", root)
		}
	}

	// 遍历路径
	for i := 1; i < len(parts); i++ {
		part := parts[i]
		if part == "" {
			continue
		}

		var err error
		current, err = r.getFieldOrIndex(current, part)
		if err != nil {
			return nil, fmt.Errorf("cannot resolve path '%s' at '%s': %w", path, part, err)
		}
	}

	return current, nil
}

// getFieldOrIndex 获取字段或数组索引
func (r *VariableResolver) getFieldOrIndex(v any, key string) (any, error) {
	if v == nil {
		return nil, fmt.Errorf("cannot access '%s' on nil", key)
	}

	// 检查是否是数字索引
	if idx, err := strconv.Atoi(key); err == nil {
		return r.getIndex(v, idx)
	}

	// 字段访问
	return r.getField(v, key)
}

// getIndex 获取数组索引
func (r *VariableResolver) getIndex(v any, idx int) (any, error) {
	rv := reflect.ValueOf(v)

	switch rv.Kind() {
	case reflect.Slice, reflect.Array:
		if idx < 0 || idx >= rv.Len() {
			return nil, fmt.Errorf("index %d out of bounds (length %d)", idx, rv.Len())
		}
		return rv.Index(idx).Interface(), nil
	default:
		return nil, fmt.Errorf("cannot index type %T", v)
	}
}

// getField 获取字段
func (r *VariableResolver) getField(v any, field string) (any, error) {
	if v == nil {
		return nil, fmt.Errorf("cannot get field '%s' from nil", field)
	}

	// 处理 map
	if m, ok := v.(map[string]any); ok {
		if val, exists := m[field]; exists {
			return val, nil
		}
		return nil, fmt.Errorf("field '%s' not found in map", field)
	}

	// 处理结构体
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Ptr {
		rv = rv.Elem()
	}

	if rv.Kind() == reflect.Struct {
		fv := rv.FieldByName(field)
		if fv.IsValid() {
			return fv.Interface(), nil
		}
		// 尝试不区分大小写匹配
		for i := 0; i < rv.NumField(); i++ {
			if strings.EqualFold(rv.Type().Field(i).Name, field) {
				return rv.Field(i).Interface(), nil
			}
		}
		return nil, fmt.Errorf("field '%s' not found in struct", field)
	}

	return nil, fmt.Errorf("cannot get field '%s' from type %T", field, v)
}

// ResolveAll 解析 map 中所有字符串值的变量引用
func (r *VariableResolver) ResolveAll(data map[string]any) (map[string]any, error) {
	result := make(map[string]any)

	for k, v := range data {
		resolved, err := r.resolveValue(v)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve '%s': %w", k, err)
		}
		result[k] = resolved
	}

	return result, nil
}

// resolveValue 解析单个值
func (r *VariableResolver) resolveValue(v any) (any, error) {
	switch val := v.(type) {
	case string:
		return r.ResolveString(val)
	case map[string]any:
		return r.ResolveAll(val)
	case []any:
		result := make([]any, len(val))
		for i, item := range val {
			resolved, err := r.resolveValue(item)
			if err != nil {
				return nil, err
			}
			result[i] = resolved
		}
		return result, nil
	default:
		return v, nil
	}
}

// ResolveStringSimple 简单的字符串变量解析（用于快速替换）
func ResolveStringSimple(s string, vars map[string]any) string {
	ctx := NewEvaluationContext().WithVariables(vars)
	resolver := NewVariableResolver(ctx)
	result, _ := resolver.ResolveString(s)
	return result
}
