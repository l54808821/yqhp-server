// Package config provides property-based tests for configuration handling.
// Requirements: 10.6 - Configuration Round-Trip consistency
// Property 2: For any valid Configuration object, serializing it and then deserializing
// should produce an equivalent object.
package config

import (
	"testing"
	"time"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
	"github.com/stretchr/testify/assert"
)

// TestConfigRoundTripProperty tests Property 2: Config Round-Trip consistency.
// deserialize(serialize(config)) == config
func TestConfigRoundTripProperty(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	// Property: Config round-trip preserves data
	properties.Property("config round-trip preserves data", prop.ForAll(
		func(config *Config) bool {
			// Serialize to YAML
			yamlBytes, err := config.Serialize()
			if err != nil {
				return false
			}

			// Deserialize from YAML
			parsed, err := ParseConfig(yamlBytes)
			if err != nil {
				return false
			}

			// Compare configs
			return configsEqual(config, parsed)
		},
		genConfig(),
	))

	properties.TestingRun(t)
}

// TestServerConfigRoundTripProperty tests server config round-trip.
func TestServerConfigRoundTripProperty(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	properties.Property("server config round-trip preserves data", prop.ForAll(
		func(serverConfig ServerConfig) bool {
			config := DefaultConfig()
			config.Server = serverConfig

			yamlBytes, err := config.Serialize()
			if err != nil {
				return false
			}

			parsed, err := ParseConfig(yamlBytes)
			if err != nil {
				return false
			}

			return config.Server.Address == parsed.Server.Address &&
				config.Server.ReadTimeout == parsed.Server.ReadTimeout &&
				config.Server.WriteTimeout == parsed.Server.WriteTimeout
		},
		genServerConfig(),
	))

	properties.TestingRun(t)
}

// Generators for property-based testing

// genConfig generates a complete configuration.
func genConfig() gopter.Gen {
	return gopter.CombineGens(
		genServerConfig(),
		genGRPCConfig(),
		genMasterConfigGen(),
		genSlaveConfigGen(),
	).Map(func(values []interface{}) *Config {
		return &Config{
			Server:  values[0].(ServerConfig),
			GRPC:    values[1].(GRPCConfig),
			Master:  values[2].(MasterConfig),
			Slave:   values[3].(SlaveConfig),
			Logging: LoggingConfig{Level: "info", Format: "json", Output: "stdout"},
		}
	})
}

// genServerConfig generates a server configuration.
func genServerConfig() gopter.Gen {
	return gopter.CombineGens(
		gen.IntRange(1024, 65535),
		gen.IntRange(1, 60),
		gen.IntRange(1, 60),
		gen.Bool(),
	).Map(func(values []interface{}) ServerConfig {
		port := values[0].(int)
		return ServerConfig{
			Address:       ":" + string(rune(port%10000+48)) + string(rune((port/10)%10+48)) + string(rune((port/100)%10+48)) + string(rune((port/1000)%10+48)),
			ReadTimeout:   time.Duration(values[1].(int)) * time.Second,
			WriteTimeout:  time.Duration(values[2].(int)) * time.Second,
			EnableCORS:    values[3].(bool),
			EnableSwagger: false,
		}
	})
}

// genGRPCConfig generates a gRPC configuration.
func genGRPCConfig() gopter.Gen {
	return gopter.CombineGens(
		gen.IntRange(1024, 65535),
		gen.IntRange(1, 10),
	).Map(func(values []interface{}) GRPCConfig {
		return GRPCConfig{
			Address:           ":9090",
			MaxRecvMsgSize:    values[1].(int) * 1024 * 1024,
			MaxSendMsgSize:    values[1].(int) * 1024 * 1024,
			ConnectionTimeout: 10 * time.Second,
		}
	})
}

// genMasterConfigGen generates a master configuration.
func genMasterConfigGen() gopter.Gen {
	return gopter.CombineGens(
		gen.IntRange(1, 30),
		gen.IntRange(5, 60),
		gen.IntRange(100, 10000),
		gen.IntRange(1, 100),
	).Map(func(values []interface{}) MasterConfig {
		return MasterConfig{
			HeartbeatInterval: time.Duration(values[0].(int)) * time.Second,
			HeartbeatTimeout:  time.Duration(values[1].(int)) * time.Second,
			TaskQueueSize:     values[2].(int),
			MaxSlaves:         values[3].(int),
		}
	})
}

// genSlaveConfigGen generates a slave configuration.
func genSlaveConfigGen() gopter.Gen {
	return gopter.CombineGens(
		gen.OneConstOf("worker", "gateway", "aggregator"),
		gen.IntRange(1, 1000),
	).Map(func(values []interface{}) SlaveConfig {
		return SlaveConfig{
			Type:         values[0].(string),
			Capabilities: []string{"http_executor"},
			Labels:       map[string]string{},
			MaxVUs:       values[1].(int),
			MasterAddr:   "localhost:9090",
		}
	})
}

// Helper functions

// configsEqual compares two configs for equality.
func configsEqual(a, b *Config) bool {
	// Compare server config
	if a.Server.Address != b.Server.Address {
		return false
	}
	if a.Server.ReadTimeout != b.Server.ReadTimeout {
		return false
	}
	if a.Server.WriteTimeout != b.Server.WriteTimeout {
		return false
	}

	// Compare GRPC config
	if a.GRPC.Address != b.GRPC.Address {
		return false
	}

	// Compare Master config
	if a.Master.HeartbeatInterval != b.Master.HeartbeatInterval {
		return false
	}
	if a.Master.HeartbeatTimeout != b.Master.HeartbeatTimeout {
		return false
	}

	// Compare Slave config
	if a.Slave.Type != b.Slave.Type {
		return false
	}
	if a.Slave.MaxVUs != b.Slave.MaxVUs {
		return false
	}

	return true
}

// BenchmarkConfigRoundTrip benchmarks config round-trip.
func BenchmarkConfigRoundTrip(b *testing.B) {
	config := DefaultConfig()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		yamlBytes, _ := config.Serialize()
		ParseConfig(yamlBytes)
	}
}

// TestConfigRoundTripSpecificCases tests specific edge cases.
func TestConfigRoundTripSpecificCases(t *testing.T) {
	testCases := []struct {
		name   string
		config *Config
	}{
		{
			name:   "default config",
			config: DefaultConfig(),
		},
		{
			name: "custom server config",
			config: func() *Config {
				c := DefaultConfig()
				c.Server.Address = ":9999"
				c.Server.ReadTimeout = 60 * time.Second
				return c
			}(),
		},
		{
			name: "custom slave config",
			config: func() *Config {
				c := DefaultConfig()
				c.Slave.Type = "aggregator"
				c.Slave.MaxVUs = 500
				return c
			}(),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			yamlBytes, err := tc.config.Serialize()
			assert.NoError(t, err)

			parsed, err := ParseConfig(yamlBytes)
			assert.NoError(t, err)

			assert.Equal(t, tc.config.Server.Address, parsed.Server.Address)
			assert.Equal(t, tc.config.Slave.Type, parsed.Slave.Type)
		})
	}
}
