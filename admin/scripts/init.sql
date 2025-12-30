-- =============================================
-- YQHP Admin 初始化SQL
-- 执行前请先创建数据库并执行建表SQL
-- =============================================

-- 清空现有数据（可选，生产环境慎用）
-- TRUNCATE TABLE sys_role_resource;
-- TRUNCATE TABLE sys_user_role;
-- TRUNCATE TABLE sys_user_app;
-- TRUNCATE TABLE sys_oauth_user;
-- TRUNCATE TABLE sys_operation_log;
-- TRUNCATE TABLE sys_login_log;
-- TRUNCATE TABLE sys_user_token;
-- TRUNCATE TABLE sys_oauth_provider;
-- TRUNCATE TABLE sys_config;
-- TRUNCATE TABLE sys_dict_data;
-- TRUNCATE TABLE sys_dict_type;
-- TRUNCATE TABLE sys_resource;
-- TRUNCATE TABLE sys_role;
-- TRUNCATE TABLE sys_user;
-- TRUNCATE TABLE sys_dept;
-- TRUNCATE TABLE sys_application;

-- =============================================
-- 0. 创建表格视图配置表（如果不存在）
-- =============================================
-- user_id = 0 表示系统视图，user_id > 0 表示用户视图
CREATE TABLE IF NOT EXISTS `sys_table_view` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime DEFAULT CURRENT_TIMESTAMP,
  `updated_at` datetime DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  `is_delete` tinyint(1) DEFAULT '0',
  `created_by` bigint unsigned DEFAULT NULL COMMENT '创建人ID',
  `updated_by` bigint unsigned DEFAULT NULL COMMENT '更新人ID',
  `user_id` bigint unsigned NOT NULL DEFAULT '0' COMMENT '用户ID(0表示系统视图)',
  `table_key` varchar(100) NOT NULL COMMENT '表格标识',
  `name` varchar(50) NOT NULL COMMENT '视图名称',
  `is_default` tinyint(1) DEFAULT '0' COMMENT '是否默认视图',
  `column_keys` text COMMENT '显示的列(JSON数组)',
  `column_fixed` text COMMENT '列固定配置(JSON数组)',
  `search_params` text COMMENT '搜索条件(JSON对象)',
  `sort` int DEFAULT '0' COMMENT '排序',
  PRIMARY KEY (`id`),
  KEY `idx_sys_table_view_is_delete` (`is_delete`),
  KEY `idx_sys_table_view_user_table` (`user_id`, `table_key`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='表格视图配置';

-- =============================================
-- 0.1 创建用户-应用关联表（如果不存在）
-- =============================================
CREATE TABLE IF NOT EXISTS `sys_user_app` (
    `id` bigint unsigned NOT NULL AUTO_INCREMENT,
    `created_at` datetime DEFAULT NULL,
    `updated_at` datetime DEFAULT NULL,
    `is_delete` tinyint(1) DEFAULT '0',
    `user_id` bigint unsigned DEFAULT NULL COMMENT '用户ID',
    `app_id` bigint unsigned DEFAULT NULL COMMENT '应用ID',
    `source` varchar(50) DEFAULT 'system' COMMENT '注册来源: system-系统创建, oauth-第三方登录, register-自主注册, invite-邀请注册',
    `first_login_at` datetime DEFAULT NULL COMMENT '首次登录时间',
    `last_login_at` datetime DEFAULT NULL COMMENT '最后登录时间',
    `login_count` int DEFAULT '0' COMMENT '登录次数',
    `status` tinyint DEFAULT '1' COMMENT '状态: 1-正常, 0-禁用',
    `remark` varchar(500) DEFAULT NULL COMMENT '备注',
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_user_app` (`user_id`, `app_id`),
    KEY `idx_sys_user_app_user_id` (`user_id`),
    KEY `idx_sys_user_app_app_id` (`app_id`),
    KEY `idx_sys_user_app_is_delete` (`is_delete`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='用户-应用关联表';

-- 为 sys_oauth_provider 表添加 app_id 字段（如果不存在）
-- ALTER TABLE `sys_oauth_provider` ADD COLUMN IF NOT EXISTS `app_id` bigint unsigned DEFAULT NULL COMMENT '应用ID，NULL表示全局配置' AFTER `updated_by`;
-- ALTER TABLE `sys_oauth_provider` ADD INDEX IF NOT EXISTS `idx_sys_oauth_provider_app_id` (`app_id`);

-- =============================================
-- 1. 初始化应用
-- =============================================
INSERT INTO sys_application (id, created_at, updated_at, is_delete, created_by, updated_by, name, code, description, icon, sort, status) VALUES
(1, NOW(), NOW(), 0, 1, 1, '后台管理系统', 'admin', '系统管理后台', 'ant-design:setting-outlined', 1, 1);

-- =============================================
-- 2. 初始化部门
-- =============================================
INSERT INTO sys_dept (id, created_at, updated_at, is_delete, created_by, updated_by, parent_id, name, code, leader, phone, email, sort, status, remark) VALUES
(1, NOW(), NOW(), 0, 1, 1, 0, '总公司', 'HQ', NULL, NULL, NULL, 0, 1, NULL);

-- =============================================
-- 3. 初始化角色
-- =============================================
INSERT INTO sys_role (id, created_at, updated_at, is_delete, created_by, updated_by, app_id, name, code, sort, status, remark) VALUES
(1, NOW(), NOW(), 0, 1, 1, 1, '超级管理员', 'admin', 0, 1, '拥有所有权限');

-- =============================================
-- 4. 初始化用户 (密码: 123456 的 MD5)
-- =============================================
INSERT INTO sys_user (id, created_at, updated_at, is_delete, created_by, updated_by, username, password, nickname, avatar, email, phone, gender, status, dept_id, last_login_at, last_login_ip, remark) VALUES
(1, NOW(), NOW(), 0, 1, 1, 'admin', 'e10adc3949ba59abbe56e057f20f883e', '管理员', NULL, NULL, NULL, 0, 1, 1, NULL, NULL, NULL);

-- =============================================
-- 5. 初始化用户角色关联
-- =============================================
INSERT INTO sys_user_role (user_id, role_id, app_id, is_delete) VALUES
(1, 1, 1, 0);

-- =============================================
-- 5.1 初始化用户-应用关联
-- =============================================
INSERT INTO sys_user_app (created_at, updated_at, is_delete, user_id, app_id, source, first_login_at, last_login_at, login_count, status) VALUES
(NOW(), NOW(), 0, 1, 1, 'system', NOW(), NOW(), 1, 1);

-- =============================================
-- 6. 初始化菜单资源
-- =============================================
-- 系统管理目录
INSERT INTO sys_resource (id, created_at, updated_at, is_delete, created_by, updated_by, app_id, parent_id, name, code, type, path, component, redirect, icon, sort, is_hidden, is_cache, is_frame, status, remark) VALUES
(1, NOW(), NOW(), 0, 1, 1, 1, 0, '系统管理', 'system', 1, '/system', NULL, NULL, 'ant-design:setting-outlined', 1, 0, 1, 0, 1, NULL);

-- 应用管理
INSERT INTO sys_resource (id, created_at, updated_at, is_delete, created_by, updated_by, app_id, parent_id, name, code, type, path, component, redirect, icon, sort, is_hidden, is_cache, is_frame, status, remark) VALUES
(2, NOW(), NOW(), 0, 1, 1, 1, 1, '应用管理', 'system:app', 2, '/system/app', 'system/app/index', NULL, 'ant-design:appstore-outlined', 0, 0, 1, 0, 1, NULL),
(3, NOW(), NOW(), 0, 1, 1, 1, 2, '新增应用', 'system:app:add', 3, NULL, NULL, NULL, NULL, 1, 0, 1, 0, 1, NULL),
(4, NOW(), NOW(), 0, 1, 1, 1, 2, '编辑应用', 'system:app:edit', 3, NULL, NULL, NULL, NULL, 2, 0, 1, 0, 1, NULL),
(5, NOW(), NOW(), 0, 1, 1, 1, 2, '删除应用', 'system:app:delete', 3, NULL, NULL, NULL, NULL, 3, 0, 1, 0, 1, NULL);

-- 用户管理
INSERT INTO sys_resource (id, created_at, updated_at, is_delete, created_by, updated_by, app_id, parent_id, name, code, type, path, component, redirect, icon, sort, is_hidden, is_cache, is_frame, status, remark) VALUES
(6, NOW(), NOW(), 0, 1, 1, 1, 1, '用户管理', 'system:user', 2, '/system/user', 'system/user/index', NULL, 'ant-design:user-outlined', 1, 0, 1, 0, 1, NULL),
(7, NOW(), NOW(), 0, 1, 1, 1, 6, '新增用户', 'system:user:add', 3, NULL, NULL, NULL, NULL, 1, 0, 1, 0, 1, NULL),
(8, NOW(), NOW(), 0, 1, 1, 1, 6, '编辑用户', 'system:user:edit', 3, NULL, NULL, NULL, NULL, 2, 0, 1, 0, 1, NULL),
(9, NOW(), NOW(), 0, 1, 1, 1, 6, '删除用户', 'system:user:delete', 3, NULL, NULL, NULL, NULL, 3, 0, 1, 0, 1, NULL),
(10, NOW(), NOW(), 0, 1, 1, 1, 6, '重置密码', 'system:user:resetPwd', 3, NULL, NULL, NULL, NULL, 4, 0, 1, 0, 1, NULL);

-- 角色管理
INSERT INTO sys_resource (id, created_at, updated_at, is_delete, created_by, updated_by, app_id, parent_id, name, code, type, path, component, redirect, icon, sort, is_hidden, is_cache, is_frame, status, remark) VALUES
(11, NOW(), NOW(), 0, 1, 1, 1, 1, '角色管理', 'system:role', 2, '/system/role', 'system/role/index', NULL, 'ant-design:team-outlined', 2, 0, 1, 0, 1, NULL),
(12, NOW(), NOW(), 0, 1, 1, 1, 11, '新增角色', 'system:role:add', 3, NULL, NULL, NULL, NULL, 1, 0, 1, 0, 1, NULL),
(13, NOW(), NOW(), 0, 1, 1, 1, 11, '编辑角色', 'system:role:edit', 3, NULL, NULL, NULL, NULL, 2, 0, 1, 0, 1, NULL),
(14, NOW(), NOW(), 0, 1, 1, 1, 11, '删除角色', 'system:role:delete', 3, NULL, NULL, NULL, NULL, 3, 0, 1, 0, 1, NULL);

-- 菜单管理
INSERT INTO sys_resource (id, created_at, updated_at, is_delete, created_by, updated_by, app_id, parent_id, name, code, type, path, component, redirect, icon, sort, is_hidden, is_cache, is_frame, status, remark) VALUES
(15, NOW(), NOW(), 0, 1, 1, 1, 1, '菜单管理', 'system:resource', 2, '/system/menu', 'system/menu/index', NULL, 'ant-design:menu-outlined', 3, 0, 1, 0, 1, NULL),
(16, NOW(), NOW(), 0, 1, 1, 1, 15, '新增菜单', 'system:resource:add', 3, NULL, NULL, NULL, NULL, 1, 0, 1, 0, 1, NULL),
(17, NOW(), NOW(), 0, 1, 1, 1, 15, '编辑菜单', 'system:resource:edit', 3, NULL, NULL, NULL, NULL, 2, 0, 1, 0, 1, NULL),
(18, NOW(), NOW(), 0, 1, 1, 1, 15, '删除菜单', 'system:resource:delete', 3, NULL, NULL, NULL, NULL, 3, 0, 1, 0, 1, NULL);

-- 部门管理
INSERT INTO sys_resource (id, created_at, updated_at, is_delete, created_by, updated_by, app_id, parent_id, name, code, type, path, component, redirect, icon, sort, is_hidden, is_cache, is_frame, status, remark) VALUES
(19, NOW(), NOW(), 0, 1, 1, 1, 1, '部门管理', 'system:dept', 2, '/system/dept', 'system/dept/index', NULL, 'ant-design:apartment-outlined', 4, 0, 1, 0, 1, NULL),
(20, NOW(), NOW(), 0, 1, 1, 1, 19, '新增部门', 'system:dept:add', 3, NULL, NULL, NULL, NULL, 1, 0, 1, 0, 1, NULL),
(21, NOW(), NOW(), 0, 1, 1, 1, 19, '编辑部门', 'system:dept:edit', 3, NULL, NULL, NULL, NULL, 2, 0, 1, 0, 1, NULL),
(22, NOW(), NOW(), 0, 1, 1, 1, 19, '删除部门', 'system:dept:delete', 3, NULL, NULL, NULL, NULL, 3, 0, 1, 0, 1, NULL);

-- 字典管理
INSERT INTO sys_resource (id, created_at, updated_at, is_delete, created_by, updated_by, app_id, parent_id, name, code, type, path, component, redirect, icon, sort, is_hidden, is_cache, is_frame, status, remark) VALUES
(23, NOW(), NOW(), 0, 1, 1, 1, 1, '字典管理', 'system:dict', 2, '/system/dict', 'system/dict/index', NULL, 'ant-design:book-outlined', 5, 0, 1, 0, 1, NULL),
(24, NOW(), NOW(), 0, 1, 1, 1, 23, '新增字典', 'system:dict:add', 3, NULL, NULL, NULL, NULL, 1, 0, 1, 0, 1, NULL),
(25, NOW(), NOW(), 0, 1, 1, 1, 23, '编辑字典', 'system:dict:edit', 3, NULL, NULL, NULL, NULL, 2, 0, 1, 0, 1, NULL),
(26, NOW(), NOW(), 0, 1, 1, 1, 23, '删除字典', 'system:dict:delete', 3, NULL, NULL, NULL, NULL, 3, 0, 1, 0, 1, NULL);

-- 参数配置
INSERT INTO sys_resource (id, created_at, updated_at, is_delete, created_by, updated_by, app_id, parent_id, name, code, type, path, component, redirect, icon, sort, is_hidden, is_cache, is_frame, status, remark) VALUES
(27, NOW(), NOW(), 0, 1, 1, 1, 1, '参数配置', 'system:config', 2, '/system/config', 'system/config/index', NULL, 'ant-design:tool-outlined', 6, 0, 1, 0, 1, NULL),
(28, NOW(), NOW(), 0, 1, 1, 1, 27, '新增配置', 'system:config:add', 3, NULL, NULL, NULL, NULL, 1, 0, 1, 0, 1, NULL),
(29, NOW(), NOW(), 0, 1, 1, 1, 27, '编辑配置', 'system:config:edit', 3, NULL, NULL, NULL, NULL, 2, 0, 1, 0, 1, NULL),
(30, NOW(), NOW(), 0, 1, 1, 1, 27, '删除配置', 'system:config:delete', 3, NULL, NULL, NULL, NULL, 3, 0, 1, 0, 1, NULL);

-- 第三方登录
INSERT INTO sys_resource (id, created_at, updated_at, is_delete, created_by, updated_by, app_id, parent_id, name, code, type, path, component, redirect, icon, sort, is_hidden, is_cache, is_frame, status, remark) VALUES
(31, NOW(), NOW(), 0, 1, 1, 1, 1, '第三方登录', 'system:oauth', 2, '/system/oauth', 'system/oauth/index', NULL, 'ant-design:api-outlined', 7, 0, 1, 0, 1, NULL),
(32, NOW(), NOW(), 0, 1, 1, 1, 31, '新增配置', 'system:oauth:add', 3, NULL, NULL, NULL, NULL, 1, 0, 1, 0, 1, NULL),
(33, NOW(), NOW(), 0, 1, 1, 1, 31, '编辑配置', 'system:oauth:edit', 3, NULL, NULL, NULL, NULL, 2, 0, 1, 0, 1, NULL),
(34, NOW(), NOW(), 0, 1, 1, 1, 31, '删除配置', 'system:oauth:delete', 3, NULL, NULL, NULL, NULL, 3, 0, 1, 0, 1, NULL);

-- 令牌管理
INSERT INTO sys_resource (id, created_at, updated_at, is_delete, created_by, updated_by, app_id, parent_id, name, code, type, path, component, redirect, icon, sort, is_hidden, is_cache, is_frame, status, remark) VALUES
(35, NOW(), NOW(), 0, 1, 1, 1, 1, '令牌管理', 'system:token', 2, '/system/token', 'system/token/index', NULL, 'ant-design:key-outlined', 8, 0, 1, 0, 1, NULL),
(36, NOW(), NOW(), 0, 1, 1, 1, 35, '踢人下线', 'system:token:kickout', 3, NULL, NULL, NULL, NULL, 1, 0, 1, 0, 1, NULL),
(37, NOW(), NOW(), 0, 1, 1, 1, 35, '禁用用户', 'system:token:disable', 3, NULL, NULL, NULL, NULL, 2, 0, 1, 0, 1, NULL),
(38, NOW(), NOW(), 0, 1, 1, 1, 35, '解禁用户', 'system:token:enable', 3, NULL, NULL, NULL, NULL, 3, 0, 1, 0, 1, NULL);

-- 日志管理
INSERT INTO sys_resource (id, created_at, updated_at, is_delete, created_by, updated_by, app_id, parent_id, name, code, type, path, component, redirect, icon, sort, is_hidden, is_cache, is_frame, status, remark) VALUES
(39, NOW(), NOW(), 0, 1, 1, 1, 1, '日志管理', 'system:log', 2, '/system/log', 'system/log/index', NULL, 'ant-design:file-text-outlined', 9, 0, 1, 0, 1, NULL),
(40, NOW(), NOW(), 0, 1, 1, 1, 39, '清空日志', 'system:log:delete', 3, NULL, NULL, NULL, NULL, 1, 0, 1, 0, 1, NULL);

-- =============================================
-- 7. 初始化角色资源关联（管理员拥有所有权限）
-- =============================================
INSERT INTO sys_role_resource (role_id, resource_id, is_delete) VALUES
(1, 1, 0), (1, 2, 0), (1, 3, 0), (1, 4, 0), (1, 5, 0),
(1, 6, 0), (1, 7, 0), (1, 8, 0), (1, 9, 0), (1, 10, 0),
(1, 11, 0), (1, 12, 0), (1, 13, 0), (1, 14, 0),
(1, 15, 0), (1, 16, 0), (1, 17, 0), (1, 18, 0),
(1, 19, 0), (1, 20, 0), (1, 21, 0), (1, 22, 0),
(1, 23, 0), (1, 24, 0), (1, 25, 0), (1, 26, 0),
(1, 27, 0), (1, 28, 0), (1, 29, 0), (1, 30, 0),
(1, 31, 0), (1, 32, 0), (1, 33, 0), (1, 34, 0),
(1, 35, 0), (1, 36, 0), (1, 37, 0), (1, 38, 0),
(1, 39, 0), (1, 40, 0);

-- =============================================
-- 8. 初始化第三方登录配置
-- =============================================
INSERT INTO sys_oauth_provider (id, created_at, updated_at, is_delete, created_by, updated_by, app_id, name, code, client_id, client_secret, redirect_uri, auth_url, token_url, user_info_url, scope, status, sort, icon, remark) VALUES
(1, NOW(), NOW(), 0, 1, 1, NULL, 'GitHub', 'github', 'your_github_client_id', 'your_github_client_secret', 'http://localhost:5666/auth/oauth/github/callback', 'https://github.com/login/oauth/authorize', 'https://github.com/login/oauth/access_token', 'https://api.github.com/user', 'user:email', 1, 1, 'github', 'GitHub 第三方登录（全局）'),
(2, NOW(), NOW(), 0, 1, 1, NULL, '微信', 'wechat', 'your_wechat_appid', 'your_wechat_secret', 'http://localhost:5555/api/auth/oauth/wechat/callback', 'https://open.weixin.qq.com/connect/qrconnect', 'https://api.weixin.qq.com/sns/oauth2/access_token', 'https://api.weixin.qq.com/sns/userinfo', 'snsapi_login', 1, 2, 'wechat', '微信扫码登录（全局）'),
(3, NOW(), NOW(), 0, 1, 1, NULL, '飞书', 'feishu', 'your_feishu_app_id', 'your_feishu_app_secret', 'http://localhost:5555/api/auth/oauth/feishu/callback', 'https://open.feishu.cn/open-apis/authen/v1/authorize', 'https://open.feishu.cn/open-apis/authen/v1/oidc/access_token', 'https://open.feishu.cn/open-apis/authen/v1/user_info', '', 1, 3, 'feishu', '飞书第三方登录（全局）'),
(4, NOW(), NOW(), 0, 1, 1, NULL, '钉钉', 'dingtalk', 'your_dingtalk_appkey', 'your_dingtalk_appsecret', 'http://localhost:5555/api/auth/oauth/dingtalk/callback', 'https://login.dingtalk.com/oauth2/auth', 'https://api.dingtalk.com/v1.0/oauth2/userAccessToken', 'https://api.dingtalk.com/v1.0/contact/users/me', 'openid', 1, 4, 'dingtalk', '钉钉第三方登录（全局）'),
(5, NOW(), NOW(), 0, 1, 1, NULL, 'QQ', 'qq', 'your_qq_appid', 'your_qq_appkey', 'http://localhost:5555/api/auth/oauth/qq/callback', 'https://graph.qq.com/oauth2.0/authorize', 'https://graph.qq.com/oauth2.0/token', 'https://graph.qq.com/user/get_user_info', 'get_user_info', 1, 5, 'qq', 'QQ第三方登录（全局）'),
(6, NOW(), NOW(), 0, 1, 1, NULL, 'Gitee', 'gitee', 'your_gitee_client_id', 'your_gitee_client_secret', 'http://localhost:5555/api/auth/oauth/gitee/callback', 'https://gitee.com/oauth/authorize', 'https://gitee.com/oauth/token', 'https://gitee.com/api/v5/user', 'user_info', 1, 6, 'gitee', 'Gitee 第三方登录（全局）');

-- =============================================
-- 9. 初始化字典类型
-- =============================================
INSERT INTO sys_dict_type (id, created_at, updated_at, is_delete, created_by, updated_by, name, code, status, remark) VALUES
(1, NOW(), NOW(), 0, 1, 1, '系统状态', 'sys_status', 1, '系统通用状态'),
(2, NOW(), NOW(), 0, 1, 1, '用户性别', 'sys_user_gender', 1, '用户性别'),
(3, NOW(), NOW(), 0, 1, 1, '是否', 'sys_yes_no', 1, '是否选项'),
(4, NOW(), NOW(), 0, 1, 1, '操作类型', 'sys_oper_type', 1, '操作日志类型'),
(5, NOW(), NOW(), 0, 1, 1, '登录状态', 'sys_login_status', 1, '登录日志状态'),
(6, NOW(), NOW(), 0, 1, 1, '通知类型', 'sys_notice_type', 1, '通知公告类型'),
(7, NOW(), NOW(), 0, 1, 1, '通知状态', 'sys_notice_status', 1, '通知公告状态');

-- =============================================
-- 10. 初始化字典数据
-- =============================================
-- 系统状态
INSERT INTO sys_dict_data (id, created_at, updated_at, is_delete, created_by, updated_by, type_code, label, value, sort, status, is_default, css_class, list_class, remark) VALUES
(1, NOW(), NOW(), 0, 1, 1, 'sys_status', '启用', '1', 1, 1, 1, '', 'success', '正常状态'),
(2, NOW(), NOW(), 0, 1, 1, 'sys_status', '禁用', '0', 2, 1, 0, '', 'error', '停用状态');

-- 用户性别
INSERT INTO sys_dict_data (id, created_at, updated_at, is_delete, created_by, updated_by, type_code, label, value, sort, status, is_default, css_class, list_class, remark) VALUES
(3, NOW(), NOW(), 0, 1, 1, 'sys_user_gender', '未知', '0', 1, 1, 1, '', 'default', '未知性别'),
(4, NOW(), NOW(), 0, 1, 1, 'sys_user_gender', '男', '1', 2, 1, 0, '', 'processing', '男性'),
(5, NOW(), NOW(), 0, 1, 1, 'sys_user_gender', '女', '2', 3, 1, 0, '', 'warning', '女性');

-- 是否
INSERT INTO sys_dict_data (id, created_at, updated_at, is_delete, created_by, updated_by, type_code, label, value, sort, status, is_default, css_class, list_class, remark) VALUES
(6, NOW(), NOW(), 0, 1, 1, 'sys_yes_no', '是', '1', 1, 1, 0, '', 'success', '是'),
(7, NOW(), NOW(), 0, 1, 1, 'sys_yes_no', '否', '0', 2, 1, 1, '', 'error', '否');

-- 操作类型
INSERT INTO sys_dict_data (id, created_at, updated_at, is_delete, created_by, updated_by, type_code, label, value, sort, status, is_default, css_class, list_class, remark) VALUES
(8, NOW(), NOW(), 0, 1, 1, 'sys_oper_type', '其他', '0', 1, 1, 1, '', 'default', '其他操作'),
(9, NOW(), NOW(), 0, 1, 1, 'sys_oper_type', '新增', '1', 2, 1, 0, '', 'success', '新增操作'),
(10, NOW(), NOW(), 0, 1, 1, 'sys_oper_type', '修改', '2', 3, 1, 0, '', 'warning', '修改操作'),
(11, NOW(), NOW(), 0, 1, 1, 'sys_oper_type', '删除', '3', 4, 1, 0, '', 'error', '删除操作'),
(12, NOW(), NOW(), 0, 1, 1, 'sys_oper_type', '查询', '4', 5, 1, 0, '', 'processing', '查询操作'),
(13, NOW(), NOW(), 0, 1, 1, 'sys_oper_type', '导出', '5', 6, 1, 0, '', 'cyan', '导出操作'),
(14, NOW(), NOW(), 0, 1, 1, 'sys_oper_type', '导入', '6', 7, 1, 0, '', 'purple', '导入操作');

-- 登录状态
INSERT INTO sys_dict_data (id, created_at, updated_at, is_delete, created_by, updated_by, type_code, label, value, sort, status, is_default, css_class, list_class, remark) VALUES
(15, NOW(), NOW(), 0, 1, 1, 'sys_login_status', '成功', '1', 1, 1, 1, '', 'success', '登录成功'),
(16, NOW(), NOW(), 0, 1, 1, 'sys_login_status', '失败', '0', 2, 1, 0, '', 'error', '登录失败');

-- 通知类型
INSERT INTO sys_dict_data (id, created_at, updated_at, is_delete, created_by, updated_by, type_code, label, value, sort, status, is_default, css_class, list_class, remark) VALUES
(17, NOW(), NOW(), 0, 1, 1, 'sys_notice_type', '通知', '1', 1, 1, 1, '', 'warning', '通知'),
(18, NOW(), NOW(), 0, 1, 1, 'sys_notice_type', '公告', '2', 2, 1, 0, '', 'success', '公告');

-- 通知状态
INSERT INTO sys_dict_data (id, created_at, updated_at, is_delete, created_by, updated_by, type_code, label, value, sort, status, is_default, css_class, list_class, remark) VALUES
(19, NOW(), NOW(), 0, 1, 1, 'sys_notice_status', '正常', '1', 1, 1, 1, '', 'success', '正常状态'),
(20, NOW(), NOW(), 0, 1, 1, 'sys_notice_status', '关闭', '0', 2, 1, 0, '', 'error', '关闭状态');
