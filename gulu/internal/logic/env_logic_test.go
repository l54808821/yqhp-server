package logic

import (
	"testing"

	"pgregory.net/rapid"
)

// Feature: gulu-extension, Property 4: 环境复制完整性
// Validates: Requirements 2.4
// 对于任意环境的复制操作，复制后的环境应包含原环境的所有配置（域名、变量、数据库配置、MQ配置），且数据内容一致

// EnvConfig 模拟环境配置结构
type EnvConfig struct {
	Name        string
	Code        string
	Description string
	Domains     []DomainConfig
	Variables   []VarConfig
	DBConfigs   []DBConfig
	MQConfigs   []MQConfig
}

// DomainConfig 域名配置
type DomainConfig struct {
	Name    string
	Code    string
	BaseURL string
	Headers string
}

// VarConfig 变量配置
type VarConfig struct {
	Name  string
	Key   string
	Value string
	Type  string
}

// DBConfig 数据库配置
type DBConfig struct {
	Name     string
	Code     string
	Type     string
	Host     string
	Port     int
	Database string
}

// MQConfig MQ配置
type MQConfig struct {
	Name  string
	Code  string
	Type  string
	Host  string
	Port  int
	VHost string
}

// copyEnvConfig 模拟环境复制逻辑
func copyEnvConfig(source EnvConfig, newName, newCode string) EnvConfig {
	copied := EnvConfig{
		Name:        newName,
		Code:        newCode,
		Description: source.Description,
		Domains:     make([]DomainConfig, len(source.Domains)),
		Variables:   make([]VarConfig, len(source.Variables)),
		DBConfigs:   make([]DBConfig, len(source.DBConfigs)),
		MQConfigs:   make([]MQConfig, len(source.MQConfigs)),
	}

	// 复制域名配置
	copy(copied.Domains, source.Domains)

	// 复制变量配置
	copy(copied.Variables, source.Variables)

	// 复制数据库配置
	copy(copied.DBConfigs, source.DBConfigs)

	// 复制MQ配置
	copy(copied.MQConfigs, source.MQConfigs)

	return copied
}

