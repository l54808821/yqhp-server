package executor

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"os"
	"time"
)

// HTTPGlobalConfig HTTP 全局配置
type HTTPGlobalConfig struct {
	BaseURL  string            `yaml:"base_url,omitempty" json:"base_url,omitempty"` // 基础 URL
	Headers  map[string]string `yaml:"headers,omitempty" json:"headers,omitempty"`   // 全局请求头
	Domains  map[string]string `yaml:"domains,omitempty" json:"domains,omitempty"`   // 多域名配置
	SSL      SSLConfig         `yaml:"ssl,omitempty" json:"ssl,omitempty"`           // SSL 配置
	Redirect RedirectConfig    `yaml:"redirect,omitempty" json:"redirect,omitempty"` // 重定向配置
	Timeout  TimeoutConfig     `yaml:"timeout,omitempty" json:"timeout,omitempty"`   // 超时配置
}

// SSLConfig SSL/TLS 配置
type SSLConfig struct {
	Verify   *bool  `yaml:"verify,omitempty" json:"verify,omitempty"` // 是否验证证书，默认 true
	CertPath string `yaml:"cert,omitempty" json:"cert,omitempty"`     // 客户端证书路径
	KeyPath  string `yaml:"key,omitempty" json:"key,omitempty"`       // 客户端私钥路径
	CAPath   string `yaml:"ca,omitempty" json:"ca,omitempty"`         // CA 证书路径
}

// RedirectConfig 重定向配置
type RedirectConfig struct {
	Follow       *bool `yaml:"follow,omitempty" json:"follow,omitempty"`               // 是否跟随重定向，默认 true
	MaxRedirects *int  `yaml:"max_redirects,omitempty" json:"max_redirects,omitempty"` // 最大重定向次数，默认 10
}

// TimeoutConfig 超时配置
type TimeoutConfig struct {
	Connect time.Duration `yaml:"connect,omitempty" json:"connect,omitempty"` // 连接超时，默认 10s
	Read    time.Duration `yaml:"read,omitempty" json:"read,omitempty"`       // 读取超时，默认 30s
	Write   time.Duration `yaml:"write,omitempty" json:"write,omitempty"`     // 写入超时，默认 30s
	Request time.Duration `yaml:"request,omitempty" json:"request,omitempty"` // 总请求超时，默认 60s
}

// DefaultHTTPGlobalConfig 返回默认 HTTP 全局配置
func DefaultHTTPGlobalConfig() *HTTPGlobalConfig {
	verify := true
	follow := true
	maxRedirects := 10

	return &HTTPGlobalConfig{
		Headers: make(map[string]string),
		Domains: make(map[string]string),
		SSL: SSLConfig{
			Verify: &verify,
		},
		Redirect: RedirectConfig{
			Follow:       &follow,
			MaxRedirects: &maxRedirects,
		},
		Timeout: TimeoutConfig{
			Connect: 10 * time.Second,
			Read:    30 * time.Second,
			Write:   30 * time.Second,
			Request: 60 * time.Second,
		},
	}
}

