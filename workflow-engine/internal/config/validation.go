package config

import (
	"fmt"
	"net"
	"strings"
	"time"
)

// ValidationError represents a configuration validation error.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// ValidationErrors is a collection of validation errors.
type ValidationErrors []ValidationError

func (e ValidationErrors) Error() string {
	if len(e) == 0 {
		return ""
	}
	var msgs []string
	for _, err := range e {
		msgs = append(msgs, err.Error())
	}
	return fmt.Sprintf("configuration validation failed:\n  - %s", strings.Join(msgs, "\n  - "))
}

// HasErrors returns true if there are any validation errors.
func (e ValidationErrors) HasErrors() bool {
	return len(e) > 0
}

// Validator validates configuration values.
type Validator struct {
	errors ValidationErrors
}

// NewValidator creates a new configuration validator.
func NewValidator() *Validator {
	return &Validator{
		errors: make(ValidationErrors, 0),
	}
}

// addError adds a validation error.
func (v *Validator) addError(field, message string) {
	v.errors = append(v.errors, ValidationError{Field: field, Message: message})
}

// Validate validates the entire configuration and returns any errors.
func (v *Validator) Validate(cfg *Config) error {
	v.errors = make(ValidationErrors, 0)

	v.validateServerConfig(&cfg.Server)
	v.validateGRPCConfig(&cfg.GRPC)
	v.validateMasterConfig(&cfg.Master)
	v.validateSlaveConfig(&cfg.Slave)
	v.validateLoggingConfig(&cfg.Logging)

	if v.errors.HasErrors() {
		return v.errors
	}
	return nil
}

// validateServerConfig validates the server configuration.
func (v *Validator) validateServerConfig(cfg *ServerConfig) {
	// Validate address
	if cfg.Address == "" {
		v.addError("server.address", "address is required")
	} else if !isValidAddress(cfg.Address) {
		v.addError("server.address", "invalid address format, expected host:port or :port")
	}

	// Validate timeouts
	if cfg.ReadTimeout < 0 {
		v.addError("server.read_timeout", "read timeout must be non-negative")
	}
	if cfg.WriteTimeout < 0 {
		v.addError("server.write_timeout", "write timeout must be non-negative")
	}
	if cfg.ReadTimeout > 0 && cfg.ReadTimeout < time.Second {
		v.addError("server.read_timeout", "read timeout should be at least 1 second")
	}
	if cfg.WriteTimeout > 0 && cfg.WriteTimeout < time.Second {
		v.addError("server.write_timeout", "write timeout should be at least 1 second")
	}
}

// validateGRPCConfig validates the gRPC configuration.
func (v *Validator) validateGRPCConfig(cfg *GRPCConfig) {
	// Validate address
	if cfg.Address == "" {
		v.addError("grpc.address", "address is required")
	} else if !isValidAddress(cfg.Address) {
		v.addError("grpc.address", "invalid address format, expected host:port or :port")
	}

	// Validate message sizes
	if cfg.MaxRecvMsgSize < 0 {
		v.addError("grpc.max_recv_msg_size", "max receive message size must be non-negative")
	}
	if cfg.MaxSendMsgSize < 0 {
		v.addError("grpc.max_send_msg_size", "max send message size must be non-negative")
	}

	// Validate connection timeout
	if cfg.ConnectionTimeout < 0 {
		v.addError("grpc.connection_timeout", "connection timeout must be non-negative")
	}
}

// validateMasterConfig validates the master configuration.
func (v *Validator) validateMasterConfig(cfg *MasterConfig) {
	// Validate heartbeat interval
	if cfg.HeartbeatInterval <= 0 {
		v.addError("master.heartbeat_interval", "heartbeat interval must be positive")
	}

	// Validate heartbeat timeout
	if cfg.HeartbeatTimeout <= 0 {
		v.addError("master.heartbeat_timeout", "heartbeat timeout must be positive")
	}

	// Heartbeat timeout should be greater than interval
	if cfg.HeartbeatTimeout > 0 && cfg.HeartbeatInterval > 0 &&
		cfg.HeartbeatTimeout <= cfg.HeartbeatInterval {
		v.addError("master.heartbeat_timeout", "heartbeat timeout should be greater than heartbeat interval")
	}

	// Validate task queue size
	if cfg.TaskQueueSize < 0 {
		v.addError("master.task_queue_size", "task queue size must be non-negative")
	}

	// Validate max slaves
	if cfg.MaxSlaves < 0 {
		v.addError("master.max_slaves", "max slaves must be non-negative")
	}
}

