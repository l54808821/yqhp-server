-- 知识库管理模块 - 数据库迁移脚本（V2 重构版）
-- 执行方式: mysql -u root -p yqhp_admin < migrations/knowledge_base.sql

-- 删除旧表（不考虑兼容性）
DROP TABLE IF EXISTS `t_knowledge_query`;
DROP TABLE IF EXISTS `t_knowledge_segment`;
DROP TABLE IF EXISTS `t_knowledge_document`;
DROP TABLE IF EXISTS `t_knowledge_base`;

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
  `embedding_model_id` BIGINT UNSIGNED DEFAULT NULL COMMENT '嵌入模型ID',
  `embedding_model_name` VARCHAR(100) DEFAULT NULL COMMENT '嵌入模型名称(冗余)',
  `embedding_dimension` INT DEFAULT 1536 COMMENT '向量维度',
  `chunk_size` INT DEFAULT 500 COMMENT '默认分块大小(字符数)',
  `chunk_overlap` INT DEFAULT 50 COMMENT '默认分块重叠(字符数)',
  `similarity_threshold` FLOAT DEFAULT 0.7 COMMENT '默认相似度阈值',
  `top_k` INT DEFAULT 5 COMMENT '默认检索数量',
  `retrieval_mode` VARCHAR(20) DEFAULT 'vector' COMMENT '检索模式: vector/keyword/hybrid',
  `rerank_model_id` BIGINT UNSIGNED DEFAULT NULL COMMENT '重排序模型ID',
  `rerank_enabled` TINYINT(1) DEFAULT 0 COMMENT '是否启用重排序',
  `qdrant_collection` VARCHAR(100) DEFAULT NULL COMMENT 'Qdrant Collection 名称',
  `document_count` INT DEFAULT 0 COMMENT '文档数量',
  `chunk_count` INT DEFAULT 0 COMMENT '分块数量',
  `metadata` JSON DEFAULT NULL COMMENT '扩展元数据',
  PRIMARY KEY (`id`),
  INDEX `idx_kb_project_id` (`project_id`),
  INDEX `idx_kb_is_delete` (`is_delete`),
  INDEX `idx_kb_type` (`type`),
  INDEX `idx_kb_status` (`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='知识库主表';

-- 知识库文档表（不再存储文件内容，只存元数据和文件路径）
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
  `content` TEXT NOT NULL COMMENT '分块文本内容',
  `position` INT NOT NULL DEFAULT 0 COMMENT '分块位置(从0开始)',
  `word_count` INT DEFAULT 0 COMMENT '分块字数',
  `tokens` INT DEFAULT 0 COMMENT '分块Token数(估算)',
  `index_node_id` VARCHAR(64) DEFAULT NULL COMMENT 'Qdrant point ID',
  `status` VARCHAR(20) DEFAULT 'active' COMMENT '状态: active/disabled',
  `enabled` TINYINT(1) DEFAULT 1 COMMENT '是否启用',
  `hit_count` INT DEFAULT 0 COMMENT '命中次数',
  `metadata` JSON DEFAULT NULL COMMENT '分块元数据',
  PRIMARY KEY (`id`),
  INDEX `idx_seg_kb_id` (`knowledge_base_id`),
  INDEX `idx_seg_doc_id` (`document_id`),
  INDEX `idx_seg_status` (`status`),
  FULLTEXT INDEX `ft_seg_content` (`content`) WITH PARSER ngram
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='知识库分块表';

-- 知识库查询历史表
CREATE TABLE `t_knowledge_query` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `created_at` DATETIME DEFAULT CURRENT_TIMESTAMP,
  `knowledge_base_id` BIGINT UNSIGNED NOT NULL COMMENT '知识库ID',
  `query_text` TEXT NOT NULL COMMENT '查询文本',
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
