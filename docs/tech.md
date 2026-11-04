# GoLDAPAdmin 技术说明文档

> 基于 Go + Vue3 的 LDAP 目录管理工具，单二进制部署，零外部运行时依赖。

---

## 一、技术栈

### 后端

| 技术 | 版本 | 用途 | 选型理由 |
|---|---|---|---|
| Go | 1.26.4 | 编程语言 | 原生并发、静态编译、单二进制、内存安全 |
| Gin | v1.12.0 | HTTP 框架 | 轻量高性能、中间件生态成熟 |
| go-ldap/ldap/v3 | v3.4.15 | LDAP 客户端 | Go 社区标准库，支持 TLS/SSL/SASL |
| modernc.org/sqlite | v1.61.1 | 嵌入式数据库 | **纯 Go 实现，无 CGO**，单二进制关键 |
| golang-jwt/jwt/v5 | v5.3.1 | JWT 认证 | 社区推荐、安全、支持 HS256 |
| viper | v1.21.0 | 配置管理 | YAML 加载 + 默认值 + 多数据源 |
| google/uuid | v1.6.0 | TraceID 生成 | 轻量、符合 RFC 4122 |
| zap | v1.28.0 | 结构化日志 | 已预留，高性能（当前用 gin.DefaultWriter） |

### 前端

| 技术 | 版本 | 用途 | 选型理由 |
|---|---|---|---|
| Vue | 3.5.44 | UI 框架 | 组合式 API、TypeScript 友好 |
| Vite | 8.3.5 | 构建工具 | 快速冷启动、HMR、Rollup 生态 |
| Element Plus | 2.14.7 | UI 组件库 | 后台管理场景最成熟 |
| Pinia | 4.1.0 | 状态管理 | 轻量、TS 友好、Vue 官方推荐 |
| Vue Router | 4.6.4 | 路由 | history 模式、导航守卫 |
| Axios | 1.21.0 | HTTP 客户端 | 拦截器封装、统一错误处理 |
| ECharts | 6.1.0 | 图表库 | 监控页统计图表 |
| TypeScript | ~6.0.3 | 类型系统 | 类型安全、IDE 友好 |
| Sass | 1.107.1 | CSS 预处理器 | 嵌套语法、变量、Mixin |

### 基础设施

| 技术 | 用途 |
|---|---|
| Go embed + fs.Sub | 前端资源内嵌到二进制 |
| Gin Middleware | TraceID / Logger / CORS / JWT Auth |

---

## 二、系统架构

### 总体架构

```
┌─────────────────────────────────────────────────────────────────┐
│                 goldapadmin (单二进制, ~40MB)                    │
│                                                                 │
│  ┌────────────────────────┐      ┌────────────────────────┐    │
│  │   嵌入式前端 (embed)    │      │     Go HTTP Server      │    │
│  │   index.html + assets  │      │     Gin Engine :8080    │    │
│  └───────────┬────────────┘      └───────────┬────────────┘    │
│              │  /                             │  /api/*          │
│              │  (NoRoute SPA 回退)           │                  │
│              │                                │                  │
│              │                    ┌────────────▼────────────┐   │
│              │                    │    Middleware Chain      │   │
│              │                    │    TraceID → Logger     │   │
│              │                    │    CORS → JWT Auth      │   │
│              │                    └────────────┬────────────┘   │
│              │                                │                  │
│              │                    ┌────────────▼────────────┐   │
│              │                    │       API Router         │   │
│              │                    │  auth/server/entry/...   │   │
│              │                    └────────────┬────────────┘   │
│              │                                │                  │
│              │                    ┌────────────▼────────────┐   │
│              │                    │     Service Layer        │   │
│              │                    │  LDAP 连接 + 业务编排   │   │
│              │                    └──────┬───────────┬───────┘   │
│              │                           │           │          │
│              ▼                           ▼           ▼          │
│  ┌──────────────────────┐   ┌─────────────────┐  ┌────────────┐  │
│  │ SQLite (嵌入式)      │   │   LDAP Server   │  │ LDAP Server│  │
│  │ ./data/goldapadmin.db│   │  (主服务器)     │  │ (其他服务器)│  │
│  └──────────────────────┘   └─────────────────┘  └────────────┘  │
└─────────────────────────────────────────────────────────────────┘
```

### 请求处理链路

```
HTTP Request
    │
    ▼
Gin Engine
    │
    ├── TraceID()     ← 生成/注入 X-Trace-ID，写入 gin.Context
    │
    ├── gin.Recovery() ← panic 恢复
    │
    ├── Logger()       ← 记录 method/path/status/latency/trace_id
    │
    ├── CORS()         ← 设置跨域头，处理 OPTIONS 预检
    │
    ▼
API Router
    │
    ├── 公开接口 (/api/auth/login, /api/health)
    │
    └── Auth() 中间件   ← 校验 JWT，提取 server_id + user_dn
            │
            ▼
        Handler 层
            │
            ▼
        Service 层 → 获取 LDAP 连接 → 执行 LDAP 操作 → 记录操作日志
            │
            ▼
        Response 统一响应 {code, message, data, trace_id}
```

---

## 三、目录结构

