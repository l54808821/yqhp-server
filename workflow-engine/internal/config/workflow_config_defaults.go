package config

import "time"

// DefaultWorkflowGlobalConfig 返回默认的工作流全局配置
func DefaultWorkflowGlobalConfig() *WorkflowGlobalConfig {
	return &WorkflowGlobalConfig{
		HTTP: &HTTPGlobalConfig{
			Domains: make(map[string]*DomainConfig),
			Timeout: &TimeoutConfig{
				Connect: 10 * time.Second,
				Read:    30 * time.Second,
				Write:   30 * time.Second,
				Total:   60 * time.Second,
			},
			Redirect: &RedirectConfig{
				Follow:       true,
				MaxRedirects: 10,
			},
			Headers: make(map[string]string),
		},
		Socket: &SocketGlobalConfig{
			DefaultProtocol: "tcp",
			Timeout: &TimeoutConfig{
				Connect: 10 * time.Second,
				Read:    30 * time.Second,
				Write:   30 * time.Second,
			},
			BufferSize: 4096,
		},
		MQ: &MQGlobalConfig{
			Connections: make(map[string]*MQConnectionConfig),
			Timeout:     30 * time.Second,
		},
		DB: &DBGlobalConfig{
			Connections: make(map[string]*DBConnectionConfig),
			Timeout:     30 * time.Second,
		},
		Variables: make(map[string]any),
	}
}

// GetDomainConfig 获取域名配置
func (c *HTTPGlobalConfig) GetDomainConfig(domain string) *DomainConfig {
	if c == nil || c.Domains == nil {
		return nil
	}
	if domain == "" {
		domain = c.DefaultDomain
	}
	if config, ok := c.Domains[domain]; ok {
		return config
	}
	return nil
}

// GetMQConnection 获取 MQ 连接配置
func (c *MQGlobalConfig) GetMQConnection(name string) *MQConnectionConfig {
	if c == nil || c.Connections == nil {
		return nil
	}
	if name == "" {
		name = c.DefaultConnection
	}
	if config, ok := c.Connections[name]; ok {
		return config
	}
	return nil
}

// GetDBConnection 获取 DB 连接配置
func (c *DBGlobalConfig) GetDBConnection(name string) *DBConnectionConfig {
	if c == nil || c.Connections == nil {
		return nil
	}
	if name == "" {
		name = c.DefaultConnection
	}
	if config, ok := c.Connections[name]; ok {
		return config
	}
	return nil
}
