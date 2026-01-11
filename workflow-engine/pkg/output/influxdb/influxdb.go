package influxdb

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"yqhp/workflow-engine/pkg/metrics"
	"yqhp/workflow-engine/pkg/output"
)

func init() {
	output.Register("influxdb", New)
}

// Config InfluxDB 配置
type Config struct {
	// URL InfluxDB 地址，格式: http://host:port/write?db=dbname
	URL string
	// Token 认证令牌（InfluxDB 2.x）
	Token string
	// Organization 组织（InfluxDB 2.x）
	Organization string
	// Bucket 存储桶（InfluxDB 2.x）
	Bucket string
	// Database 数据库名（InfluxDB 1.x）
	Database string
	// Precision 时间精度
	Precision string
	// PushInterval 推送间隔
	PushInterval time.Duration
	// BatchSize 批量大小
	BatchSize int
	// Tags 全局标签
	Tags map[string]string
}

// Output InfluxDB 输出
type Output struct {
	params    output.Params
	config    Config
	client    *http.Client
	buffer    *bytes.Buffer
	count     int
	mu        sync.Mutex
	runStatus output.RunStatus
	stopCh    chan struct{}
	wg        sync.WaitGroup
}

// New 创建 InfluxDB 输出
func New(params output.Params) (output.Output, error) {
	config, err := parseConfig(params.ConfigArgument)
	if err != nil {
		return nil, err
	}

	// 合并全局标签
	if params.Tags != nil {
		if config.Tags == nil {
			config.Tags = make(map[string]string)
		}
		for k, v := range params.Tags {
			config.Tags[k] = v
		}
	}

	return &Output{
		params: params,
		config: config,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
		buffer: &bytes.Buffer{},
		stopCh: make(chan struct{}),
	}, nil
}

// parseConfig 解析配置字符串
// 格式: influxdb=http://host:port/write?db=dbname 或
//
//	influxdb=http://host:port?token=xxx&org=xxx&bucket=xxx
func parseConfig(arg string) (Config, error) {
	config := Config{
		Precision:    "ms",
		PushInterval: time.Second,
		BatchSize:    1000,
	}

	if arg == "" {
		return config, fmt.Errorf("InfluxDB URL 不能为空")
	}

	u, err := url.Parse(arg)
	if err != nil {
		return config, fmt.Errorf("解析 InfluxDB URL 失败: %w", err)
	}

	// 构建基础 URL
	config.URL = fmt.Sprintf("%s://%s", u.Scheme, u.Host)

	// 解析查询参数
	q := u.Query()
	if db := q.Get("db"); db != "" {
		config.Database = db
	}
	if token := q.Get("token"); token != "" {
		config.Token = token
	}
	if org := q.Get("org"); org != "" {
		config.Organization = org
	}
	if bucket := q.Get("bucket"); bucket != "" {
		config.Bucket = bucket
	}

	return config, nil
}

// Description 返回描述
func (o *Output) Description() string {
	return fmt.Sprintf("influxdb (%s)", o.config.URL)
}

// Start 启动输出
func (o *Output) Start() error {
	// 启动定期推送协程
	o.wg.Add(1)
	go o.pushLoop()
	return nil
}

// Stop 停止输出
func (o *Output) Stop() error {
	close(o.stopCh)
	o.wg.Wait()

	// 推送剩余数据
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.buffer.Len() > 0 {
		return o.flush()
	}
	return nil
}

// pushLoop 定期推送数据
func (o *Output) pushLoop() {
	defer o.wg.Done()
	ticker := time.NewTicker(o.config.PushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-o.stopCh:
			return
		case <-ticker.C:
			o.mu.Lock()
			if o.buffer.Len() > 0 {
				if err := o.flush(); err != nil && o.params.Logger != nil {
					o.params.Logger.Error("推送到 InfluxDB 失败: %v", err)
				}
			}
			o.mu.Unlock()
		}
	}
}

// AddMetricSamples 添加指标样本
func (o *Output) AddMetricSamples(containers []metrics.SampleContainer) {
	o.mu.Lock()
	defer o.mu.Unlock()

	for _, container := range containers {
		for _, sample := range container.GetSamples() {
			o.writeSample(sample)
			o.count++

			// 达到批量大小时推送
			if o.count >= o.config.BatchSize {
				if err := o.flush(); err != nil && o.params.Logger != nil {
					o.params.Logger.Error("推送到 InfluxDB 失败: %v", err)
				}
			}
		}
	}
}

// writeSample 将样本写入缓冲区（Line Protocol 格式）
func (o *Output) writeSample(sample metrics.Sample) {
	// 构建 measurement
	measurement := sample.Metric.Name

	// 构建 tags
	var tags []string
	// 添加全局标签
	for k, v := range o.config.Tags {
		tags = append(tags, fmt.Sprintf("%s=%s", escapeTag(k), escapeTag(v)))
	}
	// 添加样本标签
	for k, v := range sample.Tags {
		tags = append(tags, fmt.Sprintf("%s=%s", escapeTag(k), escapeTag(v)))
	}

	// 构建 line
	var line string
	if len(tags) > 0 {
		line = fmt.Sprintf("%s,%s value=%f %d\n",
			measurement,
			strings.Join(tags, ","),
			sample.Value,
			sample.Time.UnixMilli())
	} else {
		line = fmt.Sprintf("%s value=%f %d\n",
			measurement,
			sample.Value,
			sample.Time.UnixMilli())
	}

	o.buffer.WriteString(line)
}

// flush 推送缓冲区数据到 InfluxDB
func (o *Output) flush() error {
	if o.buffer.Len() == 0 {
		return nil
	}

	data := o.buffer.Bytes()
	o.buffer.Reset()
	o.count = 0

	// 构建请求 URL
	var reqURL string
	if o.config.Token != "" {
		// InfluxDB 2.x
		reqURL = fmt.Sprintf("%s/api/v2/write?org=%s&bucket=%s&precision=%s",
			o.config.URL, o.config.Organization, o.config.Bucket, o.config.Precision)
	} else {
		// InfluxDB 1.x
		reqURL = fmt.Sprintf("%s/write?db=%s&precision=%s",
			o.config.URL, o.config.Database, o.config.Precision)
	}

	req, err := http.NewRequest("POST", reqURL, bytes.NewReader(data))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "text/plain")
	if o.config.Token != "" {
		req.Header.Set("Authorization", "Token "+o.config.Token)
	}

	resp, err := o.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("InfluxDB 返回错误 %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// SetRunStatus 设置运行状态
func (o *Output) SetRunStatus(status output.RunStatus) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.runStatus = status
}

// escapeTag 转义标签中的特殊字符
func escapeTag(s string) string {
	s = strings.ReplaceAll(s, ",", "\\,")
	s = strings.ReplaceAll(s, "=", "\\=")
	s = strings.ReplaceAll(s, " ", "\\ ")
	return s
}
