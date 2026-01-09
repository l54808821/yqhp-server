# Workflow Engine 用户手册

## 目录

1. [概述](#概述)
2. [安装与构建](#安装与构建)
3. [CLI 命令参考](#cli-命令参考)
4. [工作流定义](#工作流定义)
5. [步骤类型](#步骤类型)
6. [执行选项](#执行选项)
7. [配置管理](#配置管理)
8. [分布式执行](#分布式执行)
9. [REST API 使用](#rest-api-使用)

---

## 概述

Workflow Engine 是一个高性能的分布式工作流执行引擎，专为 API 测试和负载测试设计。

### 核心概念

- **工作流 (Workflow)**: 一组有序步骤的集合，定义在 YAML 文件中
- **步骤 (Step)**: 工作流中的单个执行单元
- **执行器 (Executor)**: 负责执行特定类型步骤的组件
- **VU (Virtual User)**: 虚拟用户，用于模拟并发负载
- **Master**: 主节点，负责调度和管理
- **Slave**: 从节点，负责实际执行任务

---

## 安装与构建

### 构建

```bash
cd yqhp/workflow-engine
go build -o workflow-engine ./cmd/main.go
```

### 验证安装

```bash
./workflow-engine version
# 输出: workflow-engine version 0.1.0

./workflow-engine help
```

---

## CLI 命令参考

### 命令结构

```
workflow-engine <command> [options]

命令:
  master    管理 Master 节点
  slave     管理 Slave 节点
  run       独立模式执行工作流
  version   显示版本信息
  help      显示帮助信息
```

### run 命令

独立模式执行工作流，无需启动 Master/Slave。

```bash
workflow-engine run [options] <workflow.yaml>
```

#### 选项

| 选项          | 类型     | 默认值     | 说明                      |
| ------------- | -------- | ---------- | ------------------------- |
| `-vus`        | int      | 工作流配置 | 虚拟用户数                |
| `-duration`   | duration | 工作流配置 | 测试持续时间 (如 30s, 5m) |
| `-iterations` | int      | 工作流配置 | 迭代次数                  |
| `-mode`       | string   | 工作流配置 | 执行模式                  |
| `-quiet`      | bool     | false      | 静默模式                  |
| `-out-json`   | string   | -          | 输出 JSON 结果到文件      |
| `-help`       | bool     | false      | 显示帮助                  |

#### 示例

```bash
# 基本执行
./workflow-engine run workflow.yaml

# 指定 VUs 和持续时间
./workflow-engine run -vus 10 -duration 30s workflow.yaml

# 指定迭代次数
./workflow-engine run -iterations 100 workflow.yaml

# 静默模式并输出 JSON
./workflow-engine run -quiet -out-json results.json workflow.yaml

# 指定执行模式
./workflow-engine run -mode shared-iterations -iterations 100 workflow.yaml
```

### master 命令

管理 Master 节点。

```bash
workflow-engine master <subcommand> [options]

子命令:
  start     启动 Master 节点
  status    查看 Master 状态
```

#### master start

启动 Master 节点。

```bash
workflow-engine master start [options]
```

| 选项                 | 类型     | 默认值 | 说明                  |
| -------------------- | -------- | ------ | --------------------- |
| `-config`            | string   | -      | 配置文件路径          |
| `-address`           | string   | :8080  | HTTP 服务地址         |
| `-grpc-address`      | string   | :9090  | gRPC 服务地址         |
| `-standalone`        | bool     | false  | 独立模式 (无需 Slave) |
| `-heartbeat-timeout` | duration | 30s    | Slave 心跳超时        |
| `-max-executions`    | int      | 100    | 最大并发执行数        |

#### 示例

```bash
# 使用默认配置启动
./workflow-engine master start

# 使用配置文件
./workflow-engine master start --config configs/config.yaml

# 自定义地址
./workflow-engine master start --address :8080 --grpc-address :9090

# 独立模式
./workflow-engine master start --standalone
```

#### master status

查看 Master 节点状态。

```bash
workflow-engine master status [options]
```

| 选项       | 类型   | 默认值                | 说明        |
| ---------- | ------ | --------------------- | ----------- |
| `-address` | string | http://localhost:8080 | Master 地址 |

### slave 命令

管理 Slave 节点。

```bash
workflow-engine slave <subcommand> [options]

子命令:
  start     启动 Slave 节点
  status    查看 Slave 状态
```

#### slave start

启动 Slave 节点。

```bash
workflow-engine slave start [options]
```

| 选项            | 类型   | 默认值                        | 说明                                    |
| --------------- | ------ | ----------------------------- | --------------------------------------- |
| `-config`       | string | -                             | 配置文件路径                            |
| `-id`           | string | 自动生成                      | Slave ID                                |
| `-type`         | string | worker                        | Slave 类型: worker, gateway, aggregator |
| `-address`      | string | :9091                         | 监听地址                                |
| `-master`       | string | localhost:9090                | Master gRPC 地址                        |
| `-max-vus`      | int    | 100                           | 最大虚拟用户数                          |
| `-capabilities` | string | http_executor,script_executor | 能力列表 (逗号分隔)                     |
| `-labels`       | string | -                             | 标签 (key=value 格式，逗号分隔)         |

#### 示例

```bash
# 基本启动
./workflow-engine slave start --master localhost:9090

# 指定 ID 和类型
./workflow-engine slave start --id slave-1 --type worker --master localhost:9090

# 指定能力和标签
./workflow-engine slave start \
  --master localhost:9090 \
  --capabilities http_executor,script_executor,db_executor \
  --labels region=us-east,env=prod

# 使用配置文件
./workflow-engine slave start --config configs/config.yaml
```

---

## 工作流定义

工作流使用 YAML 格式定义。

### 基本结构

```yaml
# 工作流唯一标识 (必需)
id: my-workflow

# 工作流名称 (必需)
name: My Workflow

# 描述 (可选)
description: 工作流描述

# 变量定义 (可选)
variables:
  base_url: "https://api.example.com"
  timeout: 30

# 前置钩子 (可选)
pre_hook:
  type: script
  config:
    script: "console.log('Starting workflow')"

# 后置钩子 (可选)
post_hook:
  type: script
  config:
    script: "console.log('Workflow completed')"

# 步骤列表 (必需)
steps:
  - id: step1
    name: Step 1
    type: http
    config:
      method: GET
      url: "/api/users"

# 执行选项 (可选)
options:
  vus: 10
  duration: 5m
```

### Workflow 字段说明

| 字段          | 类型             | 必需 | 说明           |
| ------------- | ---------------- | ---- | -------------- |
| `id`          | string           | 是   | 工作流唯一标识 |
| `name`        | string           | 是   | 工作流名称     |
| `description` | string           | 否   | 工作流描述     |
| `variables`   | map              | 否   | 变量定义       |
| `pre_hook`    | Hook             | 否   | 前置钩子       |
| `post_hook`   | Hook             | 否   | 后置钩子       |
| `steps`       | []Step           | 是   | 步骤列表       |
| `options`     | ExecutionOptions | 否   | 执行选项       |

---

## 步骤类型

### Step 结构

```yaml
- id: step_id # 步骤 ID (必需)
  name: Step Name # 步骤名称 (可选)
  type: http # 步骤类型 (必需)
  config: # 步骤配置 (必需)
    method: GET
    url: "/api/users"
  pre_hook: # 前置钩子 (可选)
    type: script
    config:
      script: "..."
  post_hook: # 后置钩子 (可选)
    type: script
    config:
      script: "..."
  condition: # 条件执行 (可选)
    expression: "${status} == 200"
    then:
      - id: success_step
        type: script
        config:
          script: "..."
    else:
      - id: error_step
        type: script
        config:
          script: "..."
  loop: # 循环配置 (可选)
    mode: for
    count: 5
    steps:
      - id: loop_step
        type: script
        config:
          script: "..."
  on_error: continue # 错误处理策略 (可选)
  timeout: 30s # 超时时间 (可选)
```

### Step 字段说明

| 字段        | 类型      | 必需 | 说明                                          |
| ----------- | --------- | ---- | --------------------------------------------- |
| `id`        | string    | 是   | 步骤唯一标识                                  |
| `name`      | string    | 否   | 步骤名称                                      |
| `type`      | string    | 是   | 步骤类型: http, script, grpc, condition, loop |
| `config`    | map       | 是   | 步骤配置                                      |
| `pre_hook`  | Hook      | 否   | 前置钩子                                      |
| `post_hook` | Hook      | 否   | 后置钩子                                      |
| `condition` | Condition | 否   | 条件执行                                      |
| `loop`      | Loop      | 否   | 循环配置                                      |
| `on_error`  | string    | 否   | 错误策略: abort, continue, retry, skip        |
| `timeout`   | duration  | 否   | 超时时间                                      |

### HTTP 步骤

```yaml
- id: http_request
  type: http
  config:
    method: GET # HTTP 方法: GET, POST, PUT, DELETE, PATCH
    url: "/api/users" # URL 路径或完整 URL
    headers: # 请求头 (可选)
      Authorization: "Bearer ${token}"
      Content-Type: "application/json"
    body: | # 请求体 (可选)
      {"name": "test"}
    query: # 查询参数 (可选)
      page: 1
      size: 10
  timeout: 30s
```

### Script 步骤

```yaml
- id: script_step
  type: script
  config:
    script: |
      // JavaScript 脚本
      const result = ctx.GetVariable("response");
      ctx.SetVariable("processed", result.data);
```

### Condition 步骤

```yaml
- id: check_status
  type: condition
  condition:
    expression: "${response.status_code} == 200"
    then:
      - id: success_handler
        type: script
        config:
          script: "console.log('Success')"
    else:
      - id: error_handler
        type: script
        config:
          script: "console.log('Error')"
```

### Loop 步骤

循环步骤支持三种模式：`for`（固定次数）、`foreach`（集合遍历）、`while`（条件循环）。

#### For 循环

```yaml
- id: for_loop
  type: loop
  loop:
    mode: for
    count: 5 # 执行 5 次
    steps:
      - id: log_iteration
        type: script
        config:
          script: |
            console.log('Iteration: ' + ctx.GetVariable('loop.iteration'));
            console.log('Index: ' + ctx.GetVariable('loop.index'));
```

#### Foreach 循环

```yaml
- id: foreach_loop
  type: loop
  loop:
    mode: foreach
    items: "${users}" # 可以是表达式或字面量数组
    item_var: user # 当前元素变量名，默认为 "item"
    steps:
      - id: process_user
        type: script
        config:
          script: |
            const user = ctx.GetVariable('user');
            console.log('User: ' + user.name);
```

#### While 循环

```yaml
- id: while_loop
  type: loop
  loop:
    mode: while
    condition: "${counter} < 10"
    max_iterations: 100 # 安全限制，防止无限循环
    steps:
      - id: increment
        type: script
        config:
          script: |
            const counter = ctx.GetVariable('counter');
            ctx.SetVariable('counter', counter + 1);
```

#### 循环控制

使用 `break_condition` 和 `continue_condition` 控制循环流程：

```yaml
- id: controlled_loop
  type: loop
  loop:
    mode: for
    count: 100
    break_condition: "${loop.index} >= 10" # 满足条件时退出循环
    continue_condition: "${loop.index} % 2 == 1" # 满足条件时跳过当前迭代
    steps:
      - id: process
        type: script
        config:
          script: "console.log('Index: ' + ctx.GetVariable('loop.index'))"
```

#### 循环变量

循环执行时会自动设置以下变量：

| 变量             | 说明                            |
| ---------------- | ------------------------------- |
| `loop.index`     | 当前迭代索引 (从 0 开始)        |
| `loop.iteration` | 当前迭代次数 (从 1 开始)        |
| `loop.count`     | 总迭代次数 (for/foreach 模式)   |
| `loop.item`      | 当前元素 (foreach 模式)         |
| `{item_var}`     | 自定义元素变量名 (foreach 模式) |

#### Loop 配置字段

| 字段                 | 类型   | 必需 | 说明                                  |
| -------------------- | ------ | ---- | ------------------------------------- |
| `mode`               | string | 是   | 循环模式: for, foreach, while         |
| `count`              | int    | 否   | for 模式的迭代次数                    |
| `items`              | any    | 否   | foreach 模式的集合 (表达式或字面量)   |
| `item_var`           | string | 否   | foreach 模式的元素变量名，默认 "item" |
| `condition`          | string | 否   | while 模式的条件表达式                |
| `max_iterations`     | int    | 否   | while 模式的最大迭代次数，默认 1000   |
| `break_condition`    | string | 否   | 满足时退出循环的条件                  |
| `continue_condition` | string | 否   | 满足时跳过当前迭代的条件              |
| `steps`              | []Step | 是   | 循环体中的步骤列表                    |

### 错误处理策略

| 策略       | 说明                  |
| ---------- | --------------------- |
| `abort`    | 停止整个工作流 (默认) |
| `continue` | 继续执行下一步        |
| `retry`    | 重试当前步骤          |
| `skip`     | 跳过当前步骤          |

---

## 执行选项

### ExecutionOptions 结构

```yaml
options:
  # 虚拟用户数
  vus: 10

  # 持续时间 (与 iterations 二选一)
  duration: 5m

  # 迭代次数 (与 duration 二选一)
  iterations: 100

  # 执行模式
  mode: constant-vus

  # HTTP 引擎类型 (可选)
  http_engine: fasthttp # fasthttp (默认) 或 standard

  # 阶梯配置 (用于 ramping 模式)
  stages:
    - duration: 30s
      target: 10
    - duration: 1m
      target: 50
    - duration: 30s
      target: 0

  # 阈值配置
  thresholds:
    - metric: http_req_duration
      condition: "p(95) < 500"
    - metric: http_req_failed
      condition: "rate < 0.01"

  # Slave 选择器 (分布式模式)
  target_slaves:
    labels:
      region: us-east
```

### HTTP 引擎

工作流引擎支持两种 HTTP 引擎实现，可通过 `http_engine` 选项切换：

| 引擎       | 说明                                     |
| ---------- | ---------------------------------------- |
| `fasthttp` | 默认引擎，基于 FastHTTP 库，性能更高     |
| `standard` | 标准库引擎，基于 Go net/http，兼容性更好 |

#### FastHTTP 引擎 (默认)

- 使用连接池复用 TCP 连接，减少握手开销
- 使用对象池复用请求/响应对象，降低 GC 压力
- 更高的吞吐量，适合高并发压测场景
- 默认每个 Host 最大 1000 连接，空闲连接保持 90 秒

#### 标准库引擎

- 使用 Go 标准库 net/http
- 兼容性更好，支持更多 HTTP 特性
- 适合调试或需要特殊 HTTP 功能的场景

#### 示例: 使用标准库引擎

```yaml
id: standard-http-test
name: 使用标准库 HTTP 引擎

options:
  vus: 10
  duration: 1m
  http_engine: standard # 切换到标准库实现

steps:
  - id: request
    type: http
    config:
      method: GET
      url: "https://httpbin.org/get"
```

#### 性能对比建议

如需对比两种引擎的性能差异，可以：

1. 使用相同的工作流配置
2. 分别设置 `http_engine: fasthttp` 和 `http_engine: standard`
3. 对比执行结果中的 RPS、延迟等指标

### 执行模式

| 模式                    | 说明                     |
| ----------------------- | ------------------------ |
| `constant-vus`          | 固定 VU 数量             |
| `ramping-vus`           | 按阶段调整 VU 数量       |
| `constant-arrival-rate` | 固定请求速率             |
| `ramping-arrival-rate`  | 按阶段调整请求速率       |
| `per-vu-iterations`     | 每个 VU 执行固定迭代次数 |
| `shared-iterations`     | 所有 VU 共享总迭代次数   |
| `externally-controlled` | 通过 API 外部控制        |

### 示例: 恒定负载

```yaml
options:
  vus: 50
  duration: 5m
  mode: constant-vus
```

### 示例: 阶梯负载

```yaml
options:
  mode: ramping-vus
  stages:
    - duration: 1m
      target: 10
      name: "Warm up"
    - duration: 3m
      target: 50
      name: "Ramp up"
    - duration: 5m
      target: 50
      name: "Steady state"
    - duration: 1m
      target: 0
      name: "Ramp down"
```

### 示例: 阈值配置

```yaml
options:
  vus: 100
  duration: 5m
  thresholds:
    - metric: http_req_duration
      condition: "p(95) < 500" # 95% 请求 < 500ms
    - metric: http_req_duration
      condition: "p(99) < 1000" # 99% 请求 < 1000ms
    - metric: http_req_failed
      condition: "rate < 0.01" # 错误率 < 1%
```

---

## 配置管理

### 配置文件结构

配置文件 `configs/config.yaml`:

```yaml
# HTTP 服务配置
server:
  address: ":8080" # 监听地址
  read_timeout: 30s # 读取超时
  write_timeout: 30s # 写入超时
  enable_cors: false # 启用 CORS
  enable_swagger: false # 启用 Swagger

# gRPC 服务配置
grpc:
  address: ":9090" # gRPC 地址
  max_recv_msg_size: 4194304 # 最大接收消息大小 (4MB)
  max_send_msg_size: 4194304 # 最大发送消息大小 (4MB)
  connection_timeout: 10s # 连接超时

# Master 配置
master:
  heartbeat_interval: 5s # 心跳间隔
  heartbeat_timeout: 15s # 心跳超时
  task_queue_size: 1000 # 任务队列大小
  max_slaves: 100 # 最大 Slave 数量

# Slave 配置
slave:
  type: worker # Slave 类型
  capabilities: # 能力列表
    - http_executor
    - script_executor
  labels: {} # 标签
  max_vus: 100 # 最大 VU 数
  master_addr: "localhost:9090" # Master 地址

# 日志配置
logging:
  level: info # 日志级别: debug, info, warn, error
  format: json # 日志格式: json, text
  output: stdout # 输出: stdout, file
```

---

## 分布式执行

### 架构

```
                    ┌─────────────┐
                    │   Master    │
                    │  (调度器)   │
                    └──────┬──────┘
                           │ gRPC
           ┌───────────────┼───────────────┐
           │               │               │
    ┌──────▼──────┐ ┌──────▼──────┐ ┌──────▼──────┐
    │   Slave 1   │ │   Slave 2   │ │   Slave 3   │
    │  (执行器)   │ │  (执行器)   │ │  (执行器)   │
    └─────────────┘ └─────────────┘ └─────────────┘
```

### 启动步骤

1. 启动 Master:

```bash
./workflow-engine master start --config configs/config.yaml
```

2. 启动 Slave (可启动多个):

```bash
# Slave 1
./workflow-engine slave start --id slave-1 --master localhost:9090

# Slave 2
./workflow-engine slave start --id slave-2 --master localhost:9090
```

3. 通过 API 提交工作流:

```bash
curl -X POST http://localhost:8080/api/v1/workflows \
  -H "Content-Type: application/json" \
  -d '{"yaml": "..."}'
```

### Slave 类型

| 类型         | 说明                   |
| ------------ | ---------------------- |
| `worker`     | 工作节点，执行实际任务 |
| `gateway`    | 网关节点               |
| `aggregator` | 聚合节点，汇总指标     |

---

## REST API 使用

Master 节点提供 REST API，详细文档请参考 [API.md](API.md)。

### 常用操作

#### 健康检查

```bash
curl http://localhost:8080/health
# 响应: {"status":"healthy","timestamp":"..."}
```

#### 提交工作流

```bash
curl -X POST http://localhost:8080/api/v1/workflows \
  -H "Content-Type: application/json" \
  -d '{
    "yaml": "id: test\nname: Test\nsteps:\n  - id: s1\n    type: http\n    config:\n      method: GET\n      url: https://httpbin.org/get"
  }'
```

或提交 Workflow 对象:

```bash
curl -X POST http://localhost:8080/api/v1/workflows \
  -H "Content-Type: application/json" \
  -d '{
    "workflow": {
      "id": "test",
      "name": "Test",
      "steps": [
        {
          "id": "s1",
          "type": "http",
          "config": {
            "method": "GET",
            "url": "https://httpbin.org/get"
          }
        }
      ]
    }
  }'
```

#### 获取执行状态

```bash
curl http://localhost:8080/api/v1/executions/{execution_id}
```

#### 获取执行指标

```bash
curl http://localhost:8080/api/v1/executions/{execution_id}/metrics
```

#### 暂停/恢复执行

```bash
# 暂停
curl -X POST http://localhost:8080/api/v1/executions/{execution_id}/pause

# 恢复
curl -X POST http://localhost:8080/api/v1/executions/{execution_id}/resume
```

#### 停止执行

```bash
curl -X DELETE http://localhost:8080/api/v1/executions/{execution_id}
```

#### 扩缩 VU

```bash
curl -X POST http://localhost:8080/api/v1/executions/{execution_id}/scale \
  -H "Content-Type: application/json" \
  -d '{"target_vus": 50}'
```

#### 列出 Slave

```bash
curl http://localhost:8080/api/v1/slaves
```

---

## 完整示例

### 简单 HTTP 测试

```yaml
id: simple-http-test
name: Simple HTTP Test

steps:
  - id: get_request
    name: GET Request
    type: http
    config:
      method: GET
      url: "https://httpbin.org/get"
    timeout: 30s
```

运行:

```bash
./workflow-engine run simple-http-test.yaml
```

### 性能测试

```yaml
id: performance-test
name: Performance Test

options:
  vus: 10
  duration: 1m
  thresholds:
    - metric: http_req_duration
      condition: "p(95) < 500"

steps:
  - id: api_test
    type: http
    config:
      method: GET
      url: "https://httpbin.org/get"
```

运行:

```bash
./workflow-engine run -vus 20 -duration 2m performance-test.yaml
```

### 阶梯负载测试

```yaml
id: ramping-test
name: Ramping Load Test

options:
  mode: ramping-vus
  stages:
    - duration: 30s
      target: 10
    - duration: 1m
      target: 50
    - duration: 2m
      target: 50
    - duration: 30s
      target: 0

steps:
  - id: api_test
    type: http
    config:
      method: GET
      url: "https://httpbin.org/get"
```

### 带条件的工作流

```yaml
id: conditional-workflow
name: Conditional Workflow

steps:
  - id: check_api
    type: http
    config:
      method: GET
      url: "https://httpbin.org/status/200"

  - id: handle_response
    type: condition
    condition:
      expression: "${response.status_code} == 200"
      then:
        - id: success
          type: script
          config:
            script: "console.log('API is healthy')"
      else:
        - id: failure
          type: script
          config:
            script: "console.log('API is down')"
```
