package config

import (
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Config represents the complete configuration for the workflow engine.
type Config struct {
	Server  ServerConfig  `yaml:"server"`
	GRPC    GRPCConfig    `yaml:"grpc"`
	Master  MasterConfig  `yaml:"master"`
	Slave   SlaveConfig   `yaml:"slave"`
	Logging LoggingConfig `yaml:"logging"`
}

// ServerConfig holds HTTP server configuration.
type ServerConfig struct {
	Address       string        `yaml:"address" env:"WE_SERVER_ADDRESS"`
	ReadTimeout   time.Duration `yaml:"read_timeout" env:"WE_SERVER_READ_TIMEOUT"`
	WriteTimeout  time.Duration `yaml:"write_timeout" env:"WE_SERVER_WRITE_TIMEOUT"`
	EnableCORS    bool          `yaml:"enable_cors" env:"WE_SERVER_ENABLE_CORS"`
	EnableSwagger bool          `yaml:"enable_swagger" env:"WE_SERVER_ENABLE_SWAGGER"`
}

// GRPCConfig holds gRPC server configuration.
type GRPCConfig struct {
	Address           string        `yaml:"address" env:"WE_GRPC_ADDRESS"`
	MaxRecvMsgSize    int           `yaml:"max_recv_msg_size" env:"WE_GRPC_MAX_RECV_MSG_SIZE"`
	MaxSendMsgSize    int           `yaml:"max_send_msg_size" env:"WE_GRPC_MAX_SEND_MSG_SIZE"`
	ConnectionTimeout time.Duration `yaml:"connection_timeout" env:"WE_GRPC_CONNECTION_TIMEOUT"`
}

// MasterConfig holds master node configuration.
type MasterConfig struct {
	HeartbeatInterval time.Duration `yaml:"heartbeat_interval" env:"WE_MASTER_HEARTBEAT_INTERVAL"`
	HeartbeatTimeout  time.Duration `yaml:"heartbeat_timeout" env:"WE_MASTER_HEARTBEAT_TIMEOUT"`
	TaskQueueSize     int           `yaml:"task_queue_size" env:"WE_MASTER_TASK_QUEUE_SIZE"`
	MaxSlaves         int           `yaml:"max_slaves" env:"WE_MASTER_MAX_SLAVES"`
}

// SlaveConfig holds slave node configuration.
type SlaveConfig struct {
	Type         string            `yaml:"type" env:"WE_SLAVE_TYPE"`
	Capabilities []string          `yaml:"capabilities" env:"WE_SLAVE_CAPABILITIES"`
	Labels       map[string]string `yaml:"labels"`
	MaxVUs       int               `yaml:"max_vus" env:"WE_SLAVE_MAX_VUS"`
	MasterAddr   string            `yaml:"master_addr" env:"WE_SLAVE_MASTER_ADDR"`
}

// LoggingConfig holds logging configuration.
type LoggingConfig struct {
	Level  string `yaml:"level" env:"WE_LOG_LEVEL"`
	Format string `yaml:"format" env:"WE_LOG_FORMAT"`
	Output string `yaml:"output" env:"WE_LOG_OUTPUT"`
}

// DefaultConfig returns a Config with default values.
func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Address:       ":8080",
			ReadTimeout:   30 * time.Second,
			WriteTimeout:  30 * time.Second,
			EnableCORS:    false,
			EnableSwagger: false,
		},
		GRPC: GRPCConfig{
			Address:           ":9090",
			MaxRecvMsgSize:    4 * 1024 * 1024, // 4MB
			MaxSendMsgSize:    4 * 1024 * 1024, // 4MB
			ConnectionTimeout: 10 * time.Second,
		},
		Master: MasterConfig{
			HeartbeatInterval: 5 * time.Second,
			HeartbeatTimeout:  15 * time.Second,
			TaskQueueSize:     1000,
			MaxSlaves:         100,
		},
		Slave: SlaveConfig{
			Type:         "worker",
			Capabilities: []string{"http_executor", "script_executor"},
			Labels:       make(map[string]string),
			MaxVUs:       100,
			MasterAddr:   "localhost:9090",
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "json",
			Output: "stdout",
		},
	}
}

// Loader handles configuration loading from multiple sources.
type Loader struct {
	configPath string
	envPrefix  string
	cmdArgs    map[string]string
}

// NewLoader creates a new configuration loader.
func NewLoader() *Loader {
	return &Loader{
		envPrefix: "WE_",
		cmdArgs:   make(map[string]string),
	}
}

// WithConfigPath sets the path to the YAML configuration file.
func (l *Loader) WithConfigPath(path string) *Loader {
	l.configPath = path
	return l
}

// WithEnvPrefix sets the prefix for environment variables.
func (l *Loader) WithEnvPrefix(prefix string) *Loader {
	l.envPrefix = prefix
	return l
}

// WithCmdArgs sets command-line arguments for configuration override.
func (l *Loader) WithCmdArgs(args map[string]string) *Loader {
	l.cmdArgs = args
	return l
}

// Load loads configuration from all sources with proper precedence:
// defaults < YAML file < environment variables < command-line flags
func (l *Loader) Load() (*Config, error) {
	// Start with defaults
	cfg := DefaultConfig()

	// Load from YAML file if specified
	if l.configPath != "" {
		if err := l.loadFromFile(cfg); err != nil {
			return nil, fmt.Errorf("从文件加载配置失败: %w", err)
		}
	}

	// Apply environment variable overrides
	if err := l.applyEnvOverrides(cfg); err != nil {
		return nil, fmt.Errorf("应用环境变量覆盖失败: %w", err)
	}

	// Apply command-line argument overrides
	if err := l.applyCmdOverrides(cfg); err != nil {
		return nil, fmt.Errorf("应用命令行参数覆盖失败: %w", err)
	}

	return cfg, nil
}

