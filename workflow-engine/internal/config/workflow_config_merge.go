package config

// Merge 合并配置（other 覆盖 c）
func (c *WorkflowGlobalConfig) Merge(other *WorkflowGlobalConfig) *WorkflowGlobalConfig {
	if other == nil {
		return c
	}
	if c == nil {
		return other.Clone()
	}

	result := c.Clone()

	// 合并 HTTP 配置
	if other.HTTP != nil {
		if result.HTTP == nil {
			result.HTTP = &HTTPGlobalConfig{}
		}
		result.HTTP = result.HTTP.Merge(other.HTTP)
	}

	// 合并 Socket 配置
	if other.Socket != nil {
		if result.Socket == nil {
			result.Socket = &SocketGlobalConfig{}
		}
		result.Socket = result.Socket.Merge(other.Socket)
	}

	// 合并 MQ 配置
	if other.MQ != nil {
		if result.MQ == nil {
			result.MQ = &MQGlobalConfig{}
		}
		result.MQ = result.MQ.Merge(other.MQ)
	}

	// 合并 DB 配置
	if other.DB != nil {
		if result.DB == nil {
			result.DB = &DBGlobalConfig{}
		}
		result.DB = result.DB.Merge(other.DB)
	}

	// 合并变量
	if result.Variables == nil {
		result.Variables = make(map[string]any)
	}
	for k, v := range other.Variables {
		result.Variables[k] = v
	}

	// 覆盖环境
	if other.Environment != "" {
		result.Environment = other.Environment
	}

	return result
}

// Clone 克隆配置
func (c *WorkflowGlobalConfig) Clone() *WorkflowGlobalConfig {
	if c == nil {
		return nil
	}

	result := &WorkflowGlobalConfig{
		Environment: c.Environment,
		Variables:   make(map[string]any),
	}

	for k, v := range c.Variables {
		result.Variables[k] = v
	}

	if c.HTTP != nil {
		result.HTTP = c.HTTP.Clone()
	}
	if c.Socket != nil {
		result.Socket = c.Socket.Clone()
	}
	if c.MQ != nil {
		result.MQ = c.MQ.Clone()
	}
	if c.DB != nil {
		result.DB = c.DB.Clone()
	}

	return result
}

// Merge 合并 HTTP 配置
func (c *HTTPGlobalConfig) Merge(other *HTTPGlobalConfig) *HTTPGlobalConfig {
	if other == nil {
		return c
	}
	if c == nil {
		return other.Clone()
	}

	result := c.Clone()

	// 合并域名配置
	if result.Domains == nil {
		result.Domains = make(map[string]*DomainConfig)
	}
	for name, domain := range other.Domains {
		result.Domains[name] = domain
	}

	// 覆盖默认域名
	if other.DefaultDomain != "" {
		result.DefaultDomain = other.DefaultDomain
	}

	// 合并超时配置
	if other.Timeout != nil {
		result.Timeout = result.Timeout.Merge(other.Timeout)
	}

	// 合并重定向配置
	if other.Redirect != nil {
		result.Redirect = other.Redirect
	}

	// 合并 TLS 配置
	if other.TLS != nil {
		result.TLS = other.TLS
	}

	// 合并请求头
	if result.Headers == nil {
		result.Headers = make(map[string]string)
	}
	for k, v := range other.Headers {
		result.Headers[k] = v
	}

	return result
}

// Clone 克隆 HTTP 配置
func (c *HTTPGlobalConfig) Clone() *HTTPGlobalConfig {
	if c == nil {
		return nil
	}

	result := &HTTPGlobalConfig{
		Domains:       make(map[string]*DomainConfig),
		DefaultDomain: c.DefaultDomain,
		Headers:       make(map[string]string),
	}

	for name, domain := range c.Domains {
		result.Domains[name] = domain
	}

	if c.Timeout != nil {
		result.Timeout = c.Timeout.Clone()
	}
	if c.Redirect != nil {
		result.Redirect = &RedirectConfig{
			Follow:       c.Redirect.Follow,
			MaxRedirects: c.Redirect.MaxRedirects,
		}
	}
	if c.TLS != nil {
		result.TLS = c.TLS.Clone()
	}

	for k, v := range c.Headers {
		result.Headers[k] = v
	}

	return result
}

