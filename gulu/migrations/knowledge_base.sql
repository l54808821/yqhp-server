-- 知识库管理模块 - 数据库迁移脚本（V3 多模态 + 图知识库）
-- 执行方式: mysql -u root -p yqhp_admin < migrations/knowledge_base.sql

-- 删除旧表（不考虑兼容性）
DROP TABLE IF EXISTS `t_knowledge_query`;
DROP TABLE IF EXISTS `t_knowledge_segment`;
DROP TABLE IF EXISTS `t_knowledge_document`;
DROP TABLE IF EXISTS `t_knowledge_base`;
DROP TABLE IF EXISTS `t_knowledge_entity`;
DROP TABLE IF EXISTS `t_knowledge_relation`;

-- 知识库主表
CREATE TABLE `t_knowledge_base` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `created_at` DATETIME DEFAULT CURRENT_TIMESTAMP,
  `updated_at` DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  `is_delete` TINYINT(1) DEFAULT 0,
  `created_by` BIGINT UNSIGNED DEFAULT NULL COMMENT '创建人ID',
  `updated_by` BIGINT UNSIGNED DEFAULT NULL COMMENT '更新人ID',
  `project_id` BIGINT UNSIGNED NOT NULL COMMENT '所属项目ID',
  `name` VARCHAR(100) NOT NULL COMMENT '知识库名称',
  `description` VARCHAR(1024) DEFAULT NULL COMMENT '知识库描述',
  `type` VARCHAR(20) NOT NULL DEFAULT 'normal' COMMENT '知识库类型: normal/graph',
  `status` TINYINT DEFAULT 1 COMMENT '状态: 1-启用 0-禁用',
  -- 文本嵌入模型
  `embedding_model_id` BIGINT UNSIGNED DEFAULT NULL COMMENT '嵌入模型ID',
  `embedding_model_name` VARCHAR(100) DEFAULT NULL COMMENT '嵌入模型名称(冗余)',
  `embedding_dimension` INT DEFAULT 1536 COMMENT '向量维度',
  -- 多模态嵌入模型（Phase 2）
  `multimodal_enabled` TINYINT(1) DEFAULT 0 COMMENT '是否启用多模态',
  `multimodal_model_id` BIGINT UNSIGNED DEFAULT NULL COMMENT '多模态嵌入模型ID',
  `multimodal_model_name` VARCHAR(100) DEFAULT NULL COMMENT '多模态嵌入模型名称(冗余)',
  `multimodal_dimension` INT DEFAULT NULL COMMENT '多模态向量维度',
  -- 分块配置
  `chunk_size` INT DEFAULT 500 COMMENT '默认分块大小(字符数)',
  `chunk_overlap` INT DEFAULT 50 COMMENT '默认分块重叠(字符数)',
  -- 检索配置
  `similarity_threshold` FLOAT DEFAULT 0.3 COMMENT '默认相似度阈值',
  `top_k` INT DEFAULT 5 COMMENT '默认检索数量',
  `retrieval_mode` VARCHAR(20) DEFAULT 'vector' COMMENT '检索模式: vector/keyword/hybrid',
  `rerank_model_id` BIGINT UNSIGNED DEFAULT NULL COMMENT '重排序模型ID',
  `rerank_enabled` TINYINT(1) DEFAULT 0 COMMENT '是否启用重排序',
  -- 向量库
  `qdrant_collection` VARCHAR(100) DEFAULT NULL COMMENT 'Qdrant Collection 名称',
  -- 图数据库（Phase 3）
  `neo4j_database` VARCHAR(100) DEFAULT NULL COMMENT 'Neo4j 数据库名称',
  `graph_extract_model_id` BIGINT UNSIGNED DEFAULT NULL COMMENT '图谱抽取模型ID（LLM）',
  -- 统计
  `document_count` INT DEFAULT 0 COMMENT '文档数量',
  `chunk_count` INT DEFAULT 0 COMMENT '分块数量',
  `entity_count` INT DEFAULT 0 COMMENT '实体数量（图知识库）',
  `relation_count` INT DEFAULT 0 COMMENT '关系数量（图知识库）',
  `metadata` JSON DEFAULT NULL COMMENT '扩展元数据',
  PRIMARY KEY (`id`),
  INDEX `idx_kb_project_id` (`project_id`),
  INDEX `idx_kb_is_delete` (`is_delete`),
  INDEX `idx_kb_type` (`type`),
  INDEX `idx_kb_status` (`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='知识库主表';

-- 知识库文档表
CREATE TABLE `t_knowledge_document` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `created_at` DATETIME DEFAULT CURRENT_TIMESTAMP,
  `updated_at` DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  `knowledge_base_id` BIGINT UNSIGNED NOT NULL COMMENT '所属知识库ID',
  `name` VARCHAR(255) NOT NULL COMMENT '文档名称',
  `file_type` VARCHAR(20) DEFAULT NULL COMMENT '文件类型: pdf/txt/md/docx/html/csv/json/image',
  `file_path` VARCHAR(512) DEFAULT NULL COMMENT '文件存储路径(相对路径)',
  `file_size` BIGINT DEFAULT 0 COMMENT '文件大小(字节)',
  `word_count` INT DEFAULT 0 COMMENT '文档字数',
  `image_count` INT DEFAULT 0 COMMENT '文档内提取的图片数量',
  `chunk_setting` JSON DEFAULT NULL COMMENT '分段设置 JSON',
  `indexing_status` VARCHAR(32) DEFAULT 'waiting' COMMENT '索引状态: waiting/parsing/cleaning/splitting/indexing/completed/error/paused',
  `error_message` TEXT DEFAULT NULL COMMENT '错误信息',
  `chunk_count` INT DEFAULT 0 COMMENT '分块数量',
  `token_count` INT DEFAULT 0 COMMENT 'Token 数量(估算)',
  `parsing_completed_at` DATETIME DEFAULT NULL COMMENT '解析完成时间',
  `indexing_completed_at` DATETIME DEFAULT NULL COMMENT '索引完成时间',
  `metadata` JSON DEFAULT NULL COMMENT '文档元数据',
  PRIMARY KEY (`id`),
  INDEX `idx_doc_kb_id` (`knowledge_base_id`),
  INDEX `idx_doc_indexing_status` (`indexing_status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='知识库文档表';

-- 知识库分块表（MySQL 双写，与 Qdrant 同步）
CREATE TABLE `t_knowledge_segment` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `created_at` DATETIME DEFAULT CURRENT_TIMESTAMP,
  `updated_at` DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  `knowledge_base_id` BIGINT UNSIGNED NOT NULL COMMENT '所属知识库ID',
  `document_id` BIGINT UNSIGNED NOT NULL COMMENT '所属文档ID',
  `content` TEXT NOT NULL COMMENT '分块文本内容（图片类型存描述信息）',
  `content_type` VARCHAR(20) NOT NULL DEFAULT 'text' COMMENT '内容类型: text/image',
  `image_path` VARCHAR(512) DEFAULT NULL COMMENT '图片存储路径（content_type=image时）',
  `position` INT NOT NULL DEFAULT 0 COMMENT '分块位置(从0开始)',
  `word_count` INT DEFAULT 0 COMMENT '分块字数',
  `tokens` INT DEFAULT 0 COMMENT '分块Token数(估算)',
  `index_node_id` VARCHAR(64) DEFAULT NULL COMMENT 'Qdrant point ID',
  `vector_field` VARCHAR(20) DEFAULT 'text' COMMENT '使用的向量字段: text/image',
  `status` VARCHAR(20) DEFAULT 'active' COMMENT '状态: active/disabled',
  `enabled` TINYINT(1) DEFAULT 1 COMMENT '是否启用',
  `hit_count` INT DEFAULT 0 COMMENT '命中次数',
  `metadata` JSON DEFAULT NULL COMMENT '分块元数据',
  PRIMARY KEY (`id`),
  INDEX `idx_seg_kb_id` (`knowledge_base_id`),
  INDEX `idx_seg_doc_id` (`document_id`),
  INDEX `idx_seg_content_type` (`content_type`),
  INDEX `idx_seg_status` (`status`),
  FULLTEXT INDEX `ft_seg_content` (`content`) WITH PARSER ngram
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='知识库分块表';

-- 知识库查询历史表
CREATE TABLE `t_knowledge_query` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `created_at` DATETIME DEFAULT CURRENT_TIMESTAMP,
  `knowledge_base_id` BIGINT UNSIGNED NOT NULL COMMENT '知识库ID',
  `query_text` TEXT NOT NULL COMMENT '查询文本',
  `query_type` VARCHAR(20) DEFAULT 'text' COMMENT '查询类型: text/image',
  `retrieval_mode` VARCHAR(20) DEFAULT 'vector' COMMENT '检索方式',
  `top_k` INT DEFAULT 5 COMMENT '检索数量',
  `score_threshold` FLOAT DEFAULT 0.0 COMMENT '相似度阈值',
  `result_count` INT DEFAULT 0 COMMENT '结果数量',
  `source` VARCHAR(32) DEFAULT 'hit_testing' COMMENT '来源: hit_testing/api/workflow',
  `created_by` BIGINT UNSIGNED DEFAULT NULL COMMENT '创建人',
  PRIMARY KEY (`id`),
  INDEX `idx_query_kb_id` (`knowledge_base_id`),
  INDEX `idx_query_created_at` (`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='知识库查询历史';

-- 知识图谱实体表（Phase 3 - 图知识库）
CREATE TABLE `t_knowledge_entity` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `created_at` DATETIME DEFAULT CURRENT_TIMESTAMP,
  `updated_at` DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  `knowledge_base_id` BIGINT UNSIGNED NOT NULL COMMENT '所属知识库ID',
  `document_id` BIGINT UNSIGNED NOT NULL COMMENT '来源文档ID',
  `name` VARCHAR(255) NOT NULL COMMENT '实体名称',
  `entity_type` VARCHAR(100) NOT NULL COMMENT '实体类型（人物/地点/组织/概念等）',
  `description` TEXT DEFAULT NULL COMMENT '实体描述',
  `properties` JSON DEFAULT NULL COMMENT '实体属性 JSON',
  `neo4j_node_id` VARCHAR(64) DEFAULT NULL COMMENT 'Neo4j 节点ID',
  `mention_count` INT DEFAULT 1 COMMENT '在文档中出现的次数',
  PRIMARY KEY (`id`),
  INDEX `idx_entity_kb_id` (`knowledge_base_id`),
  INDEX `idx_entity_doc_id` (`document_id`),
  INDEX `idx_entity_type` (`entity_type`),
  INDEX `idx_entity_name` (`name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='知识图谱实体表';

-- 知识图谱关系表（Phase 3 - 图知识库）
CREATE TABLE `t_knowledge_relation` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `created_at` DATETIME DEFAULT CURRENT_TIMESTAMP,
  `knowledge_base_id` BIGINT UNSIGNED NOT NULL COMMENT '所属知识库ID',
  `document_id` BIGINT UNSIGNED NOT NULL COMMENT '来源文档ID',
  `source_entity_id` BIGINT UNSIGNED NOT NULL COMMENT '源实体ID',
  `target_entity_id` BIGINT UNSIGNED NOT NULL COMMENT '目标实体ID',
  `relation_type` VARCHAR(100) NOT NULL COMMENT '关系类型',
  `description` TEXT DEFAULT NULL COMMENT '关系描述',
  `properties` JSON DEFAULT NULL COMMENT '关系属性 JSON',
  `neo4j_rel_id` VARCHAR(64) DEFAULT NULL COMMENT 'Neo4j 关系ID',
  `weight` FLOAT DEFAULT 1.0 COMMENT '关系权重',
  PRIMARY KEY (`id`),
  INDEX `idx_rel_kb_id` (`knowledge_base_id`),
  INDEX `idx_rel_doc_id` (`document_id`),
  INDEX `idx_rel_source` (`source_entity_id`),
  INDEX `idx_rel_target` (`target_entity_id`),
  INDEX `idx_rel_type` (`relation_type`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='知识图谱关系表';
