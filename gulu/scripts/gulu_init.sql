-- Gulu 应用初始化 SQL
-- 执行前请确保已在 Admin 系统中创建好基础数据

-- 1. 插入 gulu 应用记录
INSERT INTO sys_application (name, code, description, icon, sort, status, is_delete, created_at, updated_at)
VALUES ('Gulu应用', 'gulu', 'Gulu业务应用系统', 'ant-design:appstore-outlined', 2, 1, 0, NOW(), NOW());

-- 获取 gulu 应用ID (假设为 @gulu_app_id)
SET @gulu_app_id = LAST_INSERT_ID();

-- 2. 创建 gulu 角色
INSERT INTO sys_role (app_id, name, code, sort, status, is_delete, created_at, updated_at)
VALUES 
(@gulu_app_id, 'Gulu管理员', 'gulu_admin', 1, 1, 0, NOW(), NOW()),
(@gulu_app_id, 'Gulu用户', 'gulu_user', 2, 1, 0, NOW(), NOW());

SET @gulu_admin_role_id = LAST_INSERT_ID() - 1;
SET @gulu_user_role_id = LAST_INSERT_ID();

-- 3. 创建 gulu 菜单资源

-- 3.1 仪表板目录
INSERT INTO sys_resource (app_id, parent_id, name, code, type, path, component, redirect, icon, sort, is_hidden, is_cache, is_frame, status, is_delete, created_at, updated_at)
VALUES (@gulu_app_id, NULL, '仪表板', NULL, 1, '/dashboard', 'LAYOUT', '/dashboard/index', 'ant-design:dashboard-outlined', 1, 0, 1, 0, 1, 0, NOW(), NOW());

SET @dashboard_dir_id = LAST_INSERT_ID();

-- 3.2 仪表板页面
INSERT INTO sys_resource (app_id, parent_id, name, code, type, path, component, redirect, icon, sort, is_hidden, is_cache, is_frame, status, is_delete, created_at, updated_at)
VALUES (@gulu_app_id, @dashboard_dir_id, '工作台', 'dashboard:view', 2, 'index', '/dashboard/index', NULL, 'ant-design:home-outlined', 1, 0, 1, 0, 1, 0, NOW(), NOW());

SET @dashboard_page_id = LAST_INSERT_ID();

-- 3.3 测试功能目录
INSERT INTO sys_resource (app_id, parent_id, name, code, type, path, component, redirect, icon, sort, is_hidden, is_cache, is_frame, status, is_delete, created_at, updated_at)
VALUES (@gulu_app_id, NULL, '测试功能', NULL, 1, '/test', 'LAYOUT', '/test/permission', 'ant-design:experiment-outlined', 2, 0, 1, 0, 1, 0, NOW(), NOW());

SET @test_dir_id = LAST_INSERT_ID();

-- 3.4 权限测试页面
INSERT INTO sys_resource (app_id, parent_id, name, code, type, path, component, redirect, icon, sort, is_hidden, is_cache, is_frame, status, is_delete, created_at, updated_at)
VALUES (@gulu_app_id, @test_dir_id, '权限测试', 'test:permission:view', 2, 'permission', '/test/permission/index', NULL, 'ant-design:safety-outlined', 1, 0, 1, 0, 1, 0, NOW(), NOW());

SET @permission_page_id = LAST_INSERT_ID();

-- 4. 创建按钮权限资源

-- 4.1 权限测试页面的按钮
INSERT INTO sys_resource (app_id, parent_id, name, code, type, path, component, redirect, icon, sort, is_hidden, is_cache, is_frame, status, is_delete, created_at, updated_at)
VALUES 
(@gulu_app_id, @permission_page_id, '查看按钮', 'test:permission:view:btn', 3, NULL, NULL, NULL, NULL, 1, 0, 0, 0, 1, 0, NOW(), NOW()),
(@gulu_app_id, @permission_page_id, '编辑按钮', 'test:permission:edit:btn', 3, NULL, NULL, NULL, NULL, 2, 0, 0, 0, 1, 0, NOW(), NOW()),
(@gulu_app_id, @permission_page_id, '删除按钮', 'test:permission:delete:btn', 3, NULL, NULL, NULL, NULL, 3, 0, 0, 0, 1, 0, NOW(), NOW());

SET @view_btn_id = LAST_INSERT_ID() - 2;
SET @edit_btn_id = LAST_INSERT_ID() - 1;
SET @delete_btn_id = LAST_INSERT_ID();

-- 5. 关联角色与资源

-- 5.1 gulu_admin 角色拥有所有资源
INSERT INTO sys_role_resource (role_id, resource_id, is_delete)
VALUES 
(@gulu_admin_role_id, @dashboard_dir_id, 0),
(@gulu_admin_role_id, @dashboard_page_id, 0),
(@gulu_admin_role_id, @test_dir_id, 0),
(@gulu_admin_role_id, @permission_page_id, 0),
(@gulu_admin_role_id, @view_btn_id, 0),
(@gulu_admin_role_id, @edit_btn_id, 0),
(@gulu_admin_role_id, @delete_btn_id, 0);

-- 5.2 gulu_user 角色只有查看权限
INSERT INTO sys_role_resource (role_id, resource_id, is_delete)
VALUES 
(@gulu_user_role_id, @dashboard_dir_id, 0),
(@gulu_user_role_id, @dashboard_page_id, 0),
(@gulu_user_role_id, @test_dir_id, 0),
(@gulu_user_role_id, @permission_page_id, 0),
(@gulu_user_role_id, @view_btn_id, 0);

-- 完成提示
SELECT 'Gulu 应用初始化完成!' AS message;
SELECT @gulu_app_id AS gulu_app_id, @gulu_admin_role_id AS admin_role_id, @gulu_user_role_id AS user_role_id;
