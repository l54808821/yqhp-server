-- =============================================
-- 为所有业务表添加 created_by 和 updated_by 字段
-- 执行前请备份数据库
-- =============================================

-- 1. sys_application 应用表
ALTER TABLE sys_application 
ADD COLUMN created_by BIGINT UNSIGNED NULL COMMENT '创建人ID' AFTER is_delete,
ADD COLUMN updated_by BIGINT UNSIGNED NULL COMMENT '更新人ID' AFTER created_by;

-- 2. sys_dept 部门表
ALTER TABLE sys_dept 
ADD COLUMN created_by BIGINT UNSIGNED NULL COMMENT '创建人ID' AFTER is_delete,
ADD COLUMN updated_by BIGINT UNSIGNED NULL COMMENT '更新人ID' AFTER created_by;

-- 3. sys_role 角色表
ALTER TABLE sys_role 
ADD COLUMN created_by BIGINT UNSIGNED NULL COMMENT '创建人ID' AFTER is_delete,
ADD COLUMN updated_by BIGINT UNSIGNED NULL COMMENT '更新人ID' AFTER created_by;

-- 4. sys_user 用户表
ALTER TABLE sys_user 
ADD COLUMN created_by BIGINT UNSIGNED NULL COMMENT '创建人ID' AFTER is_delete,
ADD COLUMN updated_by BIGINT UNSIGNED NULL COMMENT '更新人ID' AFTER created_by;

-- 5. sys_resource 资源/菜单表
ALTER TABLE sys_resource 
ADD COLUMN created_by BIGINT UNSIGNED NULL COMMENT '创建人ID' AFTER is_delete,
ADD COLUMN updated_by BIGINT UNSIGNED NULL COMMENT '更新人ID' AFTER created_by;

-- 6. sys_config 配置表
ALTER TABLE sys_config 
ADD COLUMN created_by BIGINT UNSIGNED NULL COMMENT '创建人ID' AFTER is_delete,
ADD COLUMN updated_by BIGINT UNSIGNED NULL COMMENT '更新人ID' AFTER created_by;

-- 7. sys_dict_type 字典类型表
ALTER TABLE sys_dict_type 
ADD COLUMN created_by BIGINT UNSIGNED NULL COMMENT '创建人ID' AFTER is_delete,
ADD COLUMN updated_by BIGINT UNSIGNED NULL COMMENT '更新人ID' AFTER created_by;

-- 8. sys_dict_data 字典数据表
ALTER TABLE sys_dict_data 
ADD COLUMN created_by BIGINT UNSIGNED NULL COMMENT '创建人ID' AFTER is_delete,
ADD COLUMN updated_by BIGINT UNSIGNED NULL COMMENT '更新人ID' AFTER created_by;

-- 9. sys_oauth_provider OAuth提供商表
ALTER TABLE sys_oauth_provider 
ADD COLUMN created_by BIGINT UNSIGNED NULL COMMENT '创建人ID' AFTER is_delete,
ADD COLUMN updated_by BIGINT UNSIGNED NULL COMMENT '更新人ID' AFTER created_by;

-- =============================================
-- 更新现有数据的 created_by 和 updated_by 为管理员(ID=1)
-- =============================================
UPDATE sys_application SET created_by = 1, updated_by = 1 WHERE created_by IS NULL;
UPDATE sys_dept SET created_by = 1, updated_by = 1 WHERE created_by IS NULL;
UPDATE sys_role SET created_by = 1, updated_by = 1 WHERE created_by IS NULL;
UPDATE sys_user SET created_by = 1, updated_by = 1 WHERE created_by IS NULL;
UPDATE sys_resource SET created_by = 1, updated_by = 1 WHERE created_by IS NULL;
UPDATE sys_config SET created_by = 1, updated_by = 1 WHERE created_by IS NULL;
UPDATE sys_dict_type SET created_by = 1, updated_by = 1 WHERE created_by IS NULL;
UPDATE sys_dict_data SET created_by = 1, updated_by = 1 WHERE created_by IS NULL;
UPDATE sys_oauth_provider SET created_by = 1, updated_by = 1 WHERE created_by IS NULL;
