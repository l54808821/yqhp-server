# Workflow Engine V2 用户手册

## 目录

1. [概述](#概述)
2. [工作流定义](#工作流定义)
3. [步骤类型](#步骤类型)
4. [断言系统](#断言系统)
5. [提取器系统](#提取器系统)
6. [流程控制](#流程控制)
7. [脚本片段](#脚本片段)
8. [配置管理](#配置管理)
9. [变量系统](#变量系统)
10. [性能测试](#性能测试)
11. [分布式执行](#分布式执行)
12. [HTTP API 使用](#http-api-使用)
13. [最佳实践](#最佳实践)

---

## 概述

Workflow Engine V2 是一个高性能的分布式工作流执行引擎，专为 API 测试和负载测试设计。它支持多种协议、丰富的断言和提取能力，以及灵活的流程控制。

### 核心概念

-   **工作流 (Workflow)**: 一组有序步骤的集合
-   **步骤 (Step)**: 工作流中的单个执行单元
-   **执行器 (Executor)**: 负责执行特定类型步骤的组件
-   **关键字 (Keyword)**: 断言和提取器的统称
-   **脚本片段 (Script Fragment)**: 可复用的步骤集合

---

## 工作流定义

### 基本结构

```yaml
id: workflow-id # 工作流唯一标识
name: Workflow Name # 工作流名称
description: Description # 描述（可选）
version: "1.0" # 版本（可选）

imports: # 导入外部脚本（可选）
    - scripts/common.yaml

config: # 全局配置（可选）
    http:
        default_domain: api
        domains:
            api:
                base_url: "https://api.example.com"

scripts: # 脚本片段定义（可选）
    login:
        params:
            - name: username
              type: string
        steps:
            - id: do_login
              type: http
              config:
                  method: POST
                  url: "/login"

variables: # 变量定义（可选）
    base_url: "https://api.example.com"
    timeout: 30

steps: # 步骤列表
    - id: step1
      type: http
      config:
          method: GET
          url: "/api/users"

options: # 执行选项（可选）
    concurrency: 10
    iterations: 100
    duration: 5m
```

---

## 步骤类型

### HTTP 步骤

发送 HTTP 请求。

```yaml
- id: http_request
  type: http
  config:
      method: GET|POST|PUT|DELETE|PATCH
      url: "/api/endpoint"
      headers:
          Authorization: "Bearer ${token}"
      body: |
          {"key": "value"}
      query:
          page: 1
          size: 10
  timeout: 30s
  pre_scripts:
      - script: "ctx.SetVariable('start_time', time.Now())"
  post_scripts:
      - script: "ctx.SetVariable('response_time', time.Since(start_time))"
```

### Socket 步骤

TCP/UDP 通信。

```yaml
- id: socket_request
  type: socket
  config:
      protocol: tcp|udp
      host: "localhost"
      port: 8080
      action: connect|send|receive|close
      data: "Hello, Server!"
      delimiter: "\n" # 接收分隔符
      length: 1024 # 固定长度接收
      tls:
          enabled: true
          insecure_skip_verify: true
```

### MQ 步骤

消息队列操作。

```yaml
- id: mq_publish
  type: mq
  config:
      connection: kafka # 连接名称
      action: publish|consume
      topic: "test-topic"
      message: |
          {"event": "test"}
      timeout: 10s
```

### DB 步骤

数据库操作。

```yaml
- id: db_query
  type: db
  config:
      connection: mysql # 连接名称
      action: query|execute|count|exists
      sql: "SELECT * FROM users WHERE id = ?"
      params:
          - 1
```

### Script 步骤

执行内联脚本。

```yaml
- id: script_step
  type: script
  config:
      script: |
          result := ctx.GetVariable("response")
          ctx.SetVariable("processed", result.data)
```

### Call 步骤

调用脚本片段。

```yaml
- id: call_login
  type: call
  script: login
  args:
      username: "admin"
      password: "secret"
  output:
      token: "${_token}"
```

---

## 断言系统

断言用于验证响应是否符合预期。

### 比较断言

```yaml
assertions:
    - type: equals
      actual: "${response.status_code}"
      expected: 200

    - type: not_equals
      actual: "${response.data.error}"
      expected: null

    - type: greater_than
      actual: "${response.data.count}"
      expected: 0

    - type: less_or_equal
      actual: "${response.duration}"
      expected: 1000
```

### 字符串断言

```yaml
assertions:
    - type: contains
      actual: "${response.body}"
      expected: "success"

    - type: starts_with
      actual: "${response.data.name}"
      expected: "user_"

    - type: matches
      actual: "${response.data.email}"
      expected: "^[a-z]+@example\\.com$"
```

### 集合断言

```yaml
assertions:
    - type: in
      actual: "${response.status_code}"
      expected: [200, 201, 204]

    - type: is_empty
      actual: "${response.data.errors}"

    - type: length_equals
      actual: "${response.data.items}"
      expected: 10
```

### 类型断言

```yaml
assertions:
    - type: is_null
      actual: "${response.data.deleted_at}"

    - type: is_not_null
      actual: "${response.data.id}"

    - type: is_type
      actual: "${response.data.count}"
      expected: number
```

---

## 提取器系统

提取器用于从响应中提取数据并存储到变量。

### JSONPath 提取器

```yaml
extractors:
    - type: jsonpath
      source: "${response.body}"
      path: "$.data.users[0].id"
      variable: "first_user_id"
      default: 0

    - type: jsonpath
      source: "${response.body}"
      path: "$.data.items[*].name"
      variable: "all_names"
      index: 0 # 提取第一个元素
```

### 正则表达式提取器

```yaml
extractors:
    - type: regex
      source: "${response.body}"
      pattern: "token=([a-zA-Z0-9]+)"
      group: 1
      variable: "auth_token"
```

### Header 提取器

```yaml
extractors:
    - type: header
      name: "X-Request-Id"
      variable: "request_id"

    - type: cookie
      name: "session_id"
      variable: "session"
```

### XPath 提取器 (XML)

```yaml
extractors:
    - type: xpath
      source: "${response.body}"
      path: "//user/name/text()"
      variable: "user_name"
```

---

## 流程控制

### If 条件

```yaml
- id: check_status
  type: if
  condition: "${response.status_code} == 200"
  then:
      - id: success_step
        type: script
        config:
            script: "log('Success!')"
  else_if:
      - condition: "${response.status_code} == 404"
        steps:
            - id: not_found_step
              type: script
              config:
                  script: "log('Not found')"
  else:
      - id: error_step
        type: script
        config:
            script: "log('Error!')"
```

### While 循环

```yaml
- id: wait_for_ready
  type: while
  condition: "${status} != 'ready'"
  max_iterations: 10
  steps:
      - id: check_status
        type: http
        config:
            method: GET
            url: "/status"
      - id: extract_status
        type: script
        config:
            script: "ctx.SetVariable('status', response.data.status)"
```

### For 循环

```yaml
- id: iterate_pages
  type: for
  start: 1
  end: 10
  step: 1
  steps:
      - id: get_page
        type: http
        config:
            method: GET
            url: "/api/items?page=${i}"
```

### Foreach 循环

```yaml
- id: process_users
  type: foreach
  items: "${users}"
  item_var: user
  index_var: idx
  steps:
      - id: update_user
        type: http
        config:
            method: PUT
            url: "/api/users/${user.id}"
            body: |
                {"name": "${user.name}"}
```

### Parallel 并行执行

```yaml
- id: parallel_requests
  type: parallel
  max_concurrent: 5
  fail_fast: true
  steps:
      - id: req1
        type: http
        config:
            method: GET
            url: "/api/1"
      - id: req2
        type: http
        config:
            method: GET
            url: "/api/2"
```

### Retry 重试

```yaml
- id: retry_request
  type: retry
  retry:
      max_attempts: 3
      delay: 1s
      backoff: exponential # fixed, linear, exponential
      max_delay: 10s
  steps:
      - id: unstable_request
        type: http
        config:
            method: GET
            url: "/api/unstable"
```

### Sleep 等待

```yaml
- id: wait
  type: sleep
  duration: 5s
```

### WaitUntil 条件等待

```yaml
- id: wait_for_condition
  type: wait_until
  condition: "${job_status} == 'completed'"
  timeout: 60s
  interval: 2s
```

### Break/Continue

```yaml
- id: loop_with_break
  type: foreach
  items: "${items}"
  item_var: item
  label: outer_loop
  steps:
      - id: check_item
        type: if
        condition: "${item.skip}"
        then:
            - id: skip_item
              type: continue
              label: outer_loop
      - id: check_done
        type: if
        condition: "${item.done}"
        then:
            - id: break_loop
              type: break
              label: outer_loop
```

---

## 脚本片段

脚本片段是可复用的步骤集合，支持参数化调用。

### 定义脚本片段

```yaml
scripts:
    login:
        name: login
        description: 用户登录脚本
        params:
            - name: username
              type: string
              required: true
            - name: password
              type: string
              required: true
            - name: remember
              type: boolean
              default: false
        steps:
            - id: do_login
              type: http
              config:
                  method: POST
                  url: "/api/login"
                  body: |
                      {
                        "username": "${username}",
                        "password": "${password}",
                        "remember": ${remember}
                      }
            - id: extract_token
              type: script
              config:
                  script: "ctx.SetVariable('_token', response.data.token)"
        returns:
            - name: token
              value: "${_token}"
```

### 调用脚本片段

```yaml
steps:
    - id: user_login
      type: call
      script: login
      args:
          username: "admin"
          password: "secret123"
      output:
          auth_token: "${token}"
```

### 导入外部脚本

```yaml
imports:
    - scripts/auth.yaml
    - scripts/common.yaml
```

---

## 配置管理

### 配置层级

配置按以下优先级合并：步骤配置 > 工作流配置 > 全局配置

### HTTP 配置

```yaml
config:
    http:
        default_domain: api
        domains:
            api:
                base_url: "https://api.example.com"
                timeout:
                    connect: 5s
                    read: 30s
                    total: 60s
                headers:
                    Authorization: "Bearer ${token}"
                tls:
                    insecure_skip_verify: false
                    cert_file: "/path/to/cert.pem"
                    key_file: "/path/to/key.pem"
            internal:
                base_url: "http://internal-api:8080"
        timeout:
            connect: 10s
            read: 30s
            write: 30s
            total: 60s
        redirect:
            follow: true
            max_redirects: 10
        headers:
            User-Agent: "Workflow-Engine/2.0"
```

### Socket 配置

```yaml
config:
    socket:
        default_protocol: tcp
        timeout:
            connect: 10s
            read: 30s
        buffer_size: 4096
        tls:
            insecure_skip_verify: true
```

### MQ 配置

```yaml
config:
    mq:
        default_connection: kafka
        timeout: 30s
        connections:
            kafka:
                type: kafka
                brokers: "localhost:9092"
            rabbitmq:
                type: rabbitmq
                brokers: "amqp://guest:guest@localhost:5672/"
            redis:
                type: redis
                brokers: "localhost:6379"
```

### DB 配置

```yaml
config:
    db:
        default_connection: mysql
        timeout: 30s
        connections:
            mysql:
                driver: mysql
                dsn: "root:password@tcp(localhost:3306)/testdb"
                max_conns: 10
                max_idle: 5
            postgres:
                driver: postgres
                dsn: "postgres://user:pass@localhost:5432/testdb?sslmode=disable"
```

---

## 变量系统

### 变量引用

```yaml
# 基本变量
url: "${base_url}/api"

# 嵌套引用
user_name: "${response.data.users[0].name}"

# 环境变量
api_key: "${env.API_KEY}"

# 文件引用
config: "${file:configs/settings.yaml}"
```

### 内置变量

| 变量                      | 描述                       |
| ------------------------- | -------------------------- |
| `${response}`             | 当前步骤的响应             |
| `${response.status_code}` | HTTP 状态码                |
| `${response.headers}`     | 响应头                     |
| `${response.body}`        | 响应体                     |
| `${response.data}`        | 解析后的 JSON 数据         |
| `${response.duration}`    | 响应时间（毫秒）           |
| `${step_result}`          | 步骤执行结果（后置脚本中） |
| `${parallel_results}`     | 并行执行结果               |
| `${i}`                    | for 循环索引               |
| `${item}`                 | foreach 当前元素           |
| `${idx}`                  | foreach 索引               |

---

## 性能测试

Workflow Engine V2 提供强大的性能测试能力，支持多种负载模式和指标收集。

### 执行选项

在工作流中配置 `options` 来控制性能测试行为：

```yaml
id: performance-test
name: API Performance Test

options:
    # 虚拟用户数（并发数）
    vus: 100

    # 迭代次数（每个 VU 执行的次数）
    iterations: 1000

    # 持续时间（与 iterations 二选一）
    duration: 5m

    # 全局超时
    timeout: 10m

    # 失败阈值（失败率超过此值则测试失败）
    failure_threshold: 0.01 # 1%

    # 每秒请求数限制
    rate_limit: 1000

    # 预热时间
    ramp_up: 30s

    # 冷却时间
    ramp_down: 10s

steps:
    - id: api_request
      type: http
      config:
          method: GET
          url: "/api/users"
```

### 负载模式

#### 恒定负载

固定数量的虚拟用户持续执行：

```yaml
options:
    vus: 50
    duration: 5m
```

#### 阶梯负载

逐步增加负载，用于找到系统瓶颈：

```yaml
options:
    stages:
        - duration: 1m
          target: 10 # 1分钟内增加到 10 VUs
        - duration: 3m
          target: 50 # 3分钟内增加到 50 VUs
        - duration: 2m
          target: 100 # 2分钟内增加到 100 VUs
        - duration: 5m
          target: 100 # 保持 100 VUs 5分钟
        - duration: 1m
          target: 0 # 1分钟内降到 0
```

#### 峰值负载

模拟突发流量：

```yaml
options:
    stages:
        - duration: 2m
          target: 20 # 正常负载
        - duration: 10s
          target: 200 # 突然增加到 200
        - duration: 1m
          target: 200 # 保持峰值
        - duration: 10s
          target: 20 # 恢复正常
        - duration: 2m
          target: 20 # 继续正常负载
```

#### 压力测试

持续增加负载直到系统崩溃：

```yaml
options:
    stages:
        - duration: 2m
          target: 100
        - duration: 2m
          target: 200
        - duration: 2m
          target: 400
        - duration: 2m
          target: 800
        - duration: 2m
          target: 1600
```

### 性能指标

#### 内置指标

| 指标                       | 描述                |
| -------------------------- | ------------------- |
| `http_req_duration`        | HTTP 请求总耗时     |
| `http_req_waiting`         | 等待响应时间 (TTFB) |
| `http_req_connecting`      | TCP 连接时间        |
| `http_req_tls_handshaking` | TLS 握手时间        |
| `http_req_sending`         | 发送请求时间        |
| `http_req_receiving`       | 接收响应时间        |
| `http_req_blocked`         | 请求阻塞时间        |
| `http_req_failed`          | 失败请求数          |
| `http_reqs`                | 总请求数            |
| `iteration_duration`       | 单次迭代耗时        |
| `iterations`               | 总迭代数            |
| `vus`                      | 当前虚拟用户数      |
| `vus_max`                  | 最大虚拟用户数      |
| `data_sent`                | 发送数据量          |
| `data_received`            | 接收数据量          |

#### 自定义指标

```yaml
steps:
    - id: api_request
      type: http
      config:
          method: GET
          url: "/api/users"
      post_scripts:
          - script: |
                # 记录自定义指标
                metrics.Counter("custom_success_count").Add(1)
                metrics.Gauge("response_size").Set(len(response.body))
                metrics.Trend("api_latency").Add(response.duration)
```

### 阈值设置

定义性能测试的通过/失败条件：

```yaml
options:
  thresholds:
    # 95% 的请求响应时间小于 500ms
    http_req_duration:
      - p(95) < 500

    # 99% 的请求响应时间小于 1000ms
    http_req_duration:
      - p(99) < 1000

    # 平均响应时间小于 200ms
    http_req_duration:
      - avg < 200

    # 最大响应时间小于 2000ms
    http_req_duration:
      - max < 2000

    # 错误率小于 1%
    http_req_failed:
      - rate < 0.01

    # 每秒请求数大于 100
    http_reqs:
      - rate > 100

    # 自定义指标阈值
    custom_success_count:
      - count > 1000
```

### 数据驱动测试

使用外部数据进行参数化测试：

```yaml
id: data-driven-test
name: Data Driven Performance Test

variables:
    users: "${file:data/users.json}"

options:
    vus: 50
    duration: 5m

steps:
    - id: login_test
      type: foreach
      items: "${users}"
      item_var: user
      steps:
          - id: do_login
            type: http
            config:
                method: POST
                url: "/api/login"
                body: |
                    {
                      "username": "${user.username}",
                      "password": "${user.password}"
                    }
```

数据文件 `data/users.json`:

```json
[
    { "username": "user1", "password": "pass1" },
    { "username": "user2", "password": "pass2" },
    { "username": "user3", "password": "pass3" }
]
```

### 场景测试

模拟真实用户行为场景：

```yaml
id: e-commerce-scenario
name: E-Commerce User Journey

options:
    vus: 100
    duration: 10m
    scenarios:
        browse:
            weight: 60 # 60% 的用户只浏览
            steps: [browse_products]
        purchase:
            weight: 30 # 30% 的用户购买
            steps: [browse_products, add_to_cart, checkout]
        admin:
            weight: 10 # 10% 的管理员操作
            steps: [admin_login, manage_products]

scripts:
    browse_products:
        steps:
            - id: home
              type: http
              config:
                  url: "/"
            - id: products
              type: http
              config:
                  url: "/products"
            - id: product_detail
              type: http
              config:
                  url: "/products/${random_product_id}"

    add_to_cart:
        steps:
            - id: add_item
              type: http
              config:
                  method: POST
                  url: "/cart/add"
                  body: '{"product_id": "${product_id}", "quantity": 1}'

    checkout:
        steps:
            - id: view_cart
              type: http
              config:
                  url: "/cart"
            - id: place_order
              type: http
              config:
                  method: POST
                  url: "/orders"
```

### 思考时间

模拟真实用户的操作间隔：

```yaml
steps:
    - id: browse_home
      type: http
      config:
          url: "/"

    - id: think_time_1
      type: sleep
      duration: "${random(1000, 3000)}ms" # 1-3秒随机等待

    - id: browse_products
      type: http
      config:
          url: "/products"

    - id: think_time_2
      type: sleep
      duration: "${random(2000, 5000)}ms" # 2-5秒随机等待
```

### 运行性能测试

```bash
# 基本运行
./run --workflow tests/performance.yaml

# 指定 VUs 和持续时间
./run --workflow tests/performance.yaml --vus 100 --duration 5m

# 输出详细报告
./run --workflow tests/performance.yaml --out json=results.json

# 输出到多个目标
./run --workflow tests/performance.yaml \
  --out json=results.json \
  --out influxdb=http://localhost:8086/k6

# 使用配置文件
./run --workflow tests/performance.yaml --config configs/performance.yaml
```

### 报告输出

#### JSON 报告

```bash
./run --workflow test.yaml --out json=report.json
```

输出示例：

```json
{
    "metrics": {
        "http_req_duration": {
            "avg": 125.5,
            "min": 45.2,
            "max": 892.1,
            "p90": 245.8,
            "p95": 356.2,
            "p99": 678.4
        },
        "http_reqs": {
            "count": 15000,
            "rate": 50.0
        },
        "http_req_failed": {
            "count": 12,
            "rate": 0.0008
        }
    },
    "thresholds": {
        "http_req_duration{p(95)}": {
            "ok": true,
            "value": 356.2
        }
    }
}
```

#### InfluxDB + Grafana

实时监控性能指标：

```bash
./run --workflow test.yaml --out influxdb=http://localhost:8086/workflow_metrics
```

#### HTML 报告

```bash
./run --workflow test.yaml --out html=report.html
```

---

## 分布式执行

### 架构概述

Workflow Engine 采用 Master-Slave 架构进行分布式执行：

```
                    ┌─────────────┐
                    │   Master    │
                    │  (调度器)   │
                    └──────┬──────┘
                           │
           ┌───────────────┼───────────────┐
           │               │               │
    ┌──────▼──────┐ ┌──────▼──────┐ ┌──────▼──────┐
    │   Slave 1   │ │   Slave 2   │ │   Slave 3   │
    │  (执行器)   │ │  (执行器)   │ │  (执行器)   │
    └─────────────┘ └─────────────┘ └─────────────┘
```

### 启动 Master

```bash
./master --config configs/master.yaml
```

Master 配置 `configs/master.yaml`:

```yaml
server:
    address: ":8080"

grpc:
    address: ":9090"

master:
    heartbeat_interval: 5s
    heartbeat_timeout: 15s
    task_queue_size: 1000
    max_slaves: 100

logging:
    level: info
    format: json
```

### 启动 Slave

```bash
./slave --config configs/slave.yaml --master localhost:9090
```

Slave 配置 `configs/slave.yaml`:

```yaml
slave:
    type: worker
    capabilities:
        - http_executor
        - socket_executor
        - mq_executor
        - db_executor
    labels:
        region: us-east-1
        zone: a
    max_vus: 100
    master_addr: "localhost:9090"

logging:
    level: info
```

### 分布式测试

```yaml
id: distributed-test
name: Distributed Performance Test

options:
    vus: 1000
    duration: 10m

    # 分布式配置
    distribution:
        # 按标签分配
        selector:
            region: us-east-1

        # 或按权重分配
        weights:
            slave-1: 40
            slave-2: 30
            slave-3: 30

steps:
    - id: api_test
      type: http
      config:
          method: GET
          url: "/api/users"
```

### 运行分布式测试

```bash
# 连接到 Master 运行
./run --workflow tests/distributed.yaml --master localhost:8080

# 指定分布策略
./run --workflow tests/distributed.yaml \
  --master localhost:8080 \
  --distribution round-robin

# 查看 Slave 状态
./run --status --master localhost:8080
```

### 监控分布式执行

```bash
# 查看实时状态
./run --watch --master localhost:8080

# 查看 Slave 列表
curl http://localhost:8080/api/v1/slaves

# 查看执行状态
curl http://localhost:8080/api/v1/executions/{execution_id}
```

---

## HTTP API 使用

除了命令行方式，Workflow Engine 还提供完整的 HTTP API，支持通过 REST 接口管理和执行工作流。

### 启动 API 服务

```bash
# 启动 Master（包含 API 服务）
./master --config configs/master.yaml
```

API 服务默认监听 `http://localhost:8080`。

### 直接执行 YAML

最简单的方式是直接提交 YAML 文件执行：

```bash
# 直接执行 YAML 文件
curl -X POST http://localhost:8080/api/v1/execute \
  -H "Content-Type: application/x-yaml" \
  --data-binary @test.yaml
```

响应：

```json
{
    "code": 0,
    "message": "success",
    "data": {
        "execution_id": "exec-123456",
        "status": "running"
    }
}
```

### 创建并执行工作流

分步骤创建和执行：

```bash
# 1. 创建工作流
curl -X POST http://localhost:8080/api/v1/workflows \
  -H "Content-Type: application/x-yaml" \
  --data-binary @workflow.yaml

# 2. 执行工作流
curl -X POST http://localhost:8080/api/v1/workflows/my-workflow/execute \
  -H "Content-Type: application/json" \
  -d '{
    "variables": {"token": "abc123"},
    "options": {"vus": 10, "duration": "5m"}
  }'

# 3. 查看执行状态
curl http://localhost:8080/api/v1/executions/exec-123456

# 4. 获取执行结果
curl http://localhost:8080/api/v1/executions/exec-123456/result
```

### 使用 JSON 格式

也可以使用 JSON 格式提交工作流：

```bash
curl -X POST http://localhost:8080/api/v1/execute \
  -H "Content-Type: application/json" \
  -d '{
    "id": "api-test",
    "name": "API Test",
    "options": {
      "vus": 10,
      "duration": "1m"
    },
    "steps": [
      {
        "id": "test_api",
        "type": "http",
        "config": {
          "method": "GET",
          "url": "https://api.example.com/health"
        }
      }
    ]
  }'
```

### 实时监控执行

通过 WebSocket 获取实时指标：

```javascript
const ws = new WebSocket(
    "ws://localhost:8080/api/v1/executions/exec-123456/ws"
);

ws.onmessage = (event) => {
    const data = JSON.parse(event.data);
    console.log("VUs:", data.vus);
    console.log("Requests:", data.http_reqs.count);
    console.log("Avg Duration:", data.http_req_duration.avg);
};
```

或使用 Server-Sent Events：

```bash
curl -N http://localhost:8080/api/v1/executions/exec-123456/stream
```

### 下载报告

```bash
# JSON 报告
curl -o report.json \
  "http://localhost:8080/api/v1/executions/exec-123456/report?format=json"

# HTML 报告
curl -o report.html \
  "http://localhost:8080/api/v1/executions/exec-123456/report?format=html"
```

### 完整 API 参考

详细的 API 文档请参考 [API 文档](API.md)。

---

## 最佳实践

### 1. 使用配置继承

将通用配置放在全局级别，特定配置放在步骤级别：

```yaml
config:
    http:
        timeout:
            total: 30s
        headers:
            Content-Type: "application/json"

steps:
    - id: special_request
      type: http
      config:
          timeout: 60s # 覆盖全局超时
```

### 2. 复用脚本片段

将常用操作封装为脚本片段：

```yaml
scripts:
    authenticate:
        # ... 认证逻辑

    cleanup:
        # ... 清理逻辑

steps:
    - id: login
      type: call
      script: authenticate

    # ... 测试步骤

    - id: cleanup
      type: call
      script: cleanup
```

### 3. 合理使用断言

每个关键步骤都应该有断言：

```yaml
- id: create_user
  type: http
  config:
      method: POST
      url: "/users"
  assertions:
      - type: equals
        actual: "${response.status_code}"
        expected: 201
      - type: is_not_null
        actual: "${response.data.id}"
```

### 4. 错误处理

使用 `on_error` 和 `retry` 处理不稳定的操作：

```yaml
- id: external_api
  type: http
  config:
      method: GET
      url: "/external/api"
  on_error: continue
  retry:
      max_attempts: 3
      delay: 2s
      backoff: exponential
```

### 5. 并行优化

对独立的请求使用并行执行：

```yaml
- id: parallel_init
  type: parallel
  max_concurrent: 3
  steps:
      - id: get_config
        type: http
        config:
            url: "/config"
      - id: get_user
        type: http
        config:
            url: "/user"
      - id: get_settings
        type: http
        config:
            url: "/settings"
```

---

## 附录

### 支持的断言类型

| 类型               | 描述       |
| ------------------ | ---------- |
| `equals`           | 相等       |
| `not_equals`       | 不相等     |
| `greater_than`     | 大于       |
| `greater_or_equal` | 大于等于   |
| `less_than`        | 小于       |
| `less_or_equal`    | 小于等于   |
| `contains`         | 包含       |
| `not_contains`     | 不包含     |
| `starts_with`      | 以...开头  |
| `ends_with`        | 以...结尾  |
| `matches`          | 正则匹配   |
| `in`               | 在列表中   |
| `not_in`           | 不在列表中 |
| `is_empty`         | 为空       |
| `is_not_empty`     | 不为空     |
| `length_equals`    | 长度等于   |
| `is_null`          | 为 null    |
| `is_not_null`      | 不为 null  |
| `is_type`          | 类型检查   |

### 支持的提取器类型

| 类型       | 描述             |
| ---------- | ---------------- |
| `jsonpath` | JSONPath 提取    |
| `xpath`    | XPath 提取 (XML) |
| `regex`    | 正则表达式提取   |
| `header`   | HTTP Header 提取 |
| `cookie`   | Cookie 提取      |
| `status`   | 状态码提取       |
| `body`     | 响应体提取       |

### 支持的步骤类型

| 类型         | 描述           |
| ------------ | -------------- |
| `http`       | HTTP 请求      |
| `socket`     | Socket 通信    |
| `mq`         | 消息队列       |
| `db`         | 数据库操作     |
| `script`     | 内联脚本       |
| `call`       | 调用脚本片段   |
| `if`         | 条件分支       |
| `while`      | While 循环     |
| `for`        | For 循环       |
| `foreach`    | Foreach 循环   |
| `parallel`   | 并行执行       |
| `sleep`      | 等待           |
| `wait_until` | 条件等待       |
| `retry`      | 重试           |
| `break`      | 跳出循环       |
| `continue`   | 继续下一次循环 |
