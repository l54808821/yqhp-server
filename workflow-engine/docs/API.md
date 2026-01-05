# Workflow Engine V2 API 文档

## 概述

Workflow Engine V2 提供 REST API 和 gRPC API 两种接口方式，支持工作流的创建、执行、监控和管理。

### 基础信息

-   **REST API 基础 URL**: `http://localhost:8080/api/v1`
-   **gRPC 地址**: `localhost:9090`
-   **认证方式**: Bearer Token (可选)

---

## REST API

### 认证

如果启用了认证，需要在请求头中添加：

```
Authorization: Bearer <token>
```

---

## 工作流管理

### 创建工作流

从 YAML 定义创建工作流。

**请求**

```
POST /api/v1/workflows
Content-Type: application/x-yaml
```

**请求体** (YAML)

```yaml
id: my-workflow
name: My API Test
config:
    http:
        default_domain: api
        domains:
            api:
                base_url: "https://api.example.com"
steps:
    - id: get_users
      type: http
      config:
          method: GET
          url: "/users"
```

**响应**

```json
{
    "code": 0,
    "message": "success",
    "data": {
        "workflow_id": "my-workflow",
        "created_at": "2026-01-02T10:00:00Z"
    }
}
```

**cURL 示例**

```bash
curl -X POST http://localhost:8080/api/v1/workflows \
  -H "Content-Type: application/x-yaml" \
  --data-binary @workflow.yaml
```

---

### 创建工作流 (JSON)

从 JSON 定义创建工作流。

**请求**

```
POST /api/v1/workflows
Content-Type: application/json
```

**请求体** (JSON)

```json
{
    "id": "my-workflow",
    "name": "My API Test",
    "config": {
        "http": {
            "default_domain": "api",
            "domains": {
                "api": {
                    "base_url": "https://api.example.com"
                }
            }
        }
    },
    "steps": [
        {
            "id": "get_users",
            "type": "http",
            "config": {
                "method": "GET",
                "url": "/users"
            }
        }
    ]
}
```

**cURL 示例**

```bash
curl -X POST http://localhost:8080/api/v1/workflows \
  -H "Content-Type: application/json" \
  -d '{
    "id": "my-workflow",
    "name": "My API Test",
    "steps": [...]
  }'
```

---

### 获取工作流列表

**请求**

```
GET /api/v1/workflows
```

**查询参数**

| 参数     | 类型   | 描述              |
| -------- | ------ | ----------------- |
| `page`   | int    | 页码，默认 1      |
| `size`   | int    | 每页数量，默认 20 |
| `name`   | string | 按名称过滤        |
| `status` | string | 按状态过滤        |

**响应**

```json
{
    "code": 0,
    "message": "success",
    "data": {
        "total": 100,
        "page": 1,
        "size": 20,
        "items": [
            {
                "id": "my-workflow",
                "name": "My API Test",
                "description": "API test workflow",
                "version": "1.0",
                "created_at": "2026-01-02T10:00:00Z",
                "updated_at": "2026-01-02T10:00:00Z"
            }
        ]
    }
}
```

---

### 获取工作流详情

**请求**

```
GET /api/v1/workflows/{workflow_id}
```

**响应**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "id": "my-workflow",
    "name": "My API Test",
    "description": "API test workflow",
    "version": "1.0",
    "config": {...},
    "scripts": {...},
    "variables": {...},
    "steps": [...],
    "options": {...},
    "created_at": "2026-01-02T10:00:00Z",
    "updated_at": "2026-01-02T10:00:00Z"
  }
}
```

---

### 更新工作流

**请求**

```
PUT /api/v1/workflows/{workflow_id}
Content-Type: application/x-yaml
```

**请求体**

与创建工作流相同。

---

### 删除工作流

**请求**

```
DELETE /api/v1/workflows/{workflow_id}
```

**响应**

```json
{
    "code": 0,
    "message": "success"
}
```

---

## 工作流执行

### 执行工作流

执行已创建的工作流。

**请求**

```
POST /api/v1/workflows/{workflow_id}/execute
Content-Type: application/json
```

**请求体**

```json
{
    "variables": {
        "base_url": "https://api.example.com",
        "token": "my-auth-token"
    },
    "options": {
        "vus": 10,
        "duration": "5m",
        "thresholds": {
            "http_req_duration": ["p(95) < 500"]
        }
    }
}
```

**响应**

```json
{
    "code": 0,
    "message": "success",
    "data": {
        "execution_id": "exec-123456",
        "workflow_id": "my-workflow",
        "status": "running",
        "started_at": "2026-01-02T10:00:00Z"
    }
}
```

**cURL 示例**

```bash
# 简单执行
curl -X POST http://localhost:8080/api/v1/workflows/my-workflow/execute

