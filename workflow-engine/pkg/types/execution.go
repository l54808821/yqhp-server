package types

import "time"

// ExecutionOptions defines load testing parameters.
type ExecutionOptions struct {
	VUs           int               `yaml:"vus,omitempty"`
	Duration      time.Duration     `yaml:"duration,omitempty"`
	Iterations    int               `yaml:"iterations,omitempty"`
	RampUp        *RampConfig       `yaml:"ramp_up,omitempty"`
	Thresholds    []Threshold       `yaml:"thresholds,omitempty"`
	SlaveType     string            `yaml:"slave_type,omitempty"`
	TargetSlaves  *SlaveSelector    `yaml:"target_slaves,omitempty"`
	ExecutionMode ExecutionMode     `yaml:"mode,omitempty"`
	Stages        []Stage           `yaml:"stages,omitempty"`
	HTTPEngine    HTTPEngineType    `yaml:"http_engine,omitempty"`    // HTTP 引擎类型：fasthttp（默认）或 standard
	SamplingMode  SamplingMode      `yaml:"sampling_mode,omitempty"` // 采样模式：none/errors/smart
	Outputs       []OutputConfig    `yaml:"outputs,omitempty"`       // 输出配置列表
	Tags          map[string]string `yaml:"tags,omitempty"`          // 全局标签
}

// SamplingMode 定义采样模式
type SamplingMode string

const (
	SamplingModeNone   SamplingMode = "none"
	SamplingModeErrors SamplingMode = "errors"
	SamplingModeSmart  SamplingMode = "smart"
)

// OutputConfig 定义输出配置
type OutputConfig struct {
	// Type 输出类型: json, influxdb, kafka, console, csv
	Type string `yaml:"type"`
	// URL 输出目标地址或文件路径
	URL string `yaml:"url,omitempty"`
	// Options 额外配置选项
	Options map[string]string `yaml:"options,omitempty"`
}

// HTTPEngineType 定义 HTTP 引擎类型
type HTTPEngineType string

const (
	// HTTPEngineFastHTTP 使用 FastHTTP 库（默认，性能更好）
	HTTPEngineFastHTTP HTTPEngineType = "fasthttp"
	// HTTPEngineStandard 使用 Go 标准库 net/http
	HTTPEngineStandard HTTPEngineType = "standard"
)

// ExecutionMode defines the execution mode.
type ExecutionMode string

const (
	// ModeConstantVUs maintains a fixed number of VUs.
	ModeConstantVUs ExecutionMode = "constant-vus"
	// ModeRampingVUs adjusts VU count according to stages.
	ModeRampingVUs ExecutionMode = "ramping-vus"
	// ModeConstantArrivalRate maintains a fixed request rate.
	ModeConstantArrivalRate ExecutionMode = "constant-arrival-rate"
	// ModeRampingArrivalRate adjusts request rate according to stages.
	ModeRampingArrivalRate ExecutionMode = "ramping-arrival-rate"
	// ModePerVUIterations has each VU execute a fixed number of iterations.
	ModePerVUIterations ExecutionMode = "per-vu-iterations"
	// ModeSharedIterations distributes total iterations across all VUs.
	ModeSharedIterations ExecutionMode = "shared-iterations"
	// ModeExternally allows runtime control via API.
	ModeExternally ExecutionMode = "externally-controlled"
)

// Stage defines an execution stage.
type Stage struct {
	Duration time.Duration `yaml:"duration" json:"duration"`
	Target   int           `yaml:"target" json:"target"`
	Name     string        `yaml:"name,omitempty" json:"name,omitempty"`
}

// RampConfig defines ramping configuration.
type RampConfig struct {
	Stages       []Stage       `yaml:"stages"`
	StartVUs     int           `yaml:"start_vus,omitempty"`
	GracefulStop time.Duration `yaml:"graceful_stop,omitempty"`
	GracefulRamp time.Duration `yaml:"graceful_ramp,omitempty"`
}

// ArrivalRateConfig defines arrival rate configuration.
type ArrivalRateConfig struct {
	Rate            int           `yaml:"rate"`              // Requests per time unit
	TimeUnit        time.Duration `yaml:"time_unit"`         // Time unit (1s, 1m)
	PreAllocatedVUs int           `yaml:"pre_allocated_vus"` // Pre-allocated VU count
	MaxVUs          int           `yaml:"max_vus"`           // Maximum VU count
}

// Threshold defines a performance threshold.
type Threshold struct {
	Metric    string `yaml:"metric"`
	Condition string `yaml:"condition"`
}

// ThresholdResult contains the result of threshold evaluation.
type ThresholdResult struct {
	Metric    string
	Condition string
	Passed    bool
	Value     float64
}
