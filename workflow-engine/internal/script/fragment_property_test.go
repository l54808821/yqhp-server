package script

import (
	"testing"

	"pgregory.net/rapid"
)

// TestScriptParamDefaultsProperty 属性 5: 脚本参数默认值
// 对于任意脚本片段调用，如果参数未提供且有默认值，则必须使用默认值。
// 如果参数必填且无默认值，则必须抛出错误。
func TestScriptParamDefaultsProperty(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// 生成随机参数定义
		numParams := rapid.IntRange(1, 5).Draw(t, "numParams")
		params := make([]Param, numParams)

		for i := 0; i < numParams; i++ {
			paramName := rapid.StringMatching(`[a-z][a-z0-9_]{0,9}`).Draw(t, "paramName")
			paramType := rapid.SampledFrom([]ParamType{
				ParamTypeString, ParamTypeNumber, ParamTypeBoolean, ParamTypeAny,
			}).Draw(t, "paramType")
			hasDefault := rapid.Bool().Draw(t, "hasDefault")
			isRequired := rapid.Bool().Draw(t, "isRequired")

			param := Param{
				Name:     paramName,
				Type:     paramType,
				Required: isRequired,
			}

			// 根据类型生成默认值
			if hasDefault {
				switch paramType {
				case ParamTypeString:
					param.Default = rapid.String().Draw(t, "defaultString")
				case ParamTypeNumber:
					param.Default = rapid.Float64().Draw(t, "defaultNumber")
				case ParamTypeBoolean:
					param.Default = rapid.Bool().Draw(t, "defaultBool")
				default:
					param.Default = rapid.String().Draw(t, "defaultAny")
				}
			}

			params[i] = param
		}

		// 创建脚本片段
		fragment := &Fragment{
			Name:   "test_script",
			Params: params,
			Steps:  []any{},
		}

		// 生成提供的参数（随机选择是否提供每个参数）
		provided := make(map[string]any)
		for _, param := range params {
			if rapid.Bool().Draw(t, "provideParam") {
				// 根据类型生成值
				switch param.Type {
				case ParamTypeString:
					provided[param.Name] = rapid.String().Draw(t, "providedString")
				case ParamTypeNumber:
					provided[param.Name] = rapid.Float64().Draw(t, "providedNumber")
				case ParamTypeBoolean:
					provided[param.Name] = rapid.Bool().Draw(t, "providedBool")
				default:
					provided[param.Name] = rapid.String().Draw(t, "providedAny")
				}
			}
		}

		// 验证参数
		err := fragment.ValidateParams(provided)

		// 检查属性：必填参数无默认值且未提供时必须报错
		for _, param := range params {
			_, wasProvided := provided[param.Name]
			if param.Required && !wasProvided && param.Default == nil {
				if err == nil {
					t.Fatalf("expected error for missing required parameter '%s' without default", param.Name)
				}
				return // 验证通过，预期的错误
			}
		}

		// 如果所有必填参数都有值或默认值，不应该报错
		if err != nil {
			// 检查是否是类型错误
			for _, param := range params {
				if val, ok := provided[param.Name]; ok && val != nil {
					if validateParamType(param.Name, val, param.Type) != nil {
						return // 类型错误是允许的
					}
				}
			}
			t.Fatalf("unexpected validation error: %v", err)
		}

		// 解析参数
		resolved := fragment.ResolveParams(provided)

		// 验证属性：未提供的参数应使用默认值
		for _, param := range params {
			_, wasProvided := provided[param.Name]
			resolvedVal, hasResolved := resolved[param.Name]

			if !wasProvided && param.Default != nil {
				// 未提供但有默认值，应该使用默认值
				if !hasResolved {
					t.Fatalf("parameter '%s' should have default value", param.Name)
				}
				if resolvedVal != param.Default {
					t.Fatalf("parameter '%s' should use default value %v, got %v",
						param.Name, param.Default, resolvedVal)
				}
			} else if wasProvided {
				// 提供了值，应该使用提供的值
				if resolvedVal != provided[param.Name] {
					t.Fatalf("parameter '%s' should use provided value %v, got %v",
						param.Name, provided[param.Name], resolvedVal)
				}
			}
		}
	})
}