```
goldapadmin/
│
├── cmd/
│   └── server/
│       └── main.go                  # 应用入口
│                                    # 流程: 配置加载 → DB初始化 → 中间件链 → 路由注册 → 启动
│
├── config/
│   └── config.yaml                  # 运行时配置
│                                    # server.port=8080
│                                    # database.path=./data/goldapadmin.db
│                                    # jwt.secret / jwt.ttl=24h
│
├── internal/
│   ├── config/
│   │   └── config.go                # Config 结构体 + Viper 加载 + 默认值
│   │
│   ├── dao/                         # 数据访问层
│   │   ├── db.go                    # SQLite 初始化 + 自动建表迁移
│   │   │                            # modernc.org/sqlite (纯Go)
│   │   │                            # SetMaxOpenConns(1)
│   │   └── dao.go                   # ServerDAO / BookmarkDAO / LogDAO
│   │                                # 全部 ? 占位符参数化查询
│   │
│   ├── model/
│   │   └── models.go                # 全部数据模型
│   │                                # LdapServer / Bookmark / OperationLog
│   │                                # LdapEntry / LdapAttribute / TreeNode
│   │                                # ObjectClassDef / AttributeTypeDef
│   │                                # ModifyRequest / ModifyChange
│   │
│   ├── pkg/
│   │   ├── response/
│   │   │   └── response.go          # 统一响应 Response{code, message, data, trace_id}
│   │   │                            # Success() / Fail() / FailWithData()
│   │   │
│   │   ├── middleware/              # Gin 中间件
│   │   │   ├── trace.go             # TraceID() + Logger()
│   │   │   ├── cors.go              # CORS()
│   │   │   └── auth.go              # Auth() + GetServerID()/GetUserDN()/GetTraceID()
│   │   │
│   │   ├── auth/
│   │   │   └── jwt.go               # JWT GenerateToken() / ParseToken() (HS256)
│   │   │                            # Claims{server_id, user_dn}
│   │   │
│   │   └── ldap/                    # LDAP 封装层
│   │       ├── conn.go              # Connect / ConnectWithServer
│   │       │                        # 支持 ldap:// / ldaps:// / StartTLS
│   │       ├── entry.go             # EntryService: CRUD + Search + Copy + Move + Rename
│   │       │                        # HashPassword: SSHA / SHA / MD5
│   │       ├── schema.go            # SchemaService: objectClass + attributeType 解析
│   │       │                        # getAllowedAttributes + 内置默认降级
│   │       └── ldif.go              # LdifService: Import + Export + parseLdif
│   │
│   ├── service/
│   │   └── service.go               # LdapService: 业务编排层
│   │                                # GetConnParams / Connect / LogOperation
│   │                                # DAO getter 方法
│   │
│   ├── api/                         # HTTP Handler 层 (共 9 个文件)
│   │   ├── router.go                # 路由注册入口
│   │   │                            # 公开: POST /api/auth/login, GET /api/health
│   │   │                            # 认证分组: 其余所有接口
│   │   ├── auth.go                  # Login (Bind + JWT) / Logout
│   │   ├── server.go                # List / Create / Update / Delete / TestConnection
│   │   ├── entry.go                 # GetChildren / GetEntry / Create / Modify
│   │   │                            # Delete / Copy / Move / Rename
│   │   ├── schema.go                # GetObjectClasses / GetAttributeTypes
│   │   │                            # GetObjectClass / GetAttribute
│   │   │                            # GetAllowedAttributes
│   │   ├── search.go                # Search (base_dn + filter + scope + size_limit)
│   │   ├── ldif.go                  # ImportLdif (POST) / ExportLdif (GET)
│   │   ├── bookmark.go              # ListBookmarks / CreateBookmark / DeleteBookmark
│   │   ├── monitor.go               # GetMonitorStats (连接状态 + 操作统计 + 响应时间序列)
│   │   └── log.go                   # GetLogs (分页查询)
│   │
│   └── web/
│       ├── embed.go                 # //go:embed all:dist + fs.Sub
│       │                            # 静态资源路由 + SPA 回退 (NoRoute)
│       └── dist/                    # Vite 构建产物 (Makefile 从 web/dist 拷贝)
│
├── web/                             # Vue3 前端工程
│   ├── src/
│   │   ├── api/                     # API 请求封装 (共 8 个文件)
│   │   │   ├── server.ts            # 服务器配置 CRUD + TestConnection
│   │   │   ├── entry.ts             # 条目 CRUD + Copy + Move + Rename
│   │   │   ├── schema.ts            # Schema 查询
│   │   │   ├── search.ts            # 搜索
│   │   │   ├── ldif.ts              # LDIF 导入导出
│   │   │   ├── bookmark.ts          # 书签
│   │   │   ├── monitor.ts           # 监控统计
│   │   │   └── log.ts               # 操作日志
│   │   │
│   │   ├── views/                   # 页面组件 (共 12 个 .vue 文件)
│   │   │   ├── login/
│   │   │   │   └── Login.vue        # 登录页
│   │   │   ├── layout/
│   │   │   │   ├── MainLayout.vue   # 三栏主布局
│   │   │   │   ├── ServerTree.vue   # 左栏: 目录树 (el-tree 懒加载)
│   │   │   │   └── SidePanel.vue    # 右栏: Schema 参考 + 操作历史
│   │   │   ├── entry/
│   │   │   │   └── EntryView.vue    # 中栏: 条目详情 + 属性编辑
│   │   │   ├── servers/
│   │   │   │   └── ServerManager.vue # 服务器配置管理
│   │   │   ├── search/
│   │   │   │   └── AdvancedSearch.vue # 高级搜索 + 批量操作
│   │   │   ├── schema/
│   │   │   │   └── SchemaBrowser.vue # Schema 浏览 (OC + AT 双 Tab)
│   │   │   ├── bookmarks/
│   │   │   │   └── BookmarkList.vue  # 书签列表
│   │   │   ├── monitor/
│   │   │   │   └── ServerMonitor.vue # 监控面板 (ECharts)
│   │   │   ├── import/
│   │   │   │   └── LdifImport.vue    # LDIF 导入
│   │   │   └── logs/
│   │   │       └── OperationLogs.vue # 操作日志
│   │   │
│   │   ├── components/
│   │   │   └── entry/
│   │   │       └── CreateEntryDialog.vue # 新建条目弹窗 (模板选择 + 动态表单)
│   │   │
│   │   ├── store/                   # Pinia Stores (共 3 个)
│   │   │   ├── auth.ts              # login/logout + token 持久化
│   │   │   ├── server.ts            # 服务器列表 + currentServer
│   │   │   └── entry.ts             # selectedDN + selectedEntry + treeKey
│   │   │
│   │   ├── router/
│   │   │   └── index.ts             # Vue Router + 登录守卫
│   │   │
│   │   ├── utils/
│   │   │   ├── request.ts           # Axios 封装 (拦截器 + Token + 401 跳转)
│   │   │   └── dn.ts                # DN 解析 (parseDN / getRDN / getParentDN)
│   │   │
│   │   ├── types/
│   │   │   └── index.ts             # 完整 TypeScript 类型定义
│   │   │
│   │   ├── styles/
│   │   │   └── global.scss          # 全局样式 + 滚动条 + 工具类
│   │   │
│   │   ├── App.vue                  # 纯 router-view
│   │   └── main.ts                  # createApp + Pinia + Router + ElementPlus(zh-cn)
│   │
│   ├── vite.config.ts               # Vite 配置: proxy + alias + manualChunks
│   ├── tsconfig.app.json
│   └── package.json                 # 依赖声明 + scripts
│
├── docs/
│   ├── tech.md                      # 本文档
│   ├── business.md                  # 业务说明文档
│   └── superpowers/specs/
│       └── 2026-11-03-goldapadmin-design.md # 设计规格
│
├── Makefile                         # 三阶段构建流水线
├── go.mod / go.sum                  # Go module 依赖
└── README.md
```