// Merge 合并超时配置
func (c *TimeoutConfig) Merge(other *TimeoutConfig) *TimeoutConfig {
	if c == nil {
		return other
	}
	if other == nil {
		return c
	}

	result := c.Clone()

	if other.Connect > 0 {
		result.Connect = other.Connect
	}
	if other.Read > 0 {
		result.Read = other.Read
	}
	if other.Write > 0 {
		result.Write = other.Write
	}
	if other.Total > 0 {
		result.Total = other.Total
	}

	return result
}

// Clone 克隆超时配置
func (c *TimeoutConfig) Clone() *TimeoutConfig {
	if c == nil {
		return nil
	}
	return &TimeoutConfig{
		Connect: c.Connect,
		Read:    c.Read,
		Write:   c.Write,
		Total:   c.Total,
	}
}

// Clone 克隆 TLS 配置
func (c *TLSGlobalConfig) Clone() *TLSGlobalConfig {
	if c == nil {
		return nil
	}
	return &TLSGlobalConfig{
		InsecureSkipVerify: c.InsecureSkipVerify,
		CertFile:           c.CertFile,
		KeyFile:            c.KeyFile,
		CAFile:             c.CAFile,
		MinVersion:         c.MinVersion,
	}
}

// Merge 合并 Socket 配置
func (c *SocketGlobalConfig) Merge(other *SocketGlobalConfig) *SocketGlobalConfig {
	if other == nil {
		return c
	}
	if c == nil {
		return other.Clone()
	}

	result := c.Clone()

	if other.DefaultProtocol != "" {
		result.DefaultProtocol = other.DefaultProtocol
	}
	if other.Timeout != nil {
		result.Timeout = result.Timeout.Merge(other.Timeout)
	}
	if other.TLS != nil {
		result.TLS = other.TLS
	}
	if other.BufferSize > 0 {
		result.BufferSize = other.BufferSize
	}

	return result
}

// Clone 克隆 Socket 配置
func (c *SocketGlobalConfig) Clone() *SocketGlobalConfig {
	if c == nil {
		return nil
	}

	result := &SocketGlobalConfig{
		DefaultProtocol: c.DefaultProtocol,
		BufferSize:      c.BufferSize,
	}

	if c.Timeout != nil {
		result.Timeout = c.Timeout.Clone()
	}
	if c.TLS != nil {
		result.TLS = c.TLS.Clone()
	}

	return result
}

// Merge 合并 MQ 配置
func (c *MQGlobalConfig) Merge(other *MQGlobalConfig) *MQGlobalConfig {
	if other == nil {
		return c
	}
	if c == nil {
		return other.Clone()
	}

	result := c.Clone()

	if result.Connections == nil {
		result.Connections = make(map[string]*MQConnectionConfig)
	}
	for name, conn := range other.Connections {
		result.Connections[name] = conn
	}

	if other.DefaultConnection != "" {
		result.DefaultConnection = other.DefaultConnection
	}
	if other.Timeout > 0 {
		result.Timeout = other.Timeout
	}

	return result
}

// Clone 克隆 MQ 配置
func (c *MQGlobalConfig) Clone() *MQGlobalConfig {
	if c == nil {
		return nil
	}

	result := &MQGlobalConfig{
		Connections:       make(map[string]*MQConnectionConfig),
		DefaultConnection: c.DefaultConnection,
		Timeout:           c.Timeout,
	}

	for name, conn := range c.Connections {
		result.Connections[name] = &MQConnectionConfig{
			Type:     conn.Type,
			Brokers:  conn.Brokers,
			Username: conn.Username,
			Password: conn.Password,
		}
	}

	return result
}

// Merge 合并 DB 配置
func (c *DBGlobalConfig) Merge(other *DBGlobalConfig) *DBGlobalConfig {
	if other == nil {
		return c
	}
	if c == nil {
		return other.Clone()
	}

	result := c.Clone()

	if result.Connections == nil {
		result.Connections = make(map[string]*DBConnectionConfig)
	}
	for name, conn := range other.Connections {
		result.Connections[name] = conn
	}

	if other.DefaultConnection != "" {
		result.DefaultConnection = other.DefaultConnection
	}
	if other.Timeout > 0 {
		result.Timeout = other.Timeout
	}

	return result
}