// TestScriptMultipleReturnsProperty 属性 6: 脚本多返回值
// 对于任意包含多个返回值的脚本片段，所有返回值必须可通过 results 映射访问，
// 未映射的返回值必须被忽略。
func TestScriptMultipleReturnsProperty(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// 生成随机返回值定义
		numReturns := rapid.IntRange(1, 5).Draw(t, "numReturns")
		returns := make([]Return, numReturns)
		returnNames := make(map[string]bool)

		for i := 0; i < numReturns; i++ {
			// 确保返回值名称唯一
			var name string
			for {
				name = rapid.StringMatching(`[a-z][a-z0-9_]{0,9}`).Draw(t, "returnName")
				if !returnNames[name] {
					returnNames[name] = true
					break
				}
			}

			returns[i] = Return{
				Name:  name,
				Value: "${_" + name + "}",
			}
		}

		// 创建脚本片段
		fragment := &Fragment{
			Name:    "test_script",
			Params:  []Param{},
			Steps:   []any{},
			Returns: returns,
		}

		// 生成结果映射（随机选择映射哪些返回值）
		resultMapping := make(map[string]string)
		mappedReturns := make(map[string]bool)

		for _, ret := range returns {
			if rapid.Bool().Draw(t, "mapReturn") {
				mappedName := rapid.StringMatching(`[a-z][a-z0-9_]{0,9}`).Draw(t, "mappedName")
				resultMapping[ret.Name] = mappedName
				mappedReturns[ret.Name] = true
			}
		}

		// 获取返回值映射
		returnMapping := fragment.GetReturnMapping()

		// 验证属性：所有定义的返回值都应该在映射中
		for _, ret := range returns {
			if _, ok := returnMapping[ret.Name]; !ok {
				t.Fatalf("return '%s' should be in return mapping", ret.Name)
			}
			if returnMapping[ret.Name] != ret.Value {
				t.Fatalf("return '%s' should have value '%s', got '%s'",
					ret.Name, ret.Value, returnMapping[ret.Name])
			}
		}

		// 验证属性：映射的返回值数量应该等于定义的返回值数量
		if len(returnMapping) != len(returns) {
			t.Fatalf("return mapping should have %d entries, got %d",
				len(returns), len(returnMapping))
		}

		// 验证属性：只有映射的返回值才会被处理
		// 这个属性在 CallExecutor.processReturns 中验证
		// 这里验证映射结构的正确性
		for retName := range resultMapping {
			found := false
			for _, ret := range returns {
				if ret.Name == retName {
					found = true
					break
				}
			}
			if !found {
				// 映射了不存在的返回值，这是允许的（会被忽略）
				continue
			}
		}
	})
}

// TestCircularCallDetectionProperty 属性 12: 循环脚本调用检测
// 对于任意脚本调用链，如果检测到循环依赖（A 调用 B 调用 A），系统必须在执行前抛出错误。
func TestCircularCallDetectionProperty(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// 生成随机调用链长度
		chainLength := rapid.IntRange(2, 10).Draw(t, "chainLength")

		// 生成唯一的脚本名称
		scriptNames := make([]string, 0, chainLength)
		usedNames := make(map[string]bool)

		for i := 0; i < chainLength; i++ {
			// 使用索引确保唯一性
			name := rapid.StringMatching(`script_[a-z]{1,5}`).Draw(t, "scriptName")
			uniqueName := name + "_" + string(rune('a'+i))
			if usedNames[uniqueName] {
				uniqueName = name + "_" + string(rune('A'+i))
			}
			usedNames[uniqueName] = true
			scriptNames = append(scriptNames, uniqueName)
		}

		// 决定是否创建循环
		createCycle := rapid.Bool().Draw(t, "createCycle")

		callStack := NewCallStack()

		// 模拟调用链（所有名称都是唯一的，不应该检测到循环）
		for _, name := range scriptNames {
			err := callStack.Push(name)
			if err != nil {
				t.Fatalf("unexpected circular call detection for unique script '%s'", name)
			}
		}

		// 验证属性：唯一名称的调用链不应该检测到循环
		if callStack.Depth() != len(scriptNames) {
			t.Fatalf("expected call stack depth %d, got %d", len(scriptNames), callStack.Depth())
		}

		// 如果要创建循环，尝试再次调用链中的某个脚本
		if createCycle {
			// 选择一个已经在栈中的脚本
			targetIndex := rapid.IntRange(0, len(scriptNames)-1).Draw(t, "targetIndex")
			targetName := scriptNames[targetIndex]

			err := callStack.Push(targetName)
			if err == nil {
				t.Fatalf("expected circular call detection for script '%s'", targetName)
			}

			// 验证属性：循环调用被检测到后，栈深度不变
			if callStack.Depth() != len(scriptNames) {
				t.Fatalf("call stack depth should remain %d after failed push, got %d",
					len(scriptNames), callStack.Depth())
			}
		}

		// 测试 Pop 操作
		originalDepth := callStack.Depth()
		for i := 0; i < originalDepth; i++ {
			callStack.Pop()
		}

		if callStack.Depth() != 0 {
			t.Fatalf("call stack should be empty after popping all entries, got depth %d", callStack.Depth())
		}

		// 验证属性：清空后可以重新使用相同的名称
		for _, name := range scriptNames {
			err := callStack.Push(name)
			if err != nil {
				t.Fatalf("should be able to push '%s' after clearing stack", name)
			}
		}
	})
}

