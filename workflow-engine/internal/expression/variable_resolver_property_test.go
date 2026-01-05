package expression

import (
	"os"
	"testing"

	"pgregory.net/rapid"
)

// TestVariableReferenceResolutionProperty 属性 11: 变量引用解析
// 对于任意包含 ${variable} 的表达式，变量必须从执行上下文中解析。
// 嵌套引用如 ${response.data[0].field} 必须正确遍历。
func TestVariableReferenceResolutionProperty(t *testing.T) {
	t.Run("simple_variable_resolution", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			// 生成随机变量名和值
			varName := rapid.StringMatching(`[a-z][a-z0-9_]{0,9}`).Draw(t, "varName")
			varValue := rapid.String().Draw(t, "varValue")

			ctx := NewEvaluationContext()
			ctx.Set(varName, varValue)

			resolver := NewVariableResolver(ctx)

			// 测试简单变量引用
			input := "${" + varName + "}"
			result, err := resolver.ResolveString(input)
			if err != nil {
				t.Fatalf("failed to resolve '%s': %v", input, err)
			}

			if result != varValue {
				t.Fatalf("expected '%s', got '%s'", varValue, result)
			}
		})
	})

	t.Run("nested_path_resolution", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			// 生成嵌套结构
			fieldName := rapid.StringMatching(`[a-z][a-z0-9_]{0,5}`).Draw(t, "fieldName")
			fieldValue := rapid.String().Draw(t, "fieldValue")

			ctx := NewEvaluationContext()
			ctx.Set("response", map[string]any{
				"data": map[string]any{
					fieldName: fieldValue,
				},
			})

			resolver := NewVariableResolver(ctx)

			// 测试嵌套路径
			input := "${response.data." + fieldName + "}"
			result, err := resolver.ResolveString(input)
			if err != nil {
				t.Fatalf("failed to resolve '%s': %v", input, err)
			}

			if result != fieldValue {
				t.Fatalf("expected '%s', got '%s'", fieldValue, result)
			}
		})
	})

	t.Run("array_index_resolution", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			// 生成数组
			numItems := rapid.IntRange(1, 10).Draw(t, "numItems")
			items := make([]any, numItems)
			for i := 0; i < numItems; i++ {
				items[i] = rapid.String().Draw(t, "item")
			}

			ctx := NewEvaluationContext()
			ctx.Set("items", items)

			resolver := NewVariableResolver(ctx)

			// 测试数组索引 (使用索引 0)
			result, err := resolver.ResolveString("${items[0]}")
			if err != nil {
				t.Fatalf("failed to resolve array index: %v", err)
			}

			if result != items[0].(string) {
				t.Fatalf("expected '%s', got '%s'", items[0], result)
			}
		})
	})

	t.Run("mixed_content_resolution", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			// 生成随机变量
			varName := rapid.StringMatching(`[a-z][a-z0-9_]{0,5}`).Draw(t, "varName")
			varValue := rapid.StringMatching(`[a-zA-Z0-9]{1,10}`).Draw(t, "varValue")
			prefix := rapid.StringMatching(`[a-zA-Z]{0,5}`).Draw(t, "prefix")
			suffix := rapid.StringMatching(`[a-zA-Z]{0,5}`).Draw(t, "suffix")

			ctx := NewEvaluationContext()
			ctx.Set(varName, varValue)

			resolver := NewVariableResolver(ctx)

			// 测试混合内容
			input := prefix + "${" + varName + "}" + suffix
			result, err := resolver.ResolveString(input)
			if err != nil {
				t.Fatalf("failed to resolve '%s': %v", input, err)
			}

			expected := prefix + varValue + suffix
			if result != expected {
				t.Fatalf("expected '%s', got '%s'", expected, result)
			}
		})
	})

	t.Run("multiple_variables_resolution", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			// 生成多个变量
			numVars := rapid.IntRange(1, 5).Draw(t, "numVars")
			vars := make(map[string]string)
			varNames := make([]string, 0, numVars)

			for i := 0; i < numVars; i++ {
				name := rapid.StringMatching(`[a-z][a-z0-9]{0,5}`).Draw(t, "varName") + string(rune('a'+i))
				value := rapid.StringMatching(`[a-zA-Z0-9]{1,5}`).Draw(t, "varValue")
				vars[name] = value
				varNames = append(varNames, name)
			}

			ctx := NewEvaluationContext()
			for name, value := range vars {
				ctx.Set(name, value)
			}

			resolver := NewVariableResolver(ctx)

			// 构建包含多个变量的字符串
			var input string
			var expected string
			for _, name := range varNames {
				input += "${" + name + "}-"
				expected += vars[name] + "-"
			}

			result, err := resolver.ResolveString(input)
			if err != nil {
				t.Fatalf("failed to resolve '%s': %v", input, err)
			}

			if result != expected {
				t.Fatalf("expected '%s', got '%s'", expected, result)
			}
		})
	})
}