# 带参数执行
curl -X POST http://localhost:8080/api/v1/workflows/my-workflow/execute \
  -H "Content-Type: application/json" \
  -d '{
    "variables": {"token": "abc123"},
    "options": {"vus": 10, "duration": "5m"}
  }'
```

---

### 直接执行 YAML

直接提交 YAML 并执行，无需先创建工作流。

**请求**

```
POST /api/v1/execute
Content-Type: application/x-yaml
```

**请求体**

```yaml
id: inline-test
name: Inline API Test

options:
    vus: 10
    duration: 1m

config:
    http:
        domains:
            api:
                base_url: "https://api.example.com"

steps:
    - id: test_api
      type: http
      config:
          method: GET
          url: "/api/health"
      assertions:
          - type: equals
            actual: "${response.status_code}"
            expected: 200
```

**响应**

```json
{
    "code": 0,
    "message": "success",
    "data": {
        "execution_id": "exec-789012",
        "status": "running",
        "started_at": "2026-01-02T10:00:00Z"
    }
}
```

**cURL 示例**

```bash
curl -X POST http://localhost:8080/api/v1/execute \
  -H "Content-Type: application/x-yaml" \
  --data-binary @test.yaml
```

---

### 直接执行 JSON

直接提交 JSON 并执行。

**请求**

```
POST /api/v1/execute
Content-Type: application/json
```

**请求体**

```json
{
    "id": "inline-test",
    "name": "Inline API Test",
    "options": {
        "vus": 10,
        "duration": "1m"
    },
    "config": {
        "http": {
            "domains": {
                "api": {
                    "base_url": "https://api.example.com"
                }
            }
        }
    },
    "steps": [
        {
            "id": "test_api",
            "type": "http",
            "config": {
                "method": "GET",
                "url": "/api/health"
            }
        }
    ]
}
```

---

### 获取执行状态

**请求**

```
GET /api/v1/executions/{execution_id}
```

**响应**

```json
{
    "code": 0,
    "message": "success",
    "data": {
        "execution_id": "exec-123456",
        "workflow_id": "my-workflow",
        "status": "running",
        "progress": {
            "current_iteration": 150,
            "total_iterations": 1000,
            "current_vus": 10,
            "elapsed_time": "1m30s"
        },
        "metrics": {
            "http_reqs": {
                "count": 1500,
                "rate": 16.67
            },
            "http_req_duration": {
                "avg": 125.5,
                "min": 45.2,
                "max": 892.1,
                "p90": 245.8,
                "p95": 356.2,
                "p99": 678.4
            },
            "http_req_failed": {
                "count": 2,
                "rate": 0.0013
            }
        },
        "started_at": "2026-01-02T10:00:00Z"
    }
}
```

**执行状态**

| 状态        | 描述     |
| ----------- | -------- |
| `pending`   | 等待执行 |
| `running`   | 执行中   |
| `paused`    | 已暂停   |
| `completed` | 执行完成 |
| `failed`    | 执行失败 |
| `cancelled` | 已取消   |

---

### 获取执行列表

**请求**

```
GET /api/v1/executions
```

**查询参数**

| 参数          | 类型   | 描述                |
| ------------- | ------ | ------------------- |
| `page`        | int    | 页码                |
| `size`        | int    | 每页数量            |
| `workflow_id` | string | 按工作流过滤        |
| `status`      | string | 按状态过滤          |
| `start_time`  | string | 开始时间 (ISO 8601) |
| `end_time`    | string | 结束时间 (ISO 8601) |

---

### 停止执行

**请求**

```
POST /api/v1/executions/{execution_id}/stop
```

**响应**

```json
{
    "code": 0,
    "message": "success",
    "data": {
        "execution_id": "exec-123456",
        "status": "cancelled"
    }
}
```

---

### 暂停执行

**请求**

```
POST /api/v1/executions/{execution_id}/pause
```

---

### 恢复执行

**请求**

```
POST /api/v1/executions/{execution_id}/resume
```

---

### 获取执行结果

获取完成执行的详细结果。

**请求**

```
GET /api/v1/executions/{execution_id}/result
```

**响应**

```json
{
    "code": 0,
    "message": "success",
    "data": {
        "execution_id": "exec-123456",
        "workflow_id": "my-workflow",
        "status": "completed",
        "summary": {
            "total_iterations": 1000,
            "successful_iterations": 998,
            "failed_iterations": 2,
            "total_requests": 5000,
            "successful_requests": 4995,
            "failed_requests": 5,
            "total_duration": "5m2s",
            "data_sent": "1.2 MB",
            "data_received": "15.6 MB"
        },
        "metrics": {
            "http_reqs": {
                "count": 5000,
                "rate": 16.61
            },
            "http_req_duration": {
                "avg": 125.5,
                "min": 45.2,
                "max": 892.1,
                "med": 112.3,
                "p90": 245.8,
                "p95": 356.2,
                "p99": 678.4
            },
            "http_req_failed": {
                "count": 5,
                "rate": 0.001
            },
            "http_req_waiting": {
                "avg": 98.2,
                "p95": 280.5
            },
            "iteration_duration": {
                "avg": 625.8,
                "p95": 1250.3
            }
        },
        "thresholds": {
            "http_req_duration{p(95)}": {
                "ok": true,
                "value": 356.2,
                "threshold": 500
            },
            "http_req_failed{rate}": {
                "ok": true,
                "value": 0.001,
                "threshold": 0.01
            }
        },
        "errors": [
            {
                "step_id": "get_users",
                "count": 3,
                "message": "connection timeout"
            },
            {
                "step_id": "create_order",
                "count": 2,
                "message": "status code 500"
            }
        ],
        "started_at": "2026-01-02T10:00:00Z",
        "completed_at": "2026-01-02T10:05:02Z"
    }
}
```

---

### 下载执行报告

**请求**

```
GET /api/v1/executions/{execution_id}/report
```

**查询参数**

| 参数     | 类型   | 描述                            |
| -------- | ------ | ------------------------------- |
| `format` | string | 报告格式: `json`, `html`, `csv` |

**cURL 示例**

```bash
# JSON 报告
curl -o report.json \
  "http://localhost:8080/api/v1/executions/exec-123456/report?format=json"

