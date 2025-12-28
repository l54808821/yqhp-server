-- 用户表新增平台相关字段
-- 执行时间: 2024-xx-xx
-- 说明: 为用户表添加来源平台、平台唯一标识(长码)、平台唯一标识(短码)字段

ALTER TABLE `sys_user`
ADD COLUMN `platform` VARCHAR(20) DEFAULT 'system' COMMENT '用户来源平台: system-系统新建, github, wechat-微信, feishu-飞书, dingtalk-钉钉, qq, gitee' AFTER `dept_id`,
ADD COLUMN `platform_uid` VARCHAR(255) DEFAULT NULL COMMENT '平台唯一标识(长码)' AFTER `platform`,
ADD COLUMN `platform_short_id` VARCHAR(100) DEFAULT NULL COMMENT '平台唯一标识(短码)' AFTER `platform_uid`;

-- 为平台字段添加索引（可选，如果需要按平台查询用户）
-- ALTER TABLE `sys_user` ADD INDEX `idx_sys_user_platform` (`platform`);

-- 为平台唯一标识添加联合唯一索引（可选，确保同一平台下用户唯一）
-- ALTER TABLE `sys_user` ADD UNIQUE INDEX `idx_sys_user_platform_uid` (`platform`, `platform_uid`);
