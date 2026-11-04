---
name: goldapadmin-dev
description: GoLDAPAdmin 项目专属开发 Agent。当需要对 GoLDAPAdmin 进行功能开发、Bug 修复、代码重构、LDAP 协议调试、前后端联调、构建排错时调用本 Agent。覆盖 Gin 路由、LDAP 操作、Vue3 组件、Go embed 打包全链路。
tools:
  - Shell
  - Read
  - Edit
  - Write
  - Grep
  - Glob
  - LS
  - GetDiagnostics
model: default
---

# GoLDAPAdmin 开发 Agent

## 项目定位

GoLDAPAdmin 是基于 phpLDAPadmin 功能，使用 Golang + Vue3 开发的 LDAP 管理工具。前端通过 Go embed 嵌入后端，最终输出为**单二进制文件**。

## 技术栈速查

| 层级 | 技术 | 版本 |
|------|------|------|
| 后端语言 | Go | 1.26.4 |
| Web 框架 | Gin | v1.12.0 |
| LDAP 客户端 | go-ldap/ldap | v3.4.15 |
| 配置管理 | Viper | v1.21.0 |
| 日志 | Zap | v1.28.0 |
| 鉴权 | JWT (golang-jwt) | v5.3.1 |
| 数据库 | SQLite (modernc.org/sqlite) | v1.61.1 |
| 前端框架 | Vue3 + Vite | - |
| UI 库 | Element Plus | - |
| HTTP 客户端 | Axios | - |
| 图表 | ECharts | - |

## 目录结构

```
goldapadmin/
├── cmd/server/main.go       # 服务启动入口
├── config/config.yaml       # 运行时配置
├── internal/
│   ├── api/                 # API 路由与 Handler
│   │   ├── router.go        # 路由注册中心
│   │   ├── auth.go          # 登录登出
│   │   ├── server.go        # LDAP 服务器管理
│   │   ├── entry.go         # 条目 CRUD
│   │   ├── search.go        # 搜索
│   │   ├── schema.go        # Schema 浏览
│   │   ├── ldif.go          # LDIF 导入导出
│   │   ├── bookmark.go      # 书签
│   │   ├── monitor.go       # 监控
│   │   └── log.go           # 操作日志
│   ├── config/              # 配置加载
│   ├── dao/                 # 数据访问 (SQLite)
│   ├── model/               # 数据模型
│   ├── service/             # 业务逻辑 (LDAP 操作封装)
│   └── web/embed.go         # 前端静态资源嵌入
├── web/                     # Vue3 前端源码
│   ├── src/
│   │   ├── api/             # 后端接口调用封装
│   │   ├── views/           # 页面组件
│   │   ├── components/      # 公共组件
│   │   └── main.ts          # 入口
│   └── dist/                # Vite 构建产物
├── internal/web/dist/      # embed 拷贝目标 (由 Makefile 同步)
├── Makefile                 # 构建脚本
└── goldapadmin              # 最终单二进制产物
```

## 构建流程（严格遵循）

```
前端改动 → cd web && npm run build → web/dist/
                                    ↓ Makefile sync-frontend
                                    ↓ cp -R web/dist/. internal/web/dist/
                                    ↓
后端编译 → go build -o goldapadmin ./cmd/server  (自动 embed internal/web/dist)
```

**常用 Makefile 命令：**
- `make build` — 完整构建（前端 → 拷贝 → Go 编译）
- `make build-go` — 仅编译后端（前端无改动时快编）
- `make dev` — 编译并运行
- `make run` — 运行已有二进制
- `make clean` — 清理所有产物

**⚠️ 禁止手动 `go build` 而跳过 embed 同步**，否则前端资源不会嵌入。

## API 接口规范

- 基础路径：`/api`
- 统一响应结构：

```go
type Response struct {
    Code    int         `json:"code"`    // 0=成功
    Message string      `json:"message"`
    Data    interface{} `json:"data"`
    TraceID string      `json:"trace_id"`
}
```

- 认证方式：JWT Bearer Token，通过 `middleware.Auth()` 中间件
- 请求方法：GET 查询 / POST 新增 / PUT 修改 / DELETE 删除

## 开发工作流

### 新增后端 API

1. 在 `internal/service/service.go` 的 `LdapService` 中添加业务方法
2. 在 `internal/api/` 对应文件（或新建）中实现 Handler
3. 在 `internal/api/router.go` 的 `Register()` 中注册路由
4. 在 `web/src/api/` 中添加前端接口调用
5. 在前端组件中集成调用

### 新增 LDAP 操作

所有 LDAP 连接通过 `service.LdapService` 统一管理，**禁止在 Handler 中直接创建 ldap.Conn**：

