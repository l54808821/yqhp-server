package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultWorkflowGlobalConfig(t *testing.T) {
	cfg := DefaultWorkflowGlobalConfig()

	require.NotNil(t, cfg)
	require.NotNil(t, cfg.HTTP)
	require.NotNil(t, cfg.Socket)
	require.NotNil(t, cfg.MQ)
	require.NotNil(t, cfg.DB)
	require.NotNil(t, cfg.Variables)

	// HTTP defaults
	assert.Equal(t, 10*time.Second, cfg.HTTP.Timeout.Connect)
	assert.Equal(t, 30*time.Second, cfg.HTTP.Timeout.Read)
	assert.Equal(t, 30*time.Second, cfg.HTTP.Timeout.Write)
	assert.Equal(t, 60*time.Second, cfg.HTTP.Timeout.Total)
	assert.True(t, cfg.HTTP.Redirect.Follow)
	assert.Equal(t, 10, cfg.HTTP.Redirect.MaxRedirects)

	// Socket defaults
	assert.Equal(t, "tcp", cfg.Socket.DefaultProtocol)
	assert.Equal(t, 4096, cfg.Socket.BufferSize)

	// MQ defaults
	assert.Equal(t, 30*time.Second, cfg.MQ.Timeout)

	// DB defaults
	assert.Equal(t, 30*time.Second, cfg.DB.Timeout)
}

func TestWorkflowGlobalConfig_Clone(t *testing.T) {
	cfg := DefaultWorkflowGlobalConfig()
	cfg.Environment = "test"
	cfg.Variables["key"] = "value"
	cfg.HTTP.DefaultDomain = "api"
	cfg.HTTP.Domains["api"] = &DomainConfig{
		BaseURL: "https://api.example.com",
	}

	clone := cfg.Clone()

	require.NotNil(t, clone)
	assert.Equal(t, cfg.Environment, clone.Environment)
	assert.Equal(t, cfg.Variables["key"], clone.Variables["key"])
	assert.Equal(t, cfg.HTTP.DefaultDomain, clone.HTTP.DefaultDomain)
	assert.Equal(t, cfg.HTTP.Domains["api"].BaseURL, clone.HTTP.Domains["api"].BaseURL)

	// Modify clone should not affect original
	clone.Environment = "modified"
	clone.Variables["key"] = "modified"
	assert.Equal(t, "test", cfg.Environment)
	assert.Equal(t, "value", cfg.Variables["key"])
}

func TestWorkflowGlobalConfig_Merge(t *testing.T) {
	base := DefaultWorkflowGlobalConfig()
	base.Environment = "base"
	base.Variables["base_key"] = "base_value"
	base.HTTP.DefaultDomain = "base_domain"

	override := &WorkflowGlobalConfig{
		Environment: "override",
		Variables: map[string]any{
			"override_key": "override_value",
		},
		HTTP: &HTTPGlobalConfig{
			DefaultDomain: "override_domain",
			Timeout: &TimeoutConfig{
				Connect: 5 * time.Second,
			},
		},
	}

	merged := base.Merge(override)

	require.NotNil(t, merged)
	assert.Equal(t, "override", merged.Environment)
	assert.Equal(t, "base_value", merged.Variables["base_key"])
	assert.Equal(t, "override_value", merged.Variables["override_key"])
	assert.Equal(t, "override_domain", merged.HTTP.DefaultDomain)
	assert.Equal(t, 5*time.Second, merged.HTTP.Timeout.Connect)
	// Other timeout values should be preserved from base
	assert.Equal(t, 30*time.Second, merged.HTTP.Timeout.Read)
}

func TestHTTPGlobalConfig_GetDomainConfig(t *testing.T) {
	cfg := &HTTPGlobalConfig{
		DefaultDomain: "default",
		Domains: map[string]*DomainConfig{
			"default": {BaseURL: "https://default.example.com"},
			"api":     {BaseURL: "https://api.example.com"},
		},
	}

	// Get specific domain
	apiDomain := cfg.GetDomainConfig("api")
	require.NotNil(t, apiDomain)
	assert.Equal(t, "https://api.example.com", apiDomain.BaseURL)

	// Get default domain when empty
	defaultDomain := cfg.GetDomainConfig("")
	require.NotNil(t, defaultDomain)
	assert.Equal(t, "https://default.example.com", defaultDomain.BaseURL)

	// Get non-existent domain
	nonExistent := cfg.GetDomainConfig("nonexistent")
	assert.Nil(t, nonExistent)
}

