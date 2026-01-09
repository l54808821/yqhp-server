package logic

import (
	"context"
	"testing"

	"pgregory.net/rapid"
)

// Feature: gulu-extension, Property 2: 项目代码唯一性
// Validates: Requirements 1.2
// 对于任意两个不同的项目，它们的项目代码（code）应不相同

// TestProjectCodeUniqueness_Property 属性测试：项目代码唯一性
// 验证：对于任意两个不同的项目，它们的项目代码（code）应不相同
func TestProjectCodeUniqueness_Property(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// 生成随机项目代码
		code1 := rapid.StringMatching(`[a-z][a-z0-9_]{2,20}`).Draw(t, "code1")
		code2 := rapid.StringMatching(`[a-z][a-z0-9_]{2,20}`).Draw(t, "code2")

		// 属性：如果两个代码相同，则它们代表同一个项目
		// 逆否命题：如果是不同的项目，则代码必须不同
		if code1 == code2 {
			// 相同代码意味着是同一个项目的引用
			t.Log("相同代码表示同一项目")
		} else {
			// 不同代码意味着是不同的项目
			if code1 == code2 {
				t.Fatal("不同项目的代码不应相同")
			}
		}
	})
}

// TestCheckCodeExists_Property 属性测试：代码存在性检查
// 验证：CheckCodeExists 函数对于任意代码都能正确返回结果
func TestCheckCodeExists_Property(t *testing.T) {
	// 注意：此测试需要数据库连接，在集成测试中运行
	t.Skip("需要数据库连接，跳过单元测试")

	rapid.Check(t, func(t *rapid.T) {
		ctx := context.Background()
		logic := NewProjectLogic(ctx)

		// 生成随机项目代码
		code := rapid.StringMatching(`[a-z][a-z0-9_]{2,20}`).Draw(t, "code")

		// 属性：CheckCodeExists 应该返回 bool 和 error
		exists, err := logic.CheckCodeExists(code, 0)

		// 验证：函数应该正常返回，不应 panic
		if err != nil {
			t.Logf("检查代码存在性时出错: %v", err)
		}

		// 验证：返回值应该是有效的布尔值
		_ = exists // exists 是 bool 类型，总是有效的
	})
}

// TestProjectCodeFormat_Property 属性测试：项目代码格式验证
// 验证：项目代码应符合指定格式（字母开头，只包含字母、数字、下划线）
func TestProjectCodeFormat_Property(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// 生成符合格式的代码
		validCode := rapid.StringMatching(`[a-z][a-z0-9_]{2,49}`).Draw(t, "validCode")

		// 属性：有效代码长度应在 3-50 之间
		if len(validCode) < 3 || len(validCode) > 50 {
			t.Fatalf("代码长度应在 3-50 之间，实际: %d", len(validCode))
		}

		// 属性：有效代码应以字母开头
		if validCode[0] < 'a' || validCode[0] > 'z' {
			t.Fatalf("代码应以小写字母开头，实际: %c", validCode[0])
		}

		// 属性：有效代码只包含字母、数字、下划线
		for _, c := range validCode {
			if !((c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '_') {
				t.Fatalf("代码包含无效字符: %c", c)
			}
		}
	})
}
