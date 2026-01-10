# Workflow Engine REST API 文档

## 概述

Workflow Engine 提供 REST API 用于工作流的创建、执行、监控和管理。

### 基础信息

- **基础 URL**: `http://localhost:8080`
- **API 版本**: v1
- **API 前缀**: `/api/v1`

### 认证

如果启用了认证，支持以下方式：

**API Key 认证**:

```
X-API-Key: <your-api-key>
```

或通过查询参数:

```
?api_key=<your-api-key>
```

**JWT 认证**:

```
Authorization: Bearer <token>
```

---

## 健康检查

### GET /health

检查服务健康状态。

**响应**

```json
{
  "status": "healthy",
  "timestamp": "2026-01-05T10:00:00Z"
}
```

### GET /ready

检查服务就绪状态。

**响应**

```json
{
  "ready": true,
  "status": "ready",
  "timestamp": "2026-01-05T10:00:00Z"
}
```

### GET /api/v1/health

同 `/health`。

### GET /api/v1/ready

同 `/ready`。

---

## 工作流管理

### POST /api/v1/workflows

提交并执行工作流。

**请求头**

```
Content-Type: application/json
```

**请求体**

方式一：提交 YAML 字符串

```json
{
  "yaml": "id: test\nname: Test\nsteps:\n  - id: s1\n    type: http\n    config:\n      method: GET\n      url: https://httpbin.org/get"
}
```

方式二：提交 Workflow 对象

```json
{
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
}
```

**响应** (201 Created)

```json
{
  "execution_id": "exec-abc123",
  "workflow_id": "test",
  "status": "submitted"
}
```

**错误响应**

```json
{
  "error": "invalid_request",
  "message": "Either 'workflow' or 'yaml' must be provided"
}
```

**cURL 示例**

```bash
# 提交 YAML
curl -X POST http://localhost:8080/api/v1/workflows \
  -H "Content-Type: application/json" \
  -d '{
    "yaml": "id: test\nname: Test\nsteps:\n  - id: s1\n    type: http\n    config:\n      method: GET\n      url: https://httpbin.org/get"
  }'

# 提交 Workflow 对象
curl -X POST http://localhost:8080/api/v1/workflows \
  -H "Content-Type: application/json" \
  -d '{
    "workflow": {
      "id": "test",
      "name": "Test",
      "steps": [{"id": "s1", "type": "http", "config": {"method": "GET", "url": "https://httpbin.org/get"}}]
    }
  }'
```

### GET /api/v1/workflows/:id

获取工作流信息。

**路径参数**

| 参数 | 类型   | 说明      |
| ---- | ------ | --------- |
| `id` | string | 工作流 ID |

**响应**

```json
{
  "id": "test",
  "status": "running"
}
```

**cURL 示例**

```bash
curl http://localhost:8080/api/v1/workflows/test
```

### DELETE /api/v1/workflows/:id

停止工作流执行。

**路径参数**

| 参数 | 类型   | 说明      |
| ---- | ------ | --------- |
| `id` | string | 工作流 ID |

**响应**

```json
{
  "success": true,
  "message": "Workflow stopped successfully"
}
```

**cURL 示例**

```bash
curl -X DELETE http://localhost:8080/api/v1/workflows/test
```

---

## 执行管理

### GET /api/v1/executions

列出所有执行。

**响应**

```json
{
  "executions": [
    {
      "id": "exec-abc123",
      "workflow_id": "test",
      "status": "running",
      "progress": 0.5,
      "start_time": "2026-01-05T10:00:00Z"
    }
  ],
  "total": 1
}
```

**cURL 示例**

```bash
curl http://localhost:8080/api/v1/executions
```

### GET /api/v1/executions/:id

获取执行状态。

**路径参数**

| 参数 | 类型   | 说明    |
| ---- | ------ | ------- |
| `id` | string | 执行 ID |

**响应**

```json
{
  "id": "exec-abc123",
  "workflow_id": "test",
  "status": "running",
  "progress": 0.5,
  "start_time": "2026-01-05T10:00:00Z",
  "end_time": "",
  "slave_states": {
    "slave-1": {
      "slave_id": "slave-1",
      "status": "running",
      "completed_vus": 5,
      "completed_iters": 50,
      "segment_start": 0,
      "segment_end": 0.5
    }
  },
  "errors": []
}
```

