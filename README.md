# GoLDAPAdmin

> 轻量级、Web 端的 LDAP 目录管理工具，用 Go + Vue3 构建，对标 phpLDAPadmin。

![Go](https://img.shields.io/badge/Go-1.26%2B-00ADD8?style=flat-square&logo=go)
![Vue](https://img.shields.io/badge/Vue-3.5%2B-42B883?style=flat-square&logo=vuedotjs)
![Gin](https://img.shields.io/badge/Gin-1.12%2B-357edd?style=flat-square)
![Element Plus](https://img.shields.io/badge/Element%20Plus-2.14%2B-409EFF?style=flat-square)
![License](https://img.shields.io/badge/License-MIT-blue?style=flat-square)

---

## 目录

- [项目说明](#项目说明)
- [功能特性](#功能特性)
- [技术栈](#技术栈)
- [项目结构](#项目结构)
- [开发环境要求](#开发环境要求)
- [快速开始](#快速开始)
  - [1. 克隆项目](#1-克隆项目)
  - [2. 配置文件](#2-配置文件)
  - [3. 后端运行](#3-后端运行)
  - [4. 前端开发](#4-前端开发)
- [构建与部署](#构建与部署)
  - [完整构建（单二进制）](#完整构建单二进制)
  - [仅前端构建](#仅前端构建)
  - [仅后端编译](#仅后端编译)
  - [Systemd 部署](#systemd-部署)
  - [RPM 包构建与部署](#rpm-包构建与部署)
  - [Nginx 反向代理](#nginx-反向代理)
- [配置说明](#配置说明)
- [接口文档](#接口文档)
  - [统一响应结构](#统一响应结构)
  - [认证接口](#认证接口)
  - [服务器管理](#服务器管理)
  - [条目操作](#条目操作)
  - [Schema 浏览](#schema-浏览)
  - [搜索](#搜索)
  - [LDIF 导入导出](#ldif-导入导出)
  - [书签](#书签)
  - [监控与日志](#监控与日志)
- [开发规范](#开发规范)

---

## 项目说明

GoLDAPAdmin 是一个现代化的 LDAP（Lightweight Directory Access Protocol）目录管理 Web 应用。它提供直观的图形化界面，用于浏览、搜索和管理 OpenLDAP / Active Directory 等 LDAP 服务器上的目录条目。

**核心优势：**
- **轻量单二进制** — 前端资源通过 Go embed 嵌入后端，部署只需一个可执行文件
- **多服务器支持** — 可管理多个 LDAP/AD 服务器，支持 LDAP/LDAPS/STARTTLS
- **完整 CRUD** — 条目增删改查、移动/复制/重命名、LDAP Compare、二进制属性下载
- **批量操作** — 批量删除、批量修改，对标 phpLDAPadmin mass_delete/mass_edit
- **Schema 浏览** — objectClass、attributeType 完整展示
- **JWT 鉴权** — 内置 Token 认证，支持多服务器凭据独立管理
- **操作审计** — 所有写操作记录到 SQLite，可追溯

---

## 功能特性

| 模块 | 功能 |
|------|------|
| **服务器管理** | 多 LDAP/AD 服务器配置、连接测试、TLS/SSL/STARTTLS |
| **目录浏览** | 树状视图、按层级展开、AD 用户禁用状态标识、子节点计数 |
| **条目操作** | 创建/修改/删除、复制/移动/重命名、LDAP Compare、二进制属性 |
| **批量操作** | 批量删除（含子孙递归）、批量修改、结果逐项反馈 |
| **Schema** | objectClass / attributeType 浏览、属性类型详情、结构展示 |
| **搜索** | 基础搜索、高级搜索（多条件、过滤字符串） |
| **LDIF** | LDIF 文件导入、全量/范围导出 |
| **书签** | 常用 DN 收藏、快速跳转 |
| **监控** | rootDSE 信息、服务器状态 |
| **日志** | 操作审计日志、错误追踪、TraceID 全链路 |

---

## 技术栈

### 后端
- **Go 1.26+** — 高性能后端
- **Gin** — HTTP Web 框架
- **go-ldap/ldap/v3** — LDAP 协议客户端
- **Golang JWT** — Token 认证
- **Viper** — 配置管理
- **SQLite** — 嵌入式数据库（服务器配置、书签、日志）
- **Zap** — 结构化日志

### 前端
- **Vue 3.5+** — Composition API
- **TypeScript** — 类型安全
- **Vite 8** — 极速构建
- **Element Plus** — UI 组件库
- **Pinia** — 状态管理
- **Vue Router** — 路由管理
- **Axios** — HTTP 请求
- **ECharts** — 图表可视化

---

## 项目结构

```
goldapadmin/
├── cmd/
│   └── server/
│       └── main.go                 # 服务启动入口
├── config/
│   ├── config.yaml                 # 当前使用的配置
│   ├── config.example.dev.yaml     # 开发环境配置示例
│   ├── config.example.internal.yaml
│   └── config.example.prod.yaml    # 生产环境配置示例
├── data/
│   └── goldapadmin.db              # SQLite 数据（自动创建）
├── deploy/
│   ├── rpm/                        # RPM 打包相关
│   │   ├── build-rpm.sh            # RPM 一键构建脚本
│   │   └── goldapadmin.spec        # RPM Spec 文件
│   ├── goldapadmin.service         # systemd unit 文件
│   ├── goldapadmin-nginx.conf     # Nginx 反代配置
│   └── install-systemd.sh          # 生产部署脚本
├── docs/                           # 技术/业务设计文档
├── internal/
│   ├── api/                        # HTTP 路由处理器
│   │   ├── router.go               # 路由注册
│   │   ├── auth.go                 # 认证
│   │   ├── entry.go                # 条目操作
│   │   ├── search.go               # 搜索
│   │   ├── server.go               # 服务器管理
│   │   ├── schema.go               # Schema 浏览
│   │   ├── ldif.go                 # LDIF 导入导出
│   │   ├── bookmark.go             # 书签
│   │   ├── rootdse.go              # rootDSE
│   │   ├── monitor.go              # 监控
│   │   └── log.go                  # 日志
│   ├── config/                     # 配置加载
│   ├── dao/                        # 数据访问层（SQLite）
│   ├── model/                      # 数据模型
│   ├── pkg/
│   │   ├── auth/                   # JWT 工具
│   │   ├── ldap/                   # LDAP 操作封装
│   │   ├── middleware/             # 中间件（Auth/CORS/Trace）
│   │   └── response/               # 统一响应
│   ├── service/                    # 业务逻辑层
│   └── web/                        # 前端静态资源 embed
├── web/
│   ├── src/                        # Vue 3 前端源码
│   │   ├── views/                  # 页面组件
│   │   ├── components/             # 公共组件
│   │   └── main.ts
│   ├── public/                     # 静态资源
│   ├── dist/                       # 构建产物（自动生成）
│   ├── package.json
│   └── vite.config.ts
├── Makefile                        # 构建脚本
├── go.mod / go.sum
└── README.md
```

---

## 开发环境要求

| 依赖 | 版本要求 | 说明 |
|------|---------|------|
| Go | >= 1.22 (go.mod 声明 1.26.4) | 后端编译 |
| Node.js | >= 18 | 前端构建 |
| npm | 随 Node.js | 包管理 |
| LDAP 服务器 | OpenLDAP / Active Directory / 389 Directory Server | 测试用 LDAP 后端 |

---

## 快速开始

### 1. 克隆项目

```bash
git clone https://github.com/lynnyq/goldapadmin.git
cd goldapadmin
```

### 2. 配置文件

复制开发环境配置：

```bash
cp config/config.example.dev.yaml config/config.yaml
```

按需修改 `config/config.yaml`：

```yaml
server:
  port: 8080
  mode: debug

jwt:
  secret: dev-secret-not-for-production-xxxxxxxxxxxxx
  ttl: 24

security:
  skip_tls_verify: true
  cors:
    origins:
      - "http://localhost:5173"
```

### 3. 后端运行

```bash
# 方式一：直接 go run（最快启动）
go run ./cmd/server config/config.yaml

# 方式二：编译后运行
make build-go && ./goldapadmin
```

启动成功后访问 http://localhost:8080

### 4. 前端开发

后端运行时会自动嵌入前端资源。如需前端热更新开发：

```bash
cd web
npm install
npm run dev    # Vite dev server，默认 http://localhost:5173
```

开发时前端通过 Vite proxy 转发 API 请求到后端（需在 `vite.config.ts` 配置）。

---

## 构建与部署

### 完整构建（单二进制）

后端 + 前端打包为一个独立二进制文件：

```bash
make build
# 等价于: make build-backend
# 产物: ./goldapadmin （前端已 embed）
```

构建流程：`npm build → 拷贝 web/dist → go build`

### 仅前端构建

```bash
make build-frontend
# 产物: web/dist/
```

### 仅后端编译

适用于只修改了 Go 代码的快速迭代：

```bash
make build-go
# 前提: internal/web/dist 已是最新
```

### Systemd 部署

适用于 Ubuntu/CentOS/RHEL 等 Linux 服务器：

```bash
# 1. 构建
make build

# 2. 一键部署（需要 root 权限）
sudo bash deploy/install-systemd.sh

# 3. 管理服务
sudo systemctl enable goldapadmin    # 开机自启
sudo systemctl start goldapadmin     # 启动
sudo systemctl status goldapadmin    # 查看状态
sudo systemctl stop goldapadmin      # 停止

# 4. 查看日志
journalctl -u goldapadmin -f
```

部署后目录结构：

| 路径 | 用途 |
|------|------|
| `/opt/goldapadmin/goldapadmin` | 二进制 |
| `/opt/goldapadmin/config/config.yaml` | 配置文件 |
| `/opt/goldapadmin/data/` | SQLite 数据 |
| `/var/log/goldapadmin/` | 日志目录 |

### RPM 包构建与部署

适用于 RHEL 8+/CentOS 8+/Rocky/AlmaLinux 等发行版，支持 x86_64 和 aarch64 交叉编译：

```bash
# 一键构建 RPM
./deploy/rpm/build-rpm.sh --version 0.1.0 --release 1

# 跳过前端构建（如前端未改动）
./deploy/rpm/build-rpm.sh --skip-frontend

# 安装
sudo rpm -ivh deploy/rpm/output/rpms/goldapadmin-0.1.0-1.el9.x86_64.rpm
sudo systemctl enable --now goldapadmin
```

RPM 部署路径：

| 路径 | 用途 |
|------|------|
| `/appdata/goldapadmin/goldapadmin` | 二进制 |
| `/appdata/goldapadmin/config/config.yaml` | 配置 |
| `/appdata/goldapadmin/data/` | SQLite 数据 |

### Nginx 反向代理

生产环境建议用 Nginx 做 HTTPS 终结 + 反代：

```bash
sudo cp deploy/goldapadmin-nginx.conf /etc/nginx/conf.d/goldapadmin.conf
# 修改 server_name 和 TLS 证书路径
sudo nginx -t && sudo systemctl reload nginx
```

Nginx 配置包含：
- TLS 1.2/1.3 + HSTS 安全头
- 静态资源长缓存（7 天 + immutable）
- Gzip 压缩
- 访问控制

---

## 配置说明

完整配置文件示例（`config/config.example.prod.yaml`）：

```yaml
server:
  port: 8080                    # 监听端口
  mode: release                 # debug / release / test

database:
  path: ./data/goldapadmin.db   # 相对路径（相对 WorkingDirectory）；生产用绝对路径，如 /appdata/goldapadmin/data/goldapadmin.db

jwt:
  secret: REPLACE_WITH_OPENSSL_RAND_BASE64_VALUE_MINUS_48_BYTES_XXXXXXXXXXXXXXXXXXXXXXXXX
  ttl: 12                       # Token 有效期（小时）

log:
  level: info                   # debug / info / warn / error

security:
  skip_tls_verify: false        # 是否跳过 LDAP TLS 证书校验
  cors:
    origins:                    # CORS 白名单（生产禁止 "*"）
      - "https://ldapadmin.company.com"
```

**生成安全的 JWT Secret：**

```bash
openssl rand -base64 48
```

---

## 接口文档

### 统一响应结构

```json
{
  "code": 0,
  "message": "ok",
  "data": {},
  "trace_id": "xxxxxxxxxxxxx"
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `code` | int | 0 = 成功，非 0 = 失败 |
| `message` | string | 提示信息 |
| `data` | any | 业务数据 |
| `trace_id` | string | 请求链路 ID |

**成功 HTTP 状态码：**
- 200 OK — 通用成功
- 201 Created — 创建成功
- 204 No Content — 删除成功

**失败 HTTP 状态码：**
- 400 Bad Request — 参数错误
- 401 Unauthorized — 未认证 / Token 过期
- 403 Forbidden — 无权限
- 404 Not Found — 资源不存在
- 500 Internal Server Error — 服务内部错误

### 认证接口

| 方法 | 路径 | 认证 | 说明 |
|------|------|------|------|
| POST | `/api/auth/login` | ❌ | 用户登录，返回 JWT Token |
| GET  | `/api/auth/logout` | ✅ | 用户登出 |

**POST /api/auth/login**

请求体：
```json
{
  "server_id": 1,
  "username": "admin",
  "password": "secret"
}
```

响应：
```json
{
  "code": 0,
  "data": {
    "token": "eyJhbGciOi...",
    "user_dn": "cn=admin,dc=example,dc=com",
    "server_id": 1,
    "expires_at": "2026-11-05T14:00:00Z"
  }
}
```

**认证方式：** 后续请求需在 Header 中携带 `Authorization: Bearer <token>`

### 服务器管理

| 方法 | 路径 | 认证 | 说明 |
|------|------|------|------|
| GET  | `/api/servers` | ❌ | 获取已保存的服务器列表 |
| POST | `/api/servers` | ✅ | 新增服务器配置 |
| PUT  | `/api/servers/:id` | ✅ | 更新服务器配置 |
| DELETE | `/api/servers/:id` | ✅ | 删除服务器配置 |
| POST | `/api/servers/test` | ✅ | 测试 LDAP 连接 |

**POST /api/servers/test**

请求体：
```json
{
  "host": "ldap.example.com",
  "port": 636,
  "base_dn": "dc=example,dc=com",
  "bind_dn": "cn=admin,dc=example,dc=com",
  "bind_password": "xxx",
  "use_ssl": true,
  "skip_tls_verify": false
}
```

### 条目操作

| 方法 | 路径 | 认证 | 说明 |
|------|------|------|------|
| GET  | `/api/entry/tree` | ✅ | 获取目录树 |
| GET  | `/api/entry/children` | ✅ | 获取指定 DN 的子条目 |
| GET  | `/api/entry/get` | ✅ | 获取条目详情 |
| GET  | `/api/entry/next_id` | ✅ | 获取下一个可用 ID（自增） |
| POST | `/api/entry/create` | ✅ | 创建新条目 |
| POST | `/api/entry/modify` | ✅ | 修改条目属性 |
| DELETE | `/api/entry/delete` | ✅ | 删除条目 |
| POST | `/api/entry/copy` | ✅ | 复制条目到新位置 |
| POST | `/api/entry/move` | ✅ | 移动条目 |
| POST | `/api/entry/rename` | ✅ | 重命名（修改 RDN） |
| POST | `/api/entry/compare` | ✅ | LDAP Compare 操作 |
| GET  | `/api/entry/binary` | ✅ | 获取二进制属性（图片预览等） |
| GET  | `/api/entry/binary-download` | ✅ | 下载二进制属性文件 |
| GET  | `/api/entry/descendants` | ✅ | 列出所有子孙条目（用于批量删除确认） |
| DELETE | `/api/entry/batch` | ✅ | 批量删除（含子孙递归） |
| POST | `/api/entry/batch-modify` | ✅ | 批量修改 |

**GET /api/entry/tree**

查询参数：`base_dn`（可选，默认使用服务器 base_dn）

响应：
```json
{
  "code": 0,
  "data": [
    {
      "dn": "ou=users,dc=example,dc=com",
      "rdn": "ou=users",
      "object_classes": ["top", "organizationalUnit"],
      "has_children": true,
      "is_leaf": false,
      "child_count": 42,
      "entry_type": "ou",
      "children": [...]
    }
  ]
}
```

**POST /api/entry/modify**

请求体：
```json
{
  "dn": "uid=johndoe,ou=users,dc=example,dc=com",
  "changes": [
    {"op": "replace", "attr": "mail", "values": ["john.new@example.com"]},
    {"op": "add", "attr": "telephoneNumber", "values": ["+8613800138000"]}
  ],
  "password_algos": {
    "userPassword": "SSHA"
  }
}
```

### Schema 浏览

| 方法 | 路径 | 认证 | 说明 |
|------|------|------|------|
| GET  | `/api/schema/objectclasses` | ✅ | 获取所有 objectClass 列表 |
| GET  | `/api/schema/attributetypes` | ✅ | 获取所有 attributeType 列表 |
| GET  | `/api/schema/objectclass/:name` | ✅ | 获取 objectClass 详情 |
| GET  | `/api/schema/attribute/:name` | ✅ | 获取 attributeType 详情 |
| POST | `/api/schema/allowed-attributes` | ✅ | 获取指定 objectClass 允许的属性 |

### 搜索

| 方法 | 路径 | 认证 | 说明 |
|------|------|------|------|
| GET  | `/api/search` | ✅ | LDAP 搜索 |

**GET /api/search**

查询参数：

| 参数 | 必填 | 说明 |
|------|------|------|
| `base_dn` | ❌ | 搜索起点 DN，默认服务器 base_dn |
| `filter` | ❌ | LDAP 过滤字符串，默认 `(objectClass=*)` |
| `scope` | ❌ | 搜索范围：`base` / `one` / `sub`，默认 `sub` |
| `attributes` | ❌ | 属性列表，逗号分隔 |
| `limit` | ❌ | 返回数量限制 |

示例：
```
GET /api/search?filter=(&(objectClass=person)(uid=john*))&scope=sub&limit=100
```

### LDIF 导入导出

| 方法 | 路径 | 认证 | 说明 |
|------|------|------|------|
| POST | `/api/ldif/import` | ✅ | 上传 LDIF 文件导入 |
| GET  | `/api/ldif/export` | ✅ | 导出 LDIF（支持范围） |

**POST /api/ldif/import**

请求类型：`multipart/form-data`

| 字段 | 类型 | 说明 |
|------|------|------|
| `file` | file | LDIF 文件 |
| `dry_run` | bool | 是否仅校验不实际导入（默认 false） |

### 书签

| 方法 | 路径 | 认证 | 说明 |
|------|------|------|------|
| GET  | `/api/bookmarks` | ✅ | 获取书签列表 |
| POST | `/api/bookmarks` | ✅ | 新增书签 |
| DELETE | `/api/bookmarks/:id` | ✅ | 删除书签 |

### 监控与日志

| 方法 | 路径 | 认证 | 说明 |
|------|------|------|------|
| GET  | `/api/health` | ❌ | 健康检查（返回 `{"status":"ok"}`） |
| GET  | `/api/rootdse` | ✅ | LDAP 服务器 rootDSE 信息 |
| GET  | `/api/monitor/stats` | ✅ | 服务器监控统计 |
| GET  | `/api/logs` | ✅ | 操作日志查询 |

**GET /api/rootdse**

响应：
```json
{
  "code": 0,
  "data": {
    "namingContexts": ["dc=example,dc=com"],
    "supportedLDAPVersion": ["3"],
    "supportedSASLMechanisms": ["PLAIN", "LOGIN", "EXTERNAL"],
    "vendorName": "OpenLDAP",
    "vendorVersion": "2.4.57+dfsg-3+deb11u1"
  }
}
```

---

## 开发规范

### 后端（Go）

- **格式化** — 严格遵循 GoFmt / GoImports，禁止手动排版
- **包名** — 小写短单词，无下划线/驼峰（如 `service` 而非 `user_service`）
- **文件名** — 小写 + 下划线（如 `user_service.go`）
- **错误处理** — 禁止忽略错误，必须处理或向上抛出
- **分层架构** — `api/`（路由）→ `service/`（业务）→ `dao/`（数据访问）
- **配置管理** — 禁止硬编码，敏感配置加密存储
- **单元测试** — 覆盖率 > 60%

### 前端（Vue 3 + TypeScript）

- **格式化** — ESLint + Prettier 统一规则
- **组件命名** — 大驼峰（如 `UserList.vue`）
- **变量/方法** — 小驼峰；常量：全大写 + 下划线
- **工程结构** — `api/`（接口）、`views/`（页面）、`components/`（公共组件）、`utils/`（工具）、`store/`（状态）
- **接口抽离** — 禁止在页面直接写 HTTP 请求
- **请求拦截器** — 统一携带 Token、TraceID

### 构建命令

```bash
make build            # 完整构建（前端 + 后端）
make build-frontend   # 仅前端构建
make build-go         # 仅后端编译（Go 代码快速迭代）
make dev              # 编译并运行
make run              # 运行已有二进制
make clean            # 清理产物
make help             # 查看所有命令
```

---

## License

[MIT](LICENSE) © GoLDAPAdmin Team