// Clone 克隆 DB 配置
func (c *DBGlobalConfig) Clone() *DBGlobalConfig {
	if c == nil {
		return nil
	}

	result := &DBGlobalConfig{
		Connections:       make(map[string]*DBConnectionConfig),
		DefaultConnection: c.DefaultConnection,
		Timeout:           c.Timeout,
	}

	for name, conn := range c.Connections {
		result.Connections[name] = &DBConnectionConfig{
			Driver:   conn.Driver,
			DSN:      conn.DSN,
			MaxConns: conn.MaxConns,
			MaxIdle:  conn.MaxIdle,
		}
	}

	return result
}

// MergeTimeout 合并超时配置（步骤 > 域名 > 全局）
func (c *HTTPGlobalConfig) MergeTimeout(domain string, stepTimeout *TimeoutConfig) *TimeoutConfig {
	result := &TimeoutConfig{}

	// 1. 应用全局配置
	if c != nil && c.Timeout != nil {
		*result = *c.Timeout
	}

	// 2. 应用域名配置
	if domainConfig := c.GetDomainConfig(domain); domainConfig != nil && domainConfig.Timeout != nil {
		if domainConfig.Timeout.Connect > 0 {
			result.Connect = domainConfig.Timeout.Connect
		}
		if domainConfig.Timeout.Read > 0 {
			result.Read = domainConfig.Timeout.Read
		}
		if domainConfig.Timeout.Write > 0 {
			result.Write = domainConfig.Timeout.Write
		}
		if domainConfig.Timeout.Total > 0 {
			result.Total = domainConfig.Timeout.Total
		}
	}

	// 3. 应用步骤配置
	if stepTimeout != nil {
		if stepTimeout.Connect > 0 {
			result.Connect = stepTimeout.Connect
		}
		if stepTimeout.Read > 0 {
			result.Read = stepTimeout.Read
		}
		if stepTimeout.Write > 0 {
			result.Write = stepTimeout.Write
		}
		if stepTimeout.Total > 0 {
			result.Total = stepTimeout.Total
		}
	}

	return result
}

// MergeRedirect 合并重定向配置（步骤 > 域名 > 全局）
func (c *HTTPGlobalConfig) MergeRedirect(domain string, stepRedirect *RedirectConfig) *RedirectConfig {
	result := &RedirectConfig{
		Follow:       true,
		MaxRedirects: 10,
	}

	// 1. 应用全局配置
	if c != nil && c.Redirect != nil {
		result.Follow = c.Redirect.Follow
		if c.Redirect.MaxRedirects > 0 {
			result.MaxRedirects = c.Redirect.MaxRedirects
		}
	}

	// 2. 应用域名配置
	if domainConfig := c.GetDomainConfig(domain); domainConfig != nil && domainConfig.Redirect != nil {
		result.Follow = domainConfig.Redirect.Follow
		if domainConfig.Redirect.MaxRedirects > 0 {
			result.MaxRedirects = domainConfig.Redirect.MaxRedirects
		}
	}

	// 3. 应用步骤配置
	if stepRedirect != nil {
		result.Follow = stepRedirect.Follow
		if stepRedirect.MaxRedirects > 0 {
			result.MaxRedirects = stepRedirect.MaxRedirects
		}
	}

	return result
}

// MergeHeaders 合并请求头（步骤 > 域名 > 全局）
func (c *HTTPGlobalConfig) MergeHeaders(domain string, stepHeaders map[string]string) map[string]string {
	result := make(map[string]string)

	// 1. 应用全局请求头
	if c != nil {
		for k, v := range c.Headers {
			result[k] = v
		}
	}

	// 2. 应用域名请求头
	if domainConfig := c.GetDomainConfig(domain); domainConfig != nil {
		for k, v := range domainConfig.Headers {
			result[k] = v
		}
	}

	// 3. 应用步骤请求头
	for k, v := range stepHeaders {
		result[k] = v
	}

	return result
}
