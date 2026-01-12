# K6 与 Workflow-Engine 对比分析

## 概述

本文档对比分析 k6 和 workflow-engine 在架构设计、执行流程、指标收集等方面的异同，为后续优化提供参考。

---

## 1. 整体架构对比

### K6 架构

```
┌─────────────────────────────────────────────────────────────┐
│                      CLI (Cobra)                            │
│                    k6 run script.js                         │
└─────────────────────────────┬───────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                     Scheduler                               │
│  ┌─────────────────────────────────────────────────────┐   │
│  │              ExecutionState                          │   │
│  │  - VU 池 (channel)                                   │   │
│  │  - 原子计数器 (initializedVUs, activeVUs, iterations)│   │
│  │  - 状态机 (Created → Running → Ended)                │   │
│  │  - 暂停/恢复控制                                     │   │
│  └─────────────────────────────────────────────────────┘   │
│                                                             │
│  ┌─────────────────────────────────────────────────────┐   │
│  │              Executors (执行器)                      │   │
│  │  - ConstantVUs      - RampingVUs                    │   │
│  │  - ConstantArrivalRate  - RampingArrivalRate        │   │
│  │  - SharedIterations - PerVUIterations               │   │
│  │  - ExternallyControlled                             │   │
│  └─────────────────────────────────────────────────────┘   │
└─────────────────────────────┬───────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                    JavaScript Runner                        │
│  - Goja JS 运行时                                           │
│  - VU 实例管理                                              │
│  - HTTP/WebSocket/gRPC 模块                                 │
└─────────────────────────────┬───────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                   Metrics Pipeline                          │
│  Sample → Channel → Output Manager → Multiple Outputs       │
│                                   → Metrics Engine          │
└─────────────────────────────────────────────────────────────┘
```

### Workflow-Engine 架构

```
┌─────────────────────────────────────────────────────────────┐
│                      CLI (手动实现)                         │
│              workflow-engine run workflow.yaml              │
└─────────────────────────────┬───────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                   Master 节点                               │
│  ┌─────────────────────────────────────────────────────┐   │
│  │              ExecutionInfo                           │   │
│  │  - 执行状态管理                                      │   │
│  │  - 进度跟踪                                          │   │
│  │  - 停止/暂停/恢复控制                                │   │
│  └─────────────────────────────────────────────────────┘   │
│                                                             │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────────┐  │
│  │  Scheduler   │  │  Aggregator  │  │  SlaveRegistry   │  │
│  │  任务调度    │  │  指标聚合    │  │  Slave 管理      │  │
│  └──────────────┘  └──────────────┘  └──────────────────┘  │
└─────────────────────────────┬───────────────────────────────┘
                              │ 分发任务
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                   Slave 节点 (TaskEngine)                   │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────────┐  │
│  │   VUPool     │  │  Executors   │  │ MetricsCollector │  │
│  │   VU 池      │  │  步骤执行器  │  │   指标收集       │  │
│  └──────────────┘  └──────────────┘  └──────────────────┘  │
└─────────────────────────────────────────────────────────────┘
```

### 架构差异分析

| 维度     | K6                        | Workflow-Engine          | 分析                         |
| -------- | ------------------------- | ------------------------ | ---------------------------- |
| 部署模式 | 单机为主，k6 Cloud 分布式 | 原生 Master-Slave 分布式 | WE 更适合私有化部署          |
| 脚本语言 | JavaScript                | YAML 工作流定义          | WE 更易上手，k6 更灵活       |
| 执行单元 | JS 函数                   | 工作流步骤               | WE 面向接口测试，k6 面向脚本 |
| 扩展性   | xk6 插件机制              | Executor 注册机制        | 都支持扩展                   |

---

## 2. 执行模式对比

### K6 执行模式 (7 种)

| 模式                    | 说明                     | 适用场景             |
| ----------------------- | ------------------------ | -------------------- |
| `constant-vus`          | 固定 VU 数持续执行       | 稳定负载测试         |
| `ramping-vus`           | VU 数按阶段递增/递减     | 阶梯压测、容量测试   |
| `constant-arrival-rate` | 固定请求到达率           | 吞吐量测试           |
| `ramping-arrival-rate`  | 请求到达率按阶段变化     | 渐进式吞吐量测试     |
| `per-vu-iterations`     | 每个 VU 执行固定迭代次数 | 功能验证             |
| `shared-iterations`     | 所有 VU 共享总迭代次数   | 快速完成固定任务量   |
| `externally-controlled` | 外部 API 控制 VU 数      | 动态调整、CI/CD 集成 |

