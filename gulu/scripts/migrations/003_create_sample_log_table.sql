-- 采样日志表: 存储压测过程中的请求/响应采样数据
CREATE TABLE IF NOT EXISTS t_sample_log (
    id              BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    execution_id    VARCHAR(64)   NOT NULL COMMENT '执行ID',
    step_id         VARCHAR(128)  NOT NULL COMMENT '步骤ID',
    step_name       VARCHAR(256)  DEFAULT '' COMMENT '步骤名称',
    timestamp       DATETIME(3)   NOT NULL COMMENT '采样时间',
    status          VARCHAR(16)   NOT NULL COMMENT '状态: success, failed, timeout',
    duration_ms     BIGINT        NOT NULL DEFAULT 0 COMMENT '耗时(毫秒)',
    request_method  VARCHAR(16)   DEFAULT '' COMMENT 'HTTP方法',
    request_url     TEXT          COMMENT '请求URL',
    request_headers JSON          COMMENT '请求头',
    request_body    TEXT          COMMENT '请求体',
    response_status INT           DEFAULT 0 COMMENT '响应状态码',
    response_headers JSON         COMMENT '响应头',
    response_body   TEXT          COMMENT '响应体',
    error_message   TEXT          COMMENT '错误信息',
    INDEX idx_sample_log_execution_id (execution_id),
    INDEX idx_sample_log_step (execution_id, step_id),
    INDEX idx_sample_log_status (execution_id, status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='采样日志表';
