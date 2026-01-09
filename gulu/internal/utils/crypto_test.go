package utils

import (
	"testing"

	"pgregory.net/rapid"
)

// Feature: gulu-extension, Property 8: 敏感数据加密存储
// Validates: Requirements 4.4, 5.4, 6.4
// 对于任意标记为敏感的变量或密码字段，存储在数据库中的值应与原始值不同（已加密），且解密后应等于原始值

// TestEncryptDecrypt_RoundTrip_Property 属性测试：加密解密Round-Trip
func TestEncryptDecrypt_RoundTrip_Property(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// 生成随机明文
		plaintext := rapid.String().Draw(t, "plaintext")

		// 加密
		encrypted, err := Encrypt(plaintext)
		if err != nil {
			t.Fatalf("加密失败: %v", err)
		}

		// 属性1：非空明文加密后应与原文不同
		if plaintext != "" && encrypted == plaintext {
			t.Fatal("加密后的值不应与原值相同")
		}

		// 解密
		decrypted, err := Decrypt(encrypted)
		if err != nil {
			t.Fatalf("解密失败: %v", err)
		}

		// 属性2：解密后应等于原始值
		if decrypted != plaintext {
			t.Fatalf("解密后的值应等于原始值，期望: %s, 实际: %s", plaintext, decrypted)
		}
	})
}

// TestEncrypt_DifferentFromOriginal_Property 属性测试：加密后与原值不同
func TestEncrypt_DifferentFromOriginal_Property(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// 生成非空随机明文
		plaintext := rapid.StringMatching(`[a-zA-Z0-9]{1,100}`).Draw(t, "plaintext")

		// 加密
		encrypted, err := Encrypt(plaintext)
		if err != nil {
			t.Fatalf("加密失败: %v", err)
		}

		// 属性：加密后的值应与原值不同
		if encrypted == plaintext {
			t.Fatal("加密后的值不应与原值相同")
		}
	})
}

// TestEncrypt_Randomness_Property 属性测试：相同明文加密结果不同（因为使用随机nonce）
func TestEncrypt_Randomness_Property(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// 生成非空随机明文
		plaintext := rapid.StringMatching(`[a-zA-Z0-9]{5,50}`).Draw(t, "plaintext")

		// 加密两次
		encrypted1, err := Encrypt(plaintext)
		if err != nil {
			t.Fatalf("第一次加密失败: %v", err)
		}

		encrypted2, err := Encrypt(plaintext)
		if err != nil {
			t.Fatalf("第二次加密失败: %v", err)
		}

		// 属性：相同明文的两次加密结果应不同（因为使用随机nonce）
		if encrypted1 == encrypted2 {
			t.Fatal("相同明文的两次加密结果应不同")
		}

		// 但解密后都应等于原文
		decrypted1, _ := Decrypt(encrypted1)
		decrypted2, _ := Decrypt(encrypted2)

		if decrypted1 != plaintext || decrypted2 != plaintext {
			t.Fatal("解密后应等于原始明文")
		}
	})
}

// TestEncrypt_EmptyString 测试空字符串
func TestEncrypt_EmptyString(t *testing.T) {
	encrypted, err := Encrypt("")
	if err != nil {
		t.Fatalf("加密空字符串失败: %v", err)
	}

	if encrypted != "" {
		t.Fatal("空字符串加密后应为空")
	}

	decrypted, err := Decrypt("")
	if err != nil {
		t.Fatalf("解密空字符串失败: %v", err)
	}

	if decrypted != "" {
		t.Fatal("空字符串解密后应为空")
	}
}

// TestEncrypt_SpecialCharacters 测试特殊字符
func TestEncrypt_SpecialCharacters(t *testing.T) {
	testCases := []string{
		"password123!@#$%",
		"中文密码测试",
		"emoji🔐🔑",
		"spaces and\ttabs\nnewlines",
		`quotes"and'special`,
	}

	for _, tc := range testCases {
		encrypted, err := Encrypt(tc)
		if err != nil {
			t.Errorf("加密失败 [%s]: %v", tc, err)
			continue
		}

		decrypted, err := Decrypt(encrypted)
		if err != nil {
			t.Errorf("解密失败 [%s]: %v", tc, err)
			continue
		}

		if decrypted != tc {
			t.Errorf("Round-trip失败，期望: %s, 实际: %s", tc, decrypted)
		}
	}
}
