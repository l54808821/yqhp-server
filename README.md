# YQHP 后端项目

基于 Go + Fiber + Sa-Token-Go + Gen + Lancet 的后台管理系统后端项目。

## 项目结构

```
yqhp/
├── admin/          # 后台管理系统
│   ├── cmd/
│   │   ├── gen/    # 数据库模型生成工具
│   │   └── server/ # 服务入口
│   ├── config/     # 配置文件
│   ├── internal/
│   │   ├── auth/       # 认证相关
│   │   ├── config/     # 配置加载
│   │   ├── handler/    # HTTP处理器
│   │   ├── middleware/ # 中间件
│   │   ├── model/      # 数据模型
│   │   ├── router/     # 路由配置
│   │   └── service/    # 业务逻辑
│   ├── Makefile
│   └── go.mod
├── common/         # 公共模块
│   ├── config/     # 公共配置
│   ├── database/   # 数据库连接
│   ├── middleware/ # 公共中间件
│   ├── redis/      # Redis连接
│   ├── response/   # 统一响应
│   └── utils/      # 工具函数
└── go.work         # Go工作区配置
```

## 技术栈

- **Web 框架**: [Fiber](https://github.com/gofiber/fiber) - 高性能 Go Web 框架
- **认证授权**: [Sa-Token-Go](https://github.com/weloe/sa-token-go) - 轻量级权限认证框架
- **ORM**: [GORM](https://gorm.io/) + [Gen](https://gorm.io/gen/) - 数据库操作
- **工具库**: [Lancet](https://github.com/duke-git/lancet) - Go 工具函数库
- **配置管理**: YAML 配置文件

## 功能模块

### 系统管理

- 用户管理 - 用户 CRUD、角色分配、密码重置
- 角色管理 - 角色 CRUD、权限分配
- 资源管理 - 菜单/按钮管理、权限标识
- 部门管理 - 组织架构管理
- 字典管理 - 数据字典维护
- 参数配置 - 系统参数管理

### 认证授权

- 登录认证 - 账号密码登录
- 第三方登录 - 微信、飞书、GitHub OAuth2
- 权限验证 - 基于角色的权限控制
- 令牌管理 - Token 管理、踢人下线

### 日志管理

- 登录日志 - 记录用户登录信息
- 操作日志 - 记录用户操作行为

## 快速开始

### 环境要求

- Go 1.23+
- MySQL 5.7+ 或 PostgreSQL 12+
- Redis 6+

### 配置数据库

编辑 `admin/config/config.yml`:

```yaml
database:
  driver: mysql
  host: 127.0.0.1
  port: 3306
  username: root
  password: root
  database: yqhp_admin
```

### 运行项目

```bash
cd admin

# 安装依赖
make tidy

# 运行服务
make run

# 或者构建后运行
make build
./bin/admin
```

### 生成数据库模型

```bash
cd admin
make gen
```

## API 文档

### 认证接口

| 方法 | 路径                      | 描述             |
| ---- | ------------------------- | ---------------- |
| POST | /api/auth/login           | 用户登录         |
| POST | /api/auth/logout          | 用户登出         |
| GET  | /api/auth/user-info       | 获取当前用户信息 |
| POST | /api/auth/change-password | 修改密码         |

### 系统管理接口

| 方法   | 路径                       | 描述         |
| ------ | -------------------------- | ------------ |
| GET    | /api/system/users          | 用户列表     |
| POST   | /api/system/users          | 创建用户     |
| PUT    | /api/system/users          | 更新用户     |
| DELETE | /api/system/users/:id      | 删除用户     |
| GET    | /api/system/roles          | 角色列表     |
| GET    | /api/system/resources/tree | 资源树       |
| GET    | /api/system/depts/tree     | 部门树       |
| GET    | /api/system/dict/types     | 字典类型列表 |
| GET    | /api/system/configs        | 配置列表     |

## 默认账号

- 用户名: admin
- 密码: 123456

## 许可证

MIT License