// loadFromFile loads configuration from a YAML file.
func (l *Loader) loadFromFile(cfg *Config) error {
	data, err := os.ReadFile(l.configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // File doesn't exist, use defaults
		}
		return fmt.Errorf("读取配置文件失败: %w", err)
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return fmt.Errorf("解析配置文件失败: %w", err)
	}

	return nil
}

// applyEnvOverrides applies environment variable overrides to the configuration.
func (l *Loader) applyEnvOverrides(cfg *Config) error {
	return l.applyEnvToStruct(reflect.ValueOf(cfg).Elem())
}

// applyEnvToStruct recursively applies environment variables to struct fields.
func (l *Loader) applyEnvToStruct(v reflect.Value) error {
	t := v.Type()

	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		fieldType := t.Field(i)

		// Handle nested structs
		if field.Kind() == reflect.Struct {
			if err := l.applyEnvToStruct(field); err != nil {
				return err
			}
			continue
		}

		// Get env tag
		envTag := fieldType.Tag.Get("env")
		if envTag == "" {
			continue
		}

		// Get environment variable value
		envValue := os.Getenv(envTag)
		if envValue == "" {
			continue
		}

		// Set the field value
		if err := setFieldValue(field, envValue); err != nil {
			return fmt.Errorf("从环境变量 %s 设置字段 %s 失败: %w", envTag, fieldType.Name, err)
		}
	}

	return nil
}

// applyCmdOverrides applies command-line argument overrides to the configuration.
func (l *Loader) applyCmdOverrides(cfg *Config) error {
	for key, value := range l.cmdArgs {
		if err := l.setConfigValue(cfg, key, value); err != nil {
			return fmt.Errorf("设置配置值 %s 失败: %w", key, err)
		}
	}
	return nil
}

// setConfigValue sets a configuration value by dot-notation path.
func (l *Loader) setConfigValue(cfg *Config, path, value string) error {
	parts := strings.Split(path, ".")
	v := reflect.ValueOf(cfg).Elem()

	for i, part := range parts {
		// Convert to title case for struct field lookup
		fieldName := strings.Title(strings.ReplaceAll(part, "_", ""))

		field := v.FieldByNameFunc(func(name string) bool {
			return strings.EqualFold(name, fieldName) || strings.EqualFold(name, part)
		})

		if !field.IsValid() {
			return fmt.Errorf("未知的配置路径: %s", path)
		}

		if i == len(parts)-1 {
			// Last part, set the value
			return setFieldValue(field, value)
		}

		// Navigate to nested struct
		if field.Kind() != reflect.Struct {
			return fmt.Errorf("期望 %s 是结构体，实际是 %s", part, field.Kind())
		}
		v = field
	}

	return nil
}

// setFieldValue sets a reflect.Value from a string value.
func setFieldValue(field reflect.Value, value string) error {
	if !field.CanSet() {
		return fmt.Errorf("无法设置字段")
	}

	switch field.Kind() {
	case reflect.String:
		field.SetString(value)

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		// Handle time.Duration specially
		if field.Type() == reflect.TypeOf(time.Duration(0)) {
			d, err := time.ParseDuration(value)
			if err != nil {
				return fmt.Errorf("无效的时间格式: %w", err)
			}
			field.SetInt(int64(d))
		} else {
			i, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				return fmt.Errorf("无效的整数: %w", err)
			}
			field.SetInt(i)
		}

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		u, err := strconv.ParseUint(value, 10, 64)
		if err != nil {
			return fmt.Errorf("无效的无符号整数: %w", err)
		}
		field.SetUint(u)

	case reflect.Float32, reflect.Float64:
		f, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return fmt.Errorf("无效的浮点数: %w", err)
		}
		field.SetFloat(f)

	case reflect.Bool:
		b, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("无效的布尔值: %w", err)
		}
		field.SetBool(b)

	case reflect.Slice:
		// Handle string slices (comma-separated)
		if field.Type().Elem().Kind() == reflect.String {
			parts := strings.Split(value, ",")
			for i := range parts {
				parts[i] = strings.TrimSpace(parts[i])
			}
			field.Set(reflect.ValueOf(parts))
		} else {
			return fmt.Errorf("不支持的切片类型: %s", field.Type().Elem().Kind())
		}

	case reflect.Map:
		// Handle string->string maps (key=value,key=value format)
		if field.Type().Key().Kind() == reflect.String && field.Type().Elem().Kind() == reflect.String {
			m := make(map[string]string)
			pairs := strings.Split(value, ",")
			for _, pair := range pairs {
				kv := strings.SplitN(strings.TrimSpace(pair), "=", 2)
				if len(kv) == 2 {
					m[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
				}
			}
			field.Set(reflect.ValueOf(m))
		} else {
			return fmt.Errorf("不支持的 map 类型")
		}

	default:
		return fmt.Errorf("不支持的字段类型: %s", field.Kind())
	}

	return nil
}

// Serialize serializes the configuration to YAML bytes.
func (c *Config) Serialize() ([]byte, error) {
	return yaml.Marshal(c)
}

// ParseConfig parses a YAML configuration from bytes.
func ParseConfig(data []byte) (*Config, error) {
	cfg := DefaultConfig()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("解析配置失败: %w", err)
	}
	return cfg, nil
}

// LoadFromFile loads configuration from a YAML file path.
func LoadFromFile(path string) (*Config, error) {
	return NewLoader().WithConfigPath(path).Load()
}

// Clone creates a deep copy of the configuration.
func (c *Config) Clone() *Config {
	data, _ := c.Serialize()
	clone, _ := ParseConfig(data)
	return clone
}
