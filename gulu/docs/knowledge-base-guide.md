# 知识库模块 — 环境搭建与使用手册

## 目录

- [1. 概述](#1-概述)
- [2. 环境搭建](#2-环境搭建)
  - [2.1 Qdrant 向量数据库](#21-qdrant-向量数据库)
  - [2.2 Neo4j 图数据库（图知识库，可选）](#22-neo4j-图数据库图知识库可选)
  - [2.3 项目配置文件](#23-项目配置文件)
- [3. 数据库迁移](#3-数据库迁移)
- [4. 嵌入模型配置](#4-嵌入模型配置)
- [5. 知识库操作指南](#5-知识库操作指南)
  - [5.1 创建知识库](#51-创建知识库)
  - [5.2 文档管理](#52-文档管理)
  - [5.3 分块管理](#53-分块管理)
  - [5.4 检索测试](#54-检索测试)
  - [5.5 在 AI 节点中挂载知识库](#55-在-ai-节点中挂载知识库)
- [6. 检索模式详解](#6-检索模式详解)
- [7. 参数调优指南](#7-参数调优指南)
- [8. API 接口参考](#8-api-接口参考)
- [9. 诊断与运维](#9-诊断与运维)
- [10. 常见问题 FAQ](#10-常见问题-faq)

---

## 1. 概述

知识库模块为 AI 节点提供外部知识检索能力（RAG），支持两种知识库类型：

| 类型 | 存储引擎 | 适用场景 | 状态 |
|------|---------|---------|------|
| **普通知识库** (`normal`) | Qdrant 向量数据库 | 文档语义检索、RAG 增强生成 | 已实现 |
| **图知识库** (`graph`) | Qdrant + MySQL + Neo4j（可选）| 实体关系推理、知识图谱问答 | 已实现（Neo4j 可选） |

**整体处理流程：**

```
文档上传
   ↓
[Stage 1] Parsing    — 文本提取 + 图片提取（多模态）
   ↓
[Stage 2] Cleaning   — 清洗空白符 / 移除 URL
   ↓
[Stage 3] Splitting  — 文本分块
   ↓
[Stage 4] Indexing   — Embedding 生成 → Qdrant 写入
                     — （可选）多模态图片向量化
                     — （图知识库）LLM 实体关系抽取 → MySQL + Neo4j
   ↓
[Stage 5] Completed
```

**检索策略（混合模式）：**

1. **上下文注入**：AI 节点执行前，系统自动用用户提问检索知识库，将结果注入系统提示词的 `[参考知识]` 段中（经典 RAG）。
2. **工具检索**：同时注册 `knowledge_search` 工具，AI 可在对话过程中主动调用，进行更精确的追问检索。

---

## 2. 环境搭建

### 2.1 Qdrant 向量数据库

**方式一：Docker 快速启动（开发环境）**

```bash
docker run -d \
  --name qdrant \
  -p 6333:6333 \
  -p 6334:6334 \
  qdrant/qdrant:latest
```

数据保存在容器内，容器删除后数据丢失，仅推荐开发使用。

---

**方式二：Docker Compose 持久化部署（推荐）**

创建 `docker-compose-qdrant.yml`：

```yaml
version: '3.8'

services:
  qdrant:
    image: qdrant/qdrant:latest
    container_name: qdrant
    restart: always
    ports:
      - "6333:6333"   # REST API / Web Dashboard
      - "6334:6334"   # gRPC API（项目使用此端口）
    volumes:
      - qdrant_data:/qdrant/storage
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

```bash
docker-compose -f docker-compose-qdrant.yml up -d
```

---

**方式三：Qdrant Cloud 托管**

1. 访问 [https://cloud.qdrant.io](https://cloud.qdrant.io) 注册账号（免费版支持 1GB 存储）
2. 创建 Cluster，获取连接信息（URL 和 API Key）
3. 在配置文件中修改对应项（见 [2.3 配置文件](#23-项目配置文件)）

---

**端口说明：**

| 端口 | 协议 | 用途 |
|------|------|------|
| 6333 | HTTP | REST API；浏览器访问 `http://localhost:6333/dashboard` 打开管理面板 |
| 6334 | gRPC | Go SDK 使用此端口（配置文件中填 `6334`） |

**健康检查：**

```bash
curl http://localhost:6333/healthz
```

---

### 2.2 Neo4j 图数据库（图知识库，可选）

仅在需要使用**图知识库**类型时才需要部署。普通知识库无需 Neo4j。

```bash
docker run -d \
  --name neo4j \
  -p 7474:7474 \
  -p 7687:7687 \
  -e NEO4J_AUTH=neo4j/your-password \
  -v neo4j_data:/data \
  neo4j:5
```

部署后，浏览器访问 `http://localhost:7474` 进入 Neo4j Browser 管理界面。

---

### 2.3 项目配置文件

编辑 `yqhp/gulu/config/config.yml`：

```yaml
# Qdrant 向量数据库（必需）
qdrant:
  host: 127.0.0.1        # Qdrant 服务地址
  port: 6334             # gRPC 端口，注意不是 6333
  api_key: ""            # API Key（本地部署可留空；Qdrant Cloud 必填）
  use_tls: false         # 是否启用 TLS（Qdrant Cloud 设为 true）
  collection_prefix: "kb_"  # Collection 名称前缀

# Neo4j 图数据库（可选，仅图知识库需要）
neo4j:
  uri: bolt://127.0.0.1:7687
  username: neo4j
  password: your-password
  database: neo4j
```

**Qdrant Cloud 示例：**

```yaml
qdrant:
  host: your-cluster-id.aws.cloud.qdrant.io
  port: 6334
  api_key: "your-qdrant-cloud-api-key"
  use_tls: true
```

---

## 3. 数据库迁移

知识库模块需要 6 张 MySQL 表，迁移脚本位于 `yqhp/gulu/migrations/knowledge_base.sql`。

> **注意：** 脚本开头包含 `DROP TABLE IF EXISTS`，会清空已有数据，请在首次初始化时执行。

```bash
# 方式一：命令行直接执行
mysql -u root -p yqhp_admin < yqhp/gulu/migrations/knowledge_base.sql

# 方式二：登录 MySQL 后执行
mysql -u root -p
USE yqhp_admin;
SOURCE /path/to/yqhp/gulu/migrations/knowledge_base.sql;
```

**创建的表：**

| 表名 | 说明 |
|------|------|
| `t_knowledge_base` | 知识库主表（名称、类型、模型配置、分块/检索参数 JSON） |
| `t_knowledge_document` | 文档表（名称、文件路径、处理状态、分块数等） |
| `t_knowledge_segment` | 分块表（文本内容、向量 ID、启用状态、命中次数；含 ngram 全文索引） |
| `t_knowledge_query` | 检索查询历史 |
| `t_knowledge_entity` | 图知识库实体表 |
| `t_knowledge_relation` | 图知识库关系表 |

**执行成功后，重新生成 GORM 模型（可选）：**

```bash
cd yqhp/gulu
go run cmd/gen/main.go
```

---

## 4. 嵌入模型配置

知识库通过 OpenAI-compatible `/v1/embeddings` 接口调用嵌入模型，支持任意兼容提供商。

### 4.1 支持的 Embedding 模型

| 提供商 | 模型名称 | 向量维度 | 说明 |
|--------|---------|---------|------|
| OpenAI | `text-embedding-3-small` | 1536 | 性价比高，推荐 |
| OpenAI | `text-embedding-3-large` | 3072 | 精度更高 |
| OpenAI | `text-embedding-ada-002` | 1536 | 旧版 |
| 硅基流动 | `BAAI/bge-m3` | 1024 | 中文优化，免费 |
| 硅基流动 | `BAAI/bge-large-zh-v1.5` | 1024 | 中文优化 |
| 阿里通义 | `text-embedding-v3` | 1024 | 中文效果好 |
| 阿里通义 | `text-embedding-v3` (Qwen3) | 2048 | 需开启 `input_type` |
| 本地 Ollama | `nomic-embed-text` | 768 | 完全离线，免费 |
| 本地 Ollama | `mxbai-embed-large` | 1024 | 完全离线，精度较高 |

> **重要：** 系统会在第一次处理文档时自动检测模型实际输出的维度，并自动创建/重建 Qdrant Collection，无需手动填写维度。

### 4.2 在模型管理中添加 Embedding 模型

1. 进入「模型管理」→「新增模型」
2. 填写信息：
   - **模型类型**：选择 `Embedding`
   - **模型 ID**：如 `text-embedding-3-small`
   - **API Key**：提供商密钥
   - **Base URL**：自定义地址（Ollama 填 `http://localhost:11434/v1`；OpenAI 可留空）
3. 保存并启用

创建知识库时，在「嵌入模型」下拉框中选择刚添加的模型即可。

### 4.3 多模态嵌入模型（可选）

若需对文档内的图片进行向量化索引，还需配置一个**多模态嵌入模型**（如 Jina CLIP、阿里 multimodal-embedding 等）。创建知识库时开启「多模态」并选择该模型。

---

## 5. 知识库操作指南

### 5.1 创建知识库

1. 进入「知识库」→「新建知识库」
2. 填写基本信息：
   - **名称**：知识库名称
   - **描述**：用途说明（可选）
   - **类型**：`普通知识库（向量检索）` 或 `图知识库`
   - **嵌入模型**：选择已配置的 Embedding 模型
   - **多模态**：是否对文档内图片进行向量化（需配置多模态模型）
   - **图抽取模型**（图知识库专用）：用于从文档中抽取实体和关系的 LLM
3. 调整检索参数（可保持默认值）：
   - **检索模式**：见 [6. 检索模式详解](#6-检索模式详解)
   - **Top-K**：每次检索返回的最大结果数（默认 5）
   - **相似度阈值**：低于此值的结果将被过滤（默认 0.3）
   - **Rerank 模型**：启用重排序以提升结果精度（可选）
4. 点击「创建」

创建成功后，系统会自动在 Qdrant 中创建对应的 Collection（命名规则：`kb_{id}`）。

### 5.2 文档管理

#### 上传文档

进入知识库 → 「文档管理」→「上传文档」，支持多文件同时上传。

**支持的文件格式：**

| 类型 | 格式 |
|------|------|
| 文本 | `.txt`、`.md`、`.csv`、`.json` |
| 文档 | `.pdf`、`.docx`、`.doc`、`.html`、`.htm` |
| 图片 | `.png`、`.jpg`、`.jpeg`、`.gif`、`.webp`、`.bmp`（需开启多模态） |

也可通过 API 直接提交纯文本内容（适合 FAQ、短文本片段等）。

#### 文档处理状态

上传后，文档异步处理，依次经过以下状态：

| 状态 | 含义 |
|------|------|
| `waiting` | 已上传，等待处理 |
| `parsing` | 提取文本内容和图片 |
| `cleaning` | 清洗文本（去除多余空白、URL 等） |
| `splitting` | 文本分块 |
| `indexing` | 生成向量并写入 Qdrant |
| `completed` | 处理完成，可被检索 |
| `error` | 处理失败，可查看错误信息后点击「重新处理」 |

#### 分块设置（文档级覆盖）

每个文档可以单独设置分块参数，覆盖知识库的默认配置：

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `chunk_size` | 500 | 每块最大字符数 |
| `chunk_overlap` | 50 | 相邻块重叠字符数 |
| `separator` | 空 | 自定义分隔符（如 `\n\n`），空则按长度切割 |
| `clean_whitespace` | false | 是否合并多余空白行 |
| `remove_urls` | false | 是否移除 URL 和邮箱 |

可以先通过「预览分块」接口查看切割效果，再提交处理。

#### 批量操作

- **批量删除**：勾选多个文档 → 删除
- **批量重新处理**：勾选多个文档 → 重新处理（用于参数调整后统一重建索引）

---

### 5.3 分块管理

进入知识库 → 文档详情 → 「分块列表」，可以：

- **查看**每个分块的内容、字符数、命中次数
- **启用/禁用**某个分块（禁用后不参与检索，但不会删除向量）
- **编辑**分块内容（保存后自动重新生成并更新 Qdrant 向量）

---

### 5.4 检索测试

在知识库详情页的「命中测试」标签，或通过 API：

```bash
curl -X POST http://localhost:5321/api/knowledge-bases/{id}/search \
  -H "Content-Type: application/json" \
  -H "satoken: your-token" \
  -d '{
    "query": "如何配置数据库连接？",
    "top_k": 5,
    "score": 0.5,
    "retrieval_mode": "hybrid"
  }'
```

返回示例：

```json
{
  "code": 0,
  "data": [
    {
      "content": "数据库连接配置说明：在 config.yml 中...",
      "content_type": "text",
      "score": 0.89,
      "document_id": 42,
      "document_name": "部署手册.pdf",
      "chunk_index": 3,
      "word_count": 125,
      "hit_count": 7
    }
  ]
}
```

每次检索会自动记录到查询历史，可在「查询历史」标签查看。

---

### 5.5 在 AI 节点中挂载知识库

1. 在工作流编辑器中选择 AI 节点
2. 切换到「知识库」标签
3. 点击「+ 添加知识库」，选择要挂载的知识库
4. 按需调整覆盖参数（不填则使用知识库默认值）

**挂载后的行为：**

- AI 节点执行时，系统自动用用户提示词检索所有已挂载的知识库，将结果注入系统提示词（格式见下方）
- 同时注册 `knowledge_search` 工具，AI 可在对话中主动调用

**注入到系统提示词的格式：**

```
[参考知识]
以下是从知识库中检索到的相关参考资料，请结合这些信息来回答用户的问题：

--- 参考 1 (来源: 产品手册.pdf, 相关度: 0.92) ---
...内容...

--- 参考 2 (来源: API文档.md, 相关度: 0.85) ---
...内容...
```

---

## 6. 检索模式详解

创建或编辑知识库时，可以通过 `retrieval_mode` 字段指定检索模式。检索时也可以通过请求参数临时覆盖。

| 模式 | 说明 | 适用场景 |
|------|------|---------|
| `vector`（默认）| 纯向量语义检索（Qdrant） | 语义相似度高的场景 |
| `keyword` | 纯关键词检索（MySQL 全文索引，降级为 LIKE）| 精确词匹配、代码检索 |
| `hybrid` | 向量 + 关键词混合，去重后取 Top-K | 兼顾语义和精确匹配，通用场景推荐 |
| `graph` | 图谱遍历检索（仅图知识库 + Neo4j 启用时有效）| 实体关系推理 |
| `hybrid_graph` | 向量 + 图谱混合 | 图知识库的综合检索 |

---

## 7. 参数调优指南

### Top-K 检索数量

每次检索返回的最大结果数。

| 场景 | 建议值 |
|------|--------|
| 简短问答 | 3 |
| 一般检索（默认）| 5 |
| 深度分析 | 10–15 |
| 综合报告 | 15–20 |

> Top-K 过大会导致上下文 Token 消耗增加、引入无关信息、响应延迟上升。

### 相似度阈值

向量余弦相似度的最低门槛（0–1），越高越严格。**请求中传 `score: 0` 则不过滤**（对齐 Dify 行为）。

| 阈值 | 效果 |
|------|------|
| 0.3（默认）| 宽松，适合中文短文本 |
| 0.5 | 一般场景 |
| 0.7 | 较严格，精确匹配 |
| 0.9 | 非常严格，可能漏检 |

### 分块大小与重叠

| 文档类型 | chunk_size 建议 | chunk_overlap 建议 |
|---------|-----------------|-------------------|
| 中文文档 | 500 | 50 |
| 英文文档 | 800 | 100 |
| 代码/技术文档 | 1000 | 200 |
| FAQ / 短文本 | 200 | 0 |

**分块大小的权衡：**
- 过小（200 以下）：精确度高，但可能丢失上下文，且向量数量大
- 过大（1000 以上）：上下文完整，但检索精度下降，LLM 上下文占用大

### Rerank 重排序

在 `hybrid` 等模式下，召回多个候选块后，可通过 Rerank 模型对结果重新打分排序，显著提升最终质量。在知识库设置中开启 `rerank_enabled` 并配置 Rerank 模型 ID 即可。

---

## 8. API 接口参考

所有接口均挂载在 `/api/knowledge-bases` 路径下，需要 `satoken` 认证头。

### 知识库 CRUD

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/knowledge-bases` | 创建知识库 |
| GET | `/api/knowledge-bases` | 获取列表（支持 name/type/status 过滤，分页） |
| GET | `/api/knowledge-bases/:id` | 获取详情 |
| PUT | `/api/knowledge-bases/:id` | 更新配置 |
| DELETE | `/api/knowledge-bases/:id` | 删除（异步清理 Qdrant、MySQL 数据） |
| PUT | `/api/knowledge-bases/:id/status` | 启用/禁用 |

### 文档管理

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/knowledge-bases/:id/documents` | 上传文档（multipart/form-data） |
| GET | `/api/knowledge-bases/:id/documents` | 获取文档列表 |
| DELETE | `/api/knowledge-bases/:id/documents/:docId` | 删除文档（同步清理向量） |
| POST | `/api/knowledge-bases/:id/documents/batch-delete` | 批量删除 |
| POST | `/api/knowledge-bases/:id/documents/:docId/reprocess` | 重新处理文档 |
| POST | `/api/knowledge-bases/:id/documents/batch-reprocess` | 批量重新处理 |
| PUT | `/api/knowledge-bases/:id/documents/:docId/process` | 以自定义分块设置重新处理 |
| POST | `/api/knowledge-bases/:id/documents/preview-chunks` | 预览分块效果（不写入） |
| GET | `/api/knowledge-bases/:id/indexing-status` | 获取所有文档的索引状态 |

### 分块管理

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/knowledge-bases/:id/documents/:docId/segments` | 获取分块列表（分页） |
| PATCH | `/api/knowledge-bases/:id/segments/:segId` | 更新分块（内容/启用状态） |

### 检索与历史

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/knowledge-bases/:id/search` | 检索知识库 |
| GET | `/api/knowledge-bases/:id/queries` | 获取查询历史 |

### 诊断

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/knowledge-bases/:id/diagnose` | 诊断向量数据状态（检测维度、Qdrant 状态、嵌入 API 连通性） |

### 图知识库

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/knowledge-bases/:id/graph/search` | 图谱关键词检索 |
| GET | `/api/knowledge-bases/:id/graph/entities` | 获取实体列表 |
| GET | `/api/knowledge-bases/:id/graph/relations` | 获取关系列表 |

### 图片访问（无需认证）

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/knowledge-bases/:id/images/:filename` | 访问文档内嵌图片（多模态场景） |

---

**检索请求体参数说明：**

```json
{
  "query": "查询文本",
  "top_k": 5,
  "score": 0.5,
  "retrieval_mode": "hybrid",
  "search_fields": "all"
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `query` | string | 查询文本（必填） |
| `top_k` | int | 返回结果数，0 则使用知识库默认值 |
| `score` | float | 相似度阈值，0 则不过滤，负数则使用知识库默认值 |
| `retrieval_mode` | string | 检索模式，空则使用知识库默认值 |
| `search_fields` | string | `text`/`image`/`all`，控制多模态场景的搜索范围 |

---

## 9. 诊断与运维

### 知识库诊断接口

当遇到检索结果为空或向量维度不匹配等问题时，可调用诊断接口：

```bash
curl http://localhost:5321/api/knowledge-bases/{id}/diagnose \
  -H "satoken: your-token"
```

返回内容包括：
- MySQL 分块数量 vs Qdrant 向量数量对比
- Qdrant Collection 的向量字段维度
- 嵌入模型 API 连通性测试及实际输出维度
- 维度不匹配时的修复建议

### Qdrant 管理面板

浏览器访问 `http://localhost:6333/dashboard` 可以：
- 查看所有 Collections 及向量统计
- 手动执行 API 请求
- 查看集群状态

### Qdrant 备份与恢复

**创建快照（Collection 级别）：**

```bash
# 备份单个 Collection
curl -X POST http://localhost:6333/collections/kb_1/snapshots

# 备份整个实例
curl -X POST http://localhost:6333/snapshots
```

**恢复快照：**

```bash
curl -X PUT http://localhost:6333/collections/kb_1/snapshots/recover \
  -H "Content-Type: application/json" \
  -d '{"location": "file:///qdrant/snapshots/kb_1-snapshot.snapshot"}'
```

**Docker Volume 备份：**

```bash
# 备份
docker cp qdrant:/qdrant/storage ./qdrant-backup-$(date +%Y%m%d)

# 恢复
docker cp ./qdrant-backup-20260219/storage qdrant:/qdrant/
docker restart qdrant
```

### Qdrant 性能调优

对于大规模知识库，可以启用磁盘映射存储降低内存占用：

```bash
curl -X PUT http://localhost:6333/collections/kb_1 \
  -H "Content-Type: application/json" \
  -d '{"optimizers_config": {"memmap_threshold": 20000}}'
```

Qdrant 内置 Prometheus 指标端点：`http://localhost:6333/metrics`，可接入 Grafana 监控。

---

## 10. 常见问题 FAQ

### Q1: 文档上传后一直处于 `waiting` 状态，没有开始处理

**原因：** 后台 goroutine 未被触发（通常不会发生），或服务刚重启、任务丢失。

**解决：** 手动点击「重新处理」按钮，或调用 `POST /api/knowledge-bases/:id/documents/:docId/reprocess`。

---

### Q2: 文档状态变为 `error`

**排查步骤：**
1. 查看文档列表中的错误信息字段（`error_message`）
2. 查看服务端日志中 `[ERROR]` 相关输出
3. 常见原因：
   - Embedding API 调用失败（API Key 无效、余额不足、网络超时）→ 检查模型配置后重试
   - Qdrant 未启动或地址配置错误 → 确认 `config.yml` 中 `qdrant.host:port` 正确，且端口为 **6334**
   - 文档内容为空（扫描版 PDF 无法提取文本）→ 转换为可选中文字的 PDF 后重新上传

---

### Q3: 检索结果为空 / 质量很差

**建议检查顺序：**
1. 调用**诊断接口**，确认 Qdrant Collection 有数据，且嵌入模型正常
2. 降低相似度阈值（如 0.7 → 0.3），观察是否有结果返回
3. 切换检索模式为 `keyword` 或 `hybrid`，对比结果
4. 检查文档内容是否被正确提取（可在「预览分块」中查看）
5. 考虑更换中文效果更好的嵌入模型（如 `BAAI/bge-m3`）

---

### Q4: 报错「向量维度不匹配」

系统会在处理文档时**自动检测模型维度**并重建 Qdrant Collection，因此一般不会出现此问题。

如果仍然报错：
1. 调用诊断接口，查看 `dimension_mismatch` 字段和 `suggestion` 内容
2. 在文档管理页面「批量重新处理」所有文档，系统会自动修正

---

### Q5: 如何使用本地模型（Ollama）做 Embedding？

1. 安装 Ollama：`curl -fsSL https://ollama.com/install.sh | sh`
2. 拉取模型：`ollama pull nomic-embed-text`（768 维）或 `ollama pull mxbai-embed-large`（1024 维）
3. 在「模型管理」添加模型：
   - 模型 ID：`nomic-embed-text`
   - Base URL：`http://localhost:11434/v1`
   - API Key：填任意非空字符串（如 `ollama`）
4. 创建知识库时选择该模型即可（维度自动检测，无需手填）

---

### Q6: 图知识库与普通知识库的区别

| 对比项 | 普通知识库 | 图知识库 |
|--------|-----------|---------|
| 存储 | Qdrant | Qdrant + MySQL（实体/关系）+ Neo4j（可选） |
| 处理 | 文本分块 → 向量化 | 文本分块 → 向量化 + LLM 实体/关系抽取 |
| 检索模式 | vector / keyword / hybrid | graph / hybrid_graph / vector / hybrid |
| 额外配置 | 无 | 需配置「图抽取模型」（LLM），Neo4j 可选 |
| 适用场景 | 通用文档 Q&A | 有明显实体关系的专业领域知识（如医疗、法律、产品关系图） |

图知识库即使不部署 Neo4j，实体和关系仍会存入 MySQL，可通过 `graph/entities` 和 `graph/relations` 接口查询。

---

### Q7: 如何迁移 / 克隆知识库数据？

1. 使用 Qdrant 快照功能备份源 Collection（见 [9. 诊断与运维](#9-诊断与运维)）
2. 在目标环境恢复快照
3. 迁移 MySQL 中的 `t_knowledge_base`、`t_knowledge_document`、`t_knowledge_segment` 等表数据
4. 更新新环境中知识库记录的 `qdrant_collection` 字段确保对应正确

---

### Q8: Qdrant 内存占用过高

1. Docker 部署时限制内存：在 `docker-compose.yml` 中设置 `deploy.resources.limits.memory: 2G`
2. 开启磁盘映射存储（参考 [9. 诊断与运维](#9-诊断与运维) 中的「Qdrant 性能调优」）
3. 删除不再使用的知识库，系统会自动清理对应 Collection
