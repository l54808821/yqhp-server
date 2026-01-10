-- 迁移脚本: 添加工作流类型字段
-- 执行: mysql -u <user> -p <database> < 001_add_workflow_type.sql

-- 为 t_workflow 表添加 workflow_type 字段
ALTER TABLE `t_workflow` 
ADD COLUMN `workflow_type` VARCHAR(20) DEFAULT 'normal' COMMENT '工作流类型: normal(普通流程), performance(压测流程), data_generation(造数流程)' 
AFTER `status`;

-- 为现有数据设置默认值
UPDATE `t_workflow` SET `workflow_type` = 'normal' WHERE `workflow_type` IS NULL;

-- 添加索引以支持按类型查询
CREATE INDEX `idx_t_workflow_type` ON `t_workflow` (`workflow_type`);

SELECT '工作流类型字段添加完成!' AS message;
