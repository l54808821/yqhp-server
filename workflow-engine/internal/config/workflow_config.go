// Package config provides workflow global configuration management for workflow engine v2.
package config

import (
	"time"
)

// WorkflowGlobalConfig 工作流全局配置
type WorkflowGlobalConfig struct {
	// HTTP 配置
	HTTP *HTTPGlobalConfig `yaml:"http,omitempty" json:"http,omitempty"`

	// Socket 配置
	Socket *SocketGlobalConfig `yaml:"socket,omitempty" json:"socket,omitempty"`

	// MQ 配置
	MQ *MQGlobalConfig `yaml:"mq,omitempty" json:"mq,omitempty"`

	// DB 配置
	DB *DBGlobalConfig `yaml:"db,omitempty" json:"db,omitempty"`

	// 变量配置
	Variables map[string]any `yaml:"variables,omitempty" json:"variables,omitempty"`

	// 环境配置
	Environment string `yaml:"environment,omitempty" json:"environment,omitempty"`
}

// HTTPGlobalConfig HTTP 全局配置
type HTTPGlobalConfig struct {
	// 多域名配置
	Domains map[string]*DomainConfig `yaml:"domains,omitempty" json:"domains,omitempty"`

	// 默认域名
	DefaultDomain string `yaml:"default_domain,omitempty" json:"default_domain,omitempty"`

	// 全局超时配置
	Timeout *TimeoutConfig `yaml:"timeout,omitempty" json:"timeout,omitempty"`

	// 全局重定向配置
	Redirect *RedirectConfig `yaml:"redirect,omitempty" json:"redirect,omitempty"`

	// 全局 TLS 配置
	TLS *TLSGlobalConfig `yaml:"tls,omitempty" json:"tls,omitempty"`

	// 全局请求头
	Headers map[string]string `yaml:"headers,omitempty" json:"headers,omitempty"`
}

// DomainConfig 域名配置
type DomainConfig struct {
	BaseURL  string            `yaml:"base_url" json:"base_url"`
	Timeout  *TimeoutConfig    `yaml:"timeout,omitempty" json:"timeout,omitempty"`
	Redirect *RedirectConfig   `yaml:"redirect,omitempty" json:"redirect,omitempty"`
	TLS      *TLSGlobalConfig  `yaml:"tls,omitempty" json:"tls,omitempty"`
	Headers  map[string]string `yaml:"headers,omitempty" json:"headers,omitempty"`
}

// TimeoutConfig 超时配置
type TimeoutConfig struct {
	Connect time.Duration `yaml:"connect,omitempty" json:"connect,omitempty"`
	Read    time.Duration `yaml:"read,omitempty" json:"read,omitempty"`
	Write   time.Duration `yaml:"write,omitempty" json:"write,omitempty"`
	Total   time.Duration `yaml:"total,omitempty" json:"total,omitempty"`
}

// RedirectConfig 重定向配置
type RedirectConfig struct {
	Follow       bool `yaml:"follow" json:"follow"`
	MaxRedirects int  `yaml:"max_redirects,omitempty" json:"max_redirects,omitempty"`
}

// TLSGlobalConfig TLS 配置
type TLSGlobalConfig struct {
	InsecureSkipVerify bool   `yaml:"insecure_skip_verify,omitempty" json:"insecure_skip_verify,omitempty"`
	CertFile           string `yaml:"cert_file,omitempty" json:"cert_file,omitempty"`
	KeyFile            string `yaml:"key_file,omitempty" json:"key_file,omitempty"`
	CAFile             string `yaml:"ca_file,omitempty" json:"ca_file,omitempty"`
	MinVersion         string `yaml:"min_version,omitempty" json:"min_version,omitempty"`
}

// SocketGlobalConfig Socket 全局配置
type SocketGlobalConfig struct {
	// 默认协议
	DefaultProtocol string `yaml:"default_protocol,omitempty" json:"default_protocol,omitempty"`

	// 默认超时
	Timeout *TimeoutConfig `yaml:"timeout,omitempty" json:"timeout,omitempty"`

	// TLS 配置
	TLS *TLSGlobalConfig `yaml:"tls,omitempty" json:"tls,omitempty"`

	// 默认缓冲区大小
	BufferSize int `yaml:"buffer_size,omitempty" json:"buffer_size,omitempty"`
}

// MQGlobalConfig MQ 全局配置
type MQGlobalConfig struct {
	// 连接配置
	Connections map[string]*MQConnectionConfig `yaml:"connections,omitempty" json:"connections,omitempty"`

	// 默认连接
	DefaultConnection string `yaml:"default_connection,omitempty" json:"default_connection,omitempty"`

	// 默认超时
	Timeout time.Duration `yaml:"timeout,omitempty" json:"timeout,omitempty"`
}

// MQConnectionConfig MQ 连接配置
type MQConnectionConfig struct {
	Type     string `yaml:"type" json:"type"` // kafka, rabbitmq, redis, etc.
	Brokers  string `yaml:"brokers,omitempty" json:"brokers,omitempty"`
	Username string `yaml:"username,omitempty" json:"username,omitempty"`
	Password string `yaml:"password,omitempty" json:"password,omitempty"`
}

// DBGlobalConfig DB 全局配置
type DBGlobalConfig struct {
	// 连接配置
	Connections map[string]*DBConnectionConfig `yaml:"connections,omitempty" json:"connections,omitempty"`

	// 默认连接
	DefaultConnection string `yaml:"default_connection,omitempty" json:"default_connection,omitempty"`

	// 默认超时
	Timeout time.Duration `yaml:"timeout,omitempty" json:"timeout,omitempty"`
}

// DBConnectionConfig DB 连接配置
type DBConnectionConfig struct {
	Driver   string `yaml:"driver" json:"driver"` // mysql, postgres, sqlite, etc.
	DSN      string `yaml:"dsn" json:"dsn"`
	MaxConns int    `yaml:"max_conns,omitempty" json:"max_conns,omitempty"`
	MaxIdle  int    `yaml:"max_idle,omitempty" json:"max_idle,omitempty"`
}
