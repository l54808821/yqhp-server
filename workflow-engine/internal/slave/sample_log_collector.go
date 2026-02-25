package slave

import (
	"encoding/json"
	"math/rand"
	"sync"
	"time"

	"yqhp/workflow-engine/pkg/types"
)

const defaultMaxSampleLogs = 1000

// SampleLogEntry 采样日志条目
type SampleLogEntry struct {
	ExecutionID     string            `json:"execution_id"`
	StepID          string            `json:"step_id"`
	StepName        string            `json:"step_name"`
	Timestamp       time.Time         `json:"timestamp"`
	Status          string            `json:"status"`
	DurationMs      int64             `json:"duration_ms"`
	RequestMethod   string            `json:"request_method"`
	RequestURL      string            `json:"request_url"`
	RequestHeaders  map[string]string `json:"request_headers,omitempty"`
	RequestBody     string            `json:"request_body,omitempty"`
	ResponseStatus  int               `json:"response_status"`
	ResponseHeaders map[string]string `json:"response_headers,omitempty"`
	ResponseBody    string            `json:"response_body,omitempty"`
	ErrorMessage    string            `json:"error_message,omitempty"`
}

// SampleLogCollector 采样日志收集器
// 根据采样模式决定是否记录每个请求的详细信息
type SampleLogCollector struct {
	mode        types.SamplingMode
	executionID string
	logs        []*SampleLogEntry
	maxLogs     int
	totalCount  int64
	mu          sync.Mutex
	flushFunc   func([]*SampleLogEntry)
	rng         *rand.Rand
}

// NewSampleLogCollector 创建采样日志收集器
func NewSampleLogCollector(executionID string, mode types.SamplingMode, flushFunc func([]*SampleLogEntry)) *SampleLogCollector {
	if mode == "" {
		mode = types.SamplingModeNone
	}
	return &SampleLogCollector{
		mode:        mode,
		executionID: executionID,
		logs:        make([]*SampleLogEntry, 0, 128),
		maxLogs:     defaultMaxSampleLogs,
		flushFunc:   flushFunc,
		rng:         rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// ShouldSample 根据采样模式判断是否需要采样当前请求
func (c *SampleLogCollector) ShouldSample(result *types.StepResult) bool {
	if c.mode == types.SamplingModeNone {
		return false
	}

	isFailed := result.Status == types.ResultStatusFailed || result.Status == types.ResultStatusTimeout

	switch c.mode {
	case types.SamplingModeErrors:
		return isFailed
	case types.SamplingModeSmart:
		if isFailed {
			return true
		}
		// 蓄水池采样：成功请求按概率采样
		c.mu.Lock()
		currentCount := len(c.logs)
		c.mu.Unlock()
		if currentCount < c.maxLogs {
			return true
		}
		// 超过上限后，以递减概率替换已有样本
		return c.rng.Intn(int(c.totalCount)+1) < c.maxLogs
	default:
		return false
	}
}

// Record 记录采样日志
func (c *SampleLogCollector) Record(stepID, stepName string, result *types.StepResult) {
	if result == nil {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	c.totalCount++

	entry := c.buildEntry(stepID, stepName, result)

	if len(c.logs) < c.maxLogs {
		c.logs = append(c.logs, entry)
	} else if c.mode == types.SamplingModeSmart && result.Status == types.ResultStatusSuccess {
		// 蓄水池采样替换
		idx := c.rng.Intn(len(c.logs))
		// 优先替换成功的日志，保留错误日志
		if c.logs[idx].Status == "success" {
			c.logs[idx] = entry
		}
	} else {
		// 错误日志优先追加（替换最旧的成功日志）
		for i, log := range c.logs {
			if log.Status == "success" {
				c.logs[i] = entry
				return
			}
		}
	}
}

func (c *SampleLogCollector) buildEntry(stepID, stepName string, result *types.StepResult) *SampleLogEntry {
	entry := &SampleLogEntry{
		ExecutionID: c.executionID,
		StepID:      stepID,
		StepName:    stepName,
		Timestamp:   result.StartTime,
		Status:      string(result.Status),
		DurationMs:  result.Duration.Milliseconds(),
	}

	if result.Error != nil {
		entry.ErrorMessage = result.Error.Error()
	}

	if httpData, ok := result.Output.(*types.HTTPResponseData); ok && httpData != nil {
		entry.ResponseStatus = httpData.StatusCode
		entry.ResponseHeaders = httpData.Headers
		entry.ResponseBody = truncateString(httpData.Body, 4096)

		if httpData.ActualRequest != nil {
			entry.RequestMethod = httpData.ActualRequest.Method
			entry.RequestURL = httpData.ActualRequest.URL
			entry.RequestHeaders = httpData.ActualRequest.Headers
			entry.RequestBody = truncateString(httpData.ActualRequest.Body, 4096)
		}
	}

	return entry
}

// Flush 将内存中的采样日志输出到外部存储
func (c *SampleLogCollector) Flush() {
	c.mu.Lock()
	logs := make([]*SampleLogEntry, len(c.logs))
	copy(logs, c.logs)
	c.mu.Unlock()

	if len(logs) > 0 && c.flushFunc != nil {
		c.flushFunc(logs)
	}
}

// GetLogs 获取当前内存中的采样日志（用于调试/实时查看）
func (c *SampleLogCollector) GetLogs() []*SampleLogEntry {
	c.mu.Lock()
	defer c.mu.Unlock()
	logs := make([]*SampleLogEntry, len(c.logs))
	copy(logs, c.logs)
	return logs
}

// Count 获取当前采样日志数量
func (c *SampleLogCollector) Count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.logs)
}

// Mode 获取采样模式
func (c *SampleLogCollector) Mode() types.SamplingMode {
	return c.mode
}

// MarshalLogs 将日志序列化为 JSON
func (c *SampleLogCollector) MarshalLogs() ([]byte, error) {
	logs := c.GetLogs()
	return json.Marshal(logs)
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}
