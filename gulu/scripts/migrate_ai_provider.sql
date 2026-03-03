-- ============================================
-- 迁移脚本：从 t_ai_model 提取供应商到 t_ai_provider
-- 执行前请先备份数据库
-- ============================================

-- 1. 创建 t_ai_provider 表（如果不存在）
CREATE TABLE IF NOT EXISTS `t_ai_provider` (
    `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `created_at` DATETIME DEFAULT NULL,
    `updated_at` DATETIME DEFAULT NULL,
    `is_delete` TINYINT(1) DEFAULT 0,
    `created_by` BIGINT UNSIGNED DEFAULT NULL COMMENT '创建人ID',
    `name` VARCHAR(100) NOT NULL COMMENT '供应商名称',
    `provider_type` VARCHAR(50) NOT NULL COMMENT '供应商类型标识',
    `api_base_url` VARCHAR(500) NOT NULL COMMENT 'API Base URL',
    `api_key` VARCHAR(500) DEFAULT '' COMMENT 'API Key',
    `icon` VARCHAR(200) DEFAULT NULL COMMENT '供应商图标',
    `description` VARCHAR(500) DEFAULT NULL COMMENT '供应商描述',
    `sort` INT DEFAULT 0 COMMENT '排序',
    `status` TINYINT DEFAULT 1 COMMENT '状态: 1-启用 0-禁用',
    PRIMARY KEY (`id`),
    INDEX `idx_t_ai_provider_is_delete` (`is_delete`),
    INDEX `idx_t_ai_provider_status` (`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='AI供应商表';

-- 2. 从现有 t_ai_model 中提取唯一的 (provider, api_base_url, api_key) 组合，插入 t_ai_provider
INSERT INTO `t_ai_provider` (`created_at`, `updated_at`, `is_delete`, `name`, `provider_type`, `api_base_url`, `api_key`, `status`)
SELECT
    NOW(), NOW(), 0,
    m.provider,
    LOWER(REPLACE(REPLACE(m.provider, ' ', ''), 'AI', 'ai')),
    m.api_base_url,
    m.api_key,
    1
FROM `t_ai_model` m
WHERE m.is_delete = 0
GROUP BY m.provider, m.api_base_url, m.api_key;

-- 3. 给 t_ai_model 添加 provider_id 列（如果不存在）
-- MySQL 不支持 ADD COLUMN IF NOT EXISTS，用存储过程兼容
DROP PROCEDURE IF EXISTS _add_provider_id;
DELIMITER //
CREATE PROCEDURE _add_provider_id()
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.COLUMNS
        WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 't_ai_model' AND COLUMN_NAME = 'provider_id'
    ) THEN
        ALTER TABLE `t_ai_model` ADD COLUMN `provider_id` BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '关联供应商ID' AFTER `created_by`;
        ALTER TABLE `t_ai_model` ADD INDEX `idx_t_ai_model_provider_id` (`provider_id`);
    END IF;
END //
DELIMITER ;
CALL _add_provider_id();
DROP PROCEDURE IF EXISTS _add_provider_id;

-- 4. 更新 t_ai_model.provider_id，关联到对应的 t_ai_provider
UPDATE `t_ai_model` m
INNER JOIN `t_ai_provider` p
    ON m.provider = p.name AND m.api_base_url = p.api_base_url AND m.api_key = p.api_key
SET m.provider_id = p.id
WHERE m.is_delete = 0;

-- 5. 验证迁移结果
SELECT '=== 迁移验证 ===' AS info;
SELECT COUNT(*) AS total_providers FROM t_ai_provider WHERE is_delete = 0;
SELECT COUNT(*) AS total_models FROM t_ai_model WHERE is_delete = 0;
SELECT COUNT(*) AS models_without_provider FROM t_ai_model WHERE is_delete = 0 AND provider_id = 0;

-- 注意：旧的 api_base_url 和 api_key 列暂时保留，确认迁移无误后可手动删除：
-- ALTER TABLE `t_ai_model` DROP COLUMN `api_base_url`;
-- ALTER TABLE `t_ai_model` DROP COLUMN `api_key`;
