# Workflow Engine 示例

本目录包含各种工作流示例，帮助你快速上手 Workflow Engine。

## 示例列表

### 基础示例

| 文件                                               | 说明                               |
| -------------------------------------------------- | ---------------------------------- |
| [01-simple-http.yaml](01-simple-http.yaml)         | 最简单的 HTTP GET 请求             |
| [02-http-post.yaml](02-http-post.yaml)             | HTTP POST 请求，包含请求头和请求体 |
| [03-http-with-query.yaml](03-http-with-query.yaml) | HTTP 请求带查询参数                |
| [04-multi-step.yaml](04-multi-step.yaml)           | 多步骤工作流 (GET/POST/PUT/DELETE) |

### 高级功能

| 文件                                             | 说明                                     |
| ------------------------------------------------ | ---------------------------------------- |
| [05-with-variables.yaml](05-with-variables.yaml) | 使用变量的工作流                         |
| [06-with-hooks.yaml](06-with-hooks.yaml)         | 使用前置和后置钩子                       |
| [07-conditional.yaml](07-conditional.yaml)       | 条件分支执行                             |
| [08-error-handling.yaml](08-error-handling.yaml) | 错误处理策略 (abort/continue/skip/retry) |

### 性能测试

| 文件                                                                           | 说明                    |
| ------------------------------------------------------------------------------ | ----------------------- |
| [09-performance-constant-vus.yaml](09-performance-constant-vus.yaml)           | 恒定 VU 模式            |
| [10-performance-ramping-vus.yaml](10-performance-ramping-vus.yaml)             | 阶梯负载模式            |
| [11-performance-iterations.yaml](11-performance-iterations.yaml)               | 共享迭代模式            |
| [12-performance-per-vu-iterations.yaml](12-performance-per-vu-iterations.yaml) | 每 VU 固定迭代模式      |
| [13-spike-test.yaml](13-spike-test.yaml)                                       | 峰值测试                |
| [14-stress-test.yaml](14-stress-test.yaml)                                     | 压力测试                |
| [15-soak-test.yaml](15-soak-test.yaml)                                         | 浸泡测试 (长时间稳定性) |

### 分布式执行

| 文件                                                                   | 说明                       |
| ---------------------------------------------------------------------- | -------------------------- |
| [12-distributed-target-slaves.yaml](12-distributed-target-slaves.yaml) | 分布式执行指定目标机器示例 |
| [16-distributed.yaml](16-distributed.yaml)                             | 分布式执行示例             |

### HTTP 引擎

| 文件                                                             | 说明                               |
| ---------------------------------------------------------------- | ---------------------------------- |
| [18-http-engine-comparison.yaml](18-http-engine-comparison.yaml) | HTTP 引擎对比 (FastHTTP vs 标准库) |

### 完整示例

| 文件                                                 | 说明                   |
| ---------------------------------------------------- | ---------------------- |
| [17-complete-example.yaml](17-complete-example.yaml) | 包含所有功能的完整示例 |

## 运行示例

### 独立模式运行

```bash
# 进入项目目录
cd yqhp/workflow-engine

# 构建
go build -o workflow-engine ./cmd/main.go

# 运行简单示例
./workflow-engine run examples/01-simple-http.yaml

# 运行性能测试
./workflow-engine run examples/09-performance-constant-vus.yaml

# 覆盖参数运行
./workflow-engine run -vus 20 -duration 2m examples/09-performance-constant-vus.yaml

# 输出 JSON 结果
./workflow-engine run -out-json results.json examples/01-simple-http.yaml
```

### 分布式模式运行

```bash
# 终端 1: 启动 Master
./workflow-engine master start

# 终端 2: 启动 Slave
./workflow-engine slave start --master localhost:9090 --labels region=us-east

# 终端 3: 通过 API 提交工作流
curl -X POST http://localhost:8080/api/v1/workflows \
  -H "Content-Type: application/json" \
  -d '{
    "yaml": "'"$(cat examples/16-distributed.yaml)"'"
  }'
```

## 执行模式说明

| 模式                    | 说明                     |
| ----------------------- | ------------------------ |
| `constant-vus`          | 固定数量的 VU 持续运行   |
| `ramping-vus`           | 按阶段调整 VU 数量       |
| `shared-iterations`     | 所有 VU 共享总迭代次数   |
| `per-vu-iterations`     | 每个 VU 执行固定迭代次数 |
| `constant-arrival-rate` | 固定请求速率             |
| `ramping-arrival-rate`  | 按阶段调整请求速率       |

## 错误处理策略

| 策略       | 说明                          |
| ---------- | ----------------------------- |
| `abort`    | 遇到错误停止整个工作流 (默认) |
| `continue` | 遇到错误继续执行下一步        |
| `skip`     | 跳过当前步骤                  |
| `retry`    | 重试当前步骤                  |

## 性能阈值

常用阈值配置：

```yaml
thresholds:
  # 响应时间阈值
  - metric: http_req_duration
    condition: "p(95) < 500" # 95% 请求 < 500ms
  - metric: http_req_duration
    condition: "p(99) < 1000" # 99% 请求 < 1000ms
  - metric: http_req_duration
    condition: "avg < 200" # 平均响应时间 < 200ms

  # 错误率阈值
  - metric: http_req_failed
    condition: "rate < 0.01" # 错误率 < 1%
```

## HTTP 引擎

工作流引擎支持两种 HTTP 引擎实现：

| 引擎       | 说明                                     |
| ---------- | ---------------------------------------- |
| `fasthttp` | 默认引擎，基于 FastHTTP 库，性能更高     |
| `standard` | 标准库引擎，基于 Go net/http，兼容性更好 |

### 切换引擎

在工作流的 `options` 中指定 `http_engine`：

```yaml
options:
  vus: 10
  duration: 30s
  http_engine: standard # 使用标准库引擎
```

### 引擎对比

FastHTTP 引擎优势：

- 连接池复用，减少 TCP/TLS 握手开销
- 对象池复用，降低 GC 压力
- 更高吞吐量，适合高并发压测

标准库引擎优势：

- 兼容性更好
- 支持更多 HTTP 特性
- 适合调试场景
