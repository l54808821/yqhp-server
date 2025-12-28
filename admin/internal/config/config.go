package config

import (
	"os"
	"sync"

	commonConfig "yqhp/common/config"

	"gopkg.in/yaml.v3"
)

// Config 应用配置
type Config struct {
	commonConfig.Config `yaml:",inline"`
	OAuth2              OAuth2Config `yaml:"oauth2"`
}

// OAuth2Config OAuth2配置
type OAuth2Config struct {
	Wechat OAuth2Provider `yaml:"wechat"`
	Feishu OAuth2Provider `yaml:"feishu"`
	GitHub OAuth2Provider `yaml:"github"`
}

// OAuth2Provider OAuth2提供商配置
type OAuth2Provider struct {
	Enabled      bool   `yaml:"enabled"`
	AppID        string `yaml:"app_id"`
	ClientID     string `yaml:"client_id"`
	AppSecret    string `yaml:"app_secret"`
	ClientSecret string `yaml:"client_secret"`
	RedirectURI  string `yaml:"redirect_uri"`
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