// validateSlaveConfig validates the slave configuration.
func (v *Validator) validateSlaveConfig(cfg *SlaveConfig) {
	// Validate type
	validTypes := map[string]bool{
		"worker":     true,
		"gateway":    true,
		"aggregator": true,
	}
	if cfg.Type == "" {
		v.addError("slave.type", "slave type is required")
	} else if !validTypes[cfg.Type] {
		v.addError("slave.type", fmt.Sprintf("invalid slave type '%s', must be one of: worker, gateway, aggregator", cfg.Type))
	}

	// Validate max VUs
	if cfg.MaxVUs < 0 {
		v.addError("slave.max_vus", "max VUs must be non-negative")
	}

	// Validate master address if specified
	if cfg.MasterAddr != "" && !isValidAddress(cfg.MasterAddr) {
		v.addError("slave.master_addr", "invalid master address format, expected host:port")
	}

	// Validate capabilities (should not be empty for workers)
	if cfg.Type == "worker" && len(cfg.Capabilities) == 0 {
		v.addError("slave.capabilities", "worker slaves must have at least one capability")
	}
}

// validateLoggingConfig validates the logging configuration.
func (v *Validator) validateLoggingConfig(cfg *LoggingConfig) {
	// Validate log level
	validLevels := map[string]bool{
		"debug": true,
		"info":  true,
		"warn":  true,
		"error": true,
		"fatal": true,
	}
	if cfg.Level == "" {
		v.addError("logging.level", "log level is required")
	} else if !validLevels[strings.ToLower(cfg.Level)] {
		v.addError("logging.level", fmt.Sprintf("invalid log level '%s', must be one of: debug, info, warn, error, fatal", cfg.Level))
	}

	// Validate log format
	validFormats := map[string]bool{
		"json": true,
		"text": true,
	}
	if cfg.Format == "" {
		v.addError("logging.format", "log format is required")
	} else if !validFormats[strings.ToLower(cfg.Format)] {
		v.addError("logging.format", fmt.Sprintf("invalid log format '%s', must be one of: json, text", cfg.Format))
	}

	// Validate output
	validOutputs := map[string]bool{
		"stdout": true,
		"stderr": true,
	}
	// Output can be a file path or stdout/stderr
	if cfg.Output != "" && !validOutputs[strings.ToLower(cfg.Output)] {
		// If not stdout/stderr, it should be a valid file path (we just check it's not empty)
		// Actual file validation would happen at runtime
	}
}

// isValidAddress checks if the address is a valid host:port format.
func isValidAddress(addr string) bool {
	if addr == "" {
		return false
	}

	// Handle :port format
	if strings.HasPrefix(addr, ":") {
		port := strings.TrimPrefix(addr, ":")
		if port == "" {
			return false
		}
		_, err := net.LookupPort("tcp", port)
		return err == nil
	}

	// Handle host:port format
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return false
	}

	// Port must be non-empty and valid
	if port == "" {
		return false
	}
	if _, err := net.LookupPort("tcp", port); err != nil {
		return false
	}

	// Host can be empty (meaning all interfaces), an IP, or a hostname
	if host != "" {
		// Try to parse as IP
		if ip := net.ParseIP(host); ip == nil {
			// Not an IP, check if it's a valid hostname (basic check)
			if !isValidHostname(host) {
				return false
			}
		}
	}

	return true
}

// isValidHostname performs basic hostname validation.
func isValidHostname(hostname string) bool {
	if len(hostname) == 0 || len(hostname) > 253 {
		return false
	}

	// Check each label
	labels := strings.Split(hostname, ".")
	for _, label := range labels {
		if len(label) == 0 || len(label) > 63 {
			return false
		}
		// Labels must start and end with alphanumeric
		if !isAlphanumeric(label[0]) || !isAlphanumeric(label[len(label)-1]) {
			return false
		}
		// Labels can contain alphanumeric and hyphens
		for _, c := range label {
			if !isAlphanumeric(byte(c)) && c != '-' {
				return false
			}
		}
	}

	return true
}

// isAlphanumeric checks if a byte is alphanumeric.
func isAlphanumeric(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
}

// Validate validates the configuration and returns any errors.
// This is a convenience method on Config.
func (c *Config) Validate() error {
	return NewValidator().Validate(c)
}

// MustValidate validates the configuration and panics if validation fails.
// This is useful for startup validation.
func (c *Config) MustValidate() {
	if err := c.Validate(); err != nil {
		panic(fmt.Sprintf("configuration validation failed: %v", err))
	}
}

