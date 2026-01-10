package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	assert.Equal(t, ":8080", cfg.Server.Address)
	assert.Equal(t, 30*time.Second, cfg.Server.ReadTimeout)
	assert.Equal(t, 5*time.Second, cfg.Master.HeartbeatInterval)
	assert.Equal(t, "worker", cfg.Slave.Type)
	assert.Equal(t, "info", cfg.Logging.Level)
}

func TestLoadFromFile(t *testing.T) {
	// Create a temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `
server:
  address: ":9000"
  read_timeout: 60s
  write_timeout: 60s
  enable_cors: true

master:
  heartbeat_interval: 10s
  heartbeat_timeout: 30s

slave:
  type: aggregator
  capabilities:
    - http_executor
    - script_executor
  max_vus: 200

logging:
  level: debug
  format: text
`
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	cfg, err := LoadFromFile(configPath)
	require.NoError(t, err)

	assert.Equal(t, ":9000", cfg.Server.Address)
	assert.Equal(t, 60*time.Second, cfg.Server.ReadTimeout)
	assert.True(t, cfg.Server.EnableCORS)
	assert.Equal(t, 10*time.Second, cfg.Master.HeartbeatInterval)
	assert.Equal(t, "aggregator", cfg.Slave.Type)
	assert.Contains(t, cfg.Slave.Capabilities, "script_executor")
	assert.Equal(t, 200, cfg.Slave.MaxVUs)
	assert.Equal(t, "debug", cfg.Logging.Level)
}

func TestLoadFromNonExistentFile(t *testing.T) {
	cfg, err := LoadFromFile("/nonexistent/path/config.yaml")
	require.NoError(t, err) // Should not error, just use defaults
	assert.Equal(t, DefaultConfig().Server.Address, cfg.Server.Address)
}

func TestEnvOverrides(t *testing.T) {
	// Set environment variables
	os.Setenv("WE_SERVER_ADDRESS", ":7070")
	os.Setenv("WE_SERVER_READ_TIMEOUT", "45s")
	os.Setenv("WE_MASTER_HEARTBEAT_INTERVAL", "20s")
	os.Setenv("WE_SLAVE_TYPE", "gateway")
	os.Setenv("WE_SLAVE_CAPABILITIES", "http_executor,custom_executor")
	os.Setenv("WE_LOG_LEVEL", "warn")
	os.Setenv("WE_SERVER_ENABLE_CORS", "true")

	defer func() {
		os.Unsetenv("WE_SERVER_ADDRESS")
		os.Unsetenv("WE_SERVER_READ_TIMEOUT")
		os.Unsetenv("WE_MASTER_HEARTBEAT_INTERVAL")
		os.Unsetenv("WE_SLAVE_TYPE")
		os.Unsetenv("WE_SLAVE_CAPABILITIES")
		os.Unsetenv("WE_LOG_LEVEL")
		os.Unsetenv("WE_SERVER_ENABLE_CORS")
	}()

	cfg, err := NewLoader().Load()
	require.NoError(t, err)

	assert.Equal(t, ":7070", cfg.Server.Address)
	assert.Equal(t, 45*time.Second, cfg.Server.ReadTimeout)
	assert.Equal(t, 20*time.Second, cfg.Master.HeartbeatInterval)
	assert.Equal(t, "gateway", cfg.Slave.Type)
	assert.Contains(t, cfg.Slave.Capabilities, "custom_executor")
	assert.Equal(t, "warn", cfg.Logging.Level)
	assert.True(t, cfg.Server.EnableCORS)
}

func TestCmdOverrides(t *testing.T) {
	cmdArgs := map[string]string{
		"server.address":            ":6060",
		"server.read_timeout":       "90s",
		"master.heartbeat_interval": "25s",
		"slave.type":                "worker",
		"logging.level":             "error",
	}

	cfg, err := NewLoader().WithCmdArgs(cmdArgs).Load()
	require.NoError(t, err)

	assert.Equal(t, ":6060", cfg.Server.Address)
	assert.Equal(t, 90*time.Second, cfg.Server.ReadTimeout)
	assert.Equal(t, 25*time.Second, cfg.Master.HeartbeatInterval)
	assert.Equal(t, "worker", cfg.Slave.Type)
	assert.Equal(t, "error", cfg.Logging.Level)
}

func TestPrecedence(t *testing.T) {
	// Create a temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `
server:
  address: ":9000"
logging:
  level: debug
`
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	// Set environment variable (should override file)
	os.Setenv("WE_SERVER_ADDRESS", ":8000")
	os.Setenv("WE_LOG_LEVEL", "info")
	defer func() {
		os.Unsetenv("WE_SERVER_ADDRESS")
		os.Unsetenv("WE_LOG_LEVEL")
	}()

	// Set command-line args (should override env)
	cmdArgs := map[string]string{
		"server.address": ":7000",
	}

	cfg, err := NewLoader().
		WithConfigPath(configPath).
		WithCmdArgs(cmdArgs).
		Load()
	require.NoError(t, err)

	// Command-line should win over env and file
	assert.Equal(t, ":7000", cfg.Server.Address)
	// Env should win over file
	assert.Equal(t, "info", cfg.Logging.Level)
}

func TestSerializeAndParse(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Server.Address = ":5000"
	cfg.Slave.Type = "aggregator"
	cfg.Slave.Capabilities = []string{"custom1", "custom2"}

	data, err := cfg.Serialize()
	require.NoError(t, err)

	parsed, err := ParseConfig(data)
	require.NoError(t, err)

	assert.Equal(t, cfg.Server.Address, parsed.Server.Address)
	assert.Equal(t, cfg.Slave.Type, parsed.Slave.Type)
	assert.Equal(t, cfg.Slave.Capabilities, parsed.Slave.Capabilities)
}

func TestClone(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Server.Address = ":5000"

	clone := cfg.Clone()

	// Modify original
	cfg.Server.Address = ":6000"

	// Clone should be unchanged
	assert.Equal(t, ":5000", clone.Server.Address)
}

func TestInvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	invalidContent := `
server:
  address: ":9000"
  invalid yaml content here
    - broken
`
	err := os.WriteFile(configPath, []byte(invalidContent), 0644)
	require.NoError(t, err)

	_, err = LoadFromFile(configPath)
	assert.Error(t, err)
}

func TestInvalidEnvValue(t *testing.T) {
	os.Setenv("WE_SERVER_READ_TIMEOUT", "invalid-duration")
	defer os.Unsetenv("WE_SERVER_READ_TIMEOUT")

	_, err := NewLoader().Load()
	assert.Error(t, err)
}

func TestInvalidCmdPath(t *testing.T) {
	cmdArgs := map[string]string{
		"nonexistent.path": "value",
	}

	_, err := NewLoader().WithCmdArgs(cmdArgs).Load()
	assert.Error(t, err)
}
