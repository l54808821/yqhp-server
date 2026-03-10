-- 定时任务表
CREATE TABLE `sys_job` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime DEFAULT NULL,
  `updated_at` datetime DEFAULT NULL,
  `is_delete` tinyint(1) DEFAULT '0',
  `created_by` bigint unsigned DEFAULT NULL COMMENT '创建人ID',
  `updated_by` bigint unsigned DEFAULT NULL COMMENT '更新人ID',
  `name` varchar(100) NOT NULL COMMENT '任务名称',
  `job_group` varchar(100) DEFAULT NULL COMMENT '任务分组',
  `handler_name` varchar(200) NOT NULL COMMENT '处理器名称',
  `cron_expression` varchar(100) NOT NULL COMMENT 'cron表达式',
  `params` text COMMENT '任务参数(JSON)',
  `status` tinyint NOT NULL DEFAULT '0' COMMENT '状态: 0-暂停 1-运行中',
  `source` varchar(50) DEFAULT 'system' COMMENT '来源: system-系统任务 agent-Agent创建',
  `source_id` bigint unsigned DEFAULT NULL COMMENT '来源ID(预留: 如agent_id)',
  `misfire_policy` tinyint DEFAULT '0' COMMENT '错过策略: 0-忽略 1-立即执行',
  `concurrent` tinyint DEFAULT '0' COMMENT '是否允许并发: 0-禁止 1-允许',
  `retry_count` int DEFAULT '0' COMMENT '失败重试次数',
  `retry_interval` int DEFAULT '0' COMMENT '重试间隔(秒)',
  `remark` varchar(500) DEFAULT NULL COMMENT '备注',
  PRIMARY KEY (`id`),
  KEY `idx_sys_job_is_delete` (`is_delete`),
  KEY `idx_sys_job_status` (`status`),
  KEY `idx_sys_job_handler_name` (`handler_name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='定时任务表';

-- 定时任务执行日志表
CREATE TABLE `sys_job_log` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime DEFAULT NULL,
  `job_id` bigint unsigned NOT NULL COMMENT '任务ID',
  `job_name` varchar(100) NOT NULL COMMENT '任务名称',
  `handler_name` varchar(200) DEFAULT NULL COMMENT '处理器名称',
  `params` text COMMENT '执行参数',
  `status` tinyint NOT NULL DEFAULT '0' COMMENT '执行状态: 0-失败 1-成功',
  `error_message` text COMMENT '错误信息',
  `start_time` datetime(3) DEFAULT NULL COMMENT '开始时间',
  `end_time` datetime(3) DEFAULT NULL COMMENT '结束时间',
  `duration` bigint DEFAULT NULL COMMENT '耗时(毫秒)',
  PRIMARY KEY (`id`),
  KEY `idx_sys_job_log_job_id` (`job_id`),
  KEY `idx_sys_job_log_status` (`status`),
  KEY `idx_sys_job_log_start_time` (`start_time`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='定时任务执行日志表';

-- =============================================
-- 菜单资源初始化（定时任务）
-- 注意: ID 从 100 开始，避免与现有菜单冲突，实际使用时请根据数据库中的最大 ID 调整
-- type: 1-目录 2-菜单 3-按钮
-- parent_id=1 表示挂在"系统管理"目录下
-- =============================================

-- 定时任务菜单
INSERT INTO sys_resource (created_at, updated_at, is_delete, created_by, updated_by, app_id, parent_id, name, code, type, path, component, redirect, icon, sort, is_hidden, is_cache, is_frame, status, remark) VALUES
(NOW(), NOW(), 0, 1, 1, 1, 1, '定时任务', 'system:job', 2, '/system/job', 'system/job/index', NULL, 'ant-design:clock-circle-outlined', 10, 0, 1, 0, 1, NULL);

SET @job_menu_id = LAST_INSERT_ID();

-- 定时任务按钮权限
INSERT INTO sys_resource (created_at, updated_at, is_delete, created_by, updated_by, app_id, parent_id, name, code, type, path, component, redirect, icon, sort, is_hidden, is_cache, is_frame, status, remark) VALUES
(NOW(), NOW(), 0, 1, 1, 1, @job_menu_id, '查询任务', 'system:job:list', 3, NULL, NULL, NULL, NULL, 1, 0, 1, 0, 1, NULL),
(NOW(), NOW(), 0, 1, 1, 1, @job_menu_id, '新增任务', 'system:job:add', 3, NULL, NULL, NULL, NULL, 2, 0, 1, 0, 1, NULL),
(NOW(), NOW(), 0, 1, 1, 1, @job_menu_id, '编辑任务', 'system:job:edit', 3, NULL, NULL, NULL, NULL, 3, 0, 1, 0, 1, NULL),
(NOW(), NOW(), 0, 1, 1, 1, @job_menu_id, '删除任务', 'system:job:delete', 3, NULL, NULL, NULL, NULL, 4, 0, 1, 0, 1, NULL);

-- 执行日志菜单
INSERT INTO sys_resource (created_at, updated_at, is_delete, created_by, updated_by, app_id, parent_id, name, code, type, path, component, redirect, icon, sort, is_hidden, is_cache, is_frame, status, remark) VALUES
(NOW(), NOW(), 0, 1, 1, 1, 1, '执行日志', 'system:jobLog', 2, '/system/job-log', 'system/job-log/index', NULL, 'ant-design:history-outlined', 11, 0, 1, 0, 1, NULL);

-- =============================================
-- 角色资源关联（为管理员角色赋予定时任务权限）
-- 注意: 请在执行完上面的菜单插入后，手动查询新插入的 resource id，然后执行下面的关联
-- 或者通过管理后台的角色管理页面手动分配权限
-- =============================================
-- INSERT INTO sys_role_resource (role_id, resource_id, is_delete)
-- SELECT 1, id, 0 FROM sys_resource WHERE code IN ('system:job', 'system:job:list', 'system:job:add', 'system:job:edit', 'system:job:delete', 'system:jobLog') AND is_delete = 0;
