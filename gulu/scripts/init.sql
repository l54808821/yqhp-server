-- Gulu 数据库初始化脚本
-- 包含所有表结构和初始数据
-- 执行: mysql -u <user> -p <database> < init.sql

-- ============================================
-- 1. 项目表 (t_project)
-- ============================================
CREATE TABLE IF NOT EXISTS `t_project` (
    `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `created_at` DATETIME DEFAULT NULL,
    `updated_at` DATETIME DEFAULT NULL,
    `is_delete` TINYINT(1) DEFAULT 0,
    `created_by` BIGINT UNSIGNED DEFAULT NULL COMMENT '创建人ID',
    `updated_by` BIGINT UNSIGNED DEFAULT NULL COMMENT '更新人ID',
    `team_id` BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '所属团队ID',
    `name` VARCHAR(100) NOT NULL COMMENT '项目名称',
    `description` VARCHAR(500) DEFAULT NULL COMMENT '项目描述',
    `icon` VARCHAR(255) DEFAULT NULL COMMENT '项目图标',
    `sort` BIGINT DEFAULT NULL COMMENT '排序',
    `status` TINYINT DEFAULT 1 COMMENT '状态: 1-启用 0-禁用',
    PRIMARY KEY (`id`),
    INDEX `idx_t_project_team_id` (`team_id`),
    INDEX `idx_t_project_is_delete` (`is_delete`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='项目表';

-- ============================================
-- 2. 环境表 (t_env)
-- 包含域名配置和变量配置的 JSON 字段
-- ============================================
CREATE TABLE IF NOT EXISTS `t_env` (
    `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `created_at` DATETIME DEFAULT NULL,
    `updated_at` DATETIME DEFAULT NULL,
    `is_delete` TINYINT(1) DEFAULT 0,
    `created_by` BIGINT UNSIGNED DEFAULT NULL COMMENT '创建人ID',
    `updated_by` BIGINT UNSIGNED DEFAULT NULL COMMENT '更新人ID',
    `project_id` BIGINT UNSIGNED NOT NULL COMMENT '所属项目ID',
    `name` VARCHAR(100) NOT NULL COMMENT '环境名称',
    `description` VARCHAR(500) DEFAULT NULL COMMENT '环境描述',
    `sort` BIGINT DEFAULT NULL COMMENT '排序',
    `status` TINYINT DEFAULT 1 COMMENT '状态: 1-启用 0-禁用',
    `domains` JSON DEFAULT NULL COMMENT '域名配置数组 [{"code":"main","name":"主域名","base_url":"https://...","headers":[...],"status":1,"sort":0}]',
    `vars` JSON DEFAULT NULL COMMENT '变量配置数组 [{"key":"API_KEY","name":"密钥","value":"...","type":"string","is_sensitive":false}]',
    `domains_version` INT DEFAULT 0 COMMENT '域名配置版本号(乐观锁)',
    `vars_version` INT DEFAULT 0 COMMENT '变量配置版本号(乐观锁)',
    PRIMARY KEY (`id`),
    INDEX `idx_t_env_project_id` (`project_id`),
    INDEX `idx_t_env_is_delete` (`is_delete`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='环境表';

-- ============================================
-- 3. 数据库配置表 (t_database_config)
-- ============================================
CREATE TABLE IF NOT EXISTS `t_database_config` (
    `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `created_at` DATETIME DEFAULT NULL,
    `updated_at` DATETIME DEFAULT NULL,
    `is_delete` TINYINT(1) DEFAULT 0,
    `project_id` BIGINT UNSIGNED NOT NULL COMMENT '所属项目ID',
    `env_id` BIGINT UNSIGNED NOT NULL COMMENT '所属环境ID',
    `name` VARCHAR(100) NOT NULL COMMENT '配置名称',
    `code` VARCHAR(50) NOT NULL COMMENT '配置代码',
    `type` VARCHAR(20) NOT NULL COMMENT '数据库类型: mysql, redis, mongodb',
    `host` VARCHAR(255) NOT NULL COMMENT '主机地址',
    `port` INT NOT NULL COMMENT '端口',
    `database` VARCHAR(100) DEFAULT NULL COMMENT '数据库名',
    `username` VARCHAR(100) DEFAULT NULL COMMENT '用户名',
    `password` VARCHAR(500) DEFAULT NULL COMMENT '密码(加密存储)',
    `options` TEXT DEFAULT NULL COMMENT '额外配置(JSON格式)',
    `description` VARCHAR(500) DEFAULT NULL COMMENT '描述',
    `status` TINYINT DEFAULT 1 COMMENT '状态: 1-启用 0-禁用',
    PRIMARY KEY (`id`),
    INDEX `idx_t_database_config_project_id` (`project_id`),
    INDEX `idx_t_database_config_env_id` (`env_id`),
    INDEX `idx_t_database_config_is_delete` (`is_delete`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='数据库配置表';

-- ============================================
-- 6. MQ配置表 (t_mq_config)
-- ============================================
CREATE TABLE IF NOT EXISTS `t_mq_config` (
    `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `created_at` DATETIME DEFAULT NULL,
    `updated_at` DATETIME DEFAULT NULL,
    `is_delete` TINYINT(1) DEFAULT 0,
    `project_id` BIGINT UNSIGNED NOT NULL COMMENT '所属项目ID',
    `env_id` BIGINT UNSIGNED NOT NULL COMMENT '所属环境ID',
    `name` VARCHAR(100) NOT NULL COMMENT '配置名称',
    `code` VARCHAR(50) NOT NULL COMMENT '配置代码',
    `type` VARCHAR(20) NOT NULL COMMENT 'MQ类型: rabbitmq, kafka, rocketmq',
    `host` VARCHAR(255) NOT NULL COMMENT '主机地址',
    `port` INT NOT NULL COMMENT '端口',
    `username` VARCHAR(100) DEFAULT NULL COMMENT '用户名',
    `password` VARCHAR(500) DEFAULT NULL COMMENT '密码(加密存储)',
    `vhost` VARCHAR(100) DEFAULT NULL COMMENT 'RabbitMQ vhost',
    `options` TEXT DEFAULT NULL COMMENT '额外配置(JSON格式)',
    `description` VARCHAR(500) DEFAULT NULL COMMENT '描述',
    `status` TINYINT DEFAULT 1 COMMENT '状态: 1-启用 0-禁用',
    PRIMARY KEY (`id`),
    INDEX `idx_t_mq_config_project_id` (`project_id`),
    INDEX `idx_t_mq_config_env_id` (`env_id`),
    INDEX `idx_t_mq_config_is_delete` (`is_delete`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='MQ配置表';

-- ============================================
-- 7. 执行机表 (t_executor)
-- ============================================
CREATE TABLE IF NOT EXISTS `t_executor` (
    `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `created_at` DATETIME DEFAULT NULL,
    `updated_at` DATETIME DEFAULT NULL,
    `is_delete` TINYINT(1) DEFAULT 0,
    `slave_id` VARCHAR(100) NOT NULL COMMENT 'workflow-engine的Slave ID',
    `name` VARCHAR(100) NOT NULL COMMENT '执行机名称',
    `type` VARCHAR(50) NOT NULL COMMENT '执行机类型: performance(压测专用), normal(普通), debug(调试专用)',
    `description` VARCHAR(500) DEFAULT NULL COMMENT '描述',
    `labels` JSON DEFAULT NULL COMMENT '标签 {"env":"prod","region":"cn-east"}',
    `max_vus` INT DEFAULT NULL COMMENT '最大虚拟用户数限制',
    `priority` INT DEFAULT 0 COMMENT '优先级，数值越大优先级越高',
    `status` TINYINT DEFAULT 1 COMMENT '状态: 1-启用 0-禁用',
    PRIMARY KEY (`id`),
    UNIQUE INDEX `idx_t_executor_slave_id` (`slave_id`),
    INDEX `idx_t_executor_is_delete` (`is_delete`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='执行机表';

-- ============================================
-- 8. 工作流表 (t_workflow)
-- ============================================
CREATE TABLE IF NOT EXISTS `t_workflow` (
    `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `created_at` DATETIME DEFAULT NULL,
    `updated_at` DATETIME DEFAULT NULL,
    `is_delete` TINYINT(1) DEFAULT 0,
    `created_by` BIGINT UNSIGNED DEFAULT NULL COMMENT '创建人ID',
    `updated_by` BIGINT UNSIGNED DEFAULT NULL COMMENT '更新人ID',
    `project_id` BIGINT UNSIGNED NOT NULL COMMENT '所属项目ID',
    `name` VARCHAR(100) NOT NULL COMMENT '工作流名称',
    `description` VARCHAR(500) DEFAULT NULL COMMENT '描述',
    `version` INT DEFAULT 1 COMMENT '版本号',
    `definition` LONGTEXT NOT NULL COMMENT '工作流定义(JSON格式)',
    `status` TINYINT DEFAULT 1 COMMENT '状态: 1-启用 0-禁用',
    `workflow_type` VARCHAR(20) DEFAULT 'normal' COMMENT '工作流类型: normal(普通流程), performance(压测流程), data_generation(造数流程)',
    PRIMARY KEY (`id`),
    INDEX `idx_t_workflow_project_id` (`project_id`),
    INDEX `idx_t_workflow_is_delete` (`is_delete`),
    INDEX `idx_t_workflow_type` (`workflow_type`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='工作流表';

-- ============================================
-- 9. 执行记录表 (t_execution)
-- ============================================
CREATE TABLE IF NOT EXISTS `t_execution` (
    `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `created_at` DATETIME DEFAULT NULL,
    `updated_at` DATETIME DEFAULT NULL,
    `project_id` BIGINT UNSIGNED NOT NULL COMMENT '所属项目ID',
    `workflow_id` BIGINT UNSIGNED NOT NULL COMMENT '工作流ID',
    `env_id` BIGINT UNSIGNED NOT NULL COMMENT '执行环境ID',
    `executor_id` VARCHAR(100) DEFAULT NULL COMMENT '执行机ID(来自workflow-engine)',
    `execution_id` VARCHAR(100) NOT NULL COMMENT 'workflow-engine返回的执行ID',
    `mode` VARCHAR(20) NOT NULL DEFAULT 'execute' COMMENT '执行模式: debug, execute',
    `status` VARCHAR(20) NOT NULL COMMENT '执行状态: pending, running, completed, failed, stopped, timeout',
    `start_time` DATETIME DEFAULT NULL COMMENT '开始时间',
    `end_time` DATETIME DEFAULT NULL COMMENT '结束时间',
    `duration` BIGINT DEFAULT NULL COMMENT '执行时长(毫秒)',
    `total_steps` INT DEFAULT 0 COMMENT '总步骤数',
    `success_steps` INT DEFAULT 0 COMMENT '成功步骤数',
    `failed_steps` INT DEFAULT 0 COMMENT '失败步骤数',
    `result` LONGTEXT DEFAULT NULL COMMENT '执行结果(JSON格式)',
    `logs` LONGTEXT DEFAULT NULL COMMENT '执行日志',
    `created_by` BIGINT UNSIGNED DEFAULT NULL COMMENT '创建人ID',
    PRIMARY KEY (`id`),
    INDEX `idx_t_execution_project_id` (`project_id`),
    INDEX `idx_t_execution_workflow_id` (`workflow_id`),
    INDEX `idx_t_execution_env_id` (`env_id`),
    INDEX `idx_t_execution_execution_id` (`execution_id`),
    INDEX `idx_t_execution_mode` (`mode`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='执行记录表';

-- ============================================
-- 10. 团队表 (t_team)
-- ============================================
CREATE TABLE IF NOT EXISTS `t_team` (
    `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `created_at` DATETIME DEFAULT NULL,
    `updated_at` DATETIME DEFAULT NULL,
    `is_delete` TINYINT(1) DEFAULT 0,
    `created_by` BIGINT UNSIGNED DEFAULT NULL COMMENT '创建人ID',
    `updated_by` BIGINT UNSIGNED DEFAULT NULL COMMENT '更新人ID',
    `name` VARCHAR(100) NOT NULL COMMENT '团队名称',
    `description` VARCHAR(500) DEFAULT NULL COMMENT '团队描述',
    `status` TINYINT DEFAULT 1 COMMENT '状态: 1-启用 0-禁用',
    PRIMARY KEY (`id`),
    INDEX `idx_t_team_is_delete` (`is_delete`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='团队表';

-- ============================================
-- 11. 团队成员表 (t_team_member)
-- ============================================
CREATE TABLE IF NOT EXISTS `t_team_member` (
    `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `created_at` DATETIME DEFAULT NULL,
    `team_id` BIGINT UNSIGNED NOT NULL COMMENT '团队ID',
    `user_id` BIGINT UNSIGNED NOT NULL COMMENT '用户ID',
    `role` VARCHAR(20) NOT NULL DEFAULT 'member' COMMENT '角色: owner/admin/member',
    PRIMARY KEY (`id`),
    UNIQUE INDEX `idx_team_user` (`team_id`, `user_id`),
    INDEX `idx_t_team_member_team_id` (`team_id`),
    INDEX `idx_t_team_member_user_id` (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='团队成员表';

-- ============================================
-- 12. 项目成员表 (t_project_member)
-- ============================================
CREATE TABLE IF NOT EXISTS `t_project_member` (
    `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `created_at` DATETIME DEFAULT NULL,
    `project_id` BIGINT UNSIGNED NOT NULL COMMENT '项目ID',
    `user_id` BIGINT UNSIGNED NOT NULL COMMENT '用户ID',
    PRIMARY KEY (`id`),
    UNIQUE INDEX `idx_project_user` (`project_id`, `user_id`),
    INDEX `idx_t_project_member_project_id` (`project_id`),
    INDEX `idx_t_project_member_user_id` (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='项目成员表';

-- ============================================
-- 13. 项目权限表 (t_project_permission)
-- ============================================
CREATE TABLE IF NOT EXISTS `t_project_permission` (
    `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `created_at` DATETIME DEFAULT NULL,
    `project_id` BIGINT UNSIGNED NOT NULL COMMENT '项目ID',
    `user_id` BIGINT UNSIGNED NOT NULL COMMENT '用户ID',
    `permission_code` VARCHAR(50) NOT NULL COMMENT '权限代码',
    PRIMARY KEY (`id`),
    UNIQUE INDEX `idx_project_user_permission` (`project_id`, `user_id`, `permission_code`),
    INDEX `idx_t_project_permission_project_id` (`project_id`),
    INDEX `idx_t_project_permission_user_id` (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='项目权限表';

-- ============================================
-- 14. 工作流分类表 (t_category_workflow)
-- ============================================
CREATE TABLE IF NOT EXISTS `t_category_workflow` (
    `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `created_at` DATETIME DEFAULT NULL,
    `updated_at` DATETIME DEFAULT NULL,
    `is_delete` TINYINT(1) DEFAULT 0,
    `project_id` BIGINT UNSIGNED NOT NULL COMMENT '项目ID',
    `parent_id` BIGINT UNSIGNED DEFAULT 0 COMMENT '父分类ID，0表示根节点',
    `name` VARCHAR(100) NOT NULL COMMENT '分类名称',
    `type` VARCHAR(20) NOT NULL DEFAULT 'folder' COMMENT '类型: folder/workflow',
    `source_id` BIGINT UNSIGNED DEFAULT NULL COMMENT '关联的工作流ID，type=workflow时有值',
    `sort` INT DEFAULT 0 COMMENT '排序',
    PRIMARY KEY (`id`),
    INDEX `idx_t_category_workflow_project_id` (`project_id`),
    INDEX `idx_t_category_workflow_parent_id` (`parent_id`),
    INDEX `idx_t_category_workflow_is_delete` (`is_delete`),
    INDEX `idx_t_category_workflow_source_id` (`source_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='工作流分类表';

-- ============================================
-- 完成提示
-- ============================================
SELECT 'Gulu 数据库初始化完成!' AS message;
