-- ============================================
-- 004: 初始化内置 MCP Server 配置
-- 在 t_mcp_server 表中插入 gulu 内置 MCP 服务器记录
-- 使前端 MCP 管理页面可以直接看到并使用该服务
-- ============================================

INSERT INTO `t_mcp_server` (
    `created_at`,
    `updated_at`,
    `is_delete`,
    `name`,
    `description`,
    `transport`,
    `url`,
    `timeout`,
    `sort`,
    `status`
) VALUES (
    NOW(),
    NOW(),
    0,
    'gulu-builtin-mcp',
    'Gulu 内置 MCP 服务 — 提供项目查询、工作流查询、执行记录查询等工具，可在 AI 工作流中直接调用',
    'sse',
    'http://127.0.0.1:5322/sse',
    30,
    100,
    1
);