**执行状态**

| 状态        | 说明     |
| ----------- | -------- |
| `pending`   | 等待执行 |
| `running`   | 执行中   |
| `paused`    | 已暂停   |
| `completed` | 执行完成 |
| `failed`    | 执行失败 |
| `aborted`   | 已中止   |

**cURL 示例**

```bash
curl http://localhost:8080/api/v1/executions/exec-abc123
```

### POST /api/v1/executions/:id/pause

暂停执行。

**路径参数**

| 参数 | 类型   | 说明    |
| ---- | ------ | ------- |
| `id` | string | 执行 ID |

**响应**

```json
{
  "success": true,
  "message": "Execution paused successfully"
}
```

**cURL 示例**

```bash
curl -X POST http://localhost:8080/api/v1/executions/exec-abc123/pause
```

### POST /api/v1/executions/:id/resume

恢复执行。

**路径参数**

| 参数 | 类型   | 说明    |
| ---- | ------ | ------- |
| `id` | string | 执行 ID |

**响应**

```json
{
  "success": true,
  "message": "Execution resumed successfully"
}
```

**cURL 示例**

```bash
curl -X POST http://localhost:8080/api/v1/executions/exec-abc123/resume
```

### POST /api/v1/executions/:id/scale

扩缩执行的 VU 数量。

**路径参数**

| 参数 | 类型   | 说明    |
| ---- | ------ | ------- |
| `id` | string | 执行 ID |

**请求体**

```json
{
  "target_vus": 50
}
```

**响应**

```json
{
  "success": true,
  "message": "Execution scaled successfully"
}
```

**cURL 示例**

```bash
curl -X POST http://localhost:8080/api/v1/executions/exec-abc123/scale \
  -H "Content-Type: application/json" \
  -d '{"target_vus": 50}'
```

### DELETE /api/v1/executions/:id

停止执行。

**路径参数**

| 参数 | 类型   | 说明    |
| ---- | ------ | ------- |
| `id` | string | 执行 ID |

**响应**

```json
{
  "success": true,
  "message": "Execution stopped successfully"
}
```

**cURL 示例**

```bash
curl -X DELETE http://localhost:8080/api/v1/executions/exec-abc123
```

---

## 指标

### GET /api/v1/executions/:id/metrics

获取执行指标。

**路径参数**

| 参数 | 类型   | 说明    |
| ---- | ------ | ------- |
| `id` | string | 执行 ID |

**响应**

```json
{
  "execution_id": "exec-abc123",
  "total_vus": 10,
  "total_iterations": 100,
  "duration": "5m0s",
  "step_metrics": {
    "step1": {
      "step_id": "step1",
      "count": 100,
      "success_count": 98,
      "failure_count": 2,
      "duration": {
        "min": "45ms",
        "max": "892ms",
        "avg": "125ms",
        "p50": "112ms",
        "p90": "245ms",
        "p95": "356ms",
        "p99": "678ms"
      }
    }
  },
  "thresholds": [
    {
      "metric": "http_req_duration",
      "condition": "p(95) < 500",
      "passed": true,
      "value": 356.2
    }
  ]
}
```

**cURL 示例**

```bash
curl http://localhost:8080/api/v1/executions/exec-abc123/metrics
```

---

## Slave 管理

### GET /api/v1/slaves

列出所有 Slave 节点。

**响应**

```json
{
  "slaves": [
    {
      "id": "slave-1",
      "type": "worker",
      "address": "192.168.1.10:9091",
      "capabilities": ["http_executor", "script_executor"],
      "labels": {
        "region": "us-east"
      },
      "status": {
        "state": "online",
        "load": 0.45,
        "active_tasks": 5,
        "last_seen": "2026-01-05T10:00:55Z"
      }
    }
  ],
  "total": 1
}
```

**cURL 示例**

```bash
curl http://localhost:8080/api/v1/slaves
```

### GET /api/v1/slaves/:id

获取 Slave 详情。

**路径参数**