---

## 四、认证流程

### 登录时序

```
前端                              后端                              LDAP
 │                                │                                │
 │  POST /api/auth/login          │                                │
 │  {                             │                                │
 │    server_id: 0 (临时配置)     │                                │
 │    host: "ldap.example.com"    │                                │
 │    port: 389                   │                                │
 │    bind_dn: "cn=admin,..."     │                                │
 │    password: "******"          │                                │
 │    use_tls: true               │                                │
 │    remember: true              │                                │
 │  }                             │                                │
 │───────────────────────────────►│                                │
 │                                │                                │
 │                                │  ConnectWithServer(server)     │
 │                                │  → DialURL("ldap://host:389")  │
 │                                │  → StartTLS (if use_tls)      │
 │                                │  → Bind(bind_dn, password)     │
 │                                │───────────────────────────────►│
 │                                │                                │
 │                                │     Bind 验证                  │
 │                                │◄───────────────────────────────│
 │                                │                                │
 │                                │  (if remember)                  │
 │                                │  ServerDAO.Create(server)      │
 │                                │  → SQLite INSERT ldap_servers  │
 │                                │                                │
 │                                │  auth.GenerateToken(server.ID,  │
 │                                │     bind_dn, 24h)              │
 │                                │  → JWT {HS256, server_id,      │
 │                                │          user_dn, exp, iat}    │
 │                                │                                │
 │  { token, server, user_dn }    │                                │
 │◄───────────────────────────────│                                │
 │                                │                                │
 │  存入 Pinia + localStorage     │                                │
 │  密码存内存态                   │                                │
```

### 后续请求

```
前端                              后端                              LDAP
 │                                │                                │
 │  GET /api/entry/children       │                                │
 │  ?base_dn=ou=users,...         │                                │
 │                                │                                │
 │  Headers:                      │                                │
 │  ├─ Authorization: Bearer <JWT>│                                │
 │  ├─ X-Ldap-Password: <密码>     │                                │
 │  └─ X-Trace-ID: trace-abc12345 │                                │
 │───────────────────────────────►│                                │
 │                                │                                │
 │                                │  Auth() 中间件                  │
 │                                │  → ParseToken(JWT)             │
 │                                │  → c.Set("server_id", claims)  │
 │                                │  → c.Set("user_dn", claims)    │
 │                                │                                │
 │                                │  Handler                       │
 │                                │  → middleware.GetServerID(c)    │
 │                                │  → middleware.GetUserDN(c)      │
 │                                │  → ServerDAO.Get(server_id)    │
 │                                │  → 从 X-Ldap-Password 取密码    │
 │                                │                                │
 │                                │  ConnectWithServer(server,     │
 │                                │                       user_dn, │
 │                                │                       password)│
 │                                │───────────────────────────────►│
 │                                │                                │
 │                                │  Search(base_dn, scope=single) │
 │                                │  → 返回子条目列表              │
 │                                │◄───────────────────────────────│
 │                                │                                │
 │                                │  LogOperation(...)             │
 │                                │  → SQLite INSERT operation_logs│
 │                                │                                │
 │  Response{code:0, data:[...],  │                                │
 │            trace_id:"..."}     │                                │
 │◄───────────────────────────────│                                │
```

