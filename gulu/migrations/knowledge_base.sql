-- 知识库管理模块 - 数据库迁移脚本
-- 执行方式: mysql -u root -p yqhp_admin < migrations/knowledge_base.sql

-- 知识库主表
CREATE TABLE IF NOT EXISTS `t_knowledge_base` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `created_at` DATETIME DEFAULT NULL,
  `updated_at` DATETIME DEFAULT NULL,
  `is_delete` TINYINT(1) DEFAULT 0,
  `created_by` BIGINT UNSIGNED DEFAULT NULL COMMENT '创建人ID',
  `updated_by` BIGINT UNSIGNED DEFAULT NULL COMMENT '更新人ID',
  `project_id` BIGINT UNSIGNED NOT NULL COMMENT '所属项目ID',
  `name` VARCHAR(100) NOT NULL COMMENT '知识库名称',
  `description` VARCHAR(1024) DEFAULT NULL COMMENT '知识库描述',
  `type` VARCHAR(20) NOT NULL DEFAULT 'normal' COMMENT '知识库类型: normal-普通向量知识库, graph-图知识库',
  `status` TINYINT DEFAULT 1 COMMENT '状态: 1-启用 0-禁用',
  `embedding_model_id` BIGINT UNSIGNED DEFAULT NULL COMMENT '嵌入模型ID(关联t_ai_model)',
  `embedding_model_name` VARCHAR(100) DEFAULT NULL COMMENT '嵌入模型名称(冗余显示)',
  `embedding_dimension` INT DEFAULT 1536 COMMENT '向量维度',
  `chunk_size` INT DEFAULT 500 COMMENT '文本分块大小(字符数)',
  `chunk_overlap` INT DEFAULT 50 COMMENT '分块重叠大小(字符数)',
  `similarity_threshold` FLOAT DEFAULT 0.7 COMMENT '默认相似度阈值',
  `top_k` INT DEFAULT 5 COMMENT '默认检索数量',
  `qdrant_collection` VARCHAR(100) DEFAULT NULL COMMENT 'Qdrant Collection 名称(normal类型)',
  `neo4j_database` VARCHAR(100) DEFAULT NULL COMMENT 'Neo4j 数据库名称(graph类型)',
  `document_count` INT DEFAULT 0 COMMENT '文档数量',
  `chunk_count` INT DEFAULT 0 COMMENT '分块数量',
  `metadata` JSON DEFAULT NULL COMMENT '扩展元数据',
  PRIMARY KEY (`id`),
  INDEX `idx_t_knowledge_base_project_id` (`project_id`),
  INDEX `idx_t_knowledge_base_is_delete` (`is_delete`),
  INDEX `idx_t_knowledge_base_type` (`type`),
  INDEX `idx_t_knowledge_base_status` (`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='知识库主表';

-- 知识库文档表
CREATE TABLE IF NOT EXISTS `t_knowledge_document` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `created_at` DATETIME DEFAULT NULL,
  `updated_at` DATETIME DEFAULT NULL,
  `knowledge_base_id` BIGINT UNSIGNED NOT NULL COMMENT '所属知识库ID',
  `name` VARCHAR(255) NOT NULL COMMENT '文档名称',
  `file_type` VARCHAR(20) DEFAULT NULL COMMENT '文件类型: pdf/txt/md/docx/html/image/audio/video',
  `file_path` VARCHAR(500) DEFAULT NULL COMMENT '文件存储路径',
  `file_size` BIGINT DEFAULT 0 COMMENT '文件大小(字节)',
  `content` LONGTEXT DEFAULT NULL COMMENT '文档原始文本内容(小文件直接存储)',
  `separator` VARCHAR(50) DEFAULT '\\n\\n' COMMENT '分段标识符',
  `chunk_size` INT DEFAULT NULL COMMENT '分段最大长度(字符数), NULL则使用知识库默认值',
  `chunk_overlap` INT DEFAULT NULL COMMENT '分段重叠长度(字符数), NULL则使用知识库默认值',
  `clean_whitespace` TINYINT(1) DEFAULT 1 COMMENT '替换连续空格/换行符/制表符',
  `remove_urls` TINYINT(1) DEFAULT 0 COMMENT '删除URL和邮件地址',
  `status` VARCHAR(20) DEFAULT 'pending' COMMENT '处理状态: pending-待处理, processing-处理中, ready-就绪, failed-失败',
  `error_message` TEXT DEFAULT NULL COMMENT '错误信息',
  `chunk_count` INT DEFAULT 0 COMMENT '分块数量',
  `token_count` INT DEFAULT 0 COMMENT 'Token 数量(估算)',
  `metadata` JSON DEFAULT NULL COMMENT '文档元数据',
  PRIMARY KEY (`id`),
  INDEX `idx_t_knowledge_document_kb_id` (`knowledge_base_id`),
  INDEX `idx_t_knowledge_document_status` (`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='知识库文档表';
