# 执行机启动手册

本文档介绍如何启动和配置 Workflow Engine 执行机（Slave 节点），以便在 Gulu 平台中执行工作流。

## 目录

- [系统架构](#系统架构)
- [运行模式](#运行模式)
- [环境准备](#环境准备)
- [快速启动](#快速启动)
- [配置说明](#配置说明)
- [与 Gulu 平台集成](#与-gulu-平台集成)
- [运维管理](#运维管理)
- [常见问题](#常见问题)

## 系统架构

```
┌─────────────────────────────────────────────────────────────────┐
│                        Gulu 平台                                 │
│  ┌─────────────┐    ┌─────────────────────────────────────────┐ │
│  │   前端 UI   │───▶│           Gulu API (Go/Fiber)           │ │
│  │  (Vue 3)    │    │  ┌─────────────────────────────────────┐│ │
│  └─────────────┘    │  │    内置 Workflow Engine Master     ││ │
│                     │  │    (embedded=true, 默认模式)        ││ │
│                     │  └─────────────────────────────────────┘│ │
│                     └──────────────────┬──────────────────────┘ │
└────────────────────────────────────────┼────────────────────────┘
                                         │ HTTP
                    ┌────────────────────┼────────────────────┐
                    │                    │                    │
                    ▼                    ▼                    ▼
             ┌─────────────┐      ┌─────────────┐      ┌─────────────┐
             │   Slave 1   │      │   Slave 2   │      │   Slave N   │
             │  (执行机)    │      │  (执行机)    │      │  (执行机)    │
             └─────────────┘      └─────────────┘      └─────────────┘
```

### 组件说明

| 组件           | 说明                                                   |
| -------------- | ------------------------------------------------------ |
| Gulu 前端      | Vue 3 + TypeScript 构建的 Web 界面                     |
| Gulu API       | Go + Fiber 构建的后端服务，内置 Workflow Engine Master |
| Slave (执行机) | 实际执行工作流任务的节点                               |

## 运行模式

Gulu 支持两种运行模式：

### 1. 内置模式（默认）

Master 内置在 Gulu 服务中，无需单独启动。适合：

- 开发环境
- 小型部署
- 快速体验

配置（config.yml）：

```yaml
workflow_engine:
  embedded: true # 使用内置 Master
  standalone: true # 无需 Slave 也能执行简单工作流
  http_address: ":8080" # HTTP 服务地址
  max_executions: 100 # 最大并发执行数
```

### 2. 外部模式

使用独立部署的 Workflow Engine Master。适合：

- 生产环境
- 大规模部署
- 需要高可用

配置（config.yml）：

```yaml
workflow_engine:
  embedded: false
  external_url: "http://workflow-engine-master:8080"
```

## 环境准备

### 1. 系统要求

| 项目     | 最低要求            | 推荐配置              |
| -------- | ------------------- | --------------------- |
| 操作系统 | Linux/macOS/Windows | Linux (Ubuntu 20.04+) |
| CPU      | 2 核                | 4 核+                 |
| 内存     | 2 GB                | 4 GB+                 |
| 网络     | 能访问 Master 节点  | 内网通信              |

### 2. 构建执行机程序

```bash
# 进入 workflow-engine 目录
cd yqhp/workflow-engine

# 构建可执行文件
go build -o workflow-engine ./cmd/main.go

# 验证构建成功
./workflow-engine version
```

### 3. 确认 Master 节点已启动

执行机需要连接到 Master 节点，请确保 Master 已启动：

```bash
# 启动 Master（如果还没启动）
./workflow-engine master start --address :8080

# 验证 Master 运行状态
curl http://localhost:8080/health
```

## 快速启动

### 最简启动

```bash
# 连接到本地 Master
./workflow-engine slave start --master http://localhost:8080
```

### 带参数启动

```bash
./workflow-engine slave start \
  --master http://localhost:8080 \
  --id my-executor-1 \
  --max-vus 100 \
  --type worker
```

### 启动成功输出

```
INFO  Connecting to master at localhost:9090...
INFO  Slave registered successfully
INFO  Slave ID: my-executor-1
INFO  Type: worker
INFO  Max VUs: 100
INFO  Capabilities: [http_executor script_executor]
INFO  Slave is ready to receive tasks
```

## 配置说明

### 命令行参数

| 参数             | 说明                  | 默认值                          | 示例                              |
| ---------------- | --------------------- | ------------------------------- | --------------------------------- |
| `--master`       | Master 节点 HTTP 地址 | `http://localhost:8080`         | `http://192.168.1.100:8080`       |
| `--id`           | 执行机唯一标识        | 自动生成                        | `executor-prod-1`                 |
| `--type`         | 执行机类型            | `worker`                        | `worker`, `gateway`, `aggregator` |
| `--address`      | 执行机监听地址        | `:9091`                         | `0.0.0.0:9091`                    |
| `--max-vus`      | 最大虚拟用户数        | `100`                           | `200`                             |
| `--capabilities` | 执行能力列表          | `http_executor,script_executor` | 见下表                            |
| `--labels`       | 标签（用于任务路由）  | 无                              | `region=cn-east,env=prod`         |
| `--config`       | 配置文件路径          | 无                              | `./slave-config.yaml`             |

### 执行能力说明

| 能力              | 说明          | 适用场景               |
| ----------------- | ------------- | ---------------------- |
| `http_executor`   | HTTP 请求执行 | API 测试、接口测试     |
| `script_executor` | 脚本执行      | JavaScript/Python 脚本 |
| `db_executor`     | 数据库操作    | SQL 查询、数据验证     |
| `mq_executor`     | 消息队列操作  | Kafka/RabbitMQ 测试    |
| `socket_executor` | Socket 通信   | TCP/UDP 测试           |

### 配置文件方式

创建 `slave-config.yaml`：

```yaml
slave:
  type: worker
  capabilities:
    - http_executor
    - script_executor
    - db_executor
  labels:
    region: cn-east
    env: prod
    team: qa
  max_vus: 200
  master_url: "http://192.168.1.100:8080"

logging:
  level: info
  format: json
```

使用配置文件启动：

```bash
./workflow-engine slave start --config slave-config.yaml
```

## 与 Gulu 平台集成

### 1. 同步执行机到 Gulu

执行机启动后，需要在 Gulu 平台中同步才能使用：

**方式一：通过 UI 同步**

1. 登录 Gulu 平台
2. 进入「执行机管理」页面
3. 点击「同步执行机」按钮

**方式二：通过 API 同步**

```bash
curl -X POST http://localhost:5321/api/executors/sync \
  -H "Authorization: Bearer <your-token>"
```

### 2. 配置执行机信息

同步后，在 Gulu 平台中配置执行机：

| 字段   | 说明                     | 示例                                               |
| ------ | ------------------------ | -------------------------------------------------- |
| 名称   | 执行机显示名称           | `生产环境执行机-1`                                 |
| 类型   | 用途分类                 | `performance`(压测)、`normal`(普通)、`debug`(调试) |
| 描述   | 详细说明                 | `华东区域压测专用机器`                             |
| 标签   | 用于任务路由             | `{"region":"cn-east","env":"prod"}`                |
| 优先级 | 调度优先级（越大越优先） | `10`                                               |
| 状态   | 启用/禁用                | `启用`                                             |

### 3. 执行工作流

配置完成后，在工作流编辑器中：

1. 点击「执行」按钮
2. 选择要使用的执行机
3. 确认执行

## 运维管理

### Systemd 服务配置

创建服务文件 `/etc/systemd/system/workflow-executor.service`：

```ini
[Unit]
Description=Workflow Engine Executor (Slave)
After=network.target

[Service]
Type=simple
User=workflow
WorkingDirectory=/opt/workflow-engine
ExecStart=/opt/workflow-engine/workflow-engine slave start \
  --master http://192.168.1.100:8080 \
  --id executor-prod-1 \
  --max-vus 200 \
  --labels region=cn-east,env=prod
Restart=always
RestartSec=5
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
```

启动服务：

```bash
# 重载配置
sudo systemctl daemon-reload

# 启用开机自启
sudo systemctl enable workflow-executor

# 启动服务
sudo systemctl start workflow-executor

# 查看状态
sudo systemctl status workflow-executor

# 查看日志
journalctl -u workflow-executor -f
```

### Docker 部署

创建 `Dockerfile`：

```dockerfile
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o workflow-engine ./cmd/main.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /app
COPY --from=builder /app/workflow-engine .
ENTRYPOINT ["./workflow-engine"]
CMD ["slave", "start"]
```

运行容器：

```bash
docker run -d \
  --name workflow-executor \
  --restart always \
  workflow-engine:latest \
  slave start \
  --master http://host.docker.internal:8080 \
  --id docker-executor-1 \
  --max-vus 100
```

### 健康检查

```bash
# 检查 Master 上的执行机列表
curl http://localhost:8080/api/v1/slaves

# 检查特定执行机状态
curl http://localhost:8080/api/v1/slaves/my-executor-1
```

### 日志查看

```bash
# 实时查看日志
journalctl -u workflow-executor -f

# 查看最近 100 行
journalctl -u workflow-executor -n 100

# 按时间范围查看
journalctl -u workflow-executor --since "2024-01-01 00:00:00" --until "2024-01-01 23:59:59"
```

## 常见问题

### Q1: 执行机无法连接 Master

**症状**：启动时报错 `failed to connect to master`

**排查步骤**：

```bash
# 1. 检查 Master 是否运行
curl http://<master-host>:8080/health

# 2. 检查 HTTP 端口是否可达
nc -zv <master-host> 8080

# 3. 检查防火墙
sudo iptables -L -n | grep 8080

# 4. 检查网络连通性
ping <master-host>
```

**解决方案**：

- 确保 Master 已启动
- 检查防火墙规则，开放 8080 端口
- 确认 `--master` 参数地址正确

### Q2: 执行机频繁断开重连

**可能原因**：

1. 网络不稳定
2. Master 负载过高
3. 心跳超时设置过短

**解决方案**：

```bash
# 增加心跳超时（Master 端配置）
./workflow-engine master start --heartbeat-timeout 60s
```

### Q3: 执行机在 Gulu 平台显示离线

**排查步骤**：

1. 确认执行机进程正在运行
2. 检查 Slave ID 是否匹配
3. 尝试重新同步执行机

```bash
# 检查进程
ps aux | grep workflow-engine

# 重新同步
curl -X POST http://localhost:5321/api/executors/sync
```

### Q4: 任务执行失败

**排查步骤**：

1. 查看执行机日志
2. 检查执行机能力是否匹配任务需求
3. 检查资源使用情况

```bash
# 查看日志
journalctl -u workflow-executor -n 200

# 检查资源
top -p $(pgrep workflow-engine)
```

### Q5: 如何扩容执行机

**水平扩容**：启动更多执行机实例

```bash
# 在不同机器上启动
./workflow-engine slave start \
  --master http://192.168.1.100:8080 \
  --id executor-2 \
  --max-vus 200
```

**垂直扩容**：增加单个执行机的 VU 数

```bash
# 重启时增加 max-vus
./workflow-engine slave start \
  --master http://192.168.1.100:8080 \
  --id executor-1 \
  --max-vus 500
```

## 生产环境部署建议

### 1. 容量规划

| 场景     | 执行机数量 | 单机 VU | 总 VU    |
| -------- | ---------- | ------- | -------- |
| 小型项目 | 1-2        | 100     | 100-200  |
| 中型项目 | 3-5        | 200     | 600-1000 |
| 大型项目 | 10+        | 500     | 5000+    |

### 2. 网络配置

- Master 和 Slave 之间使用内网通信
- 确保 HTTP 端口（默认 8080）可访问
- 建议配置心跳超时为网络延迟的 3-5 倍

### 3. 监控告警

建议监控以下指标：

- 执行机在线状态
- CPU/内存使用率
- 任务执行成功率
- 响应时间

### 4. 高可用

- 部署多个执行机实现冗余
- 使用标签进行分组管理
- 配置自动重启策略

## 附录

### 完整启动示例

```bash
# 生产环境执行机
./workflow-engine slave start \
  --master http://192.168.1.100:8080 \
  --id prod-executor-cn-east-1 \
  --type worker \
  --address 0.0.0.0:9091 \
  --max-vus 200 \
  --capabilities http_executor,script_executor,db_executor \
  --labels region=cn-east,env=prod,team=qa
```
