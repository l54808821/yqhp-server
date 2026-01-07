package config

import (
	"os"
	"sync"

	commonConfig "yqhp/common/config"

	"gopkg.in/yaml.v3"
)

// GuluConfig Gulu 应用特有配置
type GuluConfig struct {
	AppCode  string `yaml:"app_code"`  // 应用编码，用于权限过滤
	AdminURL string `yaml:"admin_url"` // Admin 服务地址
}

// Config 应用配置
type Config struct {
	commonConfig.Config `yaml:",inline"`
	Gulu                GuluConfig `yaml:"gulu"`
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
