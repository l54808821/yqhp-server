# Workflow Engine 示例

本目录包含工作流引擎的核心示例。

## 示例列表

| 文件                                                 | 说明                                                      |
| ---------------------------------------------------- | --------------------------------------------------------- |
| [01-basic.yaml](01-basic.yaml)                       | 基础 HTTP 请求 (GET/POST/PUT/DELETE、变量、查询参数)      |
| [02-control-flow.yaml](02-control-flow.yaml)         | 控制流 (条件分支、For/Foreach/While 循环、break/continue) |
| [03-hooks-and-errors.yaml](03-hooks-and-errors.yaml) | 钩子和错误处理 (pre/post hook、abort/continue/skip/retry) |
| [04-performance.yaml](04-performance.yaml)           | 性能测试 (constant-vus、ramping-vus、shared-iterations)   |
| [05-stress-tests.yaml](05-stress-tests.yaml)         | 压力测试 (峰值测试、压力测试、浸泡测试)                   |
| [06-distributed.yaml](06-distributed.yaml)           | 分布式执行 (手动指定、按标签、按能力选择 Slave)           |
| [07-complete.yaml](07-complete.yaml)                 | 完整示例 (综合展示所有功能)                               |

## 快速开始

```bash
# 构建
cd yqhp/workflow-engine
go build -o workflow-engine ./cmd/main.go

# 运行基础示例
./workflow-engine run examples/01-basic.yaml

# 运行性能测试
./workflow-engine run examples/04-performance.yaml

# 覆盖参数
./workflow-engine run -vus 20 -duration 2m examples/04-performance.yaml
```

## 执行模式

| 模式                | 说明                     |
| ------------------- | ------------------------ |
| `constant-vus`      | 固定数量的 VU 持续运行   |
| `ramping-vus`       | 按阶段调整 VU 数量       |
| `shared-iterations` | 所有 VU 共享总迭代次数   |
| `per-vu-iterations` | 每个 VU 执行固定迭代次数 |

## 错误处理策略

| 策略       | 说明                          |
| ---------- | ----------------------------- |
| `abort`    | 遇到错误停止整个工作流 (默认) |
| `continue` | 遇到错误继续执行下一步        |
| `skip`     | 跳过当前步骤                  |
| `retry`    | 重试当前步骤                  |

## HTTP 引擎

| 引擎       | 说明                       |
| ---------- | -------------------------- |
| `fasthttp` | 默认，高性能，适合压测     |
| `standard` | 标准库，兼容性好，适合调试 |

## 分布式运行

```bash
# 终端 1: 启动 Master
./workflow-engine master start

# 终端 2: 启动 Slave
./workflow-engine slave start --master localhost:9090 --labels region=cn-east

# 终端 3: 提交工作流
curl -X POST http://localhost:8080/api/v1/workflows \
  -H "Content-Type: application/json" \
  -d '{"yaml": "'"$(cat examples/06-distributed.yaml)"'"}'
```
