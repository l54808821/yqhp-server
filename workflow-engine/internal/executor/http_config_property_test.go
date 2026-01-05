package executor

import (
	"testing"
	"time"

	"pgregory.net/rapid"
)

// TestProperty_HTTPConfigMergePriority tests Property 4:
// For any HTTP request, configuration must be merged with priority:
// step config > workflow config > global config.
// Step-level settings must override global settings.
//
// **属性 4: HTTP 配置合并优先级**
// **验证需求: 4.6, 8.5**
func TestProperty_HTTPConfigMergePriority(t *testing.T) {
	t.Run("step_config_overrides_global", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			// 生成全局配置
			globalBaseURL := rapid.StringMatching(`https://global[a-z]{3,8}\.com`).Draw(t, "globalBaseURL")
			globalHeader := rapid.StringMatching(`global-[a-z]{3,10}`).Draw(t, "globalHeader")
			globalTimeout := rapid.IntRange(10, 30).Draw(t, "globalTimeout")

			// 生成步骤配置（应该覆盖全局）
			stepHeader := rapid.StringMatching(`step-[a-z]{3,10}`).Draw(t, "stepHeader")
			stepTimeout := rapid.IntRange(31, 60).Draw(t, "stepTimeout")

			// 创建全局配置
			globalConfig := &HTTPGlobalConfig{
				BaseURL: globalBaseURL,
				Headers: map[string]string{
					"X-Custom": globalHeader,
				},
				Domains: make(map[string]string),
				Timeout: TimeoutConfig{
					Request: time.Duration(globalTimeout) * time.Second,
				},
			}

			// 创建步骤配置
			stepConfig := &HTTPGlobalConfig{
				Headers: map[string]string{
					"X-Custom": stepHeader,
				},
				Domains: make(map[string]string),
				Timeout: TimeoutConfig{
					Request: time.Duration(stepTimeout) * time.Second,
				},
			}

			// 合并配置
			merged := globalConfig.Merge(stepConfig)

			// 属性验证：步骤配置应该覆盖全局配置
			if merged.Headers["X-Custom"] != stepHeader {
				t.Errorf("step header should override global: got %s, want %s",
					merged.Headers["X-Custom"], stepHeader)
			}

			if merged.Timeout.Request != time.Duration(stepTimeout)*time.Second {
				t.Errorf("step timeout should override global: got %v, want %v",
					merged.Timeout.Request, time.Duration(stepTimeout)*time.Second)
			}

			// 全局 BaseURL 应该保留（步骤没有设置）
			if merged.BaseURL != globalBaseURL {
				t.Errorf("global base_url should be preserved: got %s, want %s",
					merged.BaseURL, globalBaseURL)
			}
		})
	})

	t.Run("headers_merge_correctly", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			// 生成全局 headers
			globalOnlyKey := rapid.StringMatching(`Global-[A-Z][a-z]{3,8}`).Draw(t, "globalOnlyKey")
			globalOnlyValue := rapid.StringMatching(`[a-z]{5,15}`).Draw(t, "globalOnlyValue")
			sharedKey := rapid.StringMatching(`Shared-[A-Z][a-z]{3,8}`).Draw(t, "sharedKey")
			globalSharedValue := rapid.StringMatching(`global-[a-z]{5,10}`).Draw(t, "globalSharedValue")

			// 生成步骤 headers
			stepOnlyKey := rapid.StringMatching(`Step-[A-Z][a-z]{3,8}`).Draw(t, "stepOnlyKey")
			stepOnlyValue := rapid.StringMatching(`[a-z]{5,15}`).Draw(t, "stepOnlyValue")
			stepSharedValue := rapid.StringMatching(`step-[a-z]{5,10}`).Draw(t, "stepSharedValue")

			globalConfig := &HTTPGlobalConfig{
				Headers: map[string]string{
					globalOnlyKey: globalOnlyValue,
					sharedKey:     globalSharedValue,
				},
				Domains: make(map[string]string),
			}

			stepConfig := &HTTPGlobalConfig{
				Headers: map[string]string{
					stepOnlyKey: stepOnlyValue,
					sharedKey:   stepSharedValue,
				},
				Domains: make(map[string]string),
			}

			merged := globalConfig.Merge(stepConfig)

			// 属性验证：全局独有的 header 应该保留
			if merged.Headers[globalOnlyKey] != globalOnlyValue {
				t.Errorf("global-only header should be preserved: got %s, want %s",
					merged.Headers[globalOnlyKey], globalOnlyValue)
			}

			// 属性验证：步骤独有的 header 应该添加
			if merged.Headers[stepOnlyKey] != stepOnlyValue {
				t.Errorf("step-only header should be added: got %s, want %s",
					merged.Headers[stepOnlyKey], stepOnlyValue)
			}

			// 属性验证：共享的 header 应该被步骤覆盖
			if merged.Headers[sharedKey] != stepSharedValue {
				t.Errorf("shared header should be overridden by step: got %s, want %s",
					merged.Headers[sharedKey], stepSharedValue)
			}
		})
	})

	t.Run("ssl_config_merge", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			globalVerify := rapid.Bool().Draw(t, "globalVerify")
			stepVerify := rapid.Bool().Draw(t, "stepVerify")

			globalConfig := &HTTPGlobalConfig{
				Headers: make(map[string]string),
				Domains: make(map[string]string),
				SSL: SSLConfig{
					Verify: &globalVerify,
				},
			}

			stepConfig := &HTTPGlobalConfig{
				Headers: make(map[string]string),
				Domains: make(map[string]string),
				SSL: SSLConfig{
					Verify: &stepVerify,
				},
			}

			merged := globalConfig.Merge(stepConfig)

			// 属性验证：步骤 SSL 配置应该覆盖全局
			if merged.SSL.GetVerify() != stepVerify {
				t.Errorf("step SSL verify should override global: got %v, want %v",
					merged.SSL.GetVerify(), stepVerify)
			}
		})
	})

	t.Run("redirect_config_merge", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			globalFollow := rapid.Bool().Draw(t, "globalFollow")
			globalMaxRedirects := rapid.IntRange(1, 10).Draw(t, "globalMaxRedirects")
			stepFollow := rapid.Bool().Draw(t, "stepFollow")
			stepMaxRedirects := rapid.IntRange(11, 20).Draw(t, "stepMaxRedirects")

			globalConfig := &HTTPGlobalConfig{
				Headers: make(map[string]string),
				Domains: make(map[string]string),
				Redirect: RedirectConfig{
					Follow:       &globalFollow,
					MaxRedirects: &globalMaxRedirects,
				},
			}

			stepConfig := &HTTPGlobalConfig{
				Headers: make(map[string]string),
				Domains: make(map[string]string),
				Redirect: RedirectConfig{
					Follow:       &stepFollow,
					MaxRedirects: &stepMaxRedirects,
				},
			}

			merged := globalConfig.Merge(stepConfig)

			// 属性验证：步骤重定向配置应该覆盖全局
			if merged.Redirect.GetFollow() != stepFollow {
				t.Errorf("step redirect follow should override global: got %v, want %v",
					merged.Redirect.GetFollow(), stepFollow)
			}
			if merged.Redirect.GetMaxRedirects() != stepMaxRedirects {
				t.Errorf("step max_redirects should override global: got %v, want %v",
					merged.Redirect.GetMaxRedirects(), stepMaxRedirects)
			}
		})
	})

	t.Run("timeout_config_merge", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			globalConnect := rapid.IntRange(1, 10).Draw(t, "globalConnect")
			globalRead := rapid.IntRange(10, 30).Draw(t, "globalRead")
			stepConnect := rapid.IntRange(11, 20).Draw(t, "stepConnect")
			stepRequest := rapid.IntRange(30, 60).Draw(t, "stepRequest")

			globalConfig := &HTTPGlobalConfig{
				Headers: make(map[string]string),
				Domains: make(map[string]string),
				Timeout: TimeoutConfig{
					Connect: time.Duration(globalConnect) * time.Second,
					Read:    time.Duration(globalRead) * time.Second,
				},
			}

			stepConfig := &HTTPGlobalConfig{
				Headers: make(map[string]string),
				Domains: make(map[string]string),
				Timeout: TimeoutConfig{
					Connect: time.Duration(stepConnect) * time.Second,
					Request: time.Duration(stepRequest) * time.Second,
				},
			}

			merged := globalConfig.Merge(stepConfig)

			// 属性验证：步骤超时配置应该覆盖全局
			if merged.Timeout.Connect != time.Duration(stepConnect)*time.Second {
				t.Errorf("step connect timeout should override global: got %v, want %v",
					merged.Timeout.Connect, time.Duration(stepConnect)*time.Second)
			}

			// 属性验证：全局独有的超时配置应该保留
			if merged.Timeout.Read != time.Duration(globalRead)*time.Second {
				t.Errorf("global read timeout should be preserved: got %v, want %v",
					merged.Timeout.Read, time.Duration(globalRead)*time.Second)
			}

			// 属性验证：步骤独有的超时配置应该添加
			if merged.Timeout.Request != time.Duration(stepRequest)*time.Second {
				t.Errorf("step request timeout should be added: got %v, want %v",
					merged.Timeout.Request, time.Duration(stepRequest)*time.Second)
			}
		})
	})

	t.Run("domains_merge_correctly", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			// 生成域名配置
			globalDomainKey := rapid.StringMatching(`global[a-z]{2,5}`).Draw(t, "globalDomainKey")
			globalDomainURL := rapid.StringMatching(`https://global[a-z]{3,8}\.com`).Draw(t, "globalDomainURL")
			stepDomainKey := rapid.StringMatching(`step[a-z]{2,5}`).Draw(t, "stepDomainKey")
			stepDomainURL := rapid.StringMatching(`https://step[a-z]{3,8}\.com`).Draw(t, "stepDomainURL")
			sharedDomainKey := rapid.StringMatching(`shared[a-z]{2,5}`).Draw(t, "sharedDomainKey")
			globalSharedURL := rapid.StringMatching(`https://globalshared[a-z]{3,8}\.com`).Draw(t, "globalSharedURL")
			stepSharedURL := rapid.StringMatching(`https://stepshared[a-z]{3,8}\.com`).Draw(t, "stepSharedURL")

			globalConfig := &HTTPGlobalConfig{
				Headers: make(map[string]string),
				Domains: map[string]string{
					globalDomainKey: globalDomainURL,
					sharedDomainKey: globalSharedURL,
				},
			}

			stepConfig := &HTTPGlobalConfig{
				Headers: make(map[string]string),
				Domains: map[string]string{
					stepDomainKey:   stepDomainURL,
					sharedDomainKey: stepSharedURL,
				},
			}

			merged := globalConfig.Merge(stepConfig)

			// 属性验证：全局独有的域名应该保留
			if merged.Domains[globalDomainKey] != globalDomainURL {
				t.Errorf("global-only domain should be preserved: got %s, want %s",
					merged.Domains[globalDomainKey], globalDomainURL)
			}

			// 属性验证：步骤独有的域名应该添加
			if merged.Domains[stepDomainKey] != stepDomainURL {
				t.Errorf("step-only domain should be added: got %s, want %s",
					merged.Domains[stepDomainKey], stepDomainURL)
			}

			// 属性验证：共享的域名应该被步骤覆盖
			if merged.Domains[sharedDomainKey] != stepSharedURL {
				t.Errorf("shared domain should be overridden by step: got %s, want %s",
					merged.Domains[sharedDomainKey], stepSharedURL)
			}
		})
	})
}