### Workflow-Engine 执行模式 (4 种)

| 模式                | 说明                     | 实现状态  |
| ------------------- | ------------------------ | --------- |
| `constant-vus`      | 固定 VU 数持续执行       | ✅ 已实现 |
| `ramping-vus`       | VU 数按阶段递增/递减     | ✅ 已实现 |
| `per-vu-iterations` | 每个 VU 执行固定迭代次数 | ✅ 已实现 |
| `shared-iterations` | 所有 VU 共享总迭代次数   | ✅ 已实现 |

### 差异分析

**缺失的模式：**

1. **constant-arrival-rate / ramping-arrival-rate**

   - k6 的到达率模式关注"每秒发起多少请求"，而非"有多少 VU 在运行"
   - 适用于测试系统在特定 QPS 下的表现
   - 建议后续实现

2. **externally-controlled**
   - 允许通过 REST API 动态调整 VU 数
   - workflow-engine 已有 `ScaleExecution` 接口，可扩展实现

---

## 3. VU 管理对比

### K6 VU 管理

```go
// ExecutionState 中的 VU 池
type ExecutionState struct {
    vus chan InitializedVU  // 使用 channel 作为 VU 池

    initializedVUs *int64   // 原子计数器
    activeVUs *int64
    fullIterationsCount *uint64
    interruptedIterationsCount *uint64
}

// VU 获取流程
func (es *ExecutionState) GetPlannedVU() (InitializedVU, error) {
    // 从 channel 获取预初始化的 VU
    select {
    case vu := <-es.vus:
        return vu, nil
    default:
        return nil, errors.New("no VU available")
    }
}

// VU 归还
func (es *ExecutionState) ReturnVU(vu InitializedVU) {
    es.vus <- vu
}
```

### Workflow-Engine VU 管理

```go
// VUPool 使用 map 管理
type VUPool struct {
    maxVUs int
    vus    map[int]*types.VirtualUser
    inUse  map[int]bool
    mu     sync.Mutex
}

// VU 获取
func (p *VUPool) Acquire(id int) *types.VirtualUser {
    p.mu.Lock()
    defer p.mu.Unlock()

    if p.inUse[id] {
        return nil
    }
    // 创建或复用 VU
    vu, exists := p.vus[id]
    if !exists {
        vu = &types.VirtualUser{ID: id, ...}
        p.vus[id] = vu
    }
    p.inUse[id] = true
    return vu
}
```

### 差异分析

| 维度      | K6                      | Workflow-Engine |
| --------- | ----------------------- | --------------- |
| 数据结构  | Channel (无锁)          | Map + Mutex     |
| 并发性能  | 更高 (channel 原生支持) | 一般 (需要加锁) |
| VU 初始化 | 预初始化到 channel      | 按需创建        |
| 计数器    | 原子操作                | 原子操作        |

**优化建议：** 可考虑将 VUPool 改为 channel 实现，减少锁竞争。

---

## 4. 指标收集对比

### K6 指标管道

```
VU 执行
    │
    ▼
Sample 创建
    │ {Metric, Tags, Time, Value, Metadata}
    ▼
Samples Channel (缓冲)
    │
    ▼
Output Manager (每 50ms 批量分发)
    │
    ├─→ Cloud Output (上传到 k6 Cloud)
    ├─→ JSON Output (写入文件)
    ├─→ CSV Output
    ├─→ InfluxDB Output
    ├─→ Prometheus Output
    └─→ Metrics Engine Ingester
            │
            ▼
        Metric Sink (聚合)
            │
            ├─ CounterSink: sum, rate
            ├─ GaugeSink: min, max, last
            ├─ RateSink: passes, fails
            └─ TrendSink: percentiles (HDR Histogram)
```

### Workflow-Engine 指标收集

```
VU 执行步骤
    │
    ▼
MetricsCollector.RecordStep()
    │
    ├─ 更新计数: count, successCount, failureCount
    ├─ 更新耗时: min, max, sum (实时聚合)
    ├─ 记录到直方图: histogram.Record() (固定内存)
    └─ 发送到输出通道: emitter.Emit*()
    │
    ▼
Output Manager
    │
    ├─→ Console Reporter
    ├─→ JSON Reporter
    ├─→ InfluxDB Reporter
    └─→ Webhook Reporter
    │
    ▼
Aggregator (多 Slave 聚合)
    │
    ▼
GenerateSummary()
```

