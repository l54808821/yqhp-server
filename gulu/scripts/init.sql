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
    PRIMARY KEY (`id`),
    INDEX `idx_t_env_project_id` (`project_id`),
    INDEX `idx_t_env_is_delete` (`is_delete`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='环境表';

-- ============================================
-- 3. 配置定义表 (t_config_definition)
-- 项目级别，定义有哪些配置项
-- ============================================
CREATE TABLE IF NOT EXISTS `t_config_definition` (
    `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `created_at` DATETIME DEFAULT NULL,
    `updated_at` DATETIME DEFAULT NULL,
    `is_delete` TINYINT(1) DEFAULT 0,
    `project_id` BIGINT UNSIGNED NOT NULL COMMENT '项目ID',
    `type` VARCHAR(32) NOT NULL COMMENT '配置类型: domain/variable/database/mq',
    `code` VARCHAR(64) NOT NULL COMMENT '系统生成的唯一ID',
    `name` VARCHAR(128) NOT NULL COMMENT '显示名称',
    `description` VARCHAR(500) DEFAULT '' COMMENT '描述',
    `extra` JSON DEFAULT NULL COMMENT '类型特有属性',
    `sort` INT DEFAULT 0 COMMENT '排序',
    `status` TINYINT DEFAULT 1 COMMENT '状态: 1-启用 0-禁用',
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_code` (`code`),
    INDEX `idx_project_type` (`project_id`, `type`),
    INDEX `idx_is_delete` (`is_delete`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='配置定义表';

-- ============================================
-- 4. 配置表 (t_config)
-- 环境级别，存储每个环境下的配置值
-- ============================================
CREATE TABLE IF NOT EXISTS `t_config` (
    `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `created_at` DATETIME DEFAULT NULL,
    `updated_at` DATETIME DEFAULT NULL,
    `project_id` BIGINT UNSIGNED NOT NULL COMMENT '项目ID(冗余)',
    `env_id` BIGINT UNSIGNED NOT NULL COMMENT '环境ID',
    `type` VARCHAR(32) NOT NULL COMMENT '配置类型(冗余，方便查询)',
    `code` VARCHAR(64) NOT NULL COMMENT '关联配置定义的code',
    `value` JSON NOT NULL COMMENT '配置值',
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_env_code` (`env_id`, `code`),
    INDEX `idx_env_type` (`env_id`, `type`),
    INDEX `idx_code` (`code`),
    INDEX `idx_project_id` (`project_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='配置表';

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
    `executor_config` JSON DEFAULT NULL COMMENT '执行机配置: {"strategy":"auto|manual|local","executor_id":null,"labels":{"env":"prod"}}',
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
    `source_id` BIGINT UNSIGNED NOT NULL COMMENT '来源ID(工作流ID/测试计划ID等)',
    `env_id` BIGINT UNSIGNED NOT NULL COMMENT '执行环境ID',
    `executor_id` VARCHAR(100) DEFAULT NULL COMMENT '执行机ID(来自workflow-engine)',
    `execution_id` VARCHAR(100) NOT NULL COMMENT 'workflow-engine返回的执行ID',
    `mode` VARCHAR(20) NOT NULL DEFAULT 'execute' COMMENT '执行模式: debug, execute',
    `source_type` VARCHAR(30) NOT NULL DEFAULT 'performance' COMMENT '来源类型: performance(性能测试), test_plan(测试计划), debug(调试)',
    `title` VARCHAR(256) NOT NULL DEFAULT '' COMMENT '执行标题(如工作流名称)',
    `status` VARCHAR(20) NOT NULL COMMENT '执行状态: pending, running, completed, failed, stopped, timeout',
    `start_time` DATETIME DEFAULT NULL COMMENT '开始时间',
    `end_time` DATETIME DEFAULT NULL COMMENT '结束时间',
    `duration` BIGINT DEFAULT NULL COMMENT '执行时长(毫秒)',
    `created_by` BIGINT UNSIGNED DEFAULT NULL COMMENT '创建人ID',
    PRIMARY KEY (`id`),
    INDEX `idx_t_execution_project_id` (`project_id`),
    INDEX `idx_t_execution_source_id` (`source_id`),
    INDEX `idx_t_execution_env_id` (`env_id`),
    INDEX `idx_t_execution_execution_id` (`execution_id`),
    INDEX `idx_t_execution_mode` (`mode`),
    INDEX `idx_t_execution_source_type` (`source_type`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='执行记录表';

-- ============================================
-- 9.1 性能测试执行详情表 (t_execution_perf_detail)
-- ============================================
CREATE TABLE IF NOT EXISTS `t_execution_perf_detail` (
    `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `execution_id` VARCHAR(100) NOT NULL COMMENT '关联 t_execution.execution_id',
    `total_requests` BIGINT DEFAULT 0 COMMENT '总请求数',
    `success_requests` BIGINT DEFAULT 0 COMMENT '成功请求数',
    `failed_requests` BIGINT DEFAULT 0 COMMENT '失败请求数',
    `error_rate` DOUBLE DEFAULT 0 COMMENT '错误率(%)',
    `avg_qps` DOUBLE DEFAULT 0 COMMENT '平均QPS',
    `peak_qps` DOUBLE DEFAULT 0 COMMENT '峰值QPS',
    `avg_rt_ms` DOUBLE DEFAULT 0 COMMENT '平均响应时间(ms)',
    `min_rt_ms` DOUBLE DEFAULT 0 COMMENT '最小响应时间(ms)',
    `max_rt_ms` DOUBLE DEFAULT 0 COMMENT '最大响应时间(ms)',
    `p50_rt_ms` DOUBLE DEFAULT 0 COMMENT 'P50响应时间(ms)',
    `p90_rt_ms` DOUBLE DEFAULT 0 COMMENT 'P90响应时间(ms)',
    `p95_rt_ms` DOUBLE DEFAULT 0 COMMENT 'P95响应时间(ms)',
    `p99_rt_ms` DOUBLE DEFAULT 0 COMMENT 'P99响应时间(ms)',
    `max_vus` INT DEFAULT 0 COMMENT '最大并发用户数',
    `total_iterations` BIGINT DEFAULT 0 COMMENT '总迭代数',
    `throughput_bps` DOUBLE DEFAULT 0 COMMENT '吞吐量(bytes/sec)',
    `total_data_sent` BIGINT DEFAULT 0 COMMENT '总发送数据量(bytes)',
    `total_data_received` BIGINT DEFAULT 0 COMMENT '总接收数据量(bytes)',
    `thresholds_pass_rate` DOUBLE DEFAULT 0 COMMENT '阈值通过率',
    `time_series` LONGTEXT DEFAULT NULL COMMENT '时序数据(JSON数组)',
    `step_details` LONGTEXT DEFAULT NULL COMMENT '步骤详情(JSON数组)',
    `thresholds` TEXT DEFAULT NULL COMMENT '阈值结果(JSON数组)',
    `error_analysis` TEXT DEFAULT NULL COMMENT '错误分析(JSON)',
    `vu_timeline` TEXT DEFAULT NULL COMMENT 'VU时间线(JSON数组)',
    `config` TEXT DEFAULT NULL COMMENT '执行配置(JSON)',
    `workflow_name` VARCHAR(256) DEFAULT '' COMMENT '工作流名称(快照)',
    `created_at` DATETIME DEFAULT NULL,
    `updated_at` DATETIME DEFAULT NULL,
    PRIMARY KEY (`id`),
    UNIQUE INDEX `uk_execution_id` (`execution_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='性能测试执行详情表';

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
-- 15. AI模型表 (t_ai_model)
-- ============================================
CREATE TABLE IF NOT EXISTS `t_ai_model` (
    `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `created_at` DATETIME DEFAULT NULL,
    `updated_at` DATETIME DEFAULT NULL,
    `is_delete` TINYINT(1) DEFAULT 0,
    `created_by` BIGINT UNSIGNED DEFAULT NULL COMMENT '创建人ID',
    `name` VARCHAR(100) NOT NULL COMMENT '模型名称，如 DeepSeek-V3',
    `provider` VARCHAR(100) NOT NULL COMMENT '厂商，如 DeepSeek、Kimi、智谱',
    `model_id` VARCHAR(200) NOT NULL COMMENT '模型标识符，调用API时使用的model参数',
    `version` VARCHAR(50) DEFAULT NULL COMMENT '版本号',
    `description` VARCHAR(1000) DEFAULT NULL COMMENT '模型描述',
    `api_base_url` VARCHAR(500) NOT NULL COMMENT 'API Base URL',
    `api_key` VARCHAR(500) NOT NULL COMMENT 'API Key',
    `context_length` INT DEFAULT NULL COMMENT '上下文长度，如 8192、131072',
    `param_size` VARCHAR(50) DEFAULT NULL COMMENT '参数量，如 7B、13B、671B',
    `capability_tags` JSON DEFAULT NULL COMMENT '能力标签 ["对话","FIM","Tools","视觉","MoE"]',
    `custom_tags` JSON DEFAULT NULL COMMENT '自定义标签 ["推荐","内部测试"]',
    `sort` INT DEFAULT 0 COMMENT '排序',
    `status` TINYINT DEFAULT 1 COMMENT '状态: 1-启用 0-禁用',
    PRIMARY KEY (`id`),
    INDEX `idx_t_ai_model_provider` (`provider`),
    INDEX `idx_t_ai_model_is_delete` (`is_delete`),
    INDEX `idx_t_ai_model_status` (`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='AI模型表';

-- ============================================
-- 16. MCP服务器配置表 (t_mcp_server)
-- ============================================
CREATE TABLE IF NOT EXISTS `t_mcp_server` (
    `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `created_at` DATETIME DEFAULT NULL,
    `updated_at` DATETIME DEFAULT NULL,
    `is_delete` TINYINT(1) DEFAULT NULL,
    `created_by` BIGINT UNSIGNED DEFAULT NULL COMMENT '创建人ID',
    `name` VARCHAR(100) NOT NULL COMMENT '服务器名称',
    `description` VARCHAR(500) DEFAULT NULL COMMENT '描述',
    `transport` VARCHAR(20) NOT NULL COMMENT '传输方式: stdio/sse',
    `command` VARCHAR(500) DEFAULT NULL COMMENT 'stdio模式命令',
    `args` JSON DEFAULT NULL COMMENT 'stdio模式参数',
    `url` VARCHAR(500) DEFAULT NULL COMMENT 'sse模式URL',
    `env` JSON DEFAULT NULL COMMENT '环境变量',
    `timeout` INT DEFAULT 30 COMMENT '超时秒数',
    `sort` INT DEFAULT NULL COMMENT '排序',
    `status` TINYINT DEFAULT 1 COMMENT '状态: 1-启用 0-禁用',
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_name` (`name`),
    INDEX `idx_t_mcp_server_is_delete` (`is_delete`),
    INDEX `idx_t_mcp_server_status` (`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='MCP服务器配置表';

-- ============================================
-- 17. AI Skill表 (t_skill) - 兼容 Agent Skills 开放标准
-- ============================================
CREATE TABLE IF NOT EXISTS `t_skill` (
    `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `created_at` DATETIME DEFAULT NULL,
    `updated_at` DATETIME DEFAULT NULL,
    `is_delete` TINYINT(1) DEFAULT 0,
    `created_by` BIGINT UNSIGNED DEFAULT NULL COMMENT '创建人ID',
    -- 基本信息
    `name` VARCHAR(100) NOT NULL COMMENT '显示名称（支持中文）',
    `slug` VARCHAR(64) DEFAULT NULL COMMENT '标准化名称（kebab-case，如 code-review，兼容 Agent Skills 规范）',
    `description` VARCHAR(1024) DEFAULT NULL COMMENT 'Skill描述（规范要求 max 1024）',
    `icon` VARCHAR(100) DEFAULT NULL COMMENT '图标标识',
    `category` VARCHAR(50) DEFAULT NULL COMMENT '分类: 编程/写作/分析/翻译等',
    `tags` JSON DEFAULT NULL COMMENT '标签 ["Python","代码审查"]',
    -- 核心内容（对应 SKILL.md body）
    `system_prompt` TEXT NOT NULL COMMENT '系统提示词（SKILL.md body 内容）',
    `variables` JSON DEFAULT NULL COMMENT '变量声明',
    -- 推荐配置
    `recommended_model_params` JSON DEFAULT NULL COMMENT '推荐模型参数',
    `recommended_tools` JSON DEFAULT NULL COMMENT '推荐内置工具',
    `recommended_mcp_server_ids` JSON DEFAULT NULL COMMENT '推荐MCP服务ID',
    -- Agent Skills 规范字段
    `license` VARCHAR(500) DEFAULT NULL COMMENT '许可证（Agent Skills 规范）',
    `compatibility` VARCHAR(500) DEFAULT NULL COMMENT '环境要求（Agent Skills 规范）',
    `metadata_json` JSON DEFAULT NULL COMMENT '扩展元数据（Agent Skills metadata，任意 key-value）',
    `allowed_tools` TEXT DEFAULT NULL COMMENT '预批准工具列表（Agent Skills allowed-tools，空格分隔）',
    -- 管理字段
    `type` TINYINT DEFAULT 0 COMMENT '类型: 0-用户自建 1-系统内置 2-导入',
    `is_public` TINYINT DEFAULT 0 COMMENT '是否公开（Skill广场预留）',
    `version` VARCHAR(20) DEFAULT '1.0.0' COMMENT '版本号',
    `sort` INT DEFAULT 0 COMMENT '排序',
    `status` TINYINT DEFAULT 1 COMMENT '状态: 1-启用 0-禁用',
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_t_skill_slug` (`slug`),
    INDEX `idx_t_skill_type` (`type`),
    INDEX `idx_t_skill_category` (`category`),
    INDEX `idx_t_skill_is_delete` (`is_delete`),
    INDEX `idx_t_skill_status` (`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='AI Skill表（兼容 Agent Skills 开放标准）';

-- ============================================
-- 18. Skill 资源文件表 (t_skill_resource)
-- ============================================
CREATE TABLE IF NOT EXISTS `t_skill_resource` (
    `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `created_at` DATETIME DEFAULT NULL,
    `updated_at` DATETIME DEFAULT NULL,
    `skill_id` BIGINT UNSIGNED NOT NULL COMMENT '所属 Skill ID',
    `category` VARCHAR(20) NOT NULL COMMENT '资源类别: scripts/references/assets',
    `filename` VARCHAR(255) NOT NULL COMMENT '文件名',
    `content` LONGTEXT DEFAULT NULL COMMENT '文件内容（文本类型）',
    `content_type` VARCHAR(100) DEFAULT 'text/plain' COMMENT 'MIME 类型',
    `size` INT DEFAULT 0 COMMENT '文件大小（字节）',
    PRIMARY KEY (`id`),
    INDEX `idx_skill_resource_skill_id` (`skill_id`),
    INDEX `idx_skill_resource_category` (`category`),
    UNIQUE KEY `uk_skill_resource_file` (`skill_id`, `category`, `filename`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Skill 资源文件表（scripts/references/assets）';

-- ============================================
-- 内置 Skill 初始数据
-- ============================================
INSERT INTO `t_skill` (`created_at`, `updated_at`, `is_delete`, `name`, `slug`, `description`, `icon`, `category`, `tags`, `system_prompt`, `variables`, `recommended_model_params`, `recommended_tools`, `license`, `type`, `is_public`, `version`, `sort`, `status`) VALUES
(NOW(), NOW(), 0, '代码审查专家', 'code-review', '对代码进行专业的 Code Review，指出潜在问题、安全隐患和优化建议。Use when reviewing code quality, finding bugs, or checking best practices.', 'lucide:search-code', '编程', '["代码审查","Code Review","最佳实践"]',
'你是一位资深的代码审查专家。请对用户提供的代码进行全面审查，包括：\n\n1. **代码质量**：命名规范、代码结构、可读性\n2. **潜在 Bug**：空指针、边界条件、并发问题\n3. **安全隐患**：注入攻击、敏感信息泄露、权限问题\n4. **性能优化**：算法复杂度、资源泄漏、不必要的开销\n5. **最佳实践**：设计模式、SOLID 原则、项目规范\n\n请用清晰的格式逐条列出问题，并给出改进建议和示例代码。严重程度标注为：🔴 严重 / 🟡 警告 / 🔵 建议。',
NULL, '{"temperature": 0.3, "max_tokens": 4096}', NULL, 'Apache-2.0', 1, 1, '1.0.0', 100, 1),

(NOW(), NOW(), 0, '测试用例生成器', 'test-case-generator', '根据需求描述或代码自动生成测试用例。Use when generating test cases, unit tests, or test plans from requirements or code.', 'lucide:test-tubes', '编程', '["测试","单元测试","用例生成"]',
'你是一位测试工程专家。请根据用户提供的需求描述或代码，生成全面的测试用例。\n\n要求：\n1. 覆盖正常场景、边界条件和异常场景\n2. 每个用例包含：用例名称、前置条件、测试步骤、预期结果\n3. 如果提供了代码，额外生成单元测试代码\n4. 关注等价类划分和边界值分析\n5. 考虑安全性和性能相关测试场景\n\n输出格式清晰，便于直接导入测试管理系统。',
NULL, '{"temperature": 0.5, "max_tokens": 4096}', NULL, 'Apache-2.0', 1, 1, '1.0.0', 90, 1),

(NOW(), NOW(), 0, 'SQL 专家', 'sql-expert', '精通 SQL 编写、优化和问题排查。Use when writing SQL queries, optimizing slow queries, or designing database schemas.', 'lucide:database', '编程', '["SQL","数据库","性能优化"]',
'你是一位数据库和 SQL 专家，精通 MySQL、PostgreSQL 等主流数据库。\n\n你的职责：\n1. 根据需求编写高效的 SQL 查询语句\n2. 分析和优化慢 SQL，给出执行计划解读\n3. 设计合理的表结构和索引策略\n4. 排查 SQL 相关问题（死锁、性能瓶颈等）\n\n在回答时：\n- 给出完整可执行的 SQL 代码\n- 解释查询逻辑和设计思路\n- 标注可能的性能影响和注意事项\n- 如有多种方案，对比各方案的优劣',
NULL, '{"temperature": 0.3, "max_tokens": 4096}', NULL, 'Apache-2.0', 1, 1, '1.0.0', 80, 1),

(NOW(), NOW(), 0, 'API 文档生成器', 'api-doc-generator', '根据代码自动生成清晰的 API 接口文档。Use when generating API documentation from code or interface definitions.', 'lucide:file-text', '编程', '["API","文档","接口"]',
'你是一位技术文档专家，擅长编写清晰规范的 API 文档。\n\n请根据用户提供的代码或接口信息，生成标准的 API 文档，包含：\n\n1. **接口概述**：功能说明、请求方式、URL\n2. **请求参数**：参数名、类型、必填、描述、示例值\n3. **请求示例**：完整的 cURL 或代码示例\n4. **响应格式**：字段说明和 JSON 示例\n5. **错误码**：可能的错误码和说明\n6. **注意事项**：认证方式、限流、特殊说明\n\n使用 Markdown 格式输出，确保文档可直接使用。',
NULL, '{"temperature": 0.3, "max_tokens": 4096}', NULL, 'Apache-2.0', 1, 1, '1.0.0', 70, 1),

(NOW(), NOW(), 0, '翻译助手', 'translation', '专业的多语言翻译，保持原文语气和上下文准确性。Use when translating text between languages or localizing content.', 'lucide:languages', '翻译', '["翻译","多语言","本地化"]',
'你是一位专业的多语言翻译专家。请遵循以下原则：\n\n1. **准确性**：忠实原文含义，不遗漏不添加\n2. **自然性**：使用目标语言的地道表达，避免翻译腔\n3. **一致性**：专业术语保持统一翻译\n4. **语境感知**：根据上下文选择合适的措辞和语气\n\n翻译技术文档时，保留代码块、变量名、API 名称等技术标识不翻译。\n\n如果原文有歧义或多种理解方式，请标注并提供不同版本的翻译。',
NULL, '{"temperature": 0.3, "max_tokens": 4096}', NULL, 'Apache-2.0', 1, 1, '1.0.0', 60, 1),

(NOW(), NOW(), 0, '数据分析师', 'data-analysis', '分析数据并提供洞察、趋势和可视化建议。Use when analyzing datasets, finding patterns, or generating data-driven insights.', 'lucide:bar-chart-3', '分析', '["数据分析","统计","报表"]',
'你是一位资深数据分析师。请对用户提供的数据进行深入分析：\n\n1. **数据概览**：数据规模、字段说明、数据质量评估\n2. **统计分析**：关键指标、分布特征、趋势变化\n3. **深度洞察**：异常点、相关性、潜在规律\n4. **可视化建议**：推荐合适的图表类型和展示方式\n5. **行动建议**：基于分析结论给出具体可执行的建议\n\n分析时注意：\n- 用通俗易懂的语言解释统计概念\n- 区分相关性和因果性\n- 标注数据局限性和分析假设',
NULL, '{"temperature": 0.5, "max_tokens": 4096}', NULL, 'Apache-2.0', 1, 1, '1.0.0', 50, 1),

(NOW(), NOW(), 0, '技术文档写手', 'tech-writing', '撰写清晰专业的技术文档、README和架构设计文档。Use when writing technical documentation, READMEs, or architecture design docs.', 'lucide:pen-line', '写作', '["技术文档","README","架构设计"]',
'你是一位技术写作专家，擅长编写各类技术文档。\n\n写作原则：\n1. **结构清晰**：层次分明，使用标题、列表、表格组织内容\n2. **简洁准确**：用最少的文字传达最多的信息\n3. **读者友好**：考虑目标读者的技术水平，适当解释术语\n4. **示例丰富**：提供代码示例、配置示例和使用截图建议\n5. **可维护性**：文档结构便于后续更新和维护\n\n支持的文档类型：README、API 文档、架构设计文档、部署指南、开发规范等。\n根据用户需求选择合适的模板和格式。',
NULL, '{"temperature": 0.5, "max_tokens": 4096}', NULL, 'Apache-2.0', 1, 1, '1.0.0', 40, 1),

(NOW(), NOW(), 0, '接口测试分析', 'api-test-analysis', '分析 HTTP 接口响应，判断接口行为是否符合预期。Use when analyzing HTTP responses, validating API behavior, or debugging API issues.', 'lucide:activity', '测试', '["接口测试","HTTP","断言"]',
'你是一位接口测试专家。请分析用户提供的 HTTP 请求和响应信息：\n\n1. **状态码分析**：HTTP 状态码是否符合预期\n2. **响应体验证**：数据格式、字段完整性、值的合理性\n3. **性能评估**：响应时间、数据量大小\n4. **一致性检查**：与 API 文档或预期行为的偏差\n5. **安全检查**：敏感信息泄露、认证问题\n\n给出明确的结论：\n- ✅ 通过：符合预期\n- ❌ 失败：不符合预期，说明原因\n- ⚠️ 警告：存在潜在问题\n\n如有问题，提供排查思路和建议。',
NULL, '{"temperature": 0.3, "max_tokens": 4096}', '["http_request","json_parse"]', 'Apache-2.0', 1, 1, '1.0.0', 30, 1);

-- ============================================
-- AI 工作流会话表 (t_ai_conversation)
-- ============================================
CREATE TABLE IF NOT EXISTS `t_ai_conversation` (
    `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `workflow_id` BIGINT UNSIGNED NOT NULL COMMENT '关联的工作流ID',
    `title` VARCHAR(200) DEFAULT '新的对话' COMMENT '会话标题（默认取首条消息摘要）',
    `variables` JSON DEFAULT NULL COMMENT '会话级变量（开场参数等）',
    `created_at` DATETIME DEFAULT CURRENT_TIMESTAMP,
    `updated_at` DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    `created_by` BIGINT UNSIGNED DEFAULT NULL COMMENT '创建人ID',
    PRIMARY KEY (`id`),
    INDEX `idx_ai_conv_workflow_id` (`workflow_id`),
    INDEX `idx_ai_conv_created_by` (`created_by`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='AI工作流会话表';

-- ============================================
-- AI 工作流会话消息表 (t_ai_conversation_message)
-- ============================================
CREATE TABLE IF NOT EXISTS `t_ai_conversation_message` (
    `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `conversation_id` BIGINT UNSIGNED NOT NULL COMMENT '关联的会话ID',
    `role` VARCHAR(20) NOT NULL COMMENT '消息角色: user/assistant/system',
    `content` LONGTEXT NOT NULL COMMENT '消息内容',
    `metadata` JSON DEFAULT NULL COMMENT '元信息（token用量、执行耗时、步骤结果摘要等）',
    `created_at` DATETIME DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    INDEX `idx_ai_conv_msg_conv_id` (`conversation_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='AI工作流会话消息表';

-- ============================================
-- 完成提示
-- ============================================
SELECT 'Gulu 数据库初始化完成!' AS message;