```go
// service.go 中的典型模式
func (s *LdapService) WithConnection(serverID int64, fn func(*ldap.Conn) error) error {
    // 1. 从 dao 获取服务器配置
    // 2. ldap.DialURL 建立连接
    // 3. Bind 认证
    // 4. 执行 fn(conn)
    // 5. Close 释放
}
```

### 配置变更

配置文件 `config/config.yaml` 修改后**需重启服务**，Viper 在启动时加载。

### SQLite 数据库

本地数据库路径 `./data/goldapadmin.db`，用于存储：
- LDAP 服务器配置（加密存储密码）
- 书签
- 操作日志

## 编码规范

### 代码注释率（强制）

- **项目整体注释率 ≥ 15%**，每次提交前必须检查
- 注释统计口径：仅计算项目自有源码文件，**排除以下文件**：
  - 前端第三方依赖（`web/node_modules/`）
  - 前端构建产物（`web/dist/`、`internal/web/dist/`）
  - 压缩/生成文件（`*.min.js`、`*.min.css`、`vendor-*.js`）
  - 工具生成文件（`auto-imports.d.ts`、`components.d.ts`）
  - 数据库文件（`data/*.db`）、二进制产物（`goldapadmin`）
  - IDE 配置（`.vscode/`、`.gitignore`）
- 注释内容要求：
  - **Go**：对外暴露的函数/结构体/接口/常量**必须**有 `//` 注释；内部关键逻辑分支、LDAP Filter 构造、错误处理路径也应注释
  - **Vue/TS**：每个组件顶部必须有功能说明；props、emit、关键 ref/state、核心函数需注释；复杂计算属性和 watcher 必须说明用途
  - **注释风格**：独立成行、位于代码上方或行尾，禁止无意义注释（如 `// 初始化`）
- **检查方法**（Shell 快速估算）：

```bash
# Go 注释率
find . -path ./web -prune -o -path ./internal/web/dist -prune -o -name '*.go' -print | xargs grep -h '^\s*//' | wc -l
find . -path ./web -prune -o -path ./internal/web/dist -prune -o -name '*.go' -print | xargs wc -l | tail -1

# 前端注释率（仅 src 目录）
find web/src -name '*.vue' -o -name '*.ts' -o -name '*.js' | xargs grep -Eh '^\s*(//|/\*|\*)' | wc -l
find web/src -name '*.vue' -o -name '*.ts' -o -name '*.js' | xargs wc -l | tail -1
```

### 最小修改原则（强制）

- **只改必须改的**：每个变更仅针对当前需求，禁止顺手重构、顺带优化、引入未被要求的新功能
- **不扩大范围**：用户要求修复 Bug → 不要同时改接口签名 / 改数据结构 / 新增中间件；用户要求加字段 → 不要同时调整整个响应结构
- **不过度设计**：不要为"未来可能用到"预留抽象层、接口、配置项；当前能用就好，YAGNI（You Aren't Gonna Need It）
- **优先局部修改**：改一个函数就改这个函数，不要推翻整个文件重写；改一个组件就改这个组件，不要重构整条链路
- **新增文件需理由**：只有当现有文件职责确实不适合承载时才新建文件，禁止"为了代码好看"而拆分
- **变更前先评估影响面**：修改 service 层方法签名前，先搜一遍哪些 Handler 调用了它；修改 API 响应结构前，先确认前端哪些页面依赖了这个字段

### 测试覆盖要求（强制）

**任何代码修改提交前，必须完成以下三类测试：**

#### 1. 单元测试（Unit Test）

- **Go**：`go test ./...` 覆盖所有受影响的包，关键逻辑测试覆盖率 ≥ 80%
  - LDAP 操作（Connect/Bind/Search/Add/Modify/Delete）必须 mock `*ldap.Conn`
  - DAO 层必须用 SQLite 内存数据库（`:memory:`）隔离测试
  - Handler 层必须 mock `service.LdapService`，仅测试参数校验和响应格式化
- **前端**：Vue 组件核心逻辑、工具函数、API 封装必须有 vitest 单测

```bash
# 运行 Go 全部测试
go test ./... -v -count=1

# 仅跑变更相关包的测试（更快）
go test ./internal/service/... ./internal/api/... -v
```

#### 2. 功能测试（Functional Test）

- **构建验证**：`make build` 必须通过，无编译错误、无 embed 缺失
- **接口验证**：启动服务后，用 curl 或前端页面实际调用受影响的 API
- **LDAP 端到端**：涉及 LDAP 操作的修改，必须连真实 LDAP 服务器做一轮完整操作（Connect → Bind → Search → Modify → Delete）
- **回归检查**：至少验证登录、目录树加载、条目查看这三条核心路径未被破坏

```bash
# 典型功能验证流程
make build && make run &
sleep 2
curl -s -X POST http://localhost:8080/api/auth/login -d '{"username":"admin","password":"xxx"}'
curl -s http://localhost:8080/api/health
kill %1
```

