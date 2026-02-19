package config

import (
	"os"
	"sync"
	"time"

	commonConfig "yqhp/common/config"

	"gopkg.in/yaml.v3"
)

// GuluConfig Gulu 应用特有配置
type GuluConfig struct {
	AppCode  string `yaml:"app_code"`  // 应用编码，用于权限过滤
	AdminURL string `yaml:"admin_url"` // Admin 服务地址
}

// WorkflowEngineConfig Workflow Engine 配置
type WorkflowEngineConfig struct {
	Embedded         bool           `yaml:"embedded"`          // 是否使用内置 Master
	ExternalURL      string         `yaml:"external_url"`      // 外部 Master 地址
	HTTPAddress      string         `yaml:"http_address"`      // HTTP 服务地址
	Standalone       bool           `yaml:"standalone"`        // 独立模式
	MaxExecutions    int            `yaml:"max_executions"`    // 最大并发执行数
	HeartbeatTimeout time.Duration  `yaml:"heartbeat_timeout"` // 心跳超时
	Debug            bool           `yaml:"debug"`             // 是否启用调试日志
	Outputs          []OutputConfig `yaml:"outputs"`           // 默认输出配置
}

// OutputConfig 输出配置
type OutputConfig struct {
	Type    string            `yaml:"type"`    // 输出类型: json, influxdb, kafka, console
	URL     string            `yaml:"url"`     // 输出目标地址
	Options map[string]string `yaml:"options"` // 额外配置选项
}

// QdrantConfig Qdrant 向量数据库配置
type QdrantConfig struct {
	Host             string `yaml:"host"`
	Port             int    `yaml:"port"`
	APIKey           string `yaml:"api_key"`
	UseTLS           bool   `yaml:"use_tls"`
	CollectionPrefix string `yaml:"collection_prefix"`
}

// StorageConfig 文件存储配置
type StorageConfig struct {
	Type  string            `yaml:"type"`  // local 或 s3
	Local LocalStorageConfig `yaml:"local"`
}

// LocalStorageConfig 本地存储配置
type LocalStorageConfig struct {
	BasePath string `yaml:"base_path"`
}

// Config 应用配置
type Config struct {
	commonConfig.Config `yaml:",inline"`
	Gulu                GuluConfig           `yaml:"gulu"`
	WorkflowEngine      WorkflowEngineConfig `yaml:"workflow_engine"`
	Qdrant              QdrantConfig         `yaml:"qdrant"`
	Storage             StorageConfig        `yaml:"storage"`
}

var (
	globalConfig *Config
	once         sync.Once
)

// LoadConfig 加载配置文件
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	once.Do(func() {
		globalConfig = &cfg
		// 同步到公共配置
		commonConfig.SetConfig(&cfg.Config)
	})

	return &cfg, nil
}

// GetConfig 获取全局配置
func GetConfig() *Config {
	return globalConfig
}
