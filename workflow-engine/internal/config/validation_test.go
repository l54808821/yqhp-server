package config

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	err := cfg.Validate()
	assert.NoError(t, err)
}

func TestValidateServerConfig(t *testing.T) {
	tests := []struct {
		name        string
		modify      func(*Config)
		expectError bool
		errorField  string
	}{
		{
			name:        "valid config",
			modify:      func(c *Config) {},
			expectError: false,
		},
		{
			name: "empty address",
			modify: func(c *Config) {
				c.Server.Address = ""
			},
			expectError: true,
			errorField:  "server.address",
		},
		{
			name: "invalid address format",
			modify: func(c *Config) {
				c.Server.Address = "invalid"
			},
			expectError: true,
			errorField:  "server.address",
		},
		{
			name: "negative read timeout",
			modify: func(c *Config) {
				c.Server.ReadTimeout = -1 * time.Second
			},
			expectError: true,
			errorField:  "server.read_timeout",
		},
		{
			name: "too small read timeout",
			modify: func(c *Config) {
				c.Server.ReadTimeout = 500 * time.Millisecond
			},
			expectError: true,
			errorField:  "server.read_timeout",
		},
		{
			name: "valid port only address",
			modify: func(c *Config) {
				c.Server.Address = ":9000"
			},
			expectError: false,
		},
		{
			name: "valid host:port address",
			modify: func(c *Config) {
				c.Server.Address = "localhost:9000"
			},
			expectError: false,
		},
		{
			name: "valid IP:port address",
			modify: func(c *Config) {
				c.Server.Address = "127.0.0.1:9000"
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig()
			tt.modify(cfg)
			err := cfg.Validate()

			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorField)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateGRPCConfig(t *testing.T) {
	tests := []struct {
		name        string
		modify      func(*Config)
		expectError bool
		errorField  string
	}{
		{
			name: "empty address",
			modify: func(c *Config) {
				c.GRPC.Address = ""
			},
			expectError: true,
			errorField:  "grpc.address",
		},
		{
			name: "negative max recv msg size",
			modify: func(c *Config) {
				c.GRPC.MaxRecvMsgSize = -1
			},
			expectError: true,
			errorField:  "grpc.max_recv_msg_size",
		},
		{
			name: "negative connection timeout",
			modify: func(c *Config) {
				c.GRPC.ConnectionTimeout = -1 * time.Second
			},
			expectError: true,
			errorField:  "grpc.connection_timeout",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig()
			tt.modify(cfg)
			err := cfg.Validate()

			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorField)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateMasterConfig(t *testing.T) {
	tests := []struct {
		name        string
		modify      func(*Config)
		expectError bool
		errorField  string
	}{
		{
			name: "zero heartbeat interval",
			modify: func(c *Config) {
				c.Master.HeartbeatInterval = 0
			},
			expectError: true,
			errorField:  "master.heartbeat_interval",
		},
		{
			name: "zero heartbeat timeout",
			modify: func(c *Config) {
				c.Master.HeartbeatTimeout = 0
			},
			expectError: true,
			errorField:  "master.heartbeat_timeout",
		},
		{
			name: "heartbeat timeout less than interval",
			modify: func(c *Config) {
				c.Master.HeartbeatInterval = 10 * time.Second
				c.Master.HeartbeatTimeout = 5 * time.Second
			},
			expectError: true,
			errorField:  "master.heartbeat_timeout",
		},
		{
			name: "negative task queue size",
			modify: func(c *Config) {
				c.Master.TaskQueueSize = -1
			},
			expectError: true,
			errorField:  "master.task_queue_size",
		},
		{
			name: "negative max slaves",
			modify: func(c *Config) {
				c.Master.MaxSlaves = -1
			},
			expectError: true,
			errorField:  "master.max_slaves",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig()
			tt.modify(cfg)
			err := cfg.Validate()

			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorField)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateSlaveConfig(t *testing.T) {
	tests := []struct {
		name        string
		modify      func(*Config)
		expectError bool
		errorField  string
	}{
		{
			name: "empty type",
			modify: func(c *Config) {
				c.Slave.Type = ""
			},
			expectError: true,
			errorField:  "slave.type",
		},
		{
			name: "invalid type",
			modify: func(c *Config) {
				c.Slave.Type = "invalid"
			},
			expectError: true,
			errorField:  "slave.type",
		},
		{
			name: "valid worker type",
			modify: func(c *Config) {
				c.Slave.Type = "worker"
			},
			expectError: false,
		},
		{
			name: "valid gateway type",
			modify: func(c *Config) {
				c.Slave.Type = "gateway"
				c.Slave.Capabilities = nil // gateway doesn't need capabilities
			},
			expectError: false,
		},
		{
			name: "valid aggregator type",
			modify: func(c *Config) {
				c.Slave.Type = "aggregator"
				c.Slave.Capabilities = nil // aggregator doesn't need capabilities
			},
			expectError: false,
		},
		{
			name: "worker without capabilities",
			modify: func(c *Config) {
				c.Slave.Type = "worker"
				c.Slave.Capabilities = nil
			},
			expectError: true,
			errorField:  "slave.capabilities",
		},
		{
			name: "negative max VUs",
			modify: func(c *Config) {
				c.Slave.MaxVUs = -1
			},
			expectError: true,
			errorField:  "slave.max_vus",
		},
		{
			name: "invalid master address",
			modify: func(c *Config) {
				c.Slave.MasterAddr = "invalid"
			},
			expectError: true,
			errorField:  "slave.master_addr",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig()
			tt.modify(cfg)
			err := cfg.Validate()

			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorField)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateLoggingConfig(t *testing.T) {
	tests := []struct {
		name        string
		modify      func(*Config)
		expectError bool
		errorField  string
	}{
		{
			name: "empty level",
			modify: func(c *Config) {
				c.Logging.Level = ""
			},
			expectError: true,
			errorField:  "logging.level",
		},
		{
			name: "invalid level",
			modify: func(c *Config) {
				c.Logging.Level = "invalid"
			},
			expectError: true,
			errorField:  "logging.level",
		},
		{
			name: "valid debug level",
			modify: func(c *Config) {
				c.Logging.Level = "debug"
			},
			expectError: false,
		},
		{
			name: "valid warn level",
			modify: func(c *Config) {
				c.Logging.Level = "warn"
			},
			expectError: false,
		},
		{
			name: "empty format",
			modify: func(c *Config) {
				c.Logging.Format = ""
			},
			expectError: true,
			errorField:  "logging.format",
		},
		{
			name: "invalid format",
			modify: func(c *Config) {
				c.Logging.Format = "xml"
			},
			expectError: true,
			errorField:  "logging.format",
		},
		{
			name: "valid text format",
			modify: func(c *Config) {
				c.Logging.Format = "text"
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig()
			tt.modify(cfg)
			err := cfg.Validate()

			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorField)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestMultipleValidationErrors(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Server.Address = ""
	cfg.GRPC.Address = ""
	cfg.Logging.Level = "invalid"

	err := cfg.Validate()
	require.Error(t, err)

	errStr := err.Error()
	assert.Contains(t, errStr, "server.address")
	assert.Contains(t, errStr, "grpc.address")
	assert.Contains(t, errStr, "logging.level")
}

func TestMustValidatePanics(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Server.Address = ""

	assert.Panics(t, func() {
		cfg.MustValidate()
	})
}

func TestMustValidateDoesNotPanic(t *testing.T) {
	cfg := DefaultConfig()

	assert.NotPanics(t, func() {
		cfg.MustValidate()
	})
}

func TestLoadAndValidate(t *testing.T) {
	// Test with default config file path (non-existent)
	cfg, err := LoadAndValidate("/nonexistent/path")
	require.NoError(t, err)
	assert.NotNil(t, cfg)
}

func TestGetSchema(t *testing.T) {
	schema := GetSchema()
	assert.NotNil(t, schema)
	assert.NotEmpty(t, schema.Fields)

	// Check that all expected fields are present
	fieldPaths := make(map[string]bool)
	for _, f := range schema.Fields {
		fieldPaths[f.Path] = true
	}

	expectedPaths := []string{
		"server.address",
		"grpc.address",
		"master.heartbeat_interval",
		"slave.type",
		"logging.level",
	}

	for _, path := range expectedPaths {
		assert.True(t, fieldPaths[path], "expected field %s not found in schema", path)
	}
}

func TestValidationErrorsString(t *testing.T) {
	errors := ValidationErrors{
		{Field: "field1", Message: "error1"},
		{Field: "field2", Message: "error2"},
	}

	errStr := errors.Error()
	assert.Contains(t, errStr, "field1: error1")
	assert.Contains(t, errStr, "field2: error2")
}

func TestEmptyValidationErrors(t *testing.T) {
	errors := ValidationErrors{}
	assert.Equal(t, "", errors.Error())
	assert.False(t, errors.HasErrors())
}

func TestIsValidAddress(t *testing.T) {
	tests := []struct {
		addr  string
		valid bool
	}{
		{":8080", true},
		{":9090", true},
		{"localhost:8080", true},
		{"127.0.0.1:8080", true},
		{"0.0.0.0:8080", true},
		{"example.com:8080", true},
		{"invalid", false},
		{"", false},
		{":invalid", false},
		{"host:", false},
	}

	for _, tt := range tests {
		t.Run(tt.addr, func(t *testing.T) {
			result := isValidAddress(tt.addr)
			assert.Equal(t, tt.valid, result, "address: %s", tt.addr)
		})
	}
}

func TestIsValidHostname(t *testing.T) {
	tests := []struct {
		hostname string
		valid    bool
	}{
		{"localhost", true},
		{"example.com", true},
		{"sub.example.com", true},
		{"my-host", true},
		{"", false},
		{"-invalid", false},
		{"invalid-", false},
		{strings.Repeat("a", 64), false}, // label too long
	}

	for _, tt := range tests {
		t.Run(tt.hostname, func(t *testing.T) {
			result := isValidHostname(tt.hostname)
			assert.Equal(t, tt.valid, result, "hostname: %s", tt.hostname)
		})
	}
}