// Merge 合并配置，other 优先级更高
func (c *HTTPGlobalConfig) Merge(other *HTTPGlobalConfig) *HTTPGlobalConfig {
	if other == nil {
		return c
	}

	result := &HTTPGlobalConfig{
		BaseURL:  c.BaseURL,
		Headers:  make(map[string]string),
		Domains:  make(map[string]string),
		SSL:      c.SSL,
		Redirect: c.Redirect,
		Timeout:  c.Timeout,
	}

	// 复制 headers
	for k, v := range c.Headers {
		result.Headers[k] = v
	}
	for k, v := range other.Headers {
		result.Headers[k] = v
	}

	// 复制 domains
	for k, v := range c.Domains {
		result.Domains[k] = v
	}
	for k, v := range other.Domains {
		result.Domains[k] = v
	}

	// 覆盖 BaseURL
	if other.BaseURL != "" {
		result.BaseURL = other.BaseURL
	}

	// 合并 SSL
	if other.SSL.Verify != nil {
		result.SSL.Verify = other.SSL.Verify
	}
	if other.SSL.CertPath != "" {
		result.SSL.CertPath = other.SSL.CertPath
	}
	if other.SSL.KeyPath != "" {
		result.SSL.KeyPath = other.SSL.KeyPath
	}
	if other.SSL.CAPath != "" {
		result.SSL.CAPath = other.SSL.CAPath
	}

	// 合并 Redirect
	if other.Redirect.Follow != nil {
		result.Redirect.Follow = other.Redirect.Follow
	}
	if other.Redirect.MaxRedirects != nil {
		result.Redirect.MaxRedirects = other.Redirect.MaxRedirects
	}

	// 合并 Timeout
	if other.Timeout.Connect > 0 {
		result.Timeout.Connect = other.Timeout.Connect
	}
	if other.Timeout.Read > 0 {
		result.Timeout.Read = other.Timeout.Read
	}
	if other.Timeout.Write > 0 {
		result.Timeout.Write = other.Timeout.Write
	}
	if other.Timeout.Request > 0 {
		result.Timeout.Request = other.Timeout.Request
	}

	return result
}

// GetVerify 获取是否验证证书
func (c *SSLConfig) GetVerify() bool {
	if c.Verify == nil {
		return true
	}
	return *c.Verify
}

// GetFollow 获取是否跟随重定向
func (c *RedirectConfig) GetFollow() bool {
	if c.Follow == nil {
		return true
	}
	return *c.Follow
}

// GetMaxRedirects 获取最大重定向次数
func (c *RedirectConfig) GetMaxRedirects() int {
	if c.MaxRedirects == nil {
		return 10
	}
	return *c.MaxRedirects
}

// BuildTLSConfig 构建 TLS 配置
func (c *SSLConfig) BuildTLSConfig() (*tls.Config, error) {
	tlsConfig := &tls.Config{
		InsecureSkipVerify: !c.GetVerify(),
	}

	// 加载客户端证书
	if c.CertPath != "" && c.KeyPath != "" {
		cert, err := tls.LoadX509KeyPair(c.CertPath, c.KeyPath)
		if err != nil {
			return nil, fmt.Errorf("failed to load client certificate: %w", err)
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}

	// 加载 CA 证书
	if c.CAPath != "" {
		caCert, err := os.ReadFile(c.CAPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read CA certificate: %w", err)
		}
		caCertPool := x509.NewCertPool()
		if !caCertPool.AppendCertsFromPEM(caCert) {
			return nil, fmt.Errorf("failed to parse CA certificate")
		}
		tlsConfig.RootCAs = caCertPool
	}

	return tlsConfig, nil
}

// BuildTransport 构建 HTTP Transport
func (c *HTTPGlobalConfig) BuildTransport() (*http.Transport, error) {
	transport := &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
	}

	// 设置连接超时
	if c.Timeout.Connect > 0 {
		transport.ResponseHeaderTimeout = c.Timeout.Connect
	}

	// 构建 TLS 配置
	tlsConfig, err := c.SSL.BuildTLSConfig()
	if err != nil {
		return nil, err
	}
	transport.TLSClientConfig = tlsConfig

	return transport, nil
}

// BuildCheckRedirect 构建重定向检查函数
func (c *HTTPGlobalConfig) BuildCheckRedirect() func(*http.Request, []*http.Request) error {
	return func(req *http.Request, via []*http.Request) error {
		if !c.Redirect.GetFollow() {
			return http.ErrUseLastResponse
		}
		if len(via) >= c.Redirect.GetMaxRedirects() {
			return fmt.Errorf("stopped after %d redirects", c.Redirect.GetMaxRedirects())
		}
		return nil
	}
}

