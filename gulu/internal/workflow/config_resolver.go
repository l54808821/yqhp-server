package workflow

import (
	"fmt"
	"strings"

	"yqhp/workflow-engine/pkg/types"
)

// ResolveEnvConfigReferences resolves environment config references in workflow steps.
// This converts frontend config references (domainCode, datasourceCode, mq_config)
// into actual config values that the engine executor can consume.
// All execution paths (debug, perf, CLI) must call this before execution.
func ResolveEnvConfigReferences(steps []types.Step, config *MergedConfig) {
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
	if stepConfig == nil {
		return
	}

	// 规范化前端 params：[{key,value,...}] → [value1, value2, ...]
	if params, ok := stepConfig["params"].([]interface{}); ok && len(params) > 0 {
		plainParams := make([]interface{}, 0, len(params))
		for _, p := range params {
			if pMap, ok := p.(map[string]interface{}); ok {
				if v, exists := pMap["value"]; exists {
					plainParams = append(plainParams, v)
				}
			} else {
				plainParams = append(plainParams, p)
			}
		}
		stepConfig["params"] = plainParams
	}

	// settings.timeout 毫秒 → duration 字符串
	if settings, ok := stepConfig["settings"].(map[string]interface{}); ok {
		if timeout, ok := settings["timeout"].(float64); ok && timeout > 0 {
			stepConfig["timeout"] = fmt.Sprintf("%dms", int(timeout))
		}
	}

	// 旧格式兼容：query → sql
	if _, hasSQL := stepConfig["sql"]; !hasSQL {
		if query, ok := stepConfig["query"].(string); ok {
			stepConfig["sql"] = query
		}
	}

	if config == nil || config.Databases == nil {
		return
	}

	dsCode, _ := stepConfig["database_config"].(string)
	if dsCode == "" {
		dsCode, _ = stepConfig["datasourceCode"].(string)
	}
	if dsCode == "" {
		return
	}

	ds, ok := config.Databases[dsCode]
	if !ok || ds == nil {
		return
	}

	stepConfig["driver"] = ds.Type
	dsn := BuildDSN(ds)
	if dsn != "" {
		stepConfig["dsn"] = dsn
	}

	delete(stepConfig, "datasourceCode")
	delete(stepConfig, "database_config")
}

func resolveMQConfig(stepConfig map[string]interface{}, config *MergedConfig) {
	if stepConfig == nil || config.MQs == nil {
		return
	}

	mqCode, _ := stepConfig["mq_config"].(string)
	if mqCode == "" {
		return
	}

	mc, ok := config.MQs[mqCode]
	if !ok || mc == nil {
		return
	}

	stepConfig["type"] = mc.Type
	stepConfig["broker"] = fmt.Sprintf("%s:%d", mc.Host, mc.Port)

	if mc.Username != "" || mc.Password != "" {
		stepConfig["auth"] = map[string]interface{}{
			"username": mc.Username,
			"password": mc.Password,
		}
	}
	if mc.VHost != "" {
		if opts, ok := stepConfig["options"].(map[string]interface{}); ok {
			opts["vhost"] = mc.VHost
		} else {
			stepConfig["options"] = map[string]interface{}{"vhost": mc.VHost}
		}
	}

	delete(stepConfig, "mq_config")
}

// BuildDSN constructs a DSN connection string from a DatabaseConfig.
func BuildDSN(dc *DatabaseConfig) string {
	switch strings.ToLower(dc.Type) {
	case "mysql":
		dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s",
			dc.Username, dc.Password, dc.Host, dc.Port, dc.Database)
		if dc.Options != "" {
			dsn += "?" + dc.Options
		} else {
			dsn += "?charset=utf8mb4&parseTime=True&loc=Local"
		}
		return dsn
	case "postgres", "postgresql":
		dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s",
			dc.Host, dc.Port, dc.Username, dc.Password, dc.Database)
		if dc.Options != "" {
			dsn += " " + dc.Options
		} else {
			dsn += " sslmode=disable"
		}
		return dsn
	case "redis":
		if dc.Password != "" {
			return fmt.Sprintf("redis://%s:%s@%s:%d", dc.Username, dc.Password, dc.Host, dc.Port)
		}
		return fmt.Sprintf("%s:%d", dc.Host, dc.Port)
	case "mongodb":
		if dc.Username != "" {
			return fmt.Sprintf("mongodb://%s:%s@%s:%d/%s",
				dc.Username, dc.Password, dc.Host, dc.Port, dc.Database)
		}
		return fmt.Sprintf("mongodb://%s:%d/%s", dc.Host, dc.Port, dc.Database)
	default:
		return fmt.Sprintf("%s:%s@%s:%d/%s",
			dc.Username, dc.Password, dc.Host, dc.Port, dc.Database)
	}
}
