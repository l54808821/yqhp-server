-- 迁移脚本: 扩展执行记录表支持调试模式
-- 执行: mysql -u <user> -p <database> < 002_create_debug_session.sql

-- 添加执行模式字段
ALTER TABLE `t_execution` 
ADD COLUMN `mode` VARCHAR(20) NOT NULL DEFAULT 'execute' COMMENT '执行模式: debug, execute' AFTER `execution_id`,
ADD INDEX `idx_t_execution_mode` (`mode`);

-- 添加步骤统计字段（用于调试模式）
ALTER TABLE `t_execution`
ADD COLUMN `total_steps` INT DEFAULT 0 COMMENT '总步骤数' AFTER `duration`,
ADD COLUMN `success_steps` INT DEFAULT 0 COMMENT '成功步骤数' AFTER `total_steps`,
ADD COLUMN `failed_steps` INT DEFAULT 0 COMMENT '失败步骤数' AFTER `success_steps`;

-- 更新状态字段支持更多状态
ALTER TABLE `t_execution` 
MODIFY COLUMN `status` VARCHAR(20) NOT NULL COMMENT '执行状态: pending, running, completed, failed, stopped, timeout';

SELECT '执行记录表扩展完成!' AS message;
