package config

import (
	"os"
	"sync"

	"gopkg.in/yaml.v3"
)

// Config 全局配置结构
type Config struct {
	App      AppConfig      `yaml:"app"`
	Server   ServerConfig   `yaml:"server"`
	Database DatabaseConfig `yaml:"database"`
	Redis    RedisConfig    `yaml:"redis"`
	Log      LogConfig      `yaml:"log"`
	SaToken  SaTokenConfig  `yaml:"sa_token"`
}

// AppConfig 应用配置
type AppConfig struct {
	Name    string `yaml:"name"`
	Version string `yaml:"version"`
	Env     string `yaml:"env"` // dev, test, prod
}

// ServerConfig 服务器配置
type ServerConfig struct {
	Port         int    `yaml:"port"`
	Host         string `yaml:"host"`
	ReadTimeout  int    `yaml:"read_timeout"`
	WriteTimeout int    `yaml:"write_timeout"`
}

// DatabaseConfig 数据库配置
type DatabaseConfig struct {
	Driver          string `yaml:"driver"` // mysql, postgres
	Host            string `yaml:"host"`
	Port            int    `yaml:"port"`
	Username        string `yaml:"username"`
	Password        string `yaml:"password"`
	Database        string `yaml:"database"`
	Charset         string `yaml:"charset"`
	MaxIdleConns    int    `yaml:"max_idle_conns"`
	MaxOpenConns    int    `yaml:"max_open_conns"`
	ConnMaxLifetime int    `yaml:"conn_max_lifetime"`
}

// RedisConfig Redis配置
type RedisConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Password string `yaml:"password"`
	DB       int    `yaml:"db"`
}

// LogConfig 日志配置
type LogConfig struct {
	Level      string `yaml:"level"`  // debug, info, warn, error
	Format     string `yaml:"format"` // json, text
	Output     string `yaml:"output"` // stdout, file
	FilePath   string `yaml:"file_path"`
	MaxSize    int    `yaml:"max_size"` // MB
	MaxBackups int    `yaml:"max_backups"`
	MaxAge     int    `yaml:"max_age"` // days
}

// SaTokenConfig SaToken配置
type SaTokenConfig struct {
	TokenName     string `yaml:"token_name"`      // token名称
	TokenStyle    string `yaml:"token_style"`     // token风格: uuid, simple-uuid, random-32, random-64, random-128
	Timeout       int64  `yaml:"timeout"`         // token有效期(秒)
	ActiveTimeout int64  `yaml:"active_timeout"`  // token活跃检测超时时间(秒)
	IsConcurrent  bool   `yaml:"is_concurrent"`   // 是否允许同一账号并发登录
	IsShare       bool   `yaml:"is_share"`        // 是否共用token
	MaxLoginCount int    `yaml:"max_login_count"` // 同一账号最大登录数量
	IsLog         bool   `yaml:"is_log"`          // 是否输出日志
	JwtSecretKey  string `yaml:"jwt_secret_key"`  // JWT密钥
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
	})

	return &cfg, nil
}

// GetConfig 获取全局配置
func GetConfig() *Config {
	return globalConfig
}

// SetConfig 设置全局配置
func SetConfig(cfg *Config) {
	globalConfig = cfg
}
