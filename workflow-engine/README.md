# Workflow Engine V2

高性能分布式工作流执行引擎，支持 HTTP/Socket/MQ/DB 多协议测试，提供丰富的流程控制和断言能力。

## 特性

-   **多协议支持**: HTTP、Socket (TCP/UDP)、消息队列 (Kafka/RabbitMQ/Redis)、数据库 (MySQL/PostgreSQL/MongoDB)
-   **丰富的断言**: 比较、字符串、集合、类型断言
-   **灵活的提取器**: JSONPath、XPath、正则表达式、Header/Cookie 提取
-   **流程控制**: if/else、while、for、foreach、parallel、retry、sleep
-   **可复用脚本**: 支持脚本片段定义、参数化调用、循环检测
-   **配置继承**: 全局 → 工作流 → 步骤 三级配置合并
-   **性能测试**: 支持多种负载模式、阈值设置、实时指标收集
-   **分布式架构**: Master-Slave 架构，支持水平扩展

## 快速开始

### 安装

```bash
cd workflow-engine
go build ./cmd/...
```

### 运行示例

```bash
# 启动 Master 节点
./master --config configs/master.yaml

# 启动 Slave 节点
./slave --config configs/slave.yaml --master localhost:9090

# 执行工作流
./run --workflow examples/http_test.yaml
```

### 简单示例

```yaml
id: simple-api-test
name: Simple API Test
config:
    http:
        default_domain: api
        domains:
            api:
                base_url: "https://api.example.com"
        timeout:
            total: 30s
        headers:
            Content-Type: "application/json"

steps:
    - id: get_users
      type: http
      config:
          method: GET
          url: "/users"
      post_scripts:
          - script: |
                ctx.SetVariable("user_count", len(response.data))
```

### 性能测试示例

```yaml
id: performance-test
name: API Performance Test

options:
    vus: 100 # 100 个虚拟用户
    duration: 5m # 持续 5 分钟
    thresholds:
        http_req_duration:
            - p(95) < 500 # 95% 请求 < 500ms
        http_req_failed:
            - rate < 0.01 # 错误率 < 1%

steps:
    - id: api_request
      type: http
      config:
          method: GET
          url: "/api/users"
```

### 通过 HTTP API 执行

```bash
# 直接执行 YAML 文件
curl -X POST http://localhost:8080/api/v1/execute \
  -H "Content-Type: application/x-yaml" \
  --data-binary @test.yaml

# 查看执行状态
curl http://localhost:8080/api/v1/executions/{execution_id}

# 获取执行结果
curl http://localhost:8080/api/v1/executions/{execution_id}/result
```

## 项目结构

```
workflow-engine/
├── cmd/                    # 命令行入口
│   ├── master/            # Master 节点
│   ├── slave/             # Slave 节点
│   └── run/               # 独立运行命令
├── internal/              # 内部包
│   ├── config/            # 配置管理
│   ├── executor/          # 步骤执行器
│   │   └── flow/          # 流程控制执行器
│   ├── expression/        # 表达式求值
│   ├── keyword/           # 关键字系统
│   │   ├── assertion/     # 断言关键字
│   │   └── extractor/     # 提取器关键字
│   ├── parser/            # YAML 解析器
│   ├── response/          # 统一响应处理
│   └── script/            # 脚本片段系统
├── pkg/types/             # 公共类型定义
├── api/                   # API 定义
│   └── v1/                # gRPC/REST API
├── configs/               # 配置文件示例
└── docs/                  # 文档
```

## 文档

-   [用户手册](docs/USER_MANUAL.md) - 完整的使用指南
-   [API 文档](docs/API.md) - REST/gRPC API 参考

## 开发

### 运行测试

```bash
go test ./...
```

### 代码检查

```bash
golangci-lint run
```

## License

See the main k6 project license.
