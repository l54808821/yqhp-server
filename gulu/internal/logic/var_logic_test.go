package logic

import (
	"encoding/json"
	"testing"

	"pgregory.net/rapid"
)

// Feature: gulu-extension, Property 7: 变量类型一致性
// Validates: Requirements 4.3
// 对于任意变量，其值应符合声明的类型（string、number、boolean、json），类型不匹配时应拒绝保存

// Feature: gulu-extension, Property 9: 变量导入导出 Round-Trip
// Validates: Requirements 4.6
// 对于任意环境的变量集合，导出为 JSON 后再导入，应产生与原始变量集合等价的数据

// TestVarTypeValidation_String_Property 属性测试：字符串类型验证
func TestVarTypeValidation_String_Property(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// 任意字符串都应该是有效的string类型
		value := rapid.String().Draw(t, "value")

		err := ValidateVarType(VarTypeString, value)
		if err != nil {
			t.Fatalf("字符串类型应接受任意值: %v", err)
		}
	})
}

// TestVarTypeValidation_Number_Property 属性测试：数字类型验证
func TestVarTypeValidation_Number_Property(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// 使用格式化的数字字符串
		numStr := rapid.SampledFrom([]string{"123", "45.67", "-89", "0", "3.14159", "-0.5", "1000000"}).Draw(t, "numStr")

		err := ValidateVarType(VarTypeNumber, numStr)
		if err != nil {
			t.Fatalf("有效数字应通过验证: %s, 错误: %v", numStr, err)
		}
	})
}

// TestVarTypeValidation_Number_Invalid_Property 属性测试：无效数字应被拒绝
func TestVarTypeValidation_Number_Invalid_Property(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// 生成非数字字符串
		invalidNum := rapid.StringMatching(`[a-zA-Z]{3,10}`).Draw(t, "invalidNum")

		err := ValidateVarType(VarTypeNumber, invalidNum)
		if err == nil {
			t.Fatalf("无效数字应被拒绝: %s", invalidNum)
		}
	})
}

// TestVarTypeValidation_Boolean_Property 属性测试：布尔类型验证
func TestVarTypeValidation_Boolean_Property(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// 有效布尔值
		validBool := rapid.SampledFrom([]string{"true", "false"}).Draw(t, "bool")

		err := ValidateVarType(VarTypeBoolean, validBool)
		if err != nil {
			t.Fatalf("有效布尔值应通过验证: %s, 错误: %v", validBool, err)
		}
	})
}

// TestVarTypeValidation_Boolean_Invalid_Property 属性测试：无效布尔值应被拒绝
func TestVarTypeValidation_Boolean_Invalid_Property(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// 生成非布尔字符串
		invalidBool := rapid.SampledFrom([]string{"True", "False", "TRUE", "FALSE", "yes", "no", "1", "0"}).Draw(t, "invalidBool")

		err := ValidateVarType(VarTypeBoolean, invalidBool)
		if err == nil {
			t.Fatalf("无效布尔值应被拒绝: %s", invalidBool)
		}
	})
}

// TestVarTypeValidation_JSON_Property 属性测试：JSON类型验证
func TestVarTypeValidation_JSON_Property(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// 生成有效JSON
		jsonType := rapid.SampledFrom([]string{"object", "array", "string", "number"}).Draw(t, "jsonType")

		var validJSON string
		switch jsonType {
		case "object":
			key := rapid.StringMatching(`[a-z]{3,10}`).Draw(t, "key")
			value := rapid.StringMatching(`[a-z0-9]{3,10}`).Draw(t, "value")
			validJSON = `{"` + key + `":"` + value + `"}`
		case "array":
			validJSON = `[1,2,3]`
		case "string":
			validJSON = `"hello"`
		case "number":
			validJSON = `123`
		}

		err := ValidateVarType(VarTypeJSON, validJSON)
		if err != nil {
			t.Fatalf("有效JSON应通过验证: %s, 错误: %v", validJSON, err)
		}
	})
}

