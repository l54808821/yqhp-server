# 执行机管理手册

本文档介绍如何启动、注册和管理执行机（Slave 节点），以及如何在工作流中配置执行策略。

## 系统架构

```
┌───────────────────────────────────────────────────────────┐
│                      Gulu 平台                             │
│  ┌───────────┐    ┌─────────────────────────────────────┐ │
│  │  前端 UI  │───▶│        Gulu API (Go/Fiber)          │ │
│  │  (Vue 3)  │    │  ┌───────────────────────────────┐  │ │
│  └───────────┘    │  │  内置 Workflow Engine Master  │  │ │
│                   │  │  (embedded=true, 默认模式)     │  │ │
│                   │  └───────────────────────────────┘  │ │
│                   └──────────────┬──────────────────────┘ │
└──────────────────────────────────┼────────────────────────┘
                                   │ HTTP
                  ┌────────────────┼────────────────┐
                  ▼                ▼                ▼
           ┌───────────┐    ┌───────────┐    ┌───────────┐
           │  Slave 1  │    │  Slave 2  │    │  Slave N  │
           │  (执行机)  │    │  (执行机)  │    │  (执行机)  │
           └───────────┘    └───────────┘    └───────────┘
```

### 核心概念

- **Master**：工作流调度中心，负责接收执行请求并分发给 Slave。默认内置在 Gulu 服务中。
- **Slave（执行机）**：实际执行工作流任务的节点，可部署在不同机器上。
- **执行策略**：工作流执行时选择执行机的方式，支持本地执行、自动分配、指定执行机三种策略。

## 运行模式

### 1. 内置模式（默认，开发/小型部署）

Master 内置在 Gulu 服务中，配合 `standalone: true` 可直接在本地执行工作流，无需部署任何 Slave。

```yaml
# config.yml
workflow_engine:
  embedded: true
  standalone: true
  http_address: ":8080"
  max_executions: 100
  heartbeat_timeout: 30s
```

### 2. 分布式模式（生产环境）

Master 内置在 Gulu 中，但 `standalone: false`，工作流需要分发到 Slave 执行。适合需要横向扩展的场景。

```yaml
# config.yml
workflow_engine:
  embedded: true
  standalone: false
  http_address: ":8080"
  max_executions: 100
  heartbeat_timeout: 30s
```

### 3. 外部 Master 模式（大规模部署）

使用独立部署的 Workflow Engine Master，Gulu 通过 HTTP 与其通信。

```yaml
# config.yml
workflow_engine:
  embedded: false
  external_url: "http://workflow-engine-master:8080"
```

## 快速开始

### 1. 构建执行机

```bash
cd yqhp/workflow-engine
go build -o workflow-engine .
./workflow-engine -v
```

### 2. 启动执行机

```bash
# 最简启动：连接到本地 Master
./workflow-engine slave start --master http://localhost:8080

# 带标识和配置启动
./workflow-engine slave start \
  --master http://localhost:8080 \
  --id my-executor-1 \
  --max-vus 100 \
  --labels region=cn-east,env=prod
```

启动成功后，执行机会自动向 Master 注册，并通过心跳保持连接。

### 3. 在 Gulu 平台中管理

执行机启动后有两种方式注册到 Gulu 平台：

**方式一：自动注册（推荐）**

执行机启动后会自动注册到 Workflow Engine Master。在 Gulu 的「执行机管理」页面点击「同步执行机」即可一键导入。

**方式二：快速注册**

在「执行机管理」页面点击「快速注册」，填写名称和地址即可手动添加。

**方式三：API 注册**

```bash
# 自动注册/更新（已存在则更新，不存在则创建）
curl -X POST http://localhost:5321/api/executors/register \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <token>" \
  -d '{
    "slave_id": "my-executor-1",
    "name": "生产执行机-1",
    "address": "192.168.1.101:9091",
    "type": "normal",
    "labels": {"region": "cn-east", "env": "prod"}
  }'

# 批量同步（从 Workflow Engine 获取所有 Slave）
curl -X POST http://localhost:5321/api/executors/sync \
  -H "Authorization: Bearer <token>"
```

## 执行策略

工作流支持三种执行策略，可在工作流编辑器中通过「执行机」按钮配置，也可在执行时临时覆盖。

### 本地执行（默认）

使用 Gulu 内置引擎直接执行，无需 Slave。适合调试和小型工作流。

### 自动分配

系统自动选择负载最低、标签匹配的在线执行机。可配置标签过滤条件，例如只选择 `env=prod` 的执行机。

### 指定执行机

手动指定某台执行机执行。执行前会检查目标执行机的在线状态和可用性。

### 策略配置位置

1. **工作流级别**：在工作流编辑器中点击工具栏的「执行机」按钮，配置该工作流的默认执行策略。保存后该策略会持久化。
2. **执行时覆盖**：点击「执行」按钮时，弹窗中会显示默认策略并允许临时修改。