# HTML 报告
curl -o report.html \
  "http://localhost:8080/api/v1/executions/exec-123456/report?format=html"
```

---

## 实时监控

### WebSocket 实时指标

通过 WebSocket 获取实时执行指标。

**连接**

```
ws://localhost:8080/api/v1/executions/{execution_id}/ws
```

**消息格式**

```json
{
    "type": "metrics",
    "timestamp": "2026-01-02T10:01:30Z",
    "data": {
        "vus": 10,
        "iteration": 150,
        "http_reqs": {
            "count": 1500,
            "rate": 16.67
        },
        "http_req_duration": {
            "avg": 125.5,
            "p95": 356.2
        }
    }
}
```

**JavaScript 示例**

```javascript
const ws = new WebSocket(
    "ws://localhost:8080/api/v1/executions/exec-123456/ws"
);

ws.onmessage = (event) => {
    const data = JSON.parse(event.data);
    console.log("Metrics:", data);
};

ws.onclose = () => {
    console.log("Execution completed");
};
```

---

### Server-Sent Events (SSE)

通过 SSE 获取实时指标。

**请求**

```
GET /api/v1/executions/{execution_id}/stream
Accept: text/event-stream
```

**响应**

```
event: metrics
data: {"vus":10,"http_reqs":{"count":1500,"rate":16.67}}

event: metrics
data: {"vus":10,"http_reqs":{"count":1520,"rate":16.89}}

event: complete
data: {"status":"completed"}
```

**cURL 示例**

```bash
curl -N http://localhost:8080/api/v1/executions/exec-123456/stream
```

---

## Slave 管理

### 获取 Slave 列表

**请求**

```
GET /api/v1/slaves
```

**响应**

```json
{
    "code": 0,
    "message": "success",
    "data": {
        "total": 3,
        "items": [
            {
                "id": "slave-001",
                "address": "192.168.1.10:9091",
                "status": "online",
                "capabilities": ["http_executor", "socket_executor"],
                "labels": {
                    "region": "us-east-1",
                    "zone": "a"
                },
                "resources": {
                    "max_vus": 100,
                    "current_vus": 25,
                    "cpu_usage": 45.2,
                    "memory_usage": 62.8
                },
                "last_heartbeat": "2026-01-02T10:00:55Z"
            }
        ]
    }
}
```

---

### 获取 Slave 详情

**请求**

```
GET /api/v1/slaves/{slave_id}
```

---

### 禁用 Slave

**请求**

```
POST /api/v1/slaves/{slave_id}/disable
```

---

### 启用 Slave

**请求**

```
POST /api/v1/slaves/{slave_id}/enable
```

---

## 系统管理

### 健康检查

**请求**

```
GET /api/v1/health
```

**响应**

```json
{
    "status": "healthy",
    "version": "2.0.0",
    "uptime": "24h30m15s",
    "components": {
        "database": "healthy",
        "grpc": "healthy",
        "scheduler": "healthy"
    }
}
```

---

### 系统指标

**请求**

```
GET /api/v1/metrics
```

**响应**

```json
{
    "code": 0,
    "message": "success",
    "data": {
        "total_workflows": 50,
        "total_executions": 1200,
        "running_executions": 5,
        "total_slaves": 3,
        "online_slaves": 3,
        "total_vus_capacity": 300,
        "current_vus": 50
    }
}
```

---

### Prometheus 指标

**请求**

```
GET /metrics
```

**响应** (Prometheus 格式)

```
# HELP workflow_executions_total Total number of workflow executions
# TYPE workflow_executions_total counter
workflow_executions_total{status="completed"} 1150
workflow_executions_total{status="failed"} 50

