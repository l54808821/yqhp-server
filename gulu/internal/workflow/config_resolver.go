package workflow

// ResolveEnvConfigReferences resolves environment config references in workflow steps.
// This converts frontend config references (domainCode, datasourceCode, etc.)
// into actual config values that the engine executor can consume.
// Both debug mode and performance test mode must call this before execution.
func ResolveEnvConfigReferences(steps []Step, config *MergedConfig) {
	if config == nil {
		return
	}
	for i := range steps {
		step := &steps[i]

		switch step.Type {
		case "http":
			resolveHTTPDomainConfig(step.Config, config)
		case "db", "database":
			resolveDatabaseConfig(step.Config, config)
		case "mq":
			resolveMQConfig(step.Config, config)
		}

		if step.Loop != nil && len(step.Loop.Steps) > 0 {
			ResolveEnvConfigReferences(step.Loop.Steps, config)
		}
		if len(step.Children) > 0 {
			ResolveEnvConfigReferences(step.Children, config)
		}
		for bi := range step.Branches {
			if len(step.Branches[bi].Steps) > 0 {
				ResolveEnvConfigReferences(step.Branches[bi].Steps, config)
			}
		}
	}
}

func resolveHTTPDomainConfig(stepConfig map[string]interface{}, config *MergedConfig) {
	if stepConfig == nil || config.Domains == nil {
		return
	}

	domainCode, _ := stepConfig["domainCode"].(string)
	if domainCode == "" {
		domainCode, _ = stepConfig["domain"].(string)
	}
	if domainCode == "" {
		return
	}

	dc, ok := config.Domains[domainCode]
	if !ok || dc == nil {
		return
	}

	stepConfig["domain"] = domainCode
	stepConfig["domain_base_url"] = dc.BaseURL
	if len(dc.Headers) > 0 {
		stepConfig["domain_headers"] = dc.Headers
	}
	delete(stepConfig, "domainCode")
}

func resolveDatabaseConfig(stepConfig map[string]interface{}, config *MergedConfig) {
	if stepConfig == nil || config.Databases == nil {
		return
	}

	dsCode, _ := stepConfig["datasourceCode"].(string)
	if dsCode == "" {
		dsCode, _ = stepConfig["database_config"].(string)
	}
	if dsCode == "" {
		return
	}

	ds, ok := config.Databases[dsCode]
	if !ok || ds == nil {
		return
	}

	stepConfig["db_type"] = ds.Type
	if ds.Host != "" {
		stepConfig["host"] = ds.Host
	}
	if ds.Port > 0 {
		stepConfig["port"] = ds.Port
	}
	if ds.Database != "" {
		stepConfig["database"] = ds.Database
	}
	if ds.Username != "" {
		stepConfig["username"] = ds.Username
	}
	if ds.Password != "" {
		stepConfig["password"] = ds.Password
	}
	if ds.Options != "" {
		stepConfig["options"] = ds.Options
	}
	delete(stepConfig, "datasourceCode")
	delete(stepConfig, "database_config")
}

func resolveMQConfig(stepConfig map[string]interface{}, config *MergedConfig) {
	// MQ config resolution - placeholder for when MQ configs are supported
}
