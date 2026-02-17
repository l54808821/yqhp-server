# 知识库模块 — 向量数据库环境搭建与使用手册

## 目录

- [1. 概述](#1-概述)
- [2. Qdrant 向量数据库环境搭建](#2-qdrant-向量数据库环境搭建)
  - [2.1 Docker 单节点部署（开发环境）](#21-docker-单节点部署开发环境)
  - [2.2 Docker Compose 持久化部署（生产环境）](#22-docker-compose-持久化部署生产环境)
  - [2.3 Qdrant Cloud 托管方案](#23-qdrant-cloud-托管方案)
  - [2.4 端口与管理面板](#24-端口与管理面板)
  - [2.5 健康检查](#25-健康检查)
- [3. 项目配置对接](#3-项目配置对接)
  - [3.1 配置文件说明](#31-配置文件说明)
  - [3.2 Go SDK 依赖安装](#32-go-sdk-依赖安装)
  - [3.3 Collection 命名规范](#33-collection-命名规范)
- [4. 数据库迁移](#4-数据库迁移)
  - [4.1 执行 SQL 迁移](#41-执行-sql-迁移)
  - [4.2 GORM Gen 模型生成](#42-gorm-gen-模型生成)
- [5. 嵌入模型配置](#5-嵌入模型配置)
  - [5.1 支持的 Embedding 模型](#51-支持的-embedding-模型)
  - [5.2 维度对照表](#52-维度对照表)
  - [5.3 在模型管理中添加 Embedding 模型](#53-在模型管理中添加-embedding-模型)
- [6. 知识库使用指南](#6-知识库使用指南)
  - [6.1 创建知识库](#61-创建知识库)
  - [6.2 上传文档](#62-上传文档)
  - [6.3 文档处理流程](#63-文档处理流程)
  - [6.4 检索测试](#64-检索测试)
  - [6.5 在 AI 节点中挂载知识库](#65-在-ai-节点中挂载知识库)
- [7. 检索参数调优](#7-检索参数调优)
  - [7.1 Top-K 检索数量](#71-top-k-检索数量)
  - [7.2 相似度阈值](#72-相似度阈值)
  - [7.3 分块策略](#73-分块策略)
  - [7.4 混合检索模式](#74-混合检索模式)
- [8. Qdrant 运维指南](#8-qdrant-运维指南)
  - [8.1 备份与恢复](#81-备份与恢复)
  - [8.2 性能优化](#82-性能优化)
  - [8.3 监控](#83-监控)
- [9. Neo4j 图数据库（Phase 3）](#9-neo4j-图数据库phase-3)
- [10. 常见问题 FAQ](#10-常见问题-faq)

---

## 1. 概述

知识库模块为 AI 节点提供外部知识检索能力，支持两种知识库类型：

| 类型 | 存储引擎 | 适用场景 | 当前状态 |
|------|---------|---------|---------|
| **普通知识库** | Qdrant（向量数据库） | 文档语义检索、RAG 增强生成 | Phase 1 已实现 |
| **图知识库** | Neo4j（图数据库） | 实体关系推理、知识图谱问答 | Phase 3 规划中 |

**架构概览：**

```
用户上传文档 → 文本提取 → 文本分块 → Embedding 生成 → Qdrant 向量写入
                                                          ↓
AI 节点执行 → 用户提示词 → Embedding 查询 → Qdrant 相似度搜索 → 上下文注入 → LLM 生成回答
                                                                    ↓
                                                          knowledge_search 工具（AI 主动检索）
```

**检索策略（混合模式）：**

1. **上下文注入**：AI 调用前，系统自动用用户提示词检索知识库，将相关内容注入到系统提示词的 `[参考知识]` 段中（经典 RAG）。
2. **工具检索**：同时注册 `knowledge_search` 工具，AI 可在对话过程中主动调用，进行更精确的追问检索。

---

## 2. Qdrant 向量数据库环境搭建

### 2.1 Docker 单节点部署（开发环境）

最快的方式，适合本地开发和测试：

```bash
docker run -d \
  --name qdrant \
  -p 6333:6333 \
  -p 6334:6334 \
  qdrant/qdrant:latest
```

启动后即可使用，数据保存在容器内（容器删除后数据丢失）。

### 2.2 Docker Compose 持久化部署（生产环境）

创建 `docker-compose-qdrant.yml`：

```yaml
version: '3.8'

services:
  qdrant:
    image: qdrant/qdrant:latest
    container_name: qdrant
    restart: always
    ports:
      - "6333:6333"   # REST API
      - "6334:6334"   # gRPC API
    volumes:
      - qdrant_data:/qdrant/storage    # 数据持久化
      - ./qdrant-config.yaml:/qdrant/config/production.yaml  # 自定义配置（可选）
    environment:
      - QDRANT__SERVICE__GRPC_PORT=6334
    deploy:
      resources:
        limits:
          memory: 2G

volumes:
  qdrant_data:
    driver: local
```

启动：

```bash
docker-compose -f docker-compose-qdrant.yml up -d
```

**可选的 Qdrant 自定义配置 `qdrant-config.yaml`：**

```yaml
storage:
  # 数据存储目录
  storage_path: /qdrant/storage

  # WAL 日志
  wal:
    wal_capacity_mb: 64

  # 性能调优
  performance:
    max_search_threads: 0  # 0 = 自动检测 CPU 核数
    max_optimization_threads: 2

service:
  # API Key 认证（生产环境建议开启）
  # api_key: "your-secret-api-key"

  # 开启 gRPC
  grpc_port: 6334
  enable_tls: false

# 日志级别
log_level: INFO
```

### 2.3 Qdrant Cloud 托管方案

如果不想自行运维，可以使用 Qdrant Cloud：

1. 访问 [https://cloud.qdrant.io](https://cloud.qdrant.io) 注册账号
2. 创建 Cluster（免费版支持 1GB 存储）
3. 获取连接信息：
   - **URL**: `https://your-cluster-id.aws.cloud.qdrant.io:6334`
   - **API Key**: 在 Dashboard 中生成

配置项需要修改为：

```yaml
qdrant:
  host: your-cluster-id.aws.cloud.qdrant.io
  port: 6334
  api_key: "your-qdrant-cloud-api-key"
  use_tls: true
```

### 2.4 端口与管理面板

| 端口 | 协议 | 用途 |
|------|------|------|
| 6333 | HTTP/REST | REST API，浏览器可直接访问 |
| 6334 | gRPC | gRPC API，Go SDK 使用此端口 |

**Web 管理面板（Qdrant Dashboard）：**

浏览器访问 `http://localhost:6333/dashboard` 即可打开 Qdrant 内置的管理面板，可以：
- 查看所有 Collections
- 浏览和搜索向量数据
- 查看集群状态
- 执行 API 请求

### 2.5 健康检查

```bash
# REST 健康检查
curl http://localhost:6333/healthz

# 查看集群信息
curl http://localhost:6333/cluster

# 查看所有 Collections
curl http://localhost:6333/collections
```

---

## 3. 项目配置对接

### 3.1 配置文件说明

在 `yqhp/gulu/config/config.yml` 中配置 Qdrant 连接信息：

```yaml
# Qdrant 向量数据库配置
qdrant:
  host: 127.0.0.1         # Qdrant 服务地址
  port: 6334               # gRPC 端口（非 REST 端口）
  api_key: ""              # API Key（可选，Qdrant Cloud 或开启认证时需要）
  use_tls: false           # 是否使用 TLS（Qdrant Cloud 需要设为 true）
  collection_prefix: "kb_" # Collection 名称前缀
```

**配置项说明：**

| 配置项 | 类型 | 默认值 | 说明 |
|--------|------|--------|------|
| `host` | string | `127.0.0.1` | Qdrant 服务器地址 |
| `port` | int | `6334` | gRPC 端口。注意是 6334 而不是 6333 |
| `api_key` | string | `""` | 认证密钥，本地开发可留空 |
| `use_tls` | bool | `false` | 是否启用 TLS 加密连接 |
| `collection_prefix` | string | `"kb_"` | Collection 名称前缀，避免与其他应用冲突 |

对应的 Go 配置结构体在 `internal/config/config.go` 中：

```go
type QdrantConfig struct {
    Host             string `yaml:"host"`
    Port             int    `yaml:"port"`
    APIKey           string `yaml:"api_key"`
    UseTLS           bool   `yaml:"use_tls"`
    CollectionPrefix string `yaml:"collection_prefix"`
}
```

### 3.2 Go SDK 依赖安装

项目使用 Qdrant 官方 Go gRPC Client：

```bash
cd yqhp
go get github.com/qdrant/go-client
```

基本使用示例：

```go
import (
    "context"
    qdrant "github.com/qdrant/go-client/qdrant"
)

// 创建客户端
client, err := qdrant.NewClient(&qdrant.Config{
    Host:   "localhost",
    Port:   6334,
    APIKey: "",     // 可选
    UseTLS: false,  // 可选
})
defer client.Close()

// 创建 Collection
err = client.CreateCollection(ctx, &qdrant.CreateCollection{
    CollectionName: "kb_1",
    VectorsConfig: qdrant.NewVectorsConfigMap(
        map[string]*qdrant.VectorParams{
            "text": {
                Size:     1536,
                Distance: qdrant.Distance_Cosine,
            },
        },
    ),
})

// 写入向量
_, err = client.Upsert(ctx, &qdrant.UpsertPoints{
    CollectionName: "kb_1",
    Points: []*qdrant.PointStruct{
        {
            Id:      qdrant.NewIDNum(1),
            Vectors: qdrant.NewVectorsMap(map[string]*qdrant.Vector{
                "text": qdrant.NewVector(embedding...),
            }),
            Payload: qdrant.NewValueMap(map[string]any{
                "content":     "文档内容片段...",
                "document_id": 42,
                "chunk_index": 0,
            }),
        },
    },
})

// 相似度搜索
results, err := client.Query(ctx, &qdrant.QueryPoints{
    CollectionName: "kb_1",
    Query:          qdrant.NewQueryDense(queryVector),
    Using:          qdrant.PtrOf("text"),
    Limit:          qdrant.PtrOf(uint64(5)),
    ScoreThreshold: qdrant.PtrOf(float32(0.7)),
    WithPayload:    qdrant.NewWithPayload(true),
})
```

### 3.3 Collection 命名规范

每个知识库对应一个独立的 Qdrant Collection，命名规则：

```
{collection_prefix}{knowledge_base_id}
```

例如：配置前缀为 `kb_`，知识库 ID 为 `123`，则 Collection 名为 `kb_123`。

Collection 在创建知识库时自动创建，删除知识库时自动清理。

**Named Vectors 设计：**

当前使用 Named Vectors 方案，预留多模态扩展空间：

| 向量字段名 | 用途 | 当前状态 |
|-----------|------|---------|
| `text` | 文本语义向量 | 已实现 |
| `image` | 图片向量 | Phase 2 |
| `audio` | 音频向量 | Phase 2 |

**Payload 结构：**

每个向量点（Point）携带以下 Payload：

```json
{
  "content": "文本分块内容",
  "document_id": 42,
  "document_name": "产品说明书.pdf",
  "chunk_index": 3,
  "total_chunks": 15,
  "modality": "text"
}
```

---

## 4. 数据库迁移

### 4.1 执行 SQL 迁移

知识库模块需要两张 MySQL 元数据表，迁移脚本位于 `migrations/knowledge_base.sql`：

```bash
# 方式 1：直接执行 SQL 文件
mysql -u root -p yqhp_admin < yqhp/gulu/migrations/knowledge_base.sql

# 方式 2：登录 MySQL 后执行
mysql -u root -p
USE yqhp_admin;
SOURCE /path/to/yqhp/gulu/migrations/knowledge_base.sql;
```

创建的表：
- `t_knowledge_base` — 知识库主表（名称、类型、嵌入模型配置、分块策略等）
- `t_knowledge_document` — 知识库文档表（文档名称、处理状态、分块数等）

### 4.2 GORM Gen 模型生成

SQL 执行成功后，重新生成 GORM 模型和查询代码：

```bash
cd yqhp/gulu
go run cmd/gen/main.go
```

生成的文件：
- `internal/model/t_knowledge_base.gen.go`
- `internal/model/t_knowledge_document.gen.go`
- `internal/query/t_knowledge_base.gen.go`
- `internal/query/t_knowledge_document.gen.go`

> **注意：** 项目中已经预置了手写的模型和查询文件。如果运行 Gen 命令，会自动覆盖为完整版本。

---

## 5. 嵌入模型配置

### 5.1 支持的 Embedding 模型

知识库使用 Embedding 模型将文本/图片/音视频转换为向量。系统通过 OpenAI-compatible API 调用嵌入模型，支持以下提供商：

| 提供商 | 模型名称 | 维度 | 说明 |
|--------|---------|------|------|
| OpenAI | `text-embedding-3-small` | 1536 | 性价比最高，推荐 |
| OpenAI | `text-embedding-3-large` | 3072 | 精度更高，成本更高 |
| OpenAI | `text-embedding-ada-002` | 1536 | 旧版模型 |
| 硅基流动 | `BAAI/bge-m3` | 1024 | 中文优化，免费 |
| 硅基流动 | `BAAI/bge-large-zh-v1.5` | 1024 | 中文优化 |
| 阿里通义 | `text-embedding-v3` | 1024 | 中文效果好 |
| 本地部署 | Ollama + `nomic-embed-text` | 768 | 完全离线，免费 |
| 本地部署 | Ollama + `mxbai-embed-large` | 1024 | 完全离线，精度较高 |

### 5.2 维度对照表

> **重要：** 创建知识库时设置的「向量维度」必须与所选嵌入模型的输出维度一致，否则写入 Qdrant 时会报错。

常用维度速查：

| 维度 | 对应模型 |
|------|---------|
| 768 | nomic-embed-text, all-MiniLM-L6-v2 |
| 1024 | bge-m3, bge-large-zh, text-embedding-v3 |
| 1536 | text-embedding-3-small, text-embedding-ada-002 |
| 3072 | text-embedding-3-large |

### 5.3 在模型管理中添加 Embedding 模型

1. 进入项目 → 侧边栏「模型管理」
2. 点击「新增模型」
3. 填写信息：
   - **提供商**：选择对应提供商（如 OpenAI、硅基流动）
   - **模型 ID**：填写模型标识（如 `text-embedding-3-small`）
   - **显示名称**：便于识别的名称（如 `OpenAI Embedding Small`）
   - **API Key**：提供商的 API Key
   - **Base URL**：自定义 API 地址（本地 Ollama 填 `http://localhost:11434/v1`）
4. 保存并启用

创建知识库时，在「嵌入模型」下拉框中选择刚添加的模型即可。

---

## 6. 知识库使用指南

### 6.1 创建知识库

1. 进入项目 → 侧边栏「知识库」
2. 点击「新建知识库」
3. 在「基本信息」标签页填写：
   - **名称**：知识库名称（如「产品文档库」）
   - **描述**：用途说明
   - **类型**：选择「普通知识库（向量检索）」
   - **嵌入模型**：选择已配置的 Embedding 模型
4. 在「检索参数」标签页调整（可保持默认）：
   - **向量维度**：与嵌入模型保持一致
   - **分块大小**：默认 500 字符
   - **分块重叠**：默认 50 字符
   - **检索数量 Top-K**：默认 5
   - **相似度阈值**：默认 0.7
5. 点击「创建」

创建成功后，系统会自动在 Qdrant 中创建对应的 Collection。

### 6.2 上传文档

1. 在知识库列表中，点击卡片右上角菜单 → 「管理文档」
2. 点击「上传文档」按钮
3. 选择文件（支持多选）

**支持的文件格式：**

| 类型 | 格式 | 说明 |
|------|------|------|
| 文本 | `.txt`, `.md`, `.csv`, `.json` | 直接解析 |
| 文档 | `.pdf`, `.docx`, `.doc`, `.html` | 需要解析库 |
| 图片 | `.png`, `.jpg`, `.jpeg`, `.gif`, `.webp` | Phase 2（需 OCR/多模态模型） |
| 音频 | `.mp3`, `.wav`, `.m4a` | Phase 2（需语音转写） |
| 视频 | `.mp4`, `.avi`, `.mov` | Phase 2（提取音轨后转写） |

**也可以直接提交文本内容**（适合小段知识、FAQ 等），通过 API 调用 POST 请求传入 `content` 字段。

### 6.3 文档处理流程

上传后，文档经历以下处理步骤（异步执行）：

```
上传 → [pending] → 文本提取 → [processing] → 文本分块 → Embedding 生成 → Qdrant 写入 → [ready]
                                                                                    ↓
                                                                              失败 → [failed]（可重试）
```

**状态说明：**

| 状态 | 含义 |
|------|------|
| `pending` | 已上传，等待处理 |
| `processing` | 正在处理（提取/分块/嵌入） |
| `ready` | 处理完成，可以被检索 |
| `failed` | 处理失败，可查看错误信息并重试 |

如果文档处理失败，可以在文档列表中点击「重试」按钮重新处理。

### 6.4 检索测试

知识库创建并上传文档后，可以通过 API 进行检索测试：

```bash
curl -X POST http://localhost:5321/api/knowledge-bases/{id}/search \
  -H "Content-Type: application/json" \
  -H "satoken: your-token" \
  -d '{
    "query": "如何配置数据库连接？",
    "top_k": 5,
    "score": 0.6
  }'
```

返回结果：

```json
{
  "code": 0,
  "data": [
    {
      "content": "数据库连接配置说明：在 config.yml 中...",
      "score": 0.89,
      "document_id": 42,
      "chunk_index": 3
    },
    ...
  ]
}
```

### 6.5 在 AI 节点中挂载知识库

1. 在工作流编辑器中选择一个 AI 节点
2. 切换到「知识库」标签页
3. 点击「+ 添加知识库」
4. 在弹窗中选择要挂载的知识库
5. 根据需要调整检索参数：
   - **检索数量 Top-K**：每次检索返回的最大结果数
   - **相似度阈值**：低于此值的结果将被过滤

**挂载后的行为：**

- AI 节点执行时，系统会自动用用户提示词检索知识库，将结果注入系统提示词
- 同时注册 `knowledge_search` 工具，AI 可主动调用进行更精确的检索
- 在工具面板中可以看到 `knowledge_search` 工具已自动添加

---

## 7. 检索参数调优

### 7.1 Top-K 检索数量

**含义：** 每次检索返回的最大结果数量。

| 场景 | 建议值 | 说明 |
|------|--------|------|
| 简短问答 | 3 | 减少干扰信息 |
| 一般检索 | 5（默认） | 平衡精确度和召回率 |
| 深度分析 | 10-15 | 尽可能多地获取相关信息 |
| 综合报告 | 15-20 | 需要广泛参考时使用 |

**注意：** Top-K 过大会导致：
- 上下文过长，占用 Token 额度
- 引入不相关信息，降低回答质量
- 增加响应延迟

### 7.2 相似度阈值

**含义：** 向量余弦相似度的最低门槛，0-1 之间，越高越严格。

| 阈值 | 效果 | 适用场景 |
|------|------|---------|
| 0.5 | 宽松，召回多但可能有噪音 | 探索性检索 |
| 0.6 | 较宽松 | 一般场景 |
| 0.7（默认） | 平衡 | 大多数场景推荐 |
| 0.8 | 较严格 | 精确匹配 |
| 0.9 | 非常严格，可能漏检 | 只要高度相关的结果 |

### 7.3 分块策略

**分块大小（chunk_size）：**

| 值 | 效果 |
|----|------|
| 200-300 | 粒度细，精确度高，但可能丢失上下文 |
| 500（默认） | 平衡选择 |
| 800-1000 | 上下文丰富，但检索精确度降低 |

**分块重叠（chunk_overlap）：**

| 值 | 效果 |
|----|------|
| 0 | 无重叠，可能在边界丢失信息 |
| 50（默认） | 轻度重叠，缓解边界问题 |
| 100-150 | 中度重叠，适合关键信息分散的文档 |

**建议：**
- 中文文档：chunk_size=500, overlap=50
- 英文文档：chunk_size=800, overlap=100
- 代码文档：chunk_size=1000, overlap=200（保留更多上下文）
- FAQ/短文本：chunk_size=200, overlap=0

### 7.4 混合检索模式

知识库采用混合检索策略：

**1. 上下文注入（自动触发）**

```
用户提示词 → Embedding → Qdrant 搜索 → Top-K 结果注入系统提示词

系统提示词中添加：
[参考知识]
以下是从知识库中检索到的相关参考资料，请结合这些信息来回答用户的问题：

--- 参考 1 (来源: 产品手册.pdf, 相关度: 0.92) ---
...内容...

--- 参考 2 (来源: API文档.md, 相关度: 0.85) ---
...内容...
```

**2. 工具检索（AI 主动触发）**

```
AI 发现初始知识不够 → 调用 knowledge_search 工具 → 传入更精确的查询 → 获取补充知识 → 继续生成回答
```

两者互补：上下文注入保证 AI 「看到」相关知识，工具检索让 AI 能「主动追问」知识库。

---

## 8. Qdrant 运维指南

### 8.1 备份与恢复

**创建快照（备份）：**

```bash
# 备份单个 Collection
curl -X POST http://localhost:6333/collections/kb_1/snapshots

# 备份整个 Qdrant 实例
curl -X POST http://localhost:6333/snapshots
```

**恢复快照：**

```bash
# 从快照恢复 Collection
curl -X PUT http://localhost:6333/collections/kb_1/snapshots/recover \
  -H "Content-Type: application/json" \
  -d '{"location": "file:///qdrant/snapshots/kb_1-snapshot.snapshot"}'
```

**Docker Volume 备份：**

```bash
# 备份整个数据目录
docker cp qdrant:/qdrant/storage ./qdrant-backup-$(date +%Y%m%d)

# 恢复
docker cp ./qdrant-backup-20260217/storage qdrant:/qdrant/
docker restart qdrant
```

### 8.2 性能优化

**索引优化：**

```bash
# 为 Collection 创建 HNSW 索引（默认已创建，可调参）
curl -X PUT http://localhost:6333/collections/kb_1 \
  -H "Content-Type: application/json" \
  -d '{
    "hnsw_config": {
      "m": 16,
      "ef_construct": 100
    }
  }'
```

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `m` | 16 | 图连接数，越大越精确但越慢 |
| `ef_construct` | 100 | 构建时搜索宽度，影响索引质量 |

**内存优化：**

对于大规模知识库，可以启用磁盘存储：

```bash
curl -X PUT http://localhost:6333/collections/kb_1 \
  -H "Content-Type: application/json" \
  -d '{
    "optimizers_config": {
      "memmap_threshold": 20000
    }
  }'
```

### 8.3 监控

**查看 Collection 统计：**

```bash
curl http://localhost:6333/collections/kb_1
```

返回的关键指标：
- `vectors_count` — 向量总数
- `indexed_vectors_count` — 已索引的向量数
- `points_count` — 数据点总数
- `segments_count` — 存储段数
- `status` — Collection 状态（green/yellow/red）

**Prometheus 指标：**

Qdrant 内置 Prometheus 指标端点：

```
http://localhost:6333/metrics
```

可接入 Grafana 进行可视化监控。

---

## 9. Neo4j 图数据库（Phase 3）

> 以下为 Phase 3 规划内容，当前尚未实现。

### 部署方式

```bash
docker run -d \
  --name neo4j \
  -p 7474:7474 \
  -p 7687:7687 \
  -e NEO4J_AUTH=neo4j/your-password \
  -v neo4j_data:/data \
  neo4j:5
```

### 配置说明

在 `config.yml` 中取消注释并配置：

```yaml
neo4j:
  uri: bolt://127.0.0.1:7687
  username: neo4j
  password: your-password
  database: neo4j
```

### 图知识库工作流程

```
文档上传 → 文本提取 → LLM 实体/关系抽取 → Neo4j 图写入
                                              ↓
AI 检索 → 实体识别 → 图遍历（邻居/路径查询）→ 结构化知识注入 → LLM 生成
```

---

## 10. 常见问题 FAQ

### Q1: 创建知识库时提示「Qdrant Collection 创建失败」

**原因：** Qdrant 服务未启动或连接配置错误。

**解决：**
1. 确认 Qdrant 已启动：`curl http://localhost:6333/healthz`
2. 检查 `config.yml` 中 `qdrant.host` 和 `qdrant.port` 是否正确
3. 注意端口是 **6334**（gRPC）而不是 6333（REST）

### Q2: 文档上传后一直是「处理中」状态

**原因：** Embedding 模型调用失败或网络超时。

**解决：**
1. 检查知识库关联的 Embedding 模型是否配置正确
2. 确认 API Key 有效且有余额
3. 查看服务端日志中 `[ERROR] 文档处理失败` 相关信息
4. 点击文档列表中的「重试」按钮

### Q3: 检索结果不相关 / 质量差

**建议：**
1. 降低相似度阈值（如 0.7 → 0.5），检查是否有更多结果
2. 调整分块大小，过小可能丢失上下文
3. 更换更好的 Embedding 模型（推荐 `text-embedding-3-small` 或 `bge-m3`）
4. 检查文档内容是否被正确提取（PDF 扫描件可能需要 OCR）

### Q4: 向量维度不匹配报错

**原因：** 知识库配置的「向量维度」与嵌入模型实际输出的维度不一致。

**解决：**
1. 确认所选嵌入模型的维度（参考 5.2 维度对照表）
2. 编辑知识库，修改向量维度
3. 维度修改后，已有文档需要重新处理

### Q5: 如何用本地模型（Ollama）做 Embedding？

1. 安装 Ollama：`curl -fsSL https://ollama.com/install.sh | sh`
2. 下载 Embedding 模型：`ollama pull nomic-embed-text`
3. 在「模型管理」中添加模型：
   - 提供商：选择 Ollama 或自定义
   - 模型 ID：`nomic-embed-text`
   - Base URL：`http://localhost:11434/v1`
   - API Key：填任意非空字符串（如 `ollama`）
4. 创建知识库时选择该模型，维度设为 **768**

### Q6: Qdrant 占用内存过高怎么办？

**解决：**
1. 使用 Docker 限制内存：`deploy.resources.limits.memory: 2G`
2. 启用磁盘映射存储（参考 8.2）
3. 对不再使用的知识库执行删除，释放 Collection

### Q7: 如何迁移/克隆知识库数据？

使用 Qdrant 快照功能：
1. 创建源知识库快照
2. 在目标环境恢复快照
3. 更新 MySQL 中的知识库记录，指向新的 Collection

详见 [8.1 备份与恢复](#81-备份与恢复)。
