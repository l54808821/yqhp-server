-- =============================================
-- 移除 sys_user 表中的冗余平台字段
-- 这些信息已在 sys_oauth_user 表中存储
-- =============================================

-- 1. 移除字段（执行前请确保数据已备份）
ALTER TABLE `sys_user` 
DROP INDEX `idx_sys_user_platform`,
DROP COLUMN `platform`,
DROP COLUMN `platform_uid`,
DROP COLUMN `platform_short_id`;