// TestEnvVariableResolutionProperty 测试环境变量解析
func TestEnvVariableResolutionProperty(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// 生成随机环境变量名和值（只使用可打印字符）
		envName := "TEST_VAR_" + rapid.StringMatching(`[A-Z]{3,8}`).Draw(t, "envName")
		envValue := rapid.StringMatching(`[a-zA-Z0-9_]{1,20}`).Draw(t, "envValue")

		// 设置环境变量
		os.Setenv(envName, envValue)
		defer os.Unsetenv(envName)

		ctx := NewEvaluationContext()
		resolver := NewVariableResolver(ctx)

		// 测试环境变量引用
		input := "${env." + envName + "}"
		result, err := resolver.ResolveString(input)
		if err != nil {
			t.Fatalf("failed to resolve '%s': %v", input, err)
		}

		if result != envValue {
			t.Fatalf("expected '%s', got '%s'", envValue, result)
		}
	})
}

// TestNestedObjectResolutionProperty 测试嵌套对象解析
func TestNestedObjectResolutionProperty(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// 生成嵌套深度
		depth := rapid.IntRange(1, 5).Draw(t, "depth")

		// 构建嵌套结构（使用非空叶子值）
		leafValue := rapid.StringMatching(`[a-zA-Z0-9]{1,10}`).Draw(t, "leafValue")
		var current any = leafValue
		path := ""

		for i := depth - 1; i >= 0; i-- {
			fieldName := "field" + string(rune('a'+i))
			current = map[string]any{fieldName: current}
			if path == "" {
				path = fieldName
			} else {
				path = fieldName + "." + path
			}
		}

		ctx := NewEvaluationContext()
		ctx.Set("root", current)

		resolver := NewVariableResolver(ctx)

		// 测试嵌套路径解析
		input := "${root." + path + "}"
		result, err := resolver.ResolveString(input)
		if err != nil {
			t.Fatalf("failed to resolve '%s': %v", input, err)
		}

		// 验证结果是叶子值
		if result != leafValue {
			t.Fatalf("expected '%s', got '%s'", leafValue, result)
		}
	})
}

// TestArrayWithNestedFieldsProperty 测试数组中嵌套字段的解析
func TestArrayWithNestedFieldsProperty(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// 生成数组
		numItems := rapid.IntRange(1, 5).Draw(t, "numItems")
		items := make([]any, numItems)

		for i := 0; i < numItems; i++ {
			items[i] = map[string]any{
				"id":   i,
				"name": rapid.String().Draw(t, "name"),
			}
		}

		ctx := NewEvaluationContext()
		ctx.Set("users", items)

		resolver := NewVariableResolver(ctx)

		// 测试 ${users[0].name}
		result, err := resolver.ResolveString("${users[0].name}")
		if err != nil {
			t.Fatalf("failed to resolve: %v", err)
		}

		expected := items[0].(map[string]any)["name"].(string)
		if result != expected {
			t.Fatalf("expected '%s', got '%s'", expected, result)
		}
	})
}

// TestUnresolvedVariableProperty 测试未解析变量的处理
func TestUnresolvedVariableProperty(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// 生成不存在的变量名
		varName := rapid.StringMatching(`nonexistent_[a-z]{3,8}`).Draw(t, "varName")

		ctx := NewEvaluationContext()
		resolver := NewVariableResolver(ctx)

		// 未解析的变量应该保持原样
		input := "${" + varName + "}"
		result, err := resolver.ResolveString(input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// 未找到的变量保持原样
		if result != input {
			t.Fatalf("expected '%s' (unchanged), got '%s'", input, result)
		}
	})
}
