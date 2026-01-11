package output

import (
	"sync"
	"time"

	"yqhp/workflow-engine/pkg/metrics"
)

const (
	// sendBatchToOutputsRate 批量发送到输出的间隔
	sendBatchToOutputsRate = 50 * time.Millisecond
	// defaultSamplesChannelSize 默认样本通道大小
	defaultSamplesChannelSize = 1000
)

// Manager 管理多个输出插件
type Manager struct {
	outputs []Output
	logger  Logger
	mu      sync.RWMutex
}

// NewManager 创建新的输出管理器
func NewManager(outputs []Output, logger Logger) *Manager {
	return &Manager{
		outputs: outputs,
		logger:  logger,
	}
}

// Start 启动所有输出并开始分发指标
// 返回 wait 函数用于等待分发完成，finish 函数用于停止并清理
func (m *Manager) Start(samplesChan chan metrics.SampleContainer) (wait func(), finish func(error), err error) {
	// 启动所有输出
	if err := m.startOutputs(); err != nil {
		return nil, nil, err
	}

	wg := &sync.WaitGroup{}
	wg.Add(1)

	// 发送到所有输出的函数
	sendToOutputs := func(sampleContainers []metrics.SampleContainer) {
		m.mu.RLock()
		defer m.mu.RUnlock()
		for _, out := range m.outputs {
			out.AddMetricSamples(sampleContainers)
		}
	}

	// 启动分发协程
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(sendBatchToOutputsRate)
		defer ticker.Stop()

		buffer := make([]metrics.SampleContainer, 0, cap(samplesChan))
		for {
			select {
			case sampleContainer, ok := <-samplesChan:
				if !ok {
					// 通道关闭，发送剩余的样本
					if len(buffer) > 0 {
						sendToOutputs(buffer)
					}
					return
				}
				buffer = append(buffer, sampleContainer)
			case <-ticker.C:
				if len(buffer) > 0 {
					sendToOutputs(buffer)
					buffer = make([]metrics.SampleContainer, 0, cap(buffer))
				}
			}
		}
	}()

	wait = wg.Wait
	finish = func(testErr error) {
		wait()
		m.stopOutputs(testErr)
	}

	return wait, finish, nil
}

// startOutputs 启动所有输出
func (m *Manager) startOutputs() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i, out := range m.outputs {
		if err := out.Start(); err != nil {
			// 停止已启动的输出
			for j := 0; j < i; j++ {
				_ = m.outputs[j].Stop()
			}
			return err
		}
		if m.logger != nil {
			m.logger.Debug("输出 %s 已启动", out.Description())
		}
	}
	return nil
}

// stopOutputs 停止所有输出
func (m *Manager) stopOutputs(testErr error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 设置运行状态
	status := RunStatus{
		Status: "completed",
	}
	if testErr != nil {
		status.Status = "failed"
		status.Error = testErr
	}

	for _, out := range m.outputs {
		out.SetRunStatus(status)
		if err := out.Stop(); err != nil && m.logger != nil {
			m.logger.Error("停止输出 %s 失败: %v", out.Description(), err)
		}
	}
}

// AddOutput 添加输出
func (m *Manager) AddOutput(out Output) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.outputs = append(m.outputs, out)
}

// GetOutputs 获取所有输出
func (m *Manager) GetOutputs() []Output {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]Output, len(m.outputs))
	copy(result, m.outputs)
	return result
}

// NewSamplesChannel 创建新的样本通道
func NewSamplesChannel(size int) chan metrics.SampleContainer {
	if size <= 0 {
		size = defaultSamplesChannelSize
	}
	return make(chan metrics.SampleContainer, size)
}
