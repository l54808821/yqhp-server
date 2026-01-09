package logic

import (
	"encoding/json"
	"testing"

	"pgregory.net/rapid"
)

// Feature: gulu-extension, Property 5: 域名格式验证
// Validates: Requirements 3.6
// 对于任意域名配置，其 BaseURL 应符合有效的 URL 格式（包含协议和主机名）

// Feature: gulu-extension, Property 6: 域名请求头存储
// Validates: Requirements 3.3
// 对于任意域名配置，其 Headers 字段应为有效的 JSON 数组格式

// TestDomainURLFormat_Property 属性测试：域名URL格式验证
func TestDomainURLFormat_Property(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// 生成有效的URL
		protocol := rapid.SampledFrom([]string{"http", "https"}).Draw(t, "protocol")
		host := rapid.StringMatching(`[a-z][a-z0-9]{2,20}`).Draw(t, "host")
		tld := rapid.SampledFrom([]string{"com", "org", "net", "io", "cn"}).Draw(t, "tld")

		validURL := protocol + "://" + host + "." + tld

		// 属性：有效URL应通过验证
		err := ValidateURL(validURL)
		if err != nil {
			t.Fatalf("有效URL应通过验证: %s, 错误: %v", validURL, err)
		}
	})
}

// TestDomainURLFormat_InvalidProtocol_Property 属性测试：无效协议应被拒绝
func TestDomainURLFormat_InvalidProtocol_Property(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// 生成无效协议的URL
		invalidProtocol := rapid.SampledFrom([]string{"ftp", "ssh", "file", "mailto"}).Draw(t, "protocol")
		host := rapid.StringMatching(`[a-z][a-z0-9]{2,20}`).Draw(t, "host")

		invalidURL := invalidProtocol + "://" + host + ".com"

		// 属性：无效协议应被拒绝
		err := ValidateURL(invalidURL)
		if err == nil {
			t.Fatalf("无效协议URL应被拒绝: %s", invalidURL)
		}
	})
}

// TestDomainURLFormat_MissingProtocol_Property 属性测试：缺少协议应被拒绝
func TestDomainURLFormat_MissingProtocol_Property(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// 生成缺少协议的URL
		host := rapid.StringMatching(`[a-z][a-z0-9]{2,20}`).Draw(t, "host")
		urlWithoutProtocol := host + ".com"

		// 属性：缺少协议应被拒绝
		err := ValidateURL(urlWithoutProtocol)
		if err == nil {
			t.Fatalf("缺少协议的URL应被拒绝: %s", urlWithoutProtocol)
		}
	})
}

// TestDomainHeadersJSON_Property 属性测试：Headers JSON格式验证
func TestDomainHeadersJSON_Property(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// 生成随机数量的Header项
		headerCount := rapid.IntRange(0, 10).Draw(t, "headerCount")
		headers := make([]HeaderItem, headerCount)

		for i := 0; i < headerCount; i++ {
			headers[i] = HeaderItem{
				Key:   rapid.StringMatching(`[A-Za-z][A-Za-z0-9-]{2,30}`).Draw(t, "headerKey"),
				Value: rapid.String().Draw(t, "headerValue"),
			}
		}

		// 序列化为JSON
		data, err := json.Marshal(headers)
		if err != nil {
			t.Fatalf("Headers序列化失败: %v", err)
		}

		// 属性：序列化后应能正确反序列化
		var parsed []HeaderItem
		err = json.Unmarshal(data, &parsed)
		if err != nil {
			t.Fatalf("Headers反序列化失败: %v", err)
		}

		// 属性：反序列化后数量应一致
		if len(parsed) != len(headers) {
			t.Fatalf("Headers数量不一致，期望: %d, 实际: %d", len(headers), len(parsed))
		}

		// 属性：反序列化后内容应一致
		for i, h := range headers {
			if parsed[i].Key != h.Key || parsed[i].Value != h.Value {
				t.Fatalf("Headers内容不一致，索引: %d", i)
			}
		}
	})
}

// TestDomainHeadersRoundTrip_Property 属性测试：Headers Round-Trip
func TestDomainHeadersRoundTrip_Property(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// 生成随机Headers
		headerCount := rapid.IntRange(1, 5).Draw(t, "headerCount")
		original := make([]HeaderItem, headerCount)

		for i := 0; i < headerCount; i++ {
			original[i] = HeaderItem{
				Key:   rapid.SampledFrom([]string{"Content-Type", "Authorization", "X-Custom-Header", "Accept", "User-Agent"}).Draw(t, "key"),
				Value: rapid.StringMatching(`[a-zA-Z0-9/;=\- ]{1,50}`).Draw(t, "value"),
			}
		}

		// 序列化
		data, err := json.Marshal(original)
		if err != nil {
			t.Fatalf("序列化失败: %v", err)
		}

		// 反序列化
		var restored []HeaderItem
		err = json.Unmarshal(data, &restored)
		if err != nil {
			t.Fatalf("反序列化失败: %v", err)
		}

		// 属性：Round-Trip后应完全一致
		if len(restored) != len(original) {
			t.Fatalf("Round-Trip后数量不一致")
		}

		for i := range original {
			if restored[i].Key != original[i].Key || restored[i].Value != original[i].Value {
				t.Fatalf("Round-Trip后内容不一致，索引: %d", i)
			}
		}
	})
}

// TestValidateURL_EmptyURL 测试空URL
func TestValidateURL_EmptyURL(t *testing.T) {
	err := ValidateURL("")
	if err == nil {
		t.Fatal("空URL应被拒绝")
	}
}

// TestValidateURL_ValidURLs 测试有效URL
func TestValidateURL_ValidURLs(t *testing.T) {
	validURLs := []string{
		"http://example.com",
		"https://example.com",
		"http://localhost:8080",
		"https://api.example.com/v1",
		"http://192.168.1.1:3000",
	}

	for _, u := range validURLs {
		err := ValidateURL(u)
		if err != nil {
			t.Errorf("有效URL应通过验证: %s, 错误: %v", u, err)
		}
	}
}

// TestValidateURL_InvalidURLs 测试无效URL
func TestValidateURL_InvalidURLs(t *testing.T) {
	invalidURLs := []string{
		"example.com",       // 缺少协议
		"ftp://example.com", // 无效协议
		"http://",           // 缺少主机名
		"://example.com",    // 缺少协议名
	}

	for _, u := range invalidURLs {
		err := ValidateURL(u)
		if err == nil {
			t.Errorf("无效URL应被拒绝: %s", u)
		}
	}
}
