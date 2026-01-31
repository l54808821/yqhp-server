package model

// 配置类型常量
const (
	ConfigTypeDomain   = "domain"
	ConfigTypeVariable = "variable"
	ConfigTypeDatabase = "database"
	ConfigTypeMQ       = "mq"
)

// VariableExtra 变量类型的额外属性
type VariableExtra struct {
	VarType     string `json:"var_type"`     // 变量类型: string/number/boolean/json
	IsSensitive bool   `json:"is_sensitive"` // 是否敏感
}

// DatabaseExtra 数据库类型的额外属性
type DatabaseExtra struct {
	DBType string `json:"db_type"` // 数据库类型: mysql/redis/mongodb
}

// MQExtra MQ类型的额外属性
type MQExtra struct {
	MQType string `json:"mq_type"` // MQ类型: rabbitmq/kafka/rocketmq
}

// DomainValue 域名配置值
type DomainValue struct {
	BaseURL string       `json:"base_url"` // 基础URL
	Headers []HeaderItem `json:"headers"`  // 请求头
}

// HeaderItem 请求头项
type HeaderItem struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// VariableValue 变量配置值
type VariableValue struct {
	Value string `json:"value"` // 变量值
}

// DatabaseValue 数据库配置值
type DatabaseValue struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Database string `json:"database"`
	Username string `json:"username"`
	Password string `json:"password"` // 加密存储
	Options  string `json:"options"`  // 额外配置
}

// MQValue MQ配置值
type MQValue struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"` // 加密存储
	VHost    string `json:"vhost"`    // RabbitMQ vhost
	Options  string `json:"options"`  // 额外配置
}