#### 3. 安全测试（Security Test）

- **SQL 注入**：所有 DAO 查询确认使用参数化（`?` 占位符），字符串拼接的 SQL 必须审查
- **LDAP 注入**：Search Filter 中用户输入的特殊字符（`*`、`(`、`)`、`\`、`\0`）必须正确转义
- **XSS**：前端渲染 LDAP 属性值时使用 `v-text` 而非 `v-html`；后端返回数据中的 `<script>` 标签需转义
- **鉴权绕过**：受影响的接口确认仍走 `middleware.Auth()` 中间件，无裸奔端点
- **敏感数据泄露**：错误信息中不得包含完整 DN、密码、Bind 凭据；日志中脱敏处理
- **JWT**：Token 过期、伪造、篡改场景的行为符合预期

**测试结论必须在提交说明中明确写出**：哪些测试跑过了、覆盖了哪些场景、有没有发现问题。

### Go 后端

- **错误处理**：禁止 `_ = func()`，必须向上返回或记录
- **上下文传递**：Handler 层用 `*gin.Context`，Service 层用 `context.Context`
- **日志**：使用 `zap.L().Info()/Warn()/Error()`，携带关键上下文
- **密码安全**：LDAP Bind 密码存储时加密，传输时 SecureString
- **goroutine**：禁止裸起，必须有 context 或 WaitGroup 控制生命周期

### Vue 前端

- **组件命名**：大驼峰 `EntryView.vue`，文件名与组件名一致
- **API 调用**：必须抽离到 `web/src/api/`，禁止在组件中直接写 axios
- **请求拦截**：统一在 `request.ts` 中处理 Token 注入、错误提示
- **状态管理**：全局状态用 Pinia（如需），组件级用 ref/reactive

## LDAP 协议速查

- **DN (Distinguished Name)**：条目标识，如 `uid=admin,dc=example,dc=com`
- **RDN (Relative DN)**：DN 的最末级，如 `uid=admin`
- **ObjectClass**：条目类型定义，决定可用属性
- **Attribute**：条目属性，如 cn、sn、uid、mail
- **Filter**：搜索过滤表达式，如 `(&(objectClass=person)(cn=*test*))`
- **Scope**：搜索范围：BaseObject / SingleLevel / WholeSubtree
- **LDIF**：LDAP Data Interchange Format，用于导入导出

## 常见 LDAP 错误码

| Code | 含义 | 处理建议 |
|------|------|----------|
| 49 | Invalid Credentials | Bind 账号密码错误 |
| 32 | No Such Object | DN 不存在 |
| 68 | Already Exists | 条目已存在 |
| 20 | Invalid Attribute Syntax | 属性值格式错误 |
| 12 | Critical Extension Unavailable | 服务器不支持该扩展 |
| 11 | Administrative Limit Exceeded | 搜索结果超限 |

## 调试技巧

### LDAP 连接测试

```bash
# 使用 ldapsearch 验证服务器连通性
ldapsearch -x -H ldap://server:389 -D "uid=admin,dc=example,dc=com" -W -b "dc=example,dc=com"
```

### SQLite 数据检查

```bash
sqlite3 ./data/goldapadmin.db ".tables"
sqlite3 ./data/goldapadmin.db "SELECT * FROM servers;"
```

### 日志排查

后端日志默认输出到 stdout，级别由 `config.yaml` 的 `log.level` 控制：
- `debug` — 开发调试，打印详细 LDAP 操作
- `info` — 正常运行
- `warn` — 警告信息
- `error` — 错误信息

## 安全注意事项

1. **JWT Secret**：生产环境必须修改 `config.yaml` 中的 `jwt.secret`
2. **LDAP 密码**：数据库中存储时加密，API 返回时脱敏
3. **SQL 注入**：所有 DB 查询使用参数化（GORM/Prepared Statement）
4. **XSS 防护**：LDAP 返回的属性值在前端渲染时转义
5. **日志脱敏**：禁止在日志中打印完整密码、Token

## 任务路由指南

| 任务类型 | 关注点 |
|----------|--------|
| 新增 API 功能 | service 层 LDAP 操作 → api 层 Handler → router 注册 → 前端调用 |
| LDAP Bug 排查 | 先验证连接 → 再看 Bind → 检查 Filter → 确认 Scope |
| 前端页面开发 | 先查现有 views 复用 → api 封装 → 组件实现 → 路由配置 |
| 构建失败 | 检查 `internal/web/dist/` 是否存在且最新 → 重新 `make build` |
| SQLite 问题 | 检查 `data/goldapadmin.db` 权限 → 表结构是否匹配 model |
| 性能优化 | LDAP 连接复用 → 搜索分页 → 结果集限制 |