func TestHTTPGlobalConfig_MergeTimeout(t *testing.T) {
	cfg := &HTTPGlobalConfig{
		DefaultDomain: "api",
		Timeout: &TimeoutConfig{
			Connect: 10 * time.Second,
			Read:    30 * time.Second,
			Write:   30 * time.Second,
			Total:   60 * time.Second,
		},
		Domains: map[string]*DomainConfig{
			"api": {
				BaseURL: "https://api.example.com",
				Timeout: &TimeoutConfig{
					Connect: 5 * time.Second,
					Read:    15 * time.Second,
				},
			},
		},
	}

	// Test: step > domain > global
	stepTimeout := &TimeoutConfig{
		Connect: 2 * time.Second,
	}

	merged := cfg.MergeTimeout("api", stepTimeout)

	assert.Equal(t, 2*time.Second, merged.Connect) // from step
	assert.Equal(t, 15*time.Second, merged.Read)   // from domain
	assert.Equal(t, 30*time.Second, merged.Write)  // from global
	assert.Equal(t, 60*time.Second, merged.Total)  // from global
}

func TestHTTPGlobalConfig_MergeRedirect(t *testing.T) {
	cfg := &HTTPGlobalConfig{
		DefaultDomain: "api",
		Redirect: &RedirectConfig{
			Follow:       true,
			MaxRedirects: 10,
		},
		Domains: map[string]*DomainConfig{
			"api": {
				BaseURL: "https://api.example.com",
				Redirect: &RedirectConfig{
					Follow:       false,
					MaxRedirects: 5,
				},
			},
		},
	}

	// Test: domain overrides global
	merged := cfg.MergeRedirect("api", nil)
	assert.False(t, merged.Follow)
	assert.Equal(t, 5, merged.MaxRedirects)

	// Test: step overrides domain
	stepRedirect := &RedirectConfig{
		Follow:       true,
		MaxRedirects: 3,
	}
	merged = cfg.MergeRedirect("api", stepRedirect)
	assert.True(t, merged.Follow)
	assert.Equal(t, 3, merged.MaxRedirects)
}

func TestHTTPGlobalConfig_MergeHeaders(t *testing.T) {
	cfg := &HTTPGlobalConfig{
		DefaultDomain: "api",
		Headers: map[string]string{
			"X-Global-Header": "global",
			"Content-Type":    "application/json",
		},
		Domains: map[string]*DomainConfig{
			"api": {
				BaseURL: "https://api.example.com",
				Headers: map[string]string{
					"X-Domain-Header": "domain",
					"Content-Type":    "text/plain",
				},
			},
		},
	}

	stepHeaders := map[string]string{
		"X-Step-Header": "step",
		"Content-Type":  "application/xml",
	}

	merged := cfg.MergeHeaders("api", stepHeaders)

	assert.Equal(t, "global", merged["X-Global-Header"])
	assert.Equal(t, "domain", merged["X-Domain-Header"])
	assert.Equal(t, "step", merged["X-Step-Header"])
	assert.Equal(t, "application/xml", merged["Content-Type"]) // step wins
}

func TestMQGlobalConfig_GetMQConnection(t *testing.T) {
	cfg := &MQGlobalConfig{
		DefaultConnection: "default",
		Connections: map[string]*MQConnectionConfig{
			"default":  {Type: "kafka", Brokers: "localhost:9092"},
			"rabbitmq": {Type: "rabbitmq", Brokers: "localhost:5672"},
		},
	}

	// Get specific connection
	rabbitmq := cfg.GetMQConnection("rabbitmq")
	require.NotNil(t, rabbitmq)
	assert.Equal(t, "rabbitmq", rabbitmq.Type)

	// Get default connection when empty
	defaultConn := cfg.GetMQConnection("")
	require.NotNil(t, defaultConn)
	assert.Equal(t, "kafka", defaultConn.Type)

	// Get non-existent connection
	nonExistent := cfg.GetMQConnection("nonexistent")
	assert.Nil(t, nonExistent)
}