### 差异分析

| 维度     | K6                        | Workflow-Engine         |
| -------- | ------------------------- | ----------------------- |
| 数据流   | Sample → Channel → Output | 直接聚合 + 可选 Channel |
| 批量处理 | 50ms 批量分发             | 实时处理                |
| 百分位数 | HDR Histogram (第三方库)  | 自实现直方图            |
| 内存管理 | 固定内存                  | 固定内存 (改造后)       |
| 多输出   | 原生支持                  | 支持                    |

**改造成果：** 已将原来的原始数据存储改为直方图实时聚合，解决了内存无限增长问题。

---

## 5. 阈值评估对比

### K6 阈值

```javascript
// 脚本中定义
export const options = {
  thresholds: {
    http_req_duration: ["p(95)<500", "p(99)<1000"],
    http_req_failed: ["rate<0.01"],
    checks: ["rate>0.99"],
  },
};
```

```go
// 评估逻辑
func (me *MetricsEngine) evaluateThresholds() {
    for _, m := range me.metricsWithThresholds {
        succ, _ := m.Thresholds.Run(m.Sink, duration)
        if !succ {
            m.Tainted = true
            if m.Thresholds.Abort {
                shouldAbort = true  // 立即中止测试
            }
        }
    }
}
```

**特点：**

- 每 2 秒评估一次
- 支持 `abortOnFail` 立即中止
- 测试结束后最终评估
- 影响退出码

### Workflow-Engine 阈值

```yaml
# 工作流中定义
thresholds:
  - metric: "step_1.duration.p95"
    condition: "< 500"
  - metric: "http_req_failed"
    condition: "< 0.01"
```

```go
// 评估逻辑
func (a *DefaultMetricsAggregator) EvaluateThresholds(
    metrics *types.AggregatedMetrics,
    thresholds []types.Threshold,
) ([]types.ThresholdResult, error) {
    for _, threshold := range thresholds {
        value, _ := a.getMetricValue(metrics, threshold.Metric)
        passed, _ := a.evaluateCondition(value, threshold.Condition)
        // ...
    }
}
```

**特点：**

- 测试结束后评估
- 支持步骤级别指标
- 影响退出码

### 差异分析

| 维度     | K6                          | Workflow-Engine |
| -------- | --------------------------- | --------------- |
| 评估时机 | 持续评估 (每 2s) + 最终评估 | 仅最终评估      |
| 中止支持 | 支持 abortOnFail            | 不支持          |
| 指标粒度 | 全局指标                    | 支持步骤级别    |
| 表达式   | 支持复杂表达式              | 简单比较        |

**优化建议：** 可增加持续评估和 abortOnFail 支持。

---

## 6. 汇总生成对比

### K6 汇总

```go
// Summary 结构
type Summary struct {
    TestRunDuration time.Duration
    Thresholds      map[string]ThresholdResult
    Checks          map[string]CheckResult
    Metrics         map[string]MetricResult
    Groups          map[string]Group      // 分组统计
    Scenarios       map[string]Group      // 场景统计
}

// 用户可自定义汇总输出
export function handleSummary(data) {
    return {
        'stdout': textSummary(data),
        'summary.json': JSON.stringify(data),
        'summary.html': htmlReport(data),
    };
}
```

### Workflow-Engine 汇总

```go
// ExecutionSummary 结构
type ExecutionSummary struct {
    ExecutionID      string
    TotalVUs         int
    TotalIterations  int64
    TotalRequests    int64
    Duration         string
    SuccessRate      float64
    ErrorRate        float64
    AvgDuration      string
    P95Duration      string
    P99Duration      string
    ThresholdsPassed int
    ThresholdsFailed int
}

// 生成汇总
func (a *DefaultMetricsAggregator) GenerateSummary(
    metrics *types.AggregatedMetrics,
) (*ExecutionSummary, error)
```

### 差异分析

| 维度        | K6                   | Workflow-Engine |
| ----------- | -------------------- | --------------- |
| 分组统计    | 支持 Group/Scenario  | 按步骤分组      |
| 自定义输出  | handleSummary() 回调 | 固定格式        |
| 输出格式    | 文本/JSON/HTML       | JSON            |
| Checks 统计 | 支持                 | 不支持          |

