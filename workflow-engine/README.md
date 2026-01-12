# Workflow Engine

高性能分布式工作流执行引擎，支持 HTTP/Socket/MQ/DB 多协议测试，提供丰富的流程控制和性能测试能力。

## 特性

- **多协议支持**: HTTP、Socket (TCP/UDP)、消息队列、数据库
- **性能测试**: 支持多种负载模式 (constant-vus, ramping-vus, arrival-rate 等)
- **分布式架构**: Master-Slave 架构，支持水平扩展
- **灵活配置**: 支持 YAML 工作流定义，三级配置合并
- **REST API**: 完整的 HTTP API 支持工作流管理和监控

## 快速开始

### 构建

```bash
go build -o workflow-engine .
```

### 独立模式运行

最简单的方式是使用 `run` 命令直接执行工作流文件：

```bash
./workflow-engine run workflow.yaml
```

带参数运行：

```bash
# 指定虚拟用户数和持续时间
./workflow-engine run -u 10 -d 30s workflow.yaml

# 指定迭代次数
./workflow-engine run -i 100 workflow.yaml

# 输出 JSON 结果
./workflow-engine run --out-json results.json workflow.yaml

# 输出指标到多个目标
./workflow-engine run -o json=metrics.json -o console workflow.yaml
```

### 分布式模式运行

#### 启动 Master 节点

```bash
./workflow-engine master start --address :8080
```

使用配置文件：

```bash
./workflow-engine master start --config configs/config.yaml
```

#### 启动 Slave 节点

```bash
./workflow-engine slave start --master http://localhost:8080
```

带参数启动：

```bash
./workflow-engine slave start \
  --master http://localhost:8080 \
  --id slave-1 \
  --type worker \
  --max-vus 100 \
  --capabilities http_executor,script_executor
```

### 工作流示例

创建 `workflow.yaml` 文件：

```yaml
id: simple-api-test
name: Simple API Test
description: 简单的 API 测试示例

options:
  vus: 1
  iterations: 1

steps:
  - id: health_check
    name: Health Check
    type: http
    config:
      method: GET
      url: "https://httpbin.org/get"
    timeout: 30s
```

性能测试示例：

```yaml
id: performance-test
name: API Performance Test

options:
  vus: 10
  duration: 1m
  mode: constant-vus
  thresholds:
    - metric: http_req_duration
      condition: "p(95) < 500"

steps:
  - id: api_request
    name: API Request
    type: http
    config:
      method: GET
      url: "https://httpbin.org/get"
```

阶梯负载测试：

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
    - duration: 30s
      target: 0

steps:
  - id: api_request
    type: http
    config:
      method: GET
      url: "https://httpbin.org/get"
```

## CLI 命令参考

```
workflow-engine - 分布式工作流执行引擎

用法:
  workflow-engine [command]

命令:
  master    管理 Master 节点
  slave     管理 Slave 节点
  run       独立模式执行工作流
  help      显示帮助信息

全局选项:
  --config string   配置文件路径
  --debug           启用调试日志
  -q, --quiet       静默模式
  -v, --version     显示版本信息
  -h, --help        显示帮助

使用 "workflow-engine [command] --help" 获取命令详细帮助。
```

### run 命令

```bash
workflow-engine run [options] <workflow.yaml>

选项:
  -u, --vus int           虚拟用户数 (覆盖工作流配置)
  -d, --duration duration 测试持续时间 (如 30s, 5m)
  -i, --iterations int    迭代次数
      --mode string       执行模式 (constant-vus, ramping-vus 等)
  -o, --out stringArray   指标输出目标 (可多次指定)，格式: type=config
      --out-json string   输出 JSON 结果到文件
  -h, --help              显示帮助
```

### master 命令

```bash
workflow-engine master [command]

子命令:
  start     启动 Master 节点
  status    查看 Master 状态

master start 选项:
  --address string          HTTP 服务地址 (默认 :8080)
  --standalone              独立模式运行 (无需 Slave)
  --heartbeat-timeout       Slave 心跳超时 (默认 30s)
  --max-executions int      最大并发执行数 (默认 100)
```

### slave 命令

```bash
workflow-engine slave [command]

子命令:
  start     启动 Slave 节点
  status    查看 Slave 状态

slave start 选项:
  --id string           Slave ID (自动生成如不指定)
  --type string         Slave 类型: worker, gateway, aggregator (默认 worker)
  --address string      监听地址 (默认 :9091)
  --master string       Master 地址 (默认 localhost:9090)
  --max-vus int         最大虚拟用户数 (默认 100)
  --capabilities string 能力列表，逗号分隔
  --labels string       标签，key=value 格式，逗号分隔
```

## REST API

Master 节点提供 REST API 用于工作流管理：

```bash
# 健康检查
curl http://localhost:8080/health

# 提交工作流
curl -X POST http://localhost:8080/api/v1/workflows \
  -H "Content-Type: application/json" \
  -d '{"yaml": "..."}'

# 获取执行状态
curl http://localhost:8080/api/v1/executions/{execution_id}

# 获取执行指标
curl http://localhost:8080/api/v1/executions/{execution_id}/metrics

# 停止执行
curl -X DELETE http://localhost:8080/api/v1/executions/{execution_id}

# 列出 Slave 节点
curl http://localhost:8080/api/v1/slaves
```

## 项目结构

```
workflow-engine/
├── main.go                # 主入口
├── cmd/                   # Cobra 命令定义
│   ├── root.go           # 根命令
│   ├── run.go            # run 命令
│   ├── master.go         # master 命令
│   └── slave.go          # slave 命令
├── api/                   # API 定义
│   └── rest/              # REST API
│       └── client/        # HTTP 客户端
├── internal/              # 内部包
│   ├── config/            # 配置管理
│   ├── execution/         # 执行模式
│   ├── executor/          # 步骤执行器
│   ├── expression/        # 表达式求值
│   ├── master/            # Master 实现
│   ├── parser/            # YAML 解析器
│   ├── reporter/          # 报告生成
│   └── slave/             # Slave 实现
├── pkg/                   # 公共包
│   ├── types/             # 类型定义
│   ├── metrics/           # 指标定义
│   ├── output/            # 输出插件
│   └── logger/            # 日志
├── configs/               # 配置文件示例
├── examples/              # 工作流示例
└── docs/                  # 文档
    ├── USER_MANUAL.md     # 用户手册
    ├── API.md             # API 文档
    └── k6_comparison.md   # K6 对比分析
```

## 配置文件

默认配置文件 `configs/config.yaml`：

```yaml
server:
  address: ":8080"
  read_timeout: 30s
  write_timeout: 30s

master:
  heartbeat_interval: 5s
  heartbeat_timeout: 15s
  task_queue_size: 1000
  max_slaves: 100

slave:
  type: worker
  capabilities:
    - http_executor
    - script_executor
  max_vus: 100
  master_addr: "http://localhost:8080"

logging:
  level: info
  format: json
  output: stdout
```

## 文档

- [用户手册](docs/USER_MANUAL.md) - 完整的使用指南
- [API 文档](docs/API.md) - REST API 参考

## 运行测试

```bash
go test ./...
```

## License

See the main project license.