// ResolveURL 解析 URL，支持多域名
func (c *HTTPGlobalConfig) ResolveURL(url string, domain string) string {
	// 如果 URL 已经是完整的，直接返回
	if len(url) > 0 && (url[0] == 'h' || url[0] == 'H') {
		if len(url) > 7 && (url[:7] == "http://" || url[:8] == "https://") {
			return url
		}
	}

	// 获取基础 URL
	baseURL := c.BaseURL

	// 如果指定了域名，使用域名配置
	if domain != "" {
		if domainURL, ok := c.Domains[domain]; ok {
			baseURL = domainURL
		}
	}

	// 拼接 URL
	if baseURL == "" {
		return url
	}

	// 确保 baseURL 不以 / 结尾，url 以 / 开头
	baseURL = trimRight(baseURL, "/")
	if len(url) == 0 || url[0] != '/' {
		url = "/" + url
	}

	return baseURL + url
}

// trimRight 移除字符串右侧的指定字符
func trimRight(s string, cutset string) string {
	for len(s) > 0 && contains(cutset, s[len(s)-1]) {
		s = s[:len(s)-1]
	}
	return s
}

// contains 检查字符是否在字符串中
func contains(s string, c byte) bool {
	for i := 0; i < len(s); i++ {
		if s[i] == c {
			return true
		}
	}
	return false
}

// ParseKeyValueConfig 解析 key-value 配置（支持 map 和 array 格式）
// map 格式: {"key": "value"}
// array 格式: [{key: "key", value: "value", enabled: true}]
func ParseKeyValueConfig(raw any) map[string]string {
	result := make(map[string]string)
	switch data := raw.(type) {
	case map[string]any:
		for k, v := range data {
			if s, ok := v.(string); ok {
				result[k] = s
			}
		}
	case []any:
		for _, item := range data {
			if m, ok := item.(map[string]any); ok {
				// 检查是否启用
				if enabled, ok := m["enabled"].(bool); ok && !enabled {
					continue
				}
				key, _ := m["key"].(string)
				value, _ := m["value"].(string)
				if key != "" {
					result[key] = value
				}
			}
		}
	}
	return result
}

// BodyConfig 请求体配置结构
type BodyConfig struct {
	Type       string            // none, form-data, x-www-form-urlencoded, json, xml, text, binary, graphql
	Raw        string            // 原始内容（json, xml, text, graphql）
	FormData   map[string]string // form-data 数据
	URLEncoded map[string]string // x-www-form-urlencoded 数据
}

// ParseBodyConfig 解析请求体配置
// 支持两种格式：
// 1. 字符串格式（直接作为 raw body）
// 2. 对象格式：{type: "json", raw: "...", formData: [...], urlencoded: [...]}
func ParseBodyConfig(raw any) *BodyConfig {
	if raw == nil {
		return nil
	}

	// 字符串格式
	if s, ok := raw.(string); ok {
		if s == "" {
			return nil
		}
		return &BodyConfig{
			Type: "raw",
			Raw:  s,
		}
	}

	// 对象格式
	bodyMap, ok := raw.(map[string]any)
	if !ok {
		return nil
	}

	config := &BodyConfig{
		FormData:   make(map[string]string),
		URLEncoded: make(map[string]string),
	}

	// 获取类型
	if t, ok := bodyMap["type"].(string); ok {
		config.Type = t
	}

	// 如果是 none 类型，返回 nil
	if config.Type == "none" || config.Type == "" {
		return nil
	}

	// 解析 raw 内容
	if rawContent, ok := bodyMap["raw"].(string); ok {
		config.Raw = rawContent
	}

	// 解析 formData
	if formData, exists := bodyMap["formData"]; exists {
		config.FormData = ParseKeyValueConfig(formData)
	}

	// 解析 urlencoded
	if urlencoded, exists := bodyMap["urlencoded"]; exists {
		config.URLEncoded = ParseKeyValueConfig(urlencoded)
	}

	return config
}
