-- 知识库管理模块 - 数据库迁移脚本（V4 精简版）
-- 执行方式: mysql -u root -p yqhp_admin < migrations/knowledge_base.sql

DROP TABLE IF EXISTS `t_knowledge_query`;
DROP TABLE IF EXISTS `t_knowledge_segment`;
DROP TABLE IF EXISTS `t_knowledge_document`;
DROP TABLE IF EXISTS `t_knowledge_base`;
DROP TABLE IF EXISTS `t_knowledge_entity`;
DROP TABLE IF EXISTS `t_knowledge_relation`;

-- 知识库主表（精简版：分块+检索配置合并到 config JSON）
CREATE TABLE `t_knowledge_base` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `created_at` DATETIME DEFAULT CURRENT_TIMESTAMP,
  `updated_at` DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  `is_delete` TINYINT(1) DEFAULT 0,
  `created_by` BIGINT UNSIGNED DEFAULT NULL,
  `project_id` BIGINT UNSIGNED NOT NULL,
  `name` VARCHAR(100) NOT NULL,
  `description` VARCHAR(1024) DEFAULT NULL,
  `type` VARCHAR(20) NOT NULL DEFAULT 'normal' COMMENT 'normal / graph',
  `status` TINYINT DEFAULT 1,
  -- 模型（只存 ID，名称通过 JOIN t_ai_model 获取）
  `embedding_model_id` BIGINT UNSIGNED DEFAULT NULL,
  `multimodal_enabled` TINYINT(1) DEFAULT 0,
  `multimodal_model_id` BIGINT UNSIGNED DEFAULT NULL,
  `graph_extract_model_id` BIGINT UNSIGNED DEFAULT NULL,
  -- 向量库 / 图库
  `qdrant_collection` VARCHAR(100) DEFAULT NULL,
  `neo4j_database` VARCHAR(100) DEFAULT NULL,
  -- 配置 JSON（分块 + 检索参数，维度为自动检测后的缓存值）
  -- 结构示例: {"chunk_size":500,"chunk_overlap":50,"similarity_threshold":0.3,
  --            "top_k":5,"retrieval_mode":"vector","rerank_enabled":false,
  --            "rerank_model_id":null,"embedding_dimension":0,"multimodal_dimension":0}
  `config` JSON DEFAULT NULL,
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
  `knowledge_base_id` BIGINT UNSIGNED NOT NULL,
  `name` VARCHAR(255) NOT NULL,
  `file_type` VARCHAR(20) DEFAULT NULL COMMENT 'pdf/txt/md/docx/html/csv/json/image',
  `file_path` VARCHAR(512) DEFAULT NULL,
  `file_size` BIGINT DEFAULT 0,
  `word_count` INT DEFAULT 0,
  `image_count` INT DEFAULT 0,
  `chunk_setting` JSON DEFAULT NULL COMMENT '文档级分段设置（覆盖知识库默认）',
  `indexing_status` VARCHAR(32) DEFAULT 'waiting' COMMENT 'waiting/parsing/cleaning/splitting/indexing/completed/error',
  `error_message` TEXT DEFAULT NULL,
  `chunk_count` INT DEFAULT 0,
  `token_count` INT DEFAULT 0,
  `parsing_completed_at` DATETIME DEFAULT NULL,
  `indexing_completed_at` DATETIME DEFAULT NULL,
  PRIMARY KEY (`id`),
  INDEX `idx_doc_kb_id` (`knowledge_base_id`),
  INDEX `idx_doc_indexing_status` (`indexing_status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='知识库文档表';

-- 知识库分块表（MySQL 双写，与 Qdrant 同步）
CREATE TABLE `t_knowledge_segment` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `created_at` DATETIME DEFAULT CURRENT_TIMESTAMP,
  `updated_at` DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  `knowledge_base_id` BIGINT UNSIGNED NOT NULL,
  `document_id` BIGINT UNSIGNED NOT NULL,
  `content` TEXT NOT NULL,
  `content_type` VARCHAR(20) NOT NULL DEFAULT 'text' COMMENT 'text / image',
  `image_path` VARCHAR(512) DEFAULT NULL,
  `position` INT NOT NULL DEFAULT 0,
  `word_count` INT DEFAULT 0,
  `tokens` INT DEFAULT 0,
  `index_node_id` VARCHAR(64) DEFAULT NULL COMMENT 'Qdrant point ID',
  `vector_field` VARCHAR(20) DEFAULT 'text',
  `status` VARCHAR(20) DEFAULT 'active',
  `enabled` TINYINT(1) DEFAULT 1,
  `hit_count` INT DEFAULT 0,
  `metadata` JSON DEFAULT NULL,
  PRIMARY KEY (`id`),
  INDEX `idx_seg_kb_id` (`knowledge_base_id`),
  INDEX `idx_seg_doc_id` (`document_id`),
  INDEX `idx_seg_content_type` (`content_type`),
  INDEX `idx_seg_status` (`status`),
  FULLTEXT INDEX `ft_seg_content` (`content`) WITH PARSER ngram
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='知识库分块表';

-- 知识库查询历史
CREATE TABLE `t_knowledge_query` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `created_at` DATETIME DEFAULT CURRENT_TIMESTAMP,
  `knowledge_base_id` BIGINT UNSIGNED NOT NULL,
  `query_text` TEXT NOT NULL,
  `query_type` VARCHAR(20) DEFAULT 'text' COMMENT 'text / image',
  `retrieval_mode` VARCHAR(20) DEFAULT 'vector',
  `top_k` INT DEFAULT 5,
  `score_threshold` FLOAT DEFAULT 0.0,
  `result_count` INT DEFAULT 0,
  `source` VARCHAR(32) DEFAULT 'hit_testing' COMMENT 'hit_testing / api / workflow',
  `created_by` BIGINT UNSIGNED DEFAULT NULL,
  PRIMARY KEY (`id`),
  INDEX `idx_query_kb_id` (`knowledge_base_id`),
  INDEX `idx_query_created_at` (`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='知识库查询历史';

-- 知识图谱实体表
CREATE TABLE `t_knowledge_entity` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `created_at` DATETIME DEFAULT CURRENT_TIMESTAMP,
  `updated_at` DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  `knowledge_base_id` BIGINT UNSIGNED NOT NULL,
  `document_id` BIGINT UNSIGNED NOT NULL,
  `name` VARCHAR(255) NOT NULL,
  `entity_type` VARCHAR(100) NOT NULL,
  `description` TEXT DEFAULT NULL,
  `properties` JSON DEFAULT NULL,
  `neo4j_node_id` VARCHAR(64) DEFAULT NULL,
  `mention_count` INT DEFAULT 1,
  PRIMARY KEY (`id`),
  INDEX `idx_entity_kb_id` (`knowledge_base_id`),
  INDEX `idx_entity_doc_id` (`document_id`),
  INDEX `idx_entity_name` (`name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='知识图谱实体表';

-- 知识图谱关系表
CREATE TABLE `t_knowledge_relation` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `created_at` DATETIME DEFAULT CURRENT_TIMESTAMP,
  `knowledge_base_id` BIGINT UNSIGNED NOT NULL,
  `document_id` BIGINT UNSIGNED NOT NULL,
  `source_entity_id` BIGINT UNSIGNED NOT NULL,
  `target_entity_id` BIGINT UNSIGNED NOT NULL,
  `relation_type` VARCHAR(100) NOT NULL,
  `description` TEXT DEFAULT NULL,
  `properties` JSON DEFAULT NULL,
  `neo4j_rel_id` VARCHAR(64) DEFAULT NULL,
  `weight` FLOAT DEFAULT 1.0,
  PRIMARY KEY (`id`),
  INDEX `idx_rel_kb_id` (`knowledge_base_id`),
  INDEX `idx_rel_source` (`source_entity_id`),
  INDEX `idx_rel_target` (`target_entity_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='知识图谱关系表';