| 参数 | 类型   | 说明     |
| ---- | ------ | -------- |
| `id` | string | Slave ID |

**响应**

```json
{
  "id": "slave-1",
  "type": "worker",
  "address": "192.168.1.10:9091",
  "capabilities": ["http_executor", "script_executor"],
  "labels": {
    "region": "us-east"
  },
  "status": {
    "state": "online",
    "load": 0.45,
    "active_tasks": 5,
    "last_seen": "2026-01-05T10:00:55Z"
  }
}
```

**cURL 示例**

```bash
curl http://localhost:8080/api/v1/slaves/slave-1
```

### POST /api/v1/slaves/:id/drain

排空 Slave 节点 (停止接收新任务，等待现有任务完成)。

**路径参数**

| 参数 | 类型   | 说明     |
| ---- | ------ | -------- |
| `id` | string | Slave ID |

**响应**

```json
{
  "success": true,
  "message": "Slave drain initiated successfully"
}
```

**cURL 示例**

```bash
curl -X POST http://localhost:8080/api/v1/slaves/slave-1/drain
```

---

## WebSocket

### WS /api/v1/executions/:id/ws

通过 WebSocket 获取实时执行指标。

**连接**

```
ws://localhost:8080/api/v1/executions/{execution_id}/ws
```

**消息格式**

```json
{
  "type": "metrics",
  "timestamp": "2026-01-05T10:01:30Z",
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
  "ws://localhost:8080/api/v1/executions/exec-abc123/ws"
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

## 错误响应

所有 API 在发生错误时返回统一格式：

```json
{
  "error": "error_code",
  "message": "Error description"
}
```

### HTTP 状态码

| 状态码 | 说明           |
| ------ | -------------- |
| 200    | 成功           |
| 201    | 创建成功       |
| 400    | 请求参数错误   |
| 401    | 未授权         |
| 404    | 资源不存在     |
| 500    | 服务器内部错误 |
| 501    | 功能未实现     |

### 常见错误

| 错误码              | 说明           |
| ------------------- | -------------- |
| `invalid_request`   | 请求参数无效   |
| `invalid_workflow`  | 工作流格式错误 |
| `not_found`         | 资源不存在     |
| `submission_failed` | 提交失败       |
| `pause_failed`      | 暂停失败       |
| `resume_failed`     | 恢复失败       |
| `scale_failed`      | 扩缩失败       |
| `stop_failed`       | 停止失败       |
| `unauthorized`      | 未授权         |
| `internal_error`    | 内部错误       |

---

## 完整示例

### 提交并监控工作流

```bash
# 1. 提交工作流
RESPONSE=$(curl -s -X POST http://localhost:8080/api/v1/workflows \
  -H "Content-Type: application/json" \
  -d '{
    "workflow": {
      "id": "test",
      "name": "Test",
      "options": {"vus": 5, "iterations": 10},
      "steps": [{"id": "s1", "type": "http", "config": {"method": "GET", "url": "https://httpbin.org/get"}}]
    }
  }')

EXECUTION_ID=$(echo $RESPONSE | jq -r '.execution_id')
echo "Execution ID: $EXECUTION_ID"

# 2. 查看执行状态
curl http://localhost:8080/api/v1/executions/$EXECUTION_ID

# 3. 获取指标
curl http://localhost:8080/api/v1/executions/$EXECUTION_ID/metrics

# 4. 停止执行 (如需要)
curl -X DELETE http://localhost:8080/api/v1/executions/$EXECUTION_ID
```

### 分布式执行

```bash
# 1. 启动 Master
./workflow-engine master start

# 2. 启动 Slave (另一个终端)
./workflow-engine slave start --master http://localhost:8080

# 3. 查看 Slave 列表
curl http://localhost:8080/api/v1/slaves

# 4. 提交工作流
curl -X POST http://localhost:8080/api/v1/workflows \
  -H "Content-Type: application/json" \
  -d '{
    "workflow": {
      "id": "distributed-test",
      "name": "Distributed Test",
      "options": {"vus": 100, "duration": "5m"},
      "steps": [{"id": "s1", "type": "http", "config": {"method": "GET", "url": "https://httpbin.org/get"}}]
    }
  }'
```