# HELP workflow_execution_duration_seconds Workflow execution duration
# TYPE workflow_execution_duration_seconds histogram
workflow_execution_duration_seconds_bucket{le="60"} 500
workflow_execution_duration_seconds_bucket{le="300"} 1000
workflow_execution_duration_seconds_bucket{le="+Inf"} 1200
```

---

## 错误响应

所有 API 在发生错误时返回统一格式：

```json
{
    "code": 1001,
    "message": "workflow not found",
    "details": {
        "workflow_id": "non-existent-workflow"
    }
}
```

### 错误码

| 错误码 | 描述           |
| ------ | -------------- |
| 0      | 成功           |
| 1001   | 工作流不存在   |
| 1002   | 工作流已存在   |
| 1003   | 工作流格式错误 |
| 2001   | 执行不存在     |
| 2002   | 执行已完成     |
| 2003   | 执行参数错误   |
| 3001   | Slave 不存在   |
| 3002   | Slave 离线     |
| 4001   | 认证失败       |
| 4002   | 权限不足       |
| 5001   | 内部错误       |

---

## SDK 示例

### Go SDK

```go
package main

import (
    "fmt"
    "github.com/grafana/k6/workflow-engine/sdk"
)

func main() {
    client := sdk.NewClient("http://localhost:8080")

    // 创建工作流
    workflow := &sdk.Workflow{
        ID:   "my-test",
        Name: "My Test",
        Steps: []sdk.Step{
            {
                ID:   "step1",
                Type: "http",
                Config: map[string]any{
                    "method": "GET",
                    "url":    "https://api.example.com/users",
                },
            },
        },
    }

    err := client.CreateWorkflow(workflow)
    if err != nil {
        panic(err)
    }

    // 执行工作流
    execution, err := client.Execute("my-test", &sdk.ExecuteOptions{
        VUs:      10,
        Duration: "5m",
    })
    if err != nil {
        panic(err)
    }

    // 等待完成
    result, err := client.WaitForCompletion(execution.ID)
    if err != nil {
        panic(err)
    }

    fmt.Printf("Result: %+v\n", result)
}
```

### Python SDK

```python
from workflow_engine import Client

client = Client("http://localhost:8080")

# 创建工作流
workflow = {
    "id": "my-test",
    "name": "My Test",
    "steps": [
        {
            "id": "step1",
            "type": "http",
            "config": {
                "method": "GET",
                "url": "https://api.example.com/users"
            }
        }
    ]
}

client.create_workflow(workflow)

# 执行工作流
execution = client.execute("my-test", vus=10, duration="5m")

# 等待完成
result = client.wait_for_completion(execution.id)
print(f"Result: {result}")
```

### JavaScript/TypeScript SDK

```typescript
import { WorkflowClient } from "@workflow-engine/sdk";

const client = new WorkflowClient("http://localhost:8080");

// 创建工作流
const workflow = {
    id: "my-test",
    name: "My Test",
    steps: [
        {
            id: "step1",
            type: "http",
            config: {
                method: "GET",
                url: "https://api.example.com/users",
            },
        },
    ],
};

await client.createWorkflow(workflow);

// 执行工作流
const execution = await client.execute("my-test", {
    vus: 10,
    duration: "5m",
});

// 等待完成
const result = await client.waitForCompletion(execution.id);
console.log("Result:", result);
```

---

## 附录

### 完整 cURL 示例

```bash
# 1. 创建工作流
curl -X POST http://localhost:8080/api/v1/workflows \
  -H "Content-Type: application/x-yaml" \
  --data-binary @workflow.yaml

# 2. 执行工作流
curl -X POST http://localhost:8080/api/v1/workflows/my-workflow/execute \
  -H "Content-Type: application/json" \
  -d '{"options": {"vus": 10, "duration": "5m"}}'

# 3. 查看执行状态
curl http://localhost:8080/api/v1/executions/exec-123456

# 4. 获取执行结果
curl http://localhost:8080/api/v1/executions/exec-123456/result

# 5. 下载报告
curl -o report.html \
  "http://localhost:8080/api/v1/executions/exec-123456/report?format=html"

# 6. 直接执行 YAML
curl -X POST http://localhost:8080/api/v1/execute \
  -H "Content-Type: application/x-yaml" \
  --data-binary @test.yaml
```