## 命令行参数

| 参数 | 说明 | 默认值 | 示例 |
|------|------|--------|------|
| `--master` | Master HTTP 地址 | `http://localhost:8080` | `http://192.168.1.100:8080` |
| `--id` | 执行机唯一标识 | 自动生成（UUID） | `prod-executor-1` |
| `--address` | 执行机监听地址 | `:9091` | `0.0.0.0:9091` |
| `--type` | 执行机类型 | `worker` | `worker` |
| `--max-vus` | 最大虚拟用户数 | `100` | `200` |
| `--capabilities` | 执行能力 | 全部 | `http_executor,script_executor` |
| `--labels` | 标签（用于策略路由） | 无 | `region=cn-east,env=prod` |
| `--config` | 配置文件路径 | 无 | `./slave-config.yaml` |

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
  max_vus: 200
  master_url: "http://192.168.1.100:8080"

logging:
  level: info
  format: json
```

```bash
./workflow-engine slave start --config slave-config.yaml
```

## 执行机管理页面

管理页面支持卡片和表格两种视图模式。

### 卡片视图

默认视图，每个执行机以卡片形式展示：

- 状态指示灯（绿色在线 / 蓝色繁忙 / 红色离线）
- 负载进度条
- 类型标签（普通 / 压测专用 / 调试专用）
- 自定义标签
- 启用/禁用开关

### 表格视图

切换到表格视图查看详细信息，支持搜索和筛选。

### 概览统计

页面顶部显示执行机实时统计：总数、在线数、繁忙数、离线数。页面每 15 秒自动刷新状态。

## API 参考

| 接口 | 方法 | 说明 |
|------|------|------|
| `/api/executors` | GET | 获取执行机列表（分页） |
| `/api/executors` | POST | 创建执行机（完整参数） |
| `/api/executors/register` | POST | 注册执行机（自动创建或更新） |
| `/api/executors/sync` | POST | 从 Workflow Engine 同步执行机 |
| `/api/executors/available` | GET | 获取可用执行机列表（带运行时状态） |
| `/api/executors/by-labels` | GET | 按标签筛选执行机 |
| `/api/executors/:id` | GET | 获取执行机详情 |
| `/api/executors/:id` | PUT | 更新执行机配置 |
| `/api/executors/:id` | DELETE | 删除执行机 |
| `/api/executors/:id/status` | PUT | 更新启用/禁用状态 |

## 生产部署

### Systemd 服务

创建 `/etc/systemd/system/workflow-executor.service`：

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
  --id prod-executor-1 \
  --max-vus 200 \
  --labels region=cn-east,env=prod
Restart=always
RestartSec=5
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
```

```bash
sudo systemctl daemon-reload
sudo systemctl enable workflow-executor
sudo systemctl start workflow-executor
sudo systemctl status workflow-executor
```

### Docker 部署

```bash
docker run -d \
  --name workflow-executor \
  --restart always \
  workflow-engine:latest \
  slave start \
  --master http://host.docker.internal:8080 \
  --id docker-executor-1 \
  --max-vus 100 \
  --labels env=docker
```

### 容量规划

| 场景 | 执行机数量 | 单机 VU | 建议执行策略 |
|------|-----------|---------|-------------|
| 开发/测试 | 0（本地执行） | - | 本地执行 |
| 小型项目 | 1-2 | 100 | 手动指定 |
| 中型项目 | 3-5 | 200 | 自动分配 |
| 大型项目 | 10+ | 500 | 自动分配 + 标签路由 |

## 常见问题

### 执行机无法连接 Master

```bash
# 检查 Master 是否运行
curl http://<master-host>:8080/health

# 检查端口连通性
nc -zv <master-host> 8080

# 检查防火墙
sudo iptables -L -n | grep 8080
```

### 执行机显示离线

1. 确认执行机进程正在运行：`ps aux | grep workflow-engine`
2. 检查心跳超时配置（默认 30s）
3. 在管理页面点击「同步执行机」刷新状态
4. 确认 Slave ID 与 Gulu 中注册的一致

### 自动分配策略找不到执行机

- 确认有启用状态（`status=1`）的执行机
- 确认执行机在线（通过管理页面查看状态）
- 如果配置了标签过滤，确认有匹配标签的执行机
- 系统会选择负载最低的在线执行机，全部繁忙时会返回错误

### 如何扩容

**水平扩容**：在新机器上启动更多 Slave 实例，启动后自动注册。

```bash
./workflow-engine slave start \
  --master http://192.168.1.100:8080 \
  --id executor-new \
  --max-vus 200 \
  --labels region=cn-east,env=prod
```

**垂直扩容**：增大 `--max-vus` 参数后重启执行机。