---

## 7. 分布式执行对比

### K6 分布式

- **k6 Cloud**: 官方云服务，自动分布式
- **k6 Operator**: Kubernetes 原生分布式
- **手动分布式**: 多机器运行，手动聚合结果

### Workflow-Engine 分布式

```
Master 节点
    │
    ├─ SlaveRegistry: 管理 Slave 注册/心跳
    ├─ Scheduler: 任务分段分配
    │   - 按 Segment 划分: [0, 0.5), [0.5, 1.0)
    │   - 支持手动/标签/能力/自动选择
    └─ Aggregator: 聚合多 Slave 指标
    │
    ▼
Slave 节点
    │
    ├─ TaskEngine: 执行分配的任务分段
    ├─ MetricsCollector: 收集本地指标
    └─ 心跳上报
```

### 差异分析

| 维度       | K6              | Workflow-Engine     |
| ---------- | --------------- | ------------------- |
| 分布式模式 | 云服务/Operator | 原生 Master-Slave   |
| 任务分配   | 按 VU 分配      | 按 Segment 分配     |
| Slave 选择 | 自动            | 手动/标签/能力/自动 |
| 故障恢复   | 有限支持        | Reschedule 支持     |
| 私有化部署 | 需要 Operator   | 原生支持            |

---

## 8. 功能对比总结

| 功能                  |     K6     | Workflow-Engine | 备注           |
| --------------------- | :--------: | :-------------: | -------------- |
| **执行模式**          |
| constant-vus          |     ✅     |       ✅        |                |
| ramping-vus           |     ✅     |       ✅        |                |
| per-vu-iterations     |     ✅     |       ✅        |                |
| shared-iterations     |     ✅     |       ✅        |                |
| constant-arrival-rate |     ✅     |       ❌        | 建议实现       |
| ramping-arrival-rate  |     ✅     |       ❌        | 建议实现       |
| externally-controlled |     ✅     |       ⚠️        | 有基础，可扩展 |
| **指标收集**          |
| 实时聚合              |     ✅     |       ✅        |                |
| 固定内存              |     ✅     |       ✅        | 已改造         |
| 百分位数              |     ✅     |       ✅        |                |
| 多输出支持            |     ✅     |       ✅        |                |
| **阈值评估**          |
| 基本阈值              |     ✅     |       ✅        |                |
| 持续评估              |     ✅     |       ❌        | 建议实现       |
| abortOnFail           |     ✅     |       ❌        | 建议实现       |
| **分布式**            |
| 原生分布式            |     ⚠️     |       ✅        | WE 更强        |
| Slave 选择策略        |     ❌     |       ✅        |                |
| 故障恢复              |     ⚠️     |       ✅        |                |
| **其他**              |
| CLI 框架              |   Cobra    |      手动       | 可优化         |
| 脚本语言              | JavaScript |      YAML       | 各有优势       |
| Checks                |     ✅     |       ❌        | 可实现         |
| Groups                |     ✅     |       ⚠️        | 按步骤分组     |

---

## 9. 优化建议

### 短期优化 (1-2 周)

1. **增加持续阈值评估**

   - 每 N 秒评估一次阈值
   - 支持 abortOnFail 立即中止

2. **优化 VUPool**
   - 考虑使用 channel 替代 map+mutex
   - 减少锁竞争

### 中期优化 (1-2 月)

1. **实现到达率模式**

   - constant-arrival-rate
   - ramping-arrival-rate

2. **增加 Checks 支持**

   - 类似 k6 的断言统计
   - 集成到汇总报告

3. **CLI 框架升级**
   - 迁移到 Cobra
   - 统一命令结构

### 长期优化 (3-6 月)

1. **增强汇总功能**

   - 支持自定义汇总模板
   - HTML 报告生成

2. **可观测性增强**

   - OpenTelemetry 集成
   - 实时指标推送

3. **场景编排**
   - 多工作流串联
   - 条件分支执行

---

## 10. 结论

Workflow-Engine 在核心功能上已经具备了与 k6 相当的能力，特别是在分布式执行方面有独特优势。主要差距在于：

1. **执行模式**: 缺少到达率模式
2. **阈值评估**: 缺少持续评估和 abortOnFail
3. **汇总功能**: 相对简单

这些差距可以通过后续迭代逐步弥补。当前架构设计合理，扩展性良好，为后续优化奠定了基础。