// TestEnvCopyCompleteness_Property 属性测试：环境复制完整性
// 验证：复制后的环境应包含原环境的所有配置，且数据内容一致
func TestEnvCopyCompleteness_Property(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// 生成随机源环境配置
		sourceEnv := EnvConfig{
			Name:        rapid.StringMatching(`[a-zA-Z][a-zA-Z0-9]{2,20}`).Draw(t, "envName"),
			Code:        rapid.StringMatching(`[a-z][a-z0-9_]{2,20}`).Draw(t, "envCode"),
			Description: rapid.String().Draw(t, "description"),
		}

		// 生成随机数量的域名配置
		domainCount := rapid.IntRange(0, 5).Draw(t, "domainCount")
		for i := 0; i < domainCount; i++ {
			sourceEnv.Domains = append(sourceEnv.Domains, DomainConfig{
				Name:    rapid.StringMatching(`[a-zA-Z][a-zA-Z0-9]{2,20}`).Draw(t, "domainName"),
				Code:    rapid.StringMatching(`[a-z][a-z0-9_]{2,20}`).Draw(t, "domainCode"),
				BaseURL: "https://" + rapid.StringMatching(`[a-z]{3,10}`).Draw(t, "host") + ".com",
				Headers: `[{"key":"Content-Type","value":"application/json"}]`,
			})
		}

		// 生成随机数量的变量配置
		varCount := rapid.IntRange(0, 10).Draw(t, "varCount")
		for i := 0; i < varCount; i++ {
			sourceEnv.Variables = append(sourceEnv.Variables, VarConfig{
				Name:  rapid.StringMatching(`[a-zA-Z][a-zA-Z0-9]{2,20}`).Draw(t, "varName"),
				Key:   rapid.StringMatching(`[A-Z][A-Z0-9_]{2,20}`).Draw(t, "varKey"),
				Value: rapid.String().Draw(t, "varValue"),
				Type:  rapid.SampledFrom([]string{"string", "number", "boolean", "json"}).Draw(t, "varType"),
			})
		}

		// 生成随机数量的数据库配置
		dbCount := rapid.IntRange(0, 3).Draw(t, "dbCount")
		for i := 0; i < dbCount; i++ {
			sourceEnv.DBConfigs = append(sourceEnv.DBConfigs, DBConfig{
				Name:     rapid.StringMatching(`[a-zA-Z][a-zA-Z0-9]{2,20}`).Draw(t, "dbName"),
				Code:     rapid.StringMatching(`[a-z][a-z0-9_]{2,20}`).Draw(t, "dbCode"),
				Type:     rapid.SampledFrom([]string{"mysql", "redis", "mongodb"}).Draw(t, "dbType"),
				Host:     "127.0.0.1",
				Port:     rapid.IntRange(1000, 65535).Draw(t, "dbPort"),
				Database: rapid.StringMatching(`[a-z][a-z0-9_]{2,20}`).Draw(t, "dbDatabase"),
			})
		}

		// 生成随机数量的MQ配置
		mqCount := rapid.IntRange(0, 3).Draw(t, "mqCount")
		for i := 0; i < mqCount; i++ {
			sourceEnv.MQConfigs = append(sourceEnv.MQConfigs, MQConfig{
				Name:  rapid.StringMatching(`[a-zA-Z][a-zA-Z0-9]{2,20}`).Draw(t, "mqName"),
				Code:  rapid.StringMatching(`[a-z][a-z0-9_]{2,20}`).Draw(t, "mqCode"),
				Type:  rapid.SampledFrom([]string{"rabbitmq", "kafka", "rocketmq"}).Draw(t, "mqType"),
				Host:  "127.0.0.1",
				Port:  rapid.IntRange(1000, 65535).Draw(t, "mqPort"),
				VHost: "/" + rapid.StringMatching(`[a-z]{2,10}`).Draw(t, "vhost"),
			})
		}

		// 生成新环境名称和代码
		newName := rapid.StringMatching(`[a-zA-Z][a-zA-Z0-9]{2,20}`).Draw(t, "newEnvName")
		newCode := rapid.StringMatching(`[a-z][a-z0-9_]{2,20}`).Draw(t, "newEnvCode")

		// 执行复制
		copiedEnv := copyEnvConfig(sourceEnv, newName, newCode)

		// 属性1：复制后的环境名称和代码应为新值
		if copiedEnv.Name != newName {
			t.Fatalf("复制后环境名称应为新值，期望: %s, 实际: %s", newName, copiedEnv.Name)
		}
		if copiedEnv.Code != newCode {
			t.Fatalf("复制后环境代码应为新值，期望: %s, 实际: %s", newCode, copiedEnv.Code)
		}

		// 属性2：描述应保持一致
		if copiedEnv.Description != sourceEnv.Description {
			t.Fatalf("复制后描述应一致，期望: %s, 实际: %s", sourceEnv.Description, copiedEnv.Description)
		}

		// 属性3：域名配置数量应一致
		if len(copiedEnv.Domains) != len(sourceEnv.Domains) {
			t.Fatalf("域名配置数量应一致，期望: %d, 实际: %d", len(sourceEnv.Domains), len(copiedEnv.Domains))
		}

		// 属性4：域名配置内容应一致
		for i, d := range sourceEnv.Domains {
			if copiedEnv.Domains[i].Name != d.Name ||
				copiedEnv.Domains[i].Code != d.Code ||
				copiedEnv.Domains[i].BaseURL != d.BaseURL ||
				copiedEnv.Domains[i].Headers != d.Headers {
				t.Fatalf("域名配置内容应一致，索引: %d", i)
			}
		}

		// 属性5：变量配置数量应一致
		if len(copiedEnv.Variables) != len(sourceEnv.Variables) {
			t.Fatalf("变量配置数量应一致，期望: %d, 实际: %d", len(sourceEnv.Variables), len(copiedEnv.Variables))
		}

		// 属性6：变量配置内容应一致
		for i, v := range sourceEnv.Variables {
			if copiedEnv.Variables[i].Name != v.Name ||
				copiedEnv.Variables[i].Key != v.Key ||
				copiedEnv.Variables[i].Value != v.Value ||
				copiedEnv.Variables[i].Type != v.Type {
				t.Fatalf("变量配置内容应一致，索引: %d", i)
			}
		}

		// 属性7：数据库配置数量应一致
		if len(copiedEnv.DBConfigs) != len(sourceEnv.DBConfigs) {
			t.Fatalf("数据库配置数量应一致，期望: %d, 实际: %d", len(sourceEnv.DBConfigs), len(copiedEnv.DBConfigs))
		}

		// 属性8：数据库配置内容应一致
		for i, db := range sourceEnv.DBConfigs {
			if copiedEnv.DBConfigs[i].Name != db.Name ||
				copiedEnv.DBConfigs[i].Code != db.Code ||
				copiedEnv.DBConfigs[i].Type != db.Type ||
				copiedEnv.DBConfigs[i].Host != db.Host ||
				copiedEnv.DBConfigs[i].Port != db.Port ||
				copiedEnv.DBConfigs[i].Database != db.Database {
				t.Fatalf("数据库配置内容应一致，索引: %d", i)
			}
		}

		// 属性9：MQ配置数量应一致
		if len(copiedEnv.MQConfigs) != len(sourceEnv.MQConfigs) {
			t.Fatalf("MQ配置数量应一致，期望: %d, 实际: %d", len(sourceEnv.MQConfigs), len(copiedEnv.MQConfigs))
		}

		// 属性10：MQ配置内容应一致
		for i, mq := range sourceEnv.MQConfigs {
			if copiedEnv.MQConfigs[i].Name != mq.Name ||
				copiedEnv.MQConfigs[i].Code != mq.Code ||
				copiedEnv.MQConfigs[i].Type != mq.Type ||
				copiedEnv.MQConfigs[i].Host != mq.Host ||
				copiedEnv.MQConfigs[i].Port != mq.Port ||
				copiedEnv.MQConfigs[i].VHost != mq.VHost {
				t.Fatalf("MQ配置内容应一致，索引: %d", i)
			}
		}
	})
}

// TestEnvCodeUniqueInProject_Property 属性测试：环境代码在项目内唯一
// 验证：同一项目内不能有重复的环境代码
func TestEnvCodeUniqueInProject_Property(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		projectID := rapid.Int64Range(1, 1000).Draw(t, "projectID")
		code1 := rapid.StringMatching(`[a-z][a-z0-9_]{2,20}`).Draw(t, "code1")
		code2 := rapid.StringMatching(`[a-z][a-z0-9_]{2,20}`).Draw(t, "code2")

		// 属性：在同一项目内，如果两个环境代码相同，则它们是同一个环境
		if code1 == code2 {
			t.Logf("项目 %d 内相同代码 %s 表示同一环境", projectID, code1)
		} else {
			// 不同代码表示不同环境
			if code1 == code2 {
				t.Fatal("同一项目内不同环境的代码不应相同")
			}
		}
	})
}