// LoadAndValidate loads configuration from a file and validates it.
func LoadAndValidate(path string) (*Config, error) {
	cfg, err := LoadFromFile(path)
	if err != nil {
		return nil, err
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// Schema represents a configuration schema for documentation and validation.
type Schema struct {
	Fields []FieldSchema
}

// FieldSchema describes a configuration field.
type FieldSchema struct {
	Path        string
	Type        string
	Required    bool
	Default     string
	Description string
	EnvVar      string
	Constraints []string
}

// GetSchema returns the configuration schema.
func GetSchema() *Schema {
	return &Schema{
		Fields: []FieldSchema{
			{Path: "server.address", Type: "string", Required: true, Default: ":8080", Description: "HTTP server listen address", EnvVar: "WE_SERVER_ADDRESS", Constraints: []string{"valid host:port format"}},
			{Path: "server.read_timeout", Type: "duration", Required: false, Default: "30s", Description: "HTTP read timeout", EnvVar: "WE_SERVER_READ_TIMEOUT", Constraints: []string{"non-negative", "at least 1s if set"}},
			{Path: "server.write_timeout", Type: "duration", Required: false, Default: "30s", Description: "HTTP write timeout", EnvVar: "WE_SERVER_WRITE_TIMEOUT", Constraints: []string{"non-negative", "at least 1s if set"}},
			{Path: "server.enable_cors", Type: "bool", Required: false, Default: "false", Description: "Enable CORS", EnvVar: "WE_SERVER_ENABLE_CORS"},
			{Path: "server.enable_swagger", Type: "bool", Required: false, Default: "false", Description: "Enable Swagger documentation", EnvVar: "WE_SERVER_ENABLE_SWAGGER"},
			{Path: "grpc.address", Type: "string", Required: true, Default: ":9090", Description: "gRPC server listen address", EnvVar: "WE_GRPC_ADDRESS", Constraints: []string{"valid host:port format"}},
			{Path: "grpc.max_recv_msg_size", Type: "int", Required: false, Default: "4194304", Description: "Max receive message size in bytes", EnvVar: "WE_GRPC_MAX_RECV_MSG_SIZE", Constraints: []string{"non-negative"}},
			{Path: "grpc.max_send_msg_size", Type: "int", Required: false, Default: "4194304", Description: "Max send message size in bytes", EnvVar: "WE_GRPC_MAX_SEND_MSG_SIZE", Constraints: []string{"non-negative"}},
			{Path: "grpc.connection_timeout", Type: "duration", Required: false, Default: "10s", Description: "Connection timeout", EnvVar: "WE_GRPC_CONNECTION_TIMEOUT", Constraints: []string{"non-negative"}},
			{Path: "master.heartbeat_interval", Type: "duration", Required: true, Default: "5s", Description: "Heartbeat interval", EnvVar: "WE_MASTER_HEARTBEAT_INTERVAL", Constraints: []string{"positive"}},
			{Path: "master.heartbeat_timeout", Type: "duration", Required: true, Default: "15s", Description: "Heartbeat timeout", EnvVar: "WE_MASTER_HEARTBEAT_TIMEOUT", Constraints: []string{"positive", "greater than heartbeat_interval"}},
			{Path: "master.task_queue_size", Type: "int", Required: false, Default: "1000", Description: "Task queue size", EnvVar: "WE_MASTER_TASK_QUEUE_SIZE", Constraints: []string{"non-negative"}},
			{Path: "master.max_slaves", Type: "int", Required: false, Default: "100", Description: "Maximum number of slaves", EnvVar: "WE_MASTER_MAX_SLAVES", Constraints: []string{"non-negative"}},
			{Path: "slave.type", Type: "string", Required: true, Default: "worker", Description: "Slave type", EnvVar: "WE_SLAVE_TYPE", Constraints: []string{"one of: worker, gateway, aggregator"}},
			{Path: "slave.capabilities", Type: "[]string", Required: false, Default: "http_executor,script_executor", Description: "Slave capabilities", EnvVar: "WE_SLAVE_CAPABILITIES", Constraints: []string{"required for worker type"}},
			{Path: "slave.max_vus", Type: "int", Required: false, Default: "100", Description: "Maximum VUs", EnvVar: "WE_SLAVE_MAX_VUS", Constraints: []string{"non-negative"}},
			{Path: "slave.master_addr", Type: "string", Required: false, Default: "localhost:9090", Description: "Master address", EnvVar: "WE_SLAVE_MASTER_ADDR", Constraints: []string{"valid host:port format"}},
			{Path: "logging.level", Type: "string", Required: true, Default: "info", Description: "Log level", EnvVar: "WE_LOG_LEVEL", Constraints: []string{"one of: debug, info, warn, error, fatal"}},
			{Path: "logging.format", Type: "string", Required: true, Default: "json", Description: "Log format", EnvVar: "WE_LOG_FORMAT", Constraints: []string{"one of: json, text"}},
			{Path: "logging.output", Type: "string", Required: false, Default: "stdout", Description: "Log output", EnvVar: "WE_LOG_OUTPUT", Constraints: []string{"stdout, stderr, or file path"}},
		},
	}
}