func TestDBGlobalConfig_GetDBConnection(t *testing.T) {
	cfg := &DBGlobalConfig{
		DefaultConnection: "default",
		Connections: map[string]*DBConnectionConfig{
			"default":  {Driver: "mysql", DSN: "root:password@tcp(localhost:3306)/test"},
			"postgres": {Driver: "postgres", DSN: "postgres://localhost:5432/test"},
		},
	}

	// Get specific connection
	postgres := cfg.GetDBConnection("postgres")
	require.NotNil(t, postgres)
	assert.Equal(t, "postgres", postgres.Driver)

	// Get default connection when empty
	defaultConn := cfg.GetDBConnection("")
	require.NotNil(t, defaultConn)
	assert.Equal(t, "mysql", defaultConn.Driver)

	// Get non-existent connection
	nonExistent := cfg.GetDBConnection("nonexistent")
	assert.Nil(t, nonExistent)
}

func TestTimeoutConfig_Merge(t *testing.T) {
	base := &TimeoutConfig{
		Connect: 10 * time.Second,
		Read:    30 * time.Second,
		Write:   30 * time.Second,
		Total:   60 * time.Second,
	}

	override := &TimeoutConfig{
		Connect: 5 * time.Second,
		Read:    0, // Should not override
	}

	merged := base.Merge(override)

	assert.Equal(t, 5*time.Second, merged.Connect)
	assert.Equal(t, 30*time.Second, merged.Read)
	assert.Equal(t, 30*time.Second, merged.Write)
	assert.Equal(t, 60*time.Second, merged.Total)
}

func TestSocketGlobalConfig_Merge(t *testing.T) {
	base := &SocketGlobalConfig{
		DefaultProtocol: "tcp",
		BufferSize:      4096,
		Timeout: &TimeoutConfig{
			Connect: 10 * time.Second,
		},
	}

	override := &SocketGlobalConfig{
		DefaultProtocol: "udp",
		BufferSize:      8192,
	}

	merged := base.Merge(override)

	assert.Equal(t, "udp", merged.DefaultProtocol)
	assert.Equal(t, 8192, merged.BufferSize)
	assert.Equal(t, 10*time.Second, merged.Timeout.Connect)
}

func TestMQGlobalConfig_Merge(t *testing.T) {
	base := &MQGlobalConfig{
		DefaultConnection: "kafka",
		Timeout:           30 * time.Second,
		Connections: map[string]*MQConnectionConfig{
			"kafka": {Type: "kafka", Brokers: "localhost:9092"},
		},
	}

	override := &MQGlobalConfig{
		DefaultConnection: "rabbitmq",
		Connections: map[string]*MQConnectionConfig{
			"rabbitmq": {Type: "rabbitmq", Brokers: "localhost:5672"},
		},
	}

	merged := base.Merge(override)

	assert.Equal(t, "rabbitmq", merged.DefaultConnection)
	assert.Equal(t, 30*time.Second, merged.Timeout)
	assert.Len(t, merged.Connections, 2)
	assert.NotNil(t, merged.Connections["kafka"])
	assert.NotNil(t, merged.Connections["rabbitmq"])
}

func TestDBGlobalConfig_Merge(t *testing.T) {
	base := &DBGlobalConfig{
		DefaultConnection: "mysql",
		Timeout:           30 * time.Second,
		Connections: map[string]*DBConnectionConfig{
			"mysql": {Driver: "mysql", DSN: "root:password@tcp(localhost:3306)/test"},
		},
	}

	override := &DBGlobalConfig{
		DefaultConnection: "postgres",
		Connections: map[string]*DBConnectionConfig{
			"postgres": {Driver: "postgres", DSN: "postgres://localhost:5432/test"},
		},
	}

	merged := base.Merge(override)

	assert.Equal(t, "postgres", merged.DefaultConnection)
	assert.Equal(t, 30*time.Second, merged.Timeout)
	assert.Len(t, merged.Connections, 2)
	assert.NotNil(t, merged.Connections["mysql"])
	assert.NotNil(t, merged.Connections["postgres"])
}

func TestNilConfig_Merge(t *testing.T) {
	// Test nil base
	var nilCfg *WorkflowGlobalConfig
	other := DefaultWorkflowGlobalConfig()
	merged := nilCfg.Merge(other)
	assert.NotNil(t, merged)

	// Test nil other
	base := DefaultWorkflowGlobalConfig()
	merged = base.Merge(nil)
	assert.NotNil(t, merged)
	assert.Equal(t, base.HTTP.Timeout.Connect, merged.HTTP.Timeout.Connect)
}