### 关键设计决策

| 决策 | 原因 |
|---|---|
| JWT 只存 `server_id` + `user_dn`，**不存密码** | JWT payload 可读，密码不能落盘到 token |
| LDAP 密码通过 `X-Ldap-Password` header 每次传入 | 无状态、前端控制生命周期、可随时切换密码 |
| 密码存前端内存态（Pinia store） | 关闭浏览器即丢失，比 localStorage 安全 |
| SQLite **不存 BindDN 密码** | 只有服务器连接配置（host/port/base_dn） |
| 登录时可选"记住服务器" | 方便下次直接选择，跳过手动输入 |

---

## 五、API 接口清单

### 5.1 公共接口（无需 JWT）

| 方法 | 路径 | Handler | 说明 |
|---|---|---|---|
| POST | `/api/auth/login` | `Login` | LDAP Bind 验证 + JWT 签发 + 可选保存服务器 |
| GET | `/api/auth/logout` | `Logout` | 登出（前端清 token，后端空响应） |
| GET | `/api/health` | `StatusOK` | 健康检查 `{"status":"ok"}` |

### 5.2 需认证接口（需 `Authorization: Bearer <token>`）

#### 服务器管理

| 方法 | 路径 | Handler | 说明 |
|---|---|---|---|
| GET | `/api/servers` | `ListServers` | 获取所有保存的 LDAP 服务器 |
| POST | `/api/servers` | `CreateServer` | 创建服务器配置 |
| PUT | `/api/servers/:id` | `UpdateServer` | 更新服务器配置 |
| DELETE | `/api/servers/:id` | `DeleteServer` | 删除服务器配置 |
| POST | `/api/servers/test` | `TestConnection` | 测试连接（返回 `{connected: bool, message: string}`） |

#### 条目操作

| 方法 | 路径 | Handler | 说明 |
|---|---|---|---|
| GET | `/api/entry/children?base_dn=` | `GetChildren` | 获取子节点（ScopeSingleLevel） |
| GET | `/api/entry/get?dn=` | `GetEntry` | 获取条目详情（ScopeBaseObject，属性 `*` + `+`） |
| POST | `/api/entry/create` | `CreateEntry` | 创建条目 |
| POST | `/api/entry/modify` | `ModifyEntry` | 修改条目（支持 add/replace/delete 三种操作） |
| DELETE | `/api/entry/delete?dn=&recursive=` | `DeleteEntry` | 删除条目（recursive=true 级联） |
| POST | `/api/entry/copy` | `CopyEntry` | 复制条目（可选 recursive） |
| POST | `/api/entry/move` | `MoveEntry` | 移动条目（ModifyDN + newParentDN） |
| POST | `/api/entry/rename` | `RenameEntry` | 重命名条目（ModifyDN） |

#### Schema

| 方法 | 路径 | Handler | 说明 |
|---|---|---|---|
| GET | `/api/schema/objectclasses` | `GetObjectClasses` | 获取所有 objectClass 定义 |
| GET | `/api/schema/attributetypes` | `GetAttributeTypes` | 获取所有 attributeType 定义 |
| GET | `/api/schema/objectclass/:name` | `GetObjectClass` | 按名称查找 objectClass |
| GET | `/api/schema/attribute/:name` | `GetAttribute` | 按名称查找 attributeType |
| POST | `/api/schema/allowed-attributes` | `GetAllowedAttributes` | 批量查询多个 objectClass 的 MUST/MAY 属性 |

#### 搜索

| 方法 | 路径 | Handler | 说明 |
|---|---|---|---|
| GET | `/api/search` | `Search` | LDAP 搜索（base_dn, filter, scope, size_limit） |

#### LDIF

| 方法 | 路径 | Handler | 说明 |
|---|---|---|---|
| POST | `/api/ldif/import` | `ImportLdif` | 导入 LDIF 内容 |
| GET | `/api/ldif/export?dn=&recursive=` | `ExportLdif` | 导出 LDIF（Content-Disposition 下载） |

#### 书签

| 方法 | 路径 | Handler | 说明 |
|---|---|---|---|
| GET | `/api/bookmarks` | `ListBookmarks` | 当前服务器的书签列表 |
| POST | `/api/bookmarks` | `CreateBookmark` | 创建书签（自动关联当前 server_id） |
| DELETE | `/api/bookmarks/:id` | `DeleteBookmark` | 删除书签 |

#### 监控与日志

| 方法 | 路径 | Handler | 说明 |
|---|---|---|---|
| GET | `/api/monitor/stats` | `GetMonitorStats` | 连接状态 + 操作统计 + 响应时间序列 |
| GET | `/api/logs?page=&page_size=` | `GetLogs` | 操作日志分页查询 |