// TestProperty_URLResolution tests URL resolution with multi-domain support.
// **验证需求: 4.1, 4.3, 4.4, 4.5**
func TestProperty_URLResolution(t *testing.T) {
	t.Run("absolute_url_unchanged", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			// 生成完整 URL
			protocol := rapid.SampledFrom([]string{"http://", "https://"}).Draw(t, "protocol")
			domain := rapid.StringMatching(`[a-z]{3,10}\.[a-z]{2,4}`).Draw(t, "domain")
			path := rapid.StringMatching(`/[a-z]{2,10}`).Draw(t, "path")
			fullURL := protocol + domain + path

			config := &HTTPGlobalConfig{
				BaseURL: "https://should-not-use.com",
				Headers: make(map[string]string),
				Domains: map[string]string{
					"api": "https://api.example.com",
				},
			}

			// 属性验证：完整 URL 应该保持不变
			resolved := config.ResolveURL(fullURL, "")
			if resolved != fullURL {
				t.Errorf("absolute URL should remain unchanged: got %s, want %s",
					resolved, fullURL)
			}

			// 即使指定了域名，完整 URL 也应该保持不变
			resolved = config.ResolveURL(fullURL, "api")
			if resolved != fullURL {
				t.Errorf("absolute URL should remain unchanged even with domain: got %s, want %s",
					resolved, fullURL)
			}
		})
	})

	t.Run("relative_url_uses_base", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			baseURL := rapid.StringMatching(`https://[a-z]{3,10}\.com`).Draw(t, "baseURL")
			path := rapid.StringMatching(`/[a-z]{2,10}/[a-z]{2,10}`).Draw(t, "path")

			config := &HTTPGlobalConfig{
				BaseURL: baseURL,
				Headers: make(map[string]string),
				Domains: make(map[string]string),
			}

			resolved := config.ResolveURL(path, "")

			// 属性验证：相对 URL 应该拼接 base_url
			expected := baseURL + path
			if resolved != expected {
				t.Errorf("relative URL should use base_url: got %s, want %s",
					resolved, expected)
			}
		})
	})

	t.Run("domain_overrides_base", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			baseURL := rapid.StringMatching(`https://base[a-z]{3,8}\.com`).Draw(t, "baseURL")
			domainURL := rapid.StringMatching(`https://domain[a-z]{3,8}\.com`).Draw(t, "domainURL")
			path := rapid.StringMatching(`/[a-z]{2,10}`).Draw(t, "path")

			config := &HTTPGlobalConfig{
				BaseURL: baseURL,
				Headers: make(map[string]string),
				Domains: map[string]string{
					"api": domainURL,
				},
			}

			resolved := config.ResolveURL(path, "api")

			// 属性验证：指定域名时应该使用域名 URL
			expected := domainURL + path
			if resolved != expected {
				t.Errorf("domain should override base_url: got %s, want %s",
					resolved, expected)
			}
		})
	})

	t.Run("unknown_domain_uses_base", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			baseURL := rapid.StringMatching(`https://base[a-z]{3,8}\.com`).Draw(t, "baseURL")
			path := rapid.StringMatching(`/[a-z]{2,10}`).Draw(t, "path")

			config := &HTTPGlobalConfig{
				BaseURL: baseURL,
				Headers: make(map[string]string),
				Domains: map[string]string{
					"api": "https://api.example.com",
				},
			}

			// 使用未知域名
			resolved := config.ResolveURL(path, "unknown")

			// 属性验证：未知域名应该回退到 base_url
			expected := baseURL + path
			if resolved != expected {
				t.Errorf("unknown domain should fallback to base_url: got %s, want %s",
					resolved, expected)
			}
		})
	})
}