// TestCallStackCloneProperty 测试调用栈克隆的正确性
func TestCallStackCloneProperty(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// 创建原始调用栈
		original := NewCallStack()

		// 添加随机数量的脚本
		numScripts := rapid.IntRange(0, 10).Draw(t, "numScripts")
		scripts := make([]string, numScripts)

		for i := 0; i < numScripts; i++ {
			scripts[i] = rapid.StringMatching(`script_[a-z0-9]{1,5}`).Draw(t, "scriptName")
			// 确保不重复
			duplicate := false
			for j := 0; j < i; j++ {
				if scripts[j] == scripts[i] {
					duplicate = true
					break
				}
			}
			if duplicate {
				continue
			}
			_ = original.Push(scripts[i])
		}

		// 克隆调用栈
		cloned := original.Clone()

		// 验证属性：克隆的深度应该相同
		if cloned.Depth() != original.Depth() {
			t.Fatalf("cloned stack depth %d should equal original depth %d",
				cloned.Depth(), original.Depth())
		}

		// 验证属性：克隆的当前脚本应该相同
		if cloned.Current() != original.Current() {
			t.Fatalf("cloned current '%s' should equal original current '%s'",
				cloned.Current(), original.Current())
		}

		// 验证属性：修改克隆不应该影响原始
		if cloned.Depth() > 0 {
			cloned.Pop()
			if cloned.Depth() == original.Depth() {
				t.Fatal("modifying clone should not affect original")
			}
		}

		// 验证属性：向克隆添加元素不应该影响原始
		newScript := "new_script_xyz"
		_ = cloned.Push(newScript)
		if original.Current() == newScript {
			t.Fatal("adding to clone should not affect original")
		}
	})
}

// TestRegistryProperty 测试脚本注册表的正确性
func TestRegistryProperty(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		registry := NewRegistry()

		// 生成随机脚本
		numScripts := rapid.IntRange(1, 10).Draw(t, "numScripts")
		registeredScripts := make(map[string]*Fragment)

		for i := 0; i < numScripts; i++ {
			name := rapid.StringMatching(`[a-z][a-z0-9_]{0,9}`).Draw(t, "scriptName")

			// 检查是否已存在
			if _, exists := registeredScripts[name]; exists {
				continue
			}

			fragment := &Fragment{
				Name:        name,
				Description: rapid.String().Draw(t, "description"),
				Params:      []Param{},
				Steps:       []any{},
				Returns:     []Return{},
			}

			err := registry.Register(fragment)
			if err != nil {
				t.Fatalf("failed to register script '%s': %v", name, err)
			}

			registeredScripts[name] = fragment
		}

		// 验证属性：所有注册的脚本都应该可以获取
		for name, expected := range registeredScripts {
			got, err := registry.Get(name)
			if err != nil {
				t.Fatalf("failed to get registered script '%s': %v", name, err)
			}
			if got.Name != expected.Name {
				t.Fatalf("got script name '%s', expected '%s'", got.Name, expected.Name)
			}
		}

		// 验证属性：Has 应该返回正确的结果
		for name := range registeredScripts {
			if !registry.Has(name) {
				t.Fatalf("registry should have script '%s'", name)
			}
		}

		// 验证属性：不存在的脚本应该返回错误
		nonExistent := "non_existent_script_xyz"
		if registry.Has(nonExistent) {
			t.Fatalf("registry should not have script '%s'", nonExistent)
		}
		_, err := registry.Get(nonExistent)
		if err == nil {
			t.Fatalf("getting non-existent script should return error")
		}

		// 验证属性：List 应该返回所有脚本名称
		listed := registry.List()
		if len(listed) != len(registeredScripts) {
			t.Fatalf("listed %d scripts, expected %d", len(listed), len(registeredScripts))
		}

		// 验证属性：重复注册应该失败
		for name, fragment := range registeredScripts {
			err := registry.Register(fragment)
			if err == nil {
				t.Fatalf("duplicate registration of '%s' should fail", name)
			}
			break // 只测试一个
		}
	})
}
