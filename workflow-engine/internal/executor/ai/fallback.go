package ai

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"yqhp/workflow-engine/pkg/logger"
)

// FallbackChain 管理多个模型候选的降级链
type FallbackChain struct {
	cooldowns *CooldownTracker
}

// FallbackCandidate 一个候选模型配置
type FallbackCandidate struct {
	Provider string
	Model    string
	APIKey   string
	BaseURL  string
}

// NewFallbackChain 创建降级链
func NewFallbackChain() *FallbackChain {
	return &FallbackChain{
		cooldowns: NewCooldownTracker(60 * time.Second),
	}
}

// Execute 按顺序尝试候选模型，直到成功或全部失败
func (fc *FallbackChain) Execute(
	ctx context.Context,
	primary model.ToolCallingChatModel,
	primaryConfig *AIConfig,
	fallbacks []FallbackModelConfig,
	messages []*schema.Message,
	tools []*schema.ToolInfo,
) (*schema.Message, error) {
	logger.Debug("[FallbackChain] 开始执行, 主模型=%s, 降级模型数=%d, messages数=%d, tools数=%d",
		primaryConfig.Model, len(fallbacks), len(messages), len(tools))

	// 先尝试主模型
	if !fc.cooldowns.InCooldown(primaryConfig.Model) {
		logger.Debug("[FallbackChain] 尝试主模型 %s", primaryConfig.Model)
		resp, err := fc.callWithTools(ctx, primary, messages, tools)
		if err == nil {
			fc.cooldowns.Reset(primaryConfig.Model)
			logger.Debug("[FallbackChain] 主模型 %s 调用成功, content长度=%d", primaryConfig.Model, len([]rune(resp.Content)))
			return resp, nil
		}

		errClass := classifyLLMError(err)
		if !isRetriable(errClass) {
			logger.Debug("[FallbackChain] 主模型 %s 不可重试错误: %v", primaryConfig.Model, err)
			return nil, err
		}

		logger.Warn("[FallbackChain] 主模型 %s 失败 (%v)，尝试降级", primaryConfig.Model, err)
		fc.cooldowns.RecordFailure(primaryConfig.Model)
	} else {
		logger.Debug("[FallbackChain] 主模型 %s 在冷却期内, 跳过", primaryConfig.Model)
	}

	// 尝试降级模型
	for i, fb := range fallbacks {
		if fc.cooldowns.InCooldown(fb.Model) {
			logger.Debug("[FallbackChain] 降级模型[%d] %s 在冷却期内, 跳过", i, fb.Model)
			continue
		}

		fbConfig := &AIConfig{
			Provider: fb.Provider,
			Model:    fb.Model,
			APIKey:   fb.APIKey,
			BaseURL:  fb.BaseURL,
		}

		logger.Debug("[FallbackChain] 尝试降级模型[%d] %s (provider=%s)", i, fb.Model, fb.Provider)
		fbModel, err := createChatModelFromConfig(ctx, fbConfig)
		if err != nil {
			logger.Warn("[FallbackChain] 创建降级模型 %s 失败: %v", fb.Model, err)
			fc.cooldowns.RecordFailure(fb.Model)
			continue
		}

		resp, err := fc.callWithTools(ctx, fbModel, messages, tools)
		if err == nil {
			fc.cooldowns.Reset(fb.Model)
			logger.Debug("[FallbackChain] 降级到模型 %s 成功, content长度=%d", fb.Model, len([]rune(resp.Content)))
			return resp, nil
		}

		logger.Warn("[FallbackChain] 降级模型 %s 失败: %v", fb.Model, err)
		fc.cooldowns.RecordFailure(fb.Model)
	}

	logger.Warn("[FallbackChain] 所有模型均失败 (主模型 + %d 个降级模型)", len(fallbacks))
	return nil, fmt.Errorf("所有模型（主模型 + %d 个降级模型）均失败", len(fallbacks))
}

func (fc *FallbackChain) callWithTools(ctx context.Context, chatModel model.ToolCallingChatModel, messages []*schema.Message, tools []*schema.ToolInfo) (*schema.Message, error) {
	if len(tools) > 0 {
		return chatModel.Generate(ctx, messages, model.WithTools(tools))
	}
	return chatModel.Generate(ctx, messages)
}

func isRetriable(errClass llmErrorClass) bool {
	switch errClass {
	case llmErrorTimeout, llmErrorRateLimit, llmErrorContextWindow:
		return true
	default:
		return false
	}
}

// CooldownTracker 跟踪模型的冷却状态，避免短时间内重复尝试失败的模型
type CooldownTracker struct {
	mu       sync.RWMutex
	failures map[string]time.Time
	duration time.Duration
}

// NewCooldownTracker 创建冷却追踪器
func NewCooldownTracker(duration time.Duration) *CooldownTracker {
	return &CooldownTracker{
		failures: make(map[string]time.Time),
		duration: duration,
	}
}

// InCooldown 检查模型是否在冷却期内
func (ct *CooldownTracker) InCooldown(model string) bool {
	ct.mu.RLock()
	defer ct.mu.RUnlock()
	failTime, ok := ct.failures[model]
	if !ok {
		return false
	}
	return time.Since(failTime) < ct.duration
}

// RecordFailure 记录模型失败
func (ct *CooldownTracker) RecordFailure(model string) {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	ct.failures[model] = time.Now()
}

// Reset 重置模型的冷却状态
func (ct *CooldownTracker) Reset(model string) {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	delete(ct.failures, model)
}