---

## 六、统一响应规范

### Response 结构

```go
type Response struct {
    Code    int         `json:"code"`       // 0 = 成功，非 0 = 业务错误码
    Message string      `json:"message"`    // 提示信息
    Data    interface{} `json:"data"`       // 业务数据
    TraceID string      `json:"trace_id"`   // 全链路追踪 ID
}
```

### 示例

```json
// 成功
{
    "code": 0,
    "message": "ok",
    "data": [...],
    "trace_id": "trace-a1b2c3d4"
}

// 失败
{
    "code": 401,
    "message": "LDAP 认证失败: invalid credentials",
    "data": null,
    "trace_id": "trace-e5f6g7h8"
}
```

### 错误码约定

| 码值 | 含义 |
|---|---|
| 0 | 成功 |
| 400 | 参数错误 |
| 401 | 未登录 / 登录过期 / JWT 无效 |
| 403 | 无权限 |
| 404 | 资源不存在 |
| 500 | LDAP 操作失败 / 内部错误 |

---

## 七、数据模型

### SQLite 表结构

#### ldap_servers — LDAP 服务器配置

```sql
CREATE TABLE ldap_servers (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    name       TEXT NOT NULL,                   -- 显示名称
    host       TEXT NOT NULL,                   -- LDAP 主机
    port       INTEGER NOT NULL DEFAULT 389,   -- 端口 (389 / 636)
    base_dn    TEXT NOT NULL DEFAULT '',        -- 搜索基准 DN
    bind_dn    TEXT NOT NULL DEFAULT '',        -- 默认 Bind DN
    use_tls    INTEGER NOT NULL DEFAULT 0,      -- 是否启用 StartTLS
    use_ssl    INTEGER NOT NULL DEFAULT 0,      -- 是否启用 SSL (ldaps://)
    timeout    INTEGER NOT NULL DEFAULT 30,     -- 连接超时 (秒)
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

#### bookmarks — 目录书签

```sql
CREATE TABLE bookmarks (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    server_id  INTEGER NOT NULL,                -- 关联的 LDAP 服务器
    name       TEXT NOT NULL,                   -- 书签名称
    dn         TEXT NOT NULL,                   -- 收藏的 DN
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

#### operation_logs — 操作审计日志

```sql
CREATE TABLE operation_logs (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    server_id   INTEGER NOT NULL,               -- 关联的 LDAP 服务器
    user_dn     TEXT NOT NULL DEFAULT '',        -- 操作人 DN
    action      TEXT NOT NULL,                   -- 操作类型
    target_dn   TEXT NOT NULL DEFAULT '',        -- 目标条目 DN
    detail      TEXT NOT NULL DEFAULT '',        -- 详情
    status      TEXT NOT NULL DEFAULT 'success', -- success / failed
    error_msg   TEXT NOT NULL DEFAULT '',        -- 错误信息
    duration_ms INTEGER NOT NULL DEFAULT 0,      -- 操作耗时 (ms)
    created_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

### Go 模型

```go
// LDAP 服务器
type LdapServer struct {
    ID, Port, Timeout int
    Host, Name, BaseDN, BindDN string
    UseTLS, UseSSL bool
    CreatedAt, UpdatedAt time.Time
}

// 书签
type Bookmark struct {
    ID, ServerID int
    Name, DN string
    CreatedAt time.Time
}

// 操作日志
type OperationLog struct {
    ID, ServerID int
    UserDN, Action, TargetDN, Detail, Status, ErrorMsg string
    DurationMS int64
    CreatedAt time.Time
}

// LDAP 条目
type LdapEntry struct {
    DN            string
    ObjectClasses []string
    Attributes    []LdapAttribute
    HasChildren   bool
}

// LDAP 属性
type LdapAttribute struct {
    Name   string
    Values []string
}

// 目录树节点
type TreeNode struct {
    DN, RDN string
    ObjectClasses []string
    HasChildren   bool
}

// Schema: objectClass 定义
type ObjectClassDef struct {
    OID, Name, Description, Type string       // Type: structural / auxiliary / abstract
    Names, Must, May, Superior []string
}

// Schema: attributeType 定义
type AttributeTypeDef struct {
    OID, Name, Description, Syntax string
    Names []string
    SingleValue, Mandatory bool
}

// 修改请求
type ModifyRequest struct {
    DN      string
    Changes []ModifyChange   // Op: add / replace / delete
}

type ModifyChange struct {
    Op     string
    Attr   string
    Values []string
}
```

### SQLite 技术说明

| 项目 | 说明 |
|---|---|
| 驱动 | `modernc.org/sqlite` — **纯 Go 实现，无 CGO** |
| 连接 | `sql.Open("sqlite", "./data/goldapadmin.db")` |
| 并发控制 | `SetMaxOpenConns(1)` — SQLite 写锁特性，单序列化连接最安全 |
| 建表 | 启动时自动 `CREATE TABLE IF NOT EXISTS` |
| 迁移 | 目前仅初始建表，后续版本需加 migration 机制 |

---

## 八、LDAP 封装层详解

### conn.go — 连接管理

```go
func Connect(info ConnInfo) (*ldap.Conn, error)
// 流程:
// 1. net.Dialer{Timeout}
// 2. if UseSSL → ldap.DialURL("ldaps://host:port") + TLSConfig
//    else     → ldap.DialURL("ldap://host:port")
// 3. if UseTLS && !UseSSL → conn.StartTLS()
// 4. conn.Bind(bindDN, password)
// 5. 返回已绑定的连接

func ConnectWithServer(server *model.LdapServer, bindDN, password string) (*ldap.Conn, error)
// 从 LdapServer 构造 ConnInfo 后调用 Connect
```

### entry.go — 条目操作

```
EntryService
├── GetChildren(conn, baseDN)          → Search(scope=single, filter="(objectClass=*)", attrs=["dn","objectClass"])
├── GetEntry(conn, dn)                 → Search(scope=base, attrs=["*", "+"])
├── CreateEntry(conn, dn, attrs)       → AddRequest
├── ModifyEntry(conn, dn, changes)     → ModifyRequest (add/replace/delete)
├── DeleteEntry(conn, dn, recursive)   → DelRequest; recursive=true 时先递归删除子节点
├── CopyEntry(conn, src, dest, recur)  → GetEntry → CreateEntry; recur=true 递归拷贝子节点
├── MoveEntry(conn, src, newParent, rdn) → ModifyDNRequest
├── RenameEntry(conn, dn, newRDN)      → ModifyDNRequest (同 Move，parent=空)
└── Search(conn, baseDN, filter, scope, attrs, sizeLimit) → SearchRequest

HashPassword(algo, password)
├── SSHA → {SSHA}base64(sha1(pwd+salt)+salt)
├── SHA  → {SHA}base64(sha1(pwd))
├── MD5  → {MD5}base64(md5(pwd))
└── default → 原文返回
```

### schema.go — Schema 解析

```
SchemaService
├── GetObjectClasses(conn)           → Search("cn=schema,cn=config") 失败则 "cn=subschema" 失败则默认列表
├── GetAttributeTypes(conn)          → 同上策略
├── GetObjectClassByName(conn, name) → GetObjectClasses → 遍历查找
├── GetAttributeByName(conn, name)   → GetAttributeTypes → 遍历查找
└── GetAllowedAttributes(conn, ocs)  → 合并多个 OC 的 MUST/MAY，MUST 优先级更高

Schema 解析 (regex):
├── OID 提取: 行首数字点分十进制
├── NAME:  'xxx' 或 (xxx yyy) 形式
├── DESC:  'xxx'
├── MUST:  attr1 $ attr2 $ ... (用 $ 分隔)
├── MAY:   同上
├── SUP:   空白分隔
├── Type:  检测 AUXILIARY / ABSTRACT 关键字
└── SYNTAX / SINGLE-VALUE: 直接匹配关键字

内置默认降级 (defaultObjectClasses/defaultAttributeTypes):
└── 覆盖最常用的 7 个 OC + 50+ 个常见属性类型
```

### ldif.go — LDIF 导入导出

```
LdifService
├── Import(conn, content)             → parseLdif → 逐条 AddRequest/DelRequest
│                                      → 返回 {success, failed, errors}
│
├── Export(conn, dn, recursive)       → GetEntry → 写 LDIF 格式; recursive=true 递归子树
│
└── parseLdif(content)                → bufio.Scanner 逐行解析
                                       → 空行分隔条目
                                       → ":" 分隔属性名/值
                                       → 支持续行 (行首空格)
                                       → 跳过 # 注释
```

---

## 九、中间件详解

### TraceID 中间件

```go
// 每个请求生成唯一 TraceID
// 优先取前端传入的 X-Trace-ID header
// 否则生成 "trace-" + uuid[:8]
// 写入 gin.Context → 后续 handler 可通过 GetTraceID() 获取
// 写入 Response Header → 前端/运维可串联日志
```

### Logger 中间件

```go
// 记录 method / path / status / latency / trace_id
// 输出格式: "GET /api/servers 2xx 12.3ms trace-abc12345"
// 使用 gin.DefaultWriter (stderr)
// 注意: trace_id 必须安全断言，不能直接 .(string) — NoRoute 时可能未设置
```

### CORS 中间件

```go
// 开发环境全开
// Access-Control-Allow-Origin: *
// Access-Control-Allow-Methods: GET, POST, PUT, DELETE, OPTIONS
// Access-Control-Allow-Headers: Content-Type, Authorization, X-Trace-ID
// OPTIONS 预检直接返回 204 No Content
```

### Auth 中间件

```go
// 1. 检查 Authorization header 存在
// 2. 格式必须是 "Bearer <token>"
// 3. auth.ParseToken 解析验证
//    - HS256 签名校验
//    - 过期检查
//    - method 安全检查 (只能 HMAC)
// 4. 将 claims 写入 Context: server_id, user_dn
// 5. 验证失败 → 401 + c.Abort()
```

---

## 十、前端架构

### 状态管理 (Pinia Stores)

| Store | 状态 | Actions |
|---|---|---|
| `auth` | token (string) | login / logout |
| `server` | servers[] / currentServer | list / select / create / update / delete |
| `entry` | selectedDN / selectedEntry / treeKey | selectDN / selectEntry / refreshTree |

### 请求拦截器

```typescript
// utils/request.ts
// 请求拦截: 自动注入 Authorization / X-Ldap-Password / X-Trace-ID
// 响应拦截: 统一错误处理 + 401 跳转登录页
```

### 三栏布局数据流

```
左栏 ServerTree.vue         中栏 EntryView.vue          右栏 SidePanel.vue
    │                           │                           │
    │ 点击节点                   │                           │
    │ store.entry.selectDN(dn)  │                           │
    │──────────────────────────►│                           │
    │                           │                           │
    │                           │ 触发 GetEntry API         │
    │                           │ GET /api/entry/get?dn=...  │
    │                           │─────────────────────────► │ (联动刷新)
    │                           │                           │
    │                           │ 触发 Schema API           │
    │                           │ POST /api/schema/...      │
    │                           │─────────────────────────► │
    │                           │                           │
    │                           │ store.entry.selectedEntry  │
    │                           │──────────────────────────►│ Schema + 历史
    │                           │                           │
```

### SPA 回退机制

```go
// internal/web/embed.go
// NoRoute handler:
// if path starts with "/api/" → 返回 JSON 404
// else → fs.ReadFile(sub, "index.html") → c.Data(200, "text/html", data)
//
// 注: 不能用 c.FileFromFS — 之前测试出现 Location: ./ 重定向
// 直接 fs.ReadFile + c.Data 最可靠
```

---

## 十一、构建与部署

### 构建流水线（三阶段）

```
[1/3] Vite 构建前端
      cd web && npm install && npm run build
      → web/dist/  (index.html + assets/*)

[2/3] 拷贝到 Go embed 目录
      rm -rf internal/web/dist
      cp -R web/dist/. internal/web/dist/

[3/3] Go 编译单二进制
      go build -o goldapadmin ./cmd/server
      │
      └── cmd/server/main.go
          │
          └── import "github.com/lynnyq/goldapadmin/internal/web"
              └── //go:embed all:dist     ← 编译时自动嵌入 internal/web/dist/
```

### Makefile 命令

| 命令 | 说明 |
|---|---|
| `make build` | **完整构建**（前端 → 拷贝 → Go 编译） |
| `make build-frontend` | 仅构建前端到 web/dist |
| `make sync-frontend` | 前端构建 + 拷贝（不编译 Go） |
| `make build-backend` | 完整构建（同 `make build`） |
| `make build-go` | 仅编译 Go（改 Go 代码时快速重编，跳过前端） |
| `make dev` | build-go + 运行 |
| `make run` | 运行已有二进制 |
| `make clean` | 清理所有产物（二进制、dist、SQLite 数据） |

### 部署

```bash
# 1. 构建
make build

# 2. 部署
scp goldapadmin config/config.yaml user@server:/opt/goldapadmin/

# 3. 运行（直接指定配置文件）
cd /opt/goldapadmin
./goldapadmin ./config/config.yaml

# 或用默认路径（config/config.yaml）
./goldapadmin
```

### systemd 服务示例

```ini
# /etc/systemd/system/goldapadmin.service
[Unit]
Description=GoLDAPAdmin
After=network.target

[Service]
Type=simple
User=goldapadmin
Group=goldapadmin
WorkingDirectory=/opt/goldapadmin
ExecStart=/opt/goldapadmin/goldapadmin /opt/goldapadmin/config/config.yaml
Restart=always
RestartSec=5
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
```

```bash
systemctl daemon-reload
systemctl enable goldapadmin
systemctl start goldapadmin
```

### Dockerfile（多阶段构建）

```dockerfile
# ===== 前端构建 =====
FROM node:20-alpine AS frontend
WORKDIR /app/web
COPY web/package.json web/package-lock.json ./
RUN npm install --silent
COPY web/ ./
RUN npm run build

# ===== Go 构建 =====
FROM golang:1.26-alpine AS backend
RUN apk add --no-cache git
WORKDIR /app
COPY --from=frontend /app/web/dist ./internal/web/dist
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /goldapadmin ./cmd/server

# ===== 最终镜像 =====
FROM alpine:3.20
RUN addgroup -S appgroup && adduser -S appuser -G appgroup
COPY --from=backend /goldapadmin /usr/local/bin/goldapadmin
COPY config/config.yaml /etc/goldapadmin/config.yaml
USER appuser
VOLUME /app/data
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/goldapadmin"]
CMD ["/etc/goldapadmin/config.yaml"]
```

### 配置文件详解

```yaml
# config/config.yaml
server:
  port: 8080          # HTTP 端口
  mode: release       # Gin 模式: debug / release / test

database:
  path: ./data/goldapadmin.db   # SQLite 文件路径（自动创建目录）

jwt:
  secret: goldapadmin-secret-key-change-me  # ⚠️ 生产环境必须修改
  ttl: 24                                     # Token 有效期（小时）

log:
  level: info                                  # debug / info / warn / error
```

---

## 十二、安全设计

| 风险点 | 防护措施 | 当前状态 |
|---|---|---|
| LDAP 密码存储 | **完全不持久化**。每次请求从 header 传入，前端存内存态 | ✅ 已实现 |
| JWT 密钥泄露 | 配置文件注入，默认值仅用于开发，生产必须覆盖 | ✅ 已实现 |
| TLS 证书验证 | 开发模式 `InsecureSkipVerify=true`，生产需配置 CA | ⚠️ 开发便利但有风险 |
| SQL 注入 | 全部使用 `database/sql` 占位符参数化查询 | ✅ 已实现 |
| CORS 开放 | 开发允许 `*`，生产反向代理到同域即可避免 | ⚠️ 需运维配合 |
| 前端 XSS | Vue 模板默认转义 + Element Plus 组件安全 | ✅ 已实现 |
| 路径遍历 | embed 路径固定，SPA 回退用 `fs.ReadFile` 限定在 dist 内 | ✅ 已实现 |
| 暴力破解 | 登录无次数限制，依赖 LDAP 自身防护 | ⚠️ 可加限流中间件 |
| 密码明文传输 | 建议 HTTPS 反向代理，X-Ldap-Password 在 HTTPS 下传输才安全 | ⚠️ 需运维配合 |

---

## 十三、性能与稳定性

| 指标 | 设计 | 备注 |
|---|---|---|
| LDAP 连接 | 每次请求新建短连接 | 简单场景够用，高 QPS 可考虑 sync.Pool 连接池 |
| SQLite 并发 | `SetMaxOpenConns(1)` | 写串行化，避免 SQLite 锁竞争 |
| 请求超时 | LDAP 层默认 30s | 可在服务器配置中调整 timeout 字段 |
| TraceID | 贯穿请求处理链 + 日志 | 全链路可追踪 |
| 优雅关闭 | 目前依赖 systemd 或信号 | 可扩展 os.Signal 处理 |
| 前端性能 | manualChunks 拆分 (element-plus / echarts / vendor) | 首屏资源按需加载 |

---

## 十四、常见问题排查

### 登录失败

```
现象: "LDAP 认证失败: invalid credentials"
排查:
  1. 确认 Bind DN 完整正确 (cn=admin,dc=example,dc=com)
  2. 确认密码正确
  3. 确认网络可达 (telnet host 389)
  4. 如用 SSL: 端口应为 636
  5. 如用 TLS: 勾选 "Use TLS"，端口仍为 389
```

### 目录树为空

```
现象: 左栏空白
排查:
  1. 登录时的 Base DN 是否正确
  2. 确认该 DN 下确实有子条目
  3. 浏览器控制台 Network → GetChildren → 检查响应
  4. 如果 AD + 大子树: 可能 scope=single 返回慢，加 sizeLimit
```

### Schema 不显示

```
现象: 右栏 Schema 参考为空
排查:
  1. OpenLDAP 查 cn=schema,cn=config
  2. 有些 AD 不暴露 schema → 系统自动降级用内置默认列表
  3. 内置列表覆盖 7 个 OC + 50+ 属性，足够日常使用
```

### SQLite 锁冲突

```
现象: "database is locked"
排查:
  1. 确认只有一个 goldapadmin 实例写同一个 db 文件
  2. SetMaxOpenConns(1) 已保证串行化
  3. 可用 WAL 模式: PRAGMA journal_mode=WAL
```

---

## 十五、后续迭代规划

### P0 — 测试与加固

- [ ] 端到端集成测试（真实 OpenLDAP / AD 实例）
- [ ] LDAP 错误信息脱敏（生产不透传原始 LDAP 错误）
- [ ] TLS 证书验证配置化（生产禁用 `InsecureSkipVerify`）
- [ ] 密码传输加密（AES 加密 X-Ldap-Password 或强制 HTTPS）
- [ ] 单元测试覆盖 DAO + pkg/ldap + pkg/response

### P1 — 功能增强

- [ ] LDAP 连接池（sync.Pool，避免每次新建连接）
- [ ] 目录树右键菜单（新建/复制/移动/重命名/删除/书签）
- [ ] 批量操作进度条（LdifImport 返回进度，前端实时展示）
- [ ] 多服务器监控对比（顶部栏切换时自动刷新 monitor）
- [ ] 暗黑模式（Element Plus 主题切换）
- [ ] 国际化（zh-CN / en-US）

### P2 — 运维

- [ ] Dockerfile + docker-compose.yaml（GoLDAPAdmin + OpenLDAP 测试环境）
- [ ] systemd unit file 示例（已在上文给出）
- [ ] 优雅关闭（os.Signal + Shutdown）
- [ ] 请求限流中间件（登录防暴力破解）

---

## 十六、核心依赖版本索引

| 包 | 版本 | 用途 |
|---|---|---|
| go-ldap/ldap/v3 | v3.4.15 | LDAPv3 协议客户端 |
| gin-gonic/gin | v1.12.0 | HTTP Web 框架 |
| golang-jwt/jwt/v5 | v5.3.1 | JWT 签发与解析 |
| modernc.org/sqlite | v1.61.1 | SQLite 驱动（纯 Go） |
| spf13/viper | v1.21.0 | 配置文件加载 |
| google/uuid | v1.6.0 | UUID 生成（TraceID） |
| gin-contrib/sse | v1.1.0 | Server-Sent Events 支持 |
| vue | 3.5.44 | 前端框架 |
| element-plus | 2.14.7 | UI 组件库 |
| vite | 8.3.5 | 前端构建 |
| echarts | 6.1.0 | 图表可视化 |
| axios | 1.21.0 | HTTP 客户端 |
| pinia | 4.1.0 | 状态管理 |
| vue-router | 4.6.4 | 路由管理 |