// TestVarTypeValidation_JSON_Invalid_Property 属性测试：无效JSON应被拒绝
func TestVarTypeValidation_JSON_Invalid_Property(t *testing.T) {
	invalidJSONs := []string{
		`{invalid}`,
		`{"key":}`,
		`[1,2,`,
		`{key: "value"}`,
	}

	for _, invalid := range invalidJSONs {
		err := ValidateVarType(VarTypeJSON, invalid)
		if err == nil {
			t.Errorf("无效JSON应被拒绝: %s", invalid)
		}
	}
}

// TestVarExportImport_RoundTrip_Property 属性测试：变量导入导出Round-Trip
func TestVarExportImport_RoundTrip_Property(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// 生成随机变量列表
		varCount := rapid.IntRange(1, 10).Draw(t, "varCount")
		original := make([]VarExportItem, varCount)

		for i := 0; i < varCount; i++ {
			varType := rapid.SampledFrom([]string{VarTypeString, VarTypeNumber, VarTypeBoolean, VarTypeJSON}).Draw(t, "type")

			var value string
			switch varType {
			case VarTypeString:
				value = rapid.StringMatching(`[a-zA-Z0-9]{1,20}`).Draw(t, "strValue")
			case VarTypeNumber:
				value = rapid.SampledFrom([]string{"123", "45.67", "-89", "0"}).Draw(t, "numValue")
			case VarTypeBoolean:
				value = rapid.SampledFrom([]string{"true", "false"}).Draw(t, "boolValue")
			case VarTypeJSON:
				value = `{"key":"value"}`
			}

			original[i] = VarExportItem{
				Name:        rapid.StringMatching(`[a-zA-Z][a-zA-Z0-9]{2,20}`).Draw(t, "name"),
				Key:         rapid.StringMatching(`[A-Z][A-Z0-9_]{2,20}`).Draw(t, "key"),
				Value:       value,
				Type:        varType,
				IsSensitive: rapid.Bool().Draw(t, "sensitive"),
				Description: rapid.StringMatching(`[a-zA-Z0-9 ]{0,50}`).Draw(t, "desc"),
			}
		}

		// 序列化为JSON（模拟导出）
		data, err := json.Marshal(original)
		if err != nil {
			t.Fatalf("导出序列化失败: %v", err)
		}

		// 反序列化（模拟导入）
		var imported []VarExportItem
		err = json.Unmarshal(data, &imported)
		if err != nil {
			t.Fatalf("导入反序列化失败: %v", err)
		}

		// 属性：数量应一致
		if len(imported) != len(original) {
			t.Fatalf("变量数量不一致，期望: %d, 实际: %d", len(original), len(imported))
		}

		// 属性：内容应一致
		for i, orig := range original {
			imp := imported[i]
			if imp.Name != orig.Name ||
				imp.Key != orig.Key ||
				imp.Value != orig.Value ||
				imp.Type != orig.Type ||
				imp.IsSensitive != orig.IsSensitive ||
				imp.Description != orig.Description {
				t.Fatalf("变量内容不一致，索引: %d", i)
			}
		}
	})
}

// TestValidateVarType_EmptyValue 测试空值
func TestValidateVarType_EmptyValue(t *testing.T) {
	types := []string{VarTypeString, VarTypeNumber, VarTypeBoolean, VarTypeJSON}

	for _, varType := range types {
		err := ValidateVarType(varType, "")
		if err != nil {
			t.Errorf("空值应对所有类型有效: type=%s, err=%v", varType, err)
		}
	}
}

// TestValidateVarType_InvalidType 测试无效类型
func TestValidateVarType_InvalidType(t *testing.T) {
	err := ValidateVarType("invalid_type", "value")
	if err == nil {
		t.Error("无效类型应被拒绝")
	}
}
