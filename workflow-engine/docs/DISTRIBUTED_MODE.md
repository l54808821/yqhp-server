# 分布式模式部署指南

本文档详细介绍 Workflow Engine 的分布式模式部署和使用方法。

## 目录

- [架构概述](#架构概述)
- [快速开始](#快速开始)
- [Master 节点](#master-节点)
- [Slave 节点](#slave-节点)
- [配置文件](#配置文件)
- [REST API](#rest-api)
- [任务调度](#任务调度)
- [监控与运维](#监控与运维)
- [最佳实践](#最佳实践)
- [故障排查](#故障排查)

## 架构概述

Workflow Engine 采用 Master-Slave 架构，支持水平扩展：

```
                    ┌─────────────────┐
                    │     Client      │
                    │  (REST API/CLI) │
                    └────────┬────────┘
                             │
                             ▼
                    ┌─────────────────┐
                    │     Master      │
                    │   (调度中心)     │
                    │  HTTP: :8080    │
                    └────────┬────────┘
                             │
           ┌─────────────────┼─────────────────┐
           │                 │                 │
           ▼                 ▼                 ▼
    ┌─────────────┐   ┌─────────────┐   ┌─────────────┐
    │   Slave 1   │   │   Slave 2   │   │   Slave N   │
    │  (Worker)   │   │  (Worker)   │   │  (Worker)   │
    │  VUs: 100   │   │  VUs: 100   │   │  VUs: 100   │
    └─────────────┘   └─────────────┘   └─────────────┘
```

### 组件说明

| 组件   | 职责                                           |
| ------ | ---------------------------------------------- |
| Master | 接收工作流请求、任务调度、指标聚合、Slave 管理 |
| Slave  | 执行具体任务、运行虚拟用户、上报指标           |
| Client | 通过 REST API 或 CLI 与 Master 交互            |

### Slave 类型

| 类型       | 说明                             |
| ---------- | -------------------------------- |
| worker     | 执行工作流任务的工作节点（默认） |
| gateway    | 网关节点，用于特殊网络环境       |
| aggregator | 指标聚合节点，用于大规模部署     |

## 快速开始

### 1. 启动 Master 节点

```bash
# 使用默认配置启动
./workflow-engine master start

# 指定端口启动
./workflow-engine master start --address :8080

# 使用配置文件启动
./workflow-engine master start --config configs/config.yaml
```

### 2. 启动 Slave 节点

在另一台机器或终端启动 Slave：

```bash
# 连接到 Master
./workflow-engine slave start --master http://localhost:8080

# 指定 Slave ID 和最大 VU 数
./workflow-engine slave start --master http://localhost:8080 --id slave-1 --max-vus 200

# 带标签启动（用于任务路由）
./workflow-engine slave start --master http://localhost:8080 --labels region=cn-east,env=prod
```

### 3. 提交工作流

```bash
# 通过 REST API 提交
curl -X POST http://localhost:8080/api/v1/workflows \
  -H "Content-Type: application/json" \
  -d '{
    "workflow": {
      "id": "distributed-test",
      "name": "分布式性能测试",
      "options": {
        "vus": 100,
        "duration": "5m"
      },
      "steps": [{
        "id": "api_request",
        "type": "http",
        "config": {
          "method": "GET",
          "url": "https://httpbin.org/get"
        }
      }]
    }
  }'
```

### 4. 查看执行状态

```bash
# 查看所有执行
curl http://localhost:8080/api/v1/executions

# 查看指定执行的状态
curl http://localhost:8080/api/v1/executions/{execution_id}

# 查看执行指标
curl http://localhost:8080/api/v1/executions/{execution_id}/metrics
```

## Master 节点

### 命令行参数

```bash
./workflow-engine master start [选项]

选项:
  --config string           配置文件路径
  --address string          HTTP 服务地址 (默认 ":8080")
  --standalone              独立模式运行（无需 Slave）
  --heartbeat-timeout       Slave 心跳超时时间 (默认 30s)
  --max-executions int      最大并发执行数 (默认 100)
```

### 启动示例

```bash
# 生产环境启动
./workflow-engine master start \
  --config /etc/workflow-engine/config.yaml \
  --address 0.0.0.0:8080 \
  --max-executions 500

# 开发环境（独立模式，无需 Slave）
./workflow-engine master start --standalone
```

### 查看状态

```bash
# 查看 Master 状态
./workflow-engine master status --address http://localhost:8080

# 或使用 curl
curl http://localhost:8080/health
curl http://localhost:8080/api/v1/slaves
```

## Slave 节点

### 命令行参数

```bash
./workflow-engine slave start [选项]

选项:
  --config string       配置文件路径
  --id string           Slave ID（不指定则自动生成）
  --type string         Slave 类型: worker, gateway, aggregator (默认 "worker")
  --address string      Slave 监听地址 (默认 ":9091")
  --master string       Master HTTP 地址 (默认 "http://localhost:8080")
  --max-vus int         最大虚拟用户数 (默认 100)
  --capabilities string 能力列表，逗号分隔 (默认 "http_executor,script_executor")
  --labels string       标签，key=value 格式，逗号分隔
```

### 启动示例

```bash
# 基本启动
./workflow-engine slave start --master http://192.168.1.100:8080

# 高性能节点
./workflow-engine slave start \
  --master http://192.168.1.100:8080 \
  --id high-perf-slave-1 \
  --max-vus 500 \
  --capabilities http_executor,script_executor,db_executor

# 带标签的节点（用于定向调度）
./workflow-engine slave start \
  --master http://192.168.1.100:8080 \
  --id cn-east-slave-1 \
  --labels region=cn-east,zone=a,env=prod
```

### 能力说明

| 能力            | 说明              |
| --------------- | ----------------- |
| http_executor   | HTTP 请求执行能力 |
| script_executor | 脚本执行能力      |
| db_executor     | 数据库操作能力    |
| mq_executor     | 消息队列操作能力  |
| socket_executor | Socket 通信能力   |

## 配置文件

### 完整配置示例

创建 `config.yaml`：

```yaml
# 服务器配置
server:
  address: ":8080"
  read_timeout: 30s
  write_timeout: 30s
  enable_cors: true

# Master 配置
master:
  heartbeat_interval: 5s # 心跳检查间隔
  heartbeat_timeout: 15s # 心跳超时时间
  task_queue_size: 1000 # 任务队列大小
  max_slaves: 100 # 最大 Slave 数量

# Slave 配置
slave:
  type: worker
  capabilities:
    - http_executor
    - script_executor
  labels:
    region: cn-east
    env: prod
  max_vus: 100
  master_addr: "http://localhost:8080"

# 日志配置
logging:
  level: info # debug, info, warn, error
  format: json # json, text
  output: stdout # stdout, file
```

### Master 专用配置

```yaml
# master-config.yaml
server:
  address: "0.0.0.0:8080"

master:
  heartbeat_interval: 5s
  heartbeat_timeout: 30s
  task_queue_size: 5000
  max_slaves: 200

logging:
  level: info
  format: json
```

### Slave 专用配置

```yaml
# slave-config.yaml
slave:
  type: worker
  capabilities:
    - http_executor
    - script_executor
    - db_executor
  labels:
    region: cn-east
    zone: a
  max_vus: 200
  master_addr: "http://master.example.com:8080"

logging:
  level: info
  format: json
```

## REST API

### 工作流管理

```bash
# 提交工作流
curl -X POST http://localhost:8080/api/v1/workflows \
  -H "Content-Type: application/json" \
  -d @workflow.json

# 获取工作流状态
curl http://localhost:8080/api/v1/workflows/{workflow_id}

# 停止工作流
curl -X DELETE http://localhost:8080/api/v1/workflows/{workflow_id}
```

### 执行管理

```bash
# 列出所有执行
curl http://localhost:8080/api/v1/executions

# 获取执行状态
curl http://localhost:8080/api/v1/executions/{execution_id}

# 获取执行指标
curl http://localhost:8080/api/v1/executions/{execution_id}/metrics

# 暂停执行
curl -X POST http://localhost:8080/api/v1/executions/{execution_id}/pause

# 恢复执行
curl -X POST http://localhost:8080/api/v1/executions/{execution_id}/resume

# 动态扩缩 VU
curl -X POST http://localhost:8080/api/v1/executions/{execution_id}/scale \
  -H "Content-Type: application/json" \
  -d '{"target_vus": 200}'

# 停止执行
curl -X DELETE http://localhost:8080/api/v1/executions/{execution_id}
```

### Slave 管理

```bash
# 列出所有 Slave
curl http://localhost:8080/api/v1/slaves

# 获取 Slave 详情
curl http://localhost:8080/api/v1/slaves/{slave_id}

# 排空 Slave（停止接收新任务）
curl -X POST http://localhost:8080/api/v1/slaves/{slave_id}/drain
```

## 任务调度

### 调度策略

工作流可以通过 `target_slaves` 配置指定目标 Slave 的选择策略。

### 调度模式

| 模式       | 说明                     | 适用场景               |
| ---------- | ------------------------ | ---------------------- |
| manual     | 手动指定 Slave ID        | 需要精确控制执行节点   |
| label      | 根据标签选择 Slave       | 按区域、环境等分组调度 |
| capability | 根据能力选择 Slave       | 需要特定执行能力的任务 |
| auto       | 自动负载均衡选择（默认） | 一般场景               |

### 模式 1: 手动指定 Slave ID (manual)

直接指定要使用的 Slave 节点 ID：

```yaml
id: manual-selection-test
name: 手动指定节点测试
options:
  vus: 100
  duration: 5m
  target_slaves:
    mode: manual
    slave_ids:
      - slave-1
      - slave-2
      - slave-3

steps:
  - id: api_request
    type: http
    config:
      method: GET
      url: "https://api.example.com/test"
```

查看可用的 Slave ID：

```bash
curl http://localhost:8080/api/v1/slaves
```

### 模式 2: 按标签选择 (label)

根据 Slave 的标签进行筛选，所有指定的标签必须匹配：

```yaml
id: label-selection-test
name: 按标签选择节点测试
options:
  vus: 100
  duration: 5m
  target_slaves:
    mode: label
    labels:
      region: cn-east # 选择 cn-east 区域
      env: prod # 且环境为 prod
      zone: a # 且可用区为 a

steps:
  - id: api_request
    type: http
    config:
      method: GET
      url: "https://api.example.com/test"
```

启动带标签的 Slave：

```bash
./workflow-engine slave start \
  --master localhost:9090 \
  --id cn-east-prod-1 \
  --labels region=cn-east,env=prod,zone=a
```

### 模式 3: 按能力选择 (capability)

根据 Slave 的执行能力进行筛选：

```yaml
id: capability-selection-test
name: 按能力选择节点测试
options:
  vus: 100
  duration: 5m
  target_slaves:
    mode: capability
    capabilities:
      - http_executor # 需要 HTTP 执行能力
      - db_executor # 需要数据库执行能力

steps:
  - id: db_query
    type: db
    config:
      driver: mysql
      query: "SELECT * FROM users LIMIT 10"
```

启动带特定能力的 Slave：

```bash
./workflow-engine slave start \
  --master localhost:9090 \
  --id db-slave-1 \
  --capabilities http_executor,db_executor,script_executor
```

### 模式 4: 自动选择 (auto)

自动根据负载均衡选择 Slave，可设置数量约束：

```yaml
id: auto-selection-test
name: 自动选择节点测试
options:
  vus: 200
  duration: 5m
  target_slaves:
    mode: auto
    min_slaves: 2 # 至少需要 2 个 Slave
    max_slaves: 5 # 最多使用 5 个 Slave

steps:
  - id: api_request
    type: http
    config:
      method: GET
      url: "https://api.example.com/test"
```

### 组合使用示例

结合标签和数量约束：

```yaml
id: combined-selection-test
name: 组合选择测试
options:
  vus: 500
  duration: 10m
  target_slaves:
    mode: label
    labels:
      region: cn-east
      env: prod
    min_slaves: 3 # 至少需要 3 个匹配的 Slave

steps:
  - id: api_request
    type: http
    config:
      method: GET
      url: "https://api.example.com/test"
```

### 通过 REST API 提交

```bash
# 手动指定 Slave
curl -X POST http://localhost:8080/api/v1/workflows \
  -H "Content-Type: application/json" \
  -d '{
    "workflow": {
      "id": "targeted-test",
      "name": "指定机器测试",
      "options": {
        "vus": 100,
        "duration": "5m",
        "target_slaves": {
          "mode": "manual",
          "slave_ids": ["slave-1", "slave-2"]
        }
      },
      "steps": [{
        "id": "api_request",
        "name": "API 请求",
        "type": "http",
        "config": {
          "method": "GET",
          "url": "https://httpbin.org/get"
        }
      }]
    }
  }'

# 按标签选择
curl -X POST http://localhost:8080/api/v1/workflows \
  -H "Content-Type: application/json" \
  -d '{
    "workflow": {
      "id": "label-test",
      "name": "按标签选择测试",
      "options": {
        "vus": 100,
        "duration": "5m",
        "target_slaves": {
          "mode": "label",
          "labels": {
            "region": "cn-east",
            "env": "prod"
          }
        }
      },
      "steps": [{
        "id": "api_request",
        "name": "API 请求",
        "type": "http",
        "config": {
          "method": "GET",
          "url": "https://httpbin.org/get"
        }
      }]
    }
  }'
```

### 负载分配

Master 会根据以下因素分配负载：

1. Slave 的 `max_vus` 配置
2. Slave 当前负载（auto 模式优先选择低负载节点）
3. Slave 的能力匹配度
4. 标签匹配情况

### 调度失败处理

当无法满足调度条件时，会返回错误：

| 错误情况            | 错误信息                                                 |
| ------------------- | -------------------------------------------------------- |
| 指定的 Slave 不存在 | `slave not found: {slave_id}`                            |
| 指定的 Slave 离线   | `slave is not online: {slave_id}`                        |
| 无匹配标签的 Slave  | `no slaves found matching labels: {labels}`              |
| 无匹配能力的 Slave  | `no slaves found with capabilities: {capabilities}`      |
| Slave 数量不足      | `not enough slaves available: need {min}, have {actual}` |

## 监控与运维

### 健康检查

```bash
# Master 健康检查
curl http://localhost:8080/health
curl http://localhost:8080/ready

# 响应示例
{
  "status": "healthy",
  "timestamp": "2026-01-06T10:00:00Z"
}
```

### 实时指标（WebSocket）

```javascript
// 连接 WebSocket 获取实时指标
const ws = new WebSocket(
  "ws://localhost:8080/api/v1/executions/{execution_id}/ws"
);

ws.onmessage = (event) => {
  const metrics = JSON.parse(event.data);
  console.log("实时指标:", metrics);
};
```

### 日志查看

```bash
# Master 日志
journalctl -u workflow-engine-master -f

# Slave 日志
journalctl -u workflow-engine-slave -f
```

### Systemd 服务配置

Master 服务 (`/etc/systemd/system/workflow-engine-master.service`)：

```ini
[Unit]
Description=Workflow Engine Master
After=network.target

[Service]
Type=simple
User=workflow
ExecStart=/usr/local/bin/workflow-engine master start --config /etc/workflow-engine/master.yaml
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

Slave 服务 (`/etc/systemd/system/workflow-engine-slave.service`)：

```ini
[Unit]
Description=Workflow Engine Slave
After=network.target

[Service]
Type=simple
User=workflow
ExecStart=/usr/local/bin/workflow-engine slave start --config /etc/workflow-engine/slave.yaml
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

启动服务：

```bash
sudo systemctl daemon-reload
sudo systemctl enable workflow-engine-master
sudo systemctl start workflow-engine-master

sudo systemctl enable workflow-engine-slave
sudo systemctl start workflow-engine-slave
```

## 最佳实践

### 1. 容量规划

| 场景     | Master 配置 | Slave 数量 | 单 Slave VU |
| -------- | ----------- | ---------- | ----------- |
| 小型测试 | 2 核 4G     | 1-2        | 100         |
| 中型测试 | 4 核 8G     | 3-5        | 200         |
| 大型测试 | 8 核 16G    | 10+        | 500         |

### 2. 网络配置

- Master 和 Slave 之间建议使用内网通信
- 确保 HTTP 端口（默认 8080）可访问
- 建议配置心跳超时为网络延迟的 3-5 倍

### 3. 高可用部署

```
                    ┌─────────────┐
                    │   负载均衡   │
                    └──────┬──────┘
                           │
              ┌────────────┼────────────┐
              │            │            │
              ▼            ▼            ▼
        ┌──────────┐ ┌──────────┐ ┌──────────┐
        │ Master 1 │ │ Master 2 │ │ Master 3 │
        └────┬─────┘ └────┬─────┘ └────┬─────┘
             │            │            │
             └────────────┼────────────┘
                          │
                    ┌─────┴─────┐
                    │   Redis   │  (状态共享)
                    └───────────┘
```

### 4. 安全建议

- 生产环境启用 API 认证
- 使用 TLS 加密 HTTP 通信
- 限制 Master API 的访问来源
- 定期轮换 API Key

## 故障排查

### Slave 无法连接 Master

```bash
# 检查 Master 是否运行
curl http://master-host:8080/health

# 检查 HTTP 端口是否可达
nc -zv master-host 8080

# 检查防火墙规则
sudo iptables -L -n | grep 8080
```

### Slave 频繁断开

可能原因：

1. 网络不稳定 - 增加心跳超时时间
2. Slave 负载过高 - 减少 max_vus
3. Master 资源不足 - 扩容 Master

```yaml
# 增加心跳超时
master:
  heartbeat_timeout: 60s
```

### 任务分配不均

检查 Slave 配置：

```bash
curl http://localhost:8080/api/v1/slaves
```

确保各 Slave 的 `max_vus` 配置合理。

### 执行卡住

```bash
# 查看执行状态
curl http://localhost:8080/api/v1/executions/{execution_id}

# 查看 Slave 状态
curl http://localhost:8080/api/v1/slaves

# 强制停止执行
curl -X DELETE http://localhost:8080/api/v1/executions/{execution_id}
```

## 完整部署示例

### 三节点集群部署

```bash
# 节点 1: Master
./workflow-engine master start \
  --address 0.0.0.0:8080 \
  --max-executions 200

# 节点 2: Slave 1
./workflow-engine slave start \
  --master http://192.168.1.1:8080 \
  --id slave-1 \
  --max-vus 200 \
  --labels region=cn-east,zone=a

# 节点 3: Slave 2
./workflow-engine slave start \
  --master http://192.168.1.1:8080 \
  --id slave-2 \
  --max-vus 200 \
  --labels region=cn-east,zone=b
```

### 提交分布式测试

```bash
curl -X POST http://192.168.1.1:8080/api/v1/workflows \
  -H "Content-Type: application/json" \
  -d '{
    "workflow": {
      "id": "distributed-perf-test",
      "name": "分布式性能测试",
      "description": "使用多个 Slave 节点进行性能测试",
      "options": {
        "vus": 400,
        "duration": "10m",
        "mode": "constant-vus"
      },
      "steps": [{
        "id": "api_test",
        "name": "API 压力测试",
        "type": "http",
        "config": {
          "method": "GET",
          "url": "https://api.example.com/test"
        },
        "timeout": "30s"
      }]
    }
  }'
```

任务将自动分配到两个 Slave 节点，每个节点运行 200 个虚拟用户。
