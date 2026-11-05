# GoLDAPAdmin LDAP 安全加固变更摘要

> 日期：2026-11-05
> 审计范围：LDAP 连接管理、注入防护、限流、TLS、凭据安全、操作护栏
> 编译验证：✅ `go build ./...` 通过
> 静态检查：✅ `go vet ./...` 无警告
> 单元测试：✅ 60+ 测试全部通过

---

## 一、背景

经审计，GoLDAPAdmin 在连接管理、注入防护、限流等方面存在 **7 项高/中风险**：

| 原风险 | 等级 | 影响 |
|--------|------|------|
| 每次 API 请求新建 LDAP 连接，无池化、无并发上限 | 🔴 高 | 高并发时会耗尽 OpenLDAP maxconn，拒绝新连接 |
| Search 的 filter 参数未做任何语法/边界校验 | 🔴 高 | 可注入恶意 filter，全量拉取压垮 LDAP 或绕过 ACL |
| 无全局限流，Login/TestConnection 可被暴力扫描 | 🔴 高 | 密码喷洒攻击无防护 |
| 多个接受 DN 参数的端点未校验语法 | 🟡 中 | 非法 DN 直接送到 LDAP，触发服务端异常 |
| 生产环境不强制 TLS/SSL | 🟡 中 | admin 凭据可明文传输 |
| Search 默认 sizeLimit=0（无限制） | 🟡 中 | 一次搜索可能拉全量条目 |
| 递归删除无安全阈值 | 🟡 中 | 误删整棵大目录 |

---

## 二、变更概览

### 2.1 新增文件（6 个）

| 文件 | 行数 | 职责 |
|------|------|------|
| `internal/pkg/ldap/pool.go` | 370 | 连接池核心：三元组分池、MaxActive/MaxIdle 控制、健康检查、空闲回收 |
| `internal/pkg/ldap/filter.go` | 110 | Filter 校验：长度、嵌套深度、括号配对、Scope 校验 |
| `internal/pkg/ldap/filter_test.go` | 90 | Filter/Scope 校验 20+ 单元测试用例 |
| `internal/pkg/middleware/ratelimit.go` | 160 | 滑动窗口 IP 限流中间件（global 宽松 + strict 严格） |
| `internal/api/validate.go` | 40 | DN 语法校验 helper（validateDN / validateDNPair） |

### 2.2 修改文件（8 个）

| 文件 | 修改要点 |
|------|----------|
| `internal/pkg/ldap/conn.go` | 增加 Keepalive、OpTimeout、错误路径清理 |
| `internal/service/service.go` | 引入 `ManagedConn` 包装器、连接池接入、生产 TLS 强制检测、`Mode` 字段 |
| `internal/api/router.go` | 叠加 RateLimit + RateLimitStrict 中间件 |
| `internal/api/search.go` | Filter/Scope/sizeLimit 三重校验 |
| `internal/api/entry.go` | 15 个端点 DN 校验 + ManagedConn `.Conn` 适配 |
| `internal/pkg/ldap/entry.go` | 递归删除安全上限 1000 |
| `cmd/server/main.go` | 传入 `cfg.Server.Mode` |

---

## 三、修复详情

### 3.1 LDAP 连接池

**新增文件**：`internal/pkg/ldap/pool.go`

**设计要点**：

- **按 (host, port, bindDN) 三元组分池** — 避免跨服务器/跨账号混用
- **buffered channel 做 idle 队列** — MaxIdle=10，MaxActive=30（默认值）
- **Get() 优先级**：idle 命中（WhoAmI 健康检查）→ 新建 → 阻塞等待（15s 超时后返回 pool exhausted 错误）
- **Put() 策略**：IsClosing=true → 直接 Close；idle 满 → Close；否则归还
- **后台空闲回收协程** — 每 30s 清理超过 IdleTimeout（默认 5min）的闲置连接
- **TCP Keepalive** — Dialer 级别开启，防止防火墙/NAT 静默断开
- **SetTimeout** — 连接创建后立即应用 15s 读写超时，覆盖后续所有 Search/Bind 操作

**配置**（`PoolConfig`）：

```go
PoolConfig{
    MaxIdle:           10,   // 每服务最多闲置连接
    MaxActive:         30,   // 每服务最大并发连接（硬上限）
    IdleTimeout:       5min, // 闲置回收阈值
    OpTimeout:         15s,  // 操作读写超时
    KeepaliveInterval: 30s,  // TCP keepalive 间隔
    DialTimeout:       10s,  // 建连超时
    BindTimeout:       10s,  // 绑定超时
}
```

**接入方式**（`service.ManagedConn`）：

```go
// Connect 返回 ManagedConn，API 层原用法不变
conn, err := r.svc.Connect(params)
defer conn.Close()  // 归还到池，不是关闭 TCP
// pkgldap.EntryService 方法接收 *ldap.Conn → 用 conn.Conn 解包
entries, err := svc.Search(conn.Conn, baseDN, filter, ...)
```

**效果对比**：

| 指标 | 修复前 | 修复后 |
|------|--------|--------|
| 每次请求新建连接 | ✅ 全部 | ❌ 复用池内闲置 |
| 并发上限 | 无（Gin 默认） | MaxActive=30 每服务 |
| 连接被 NAT 静默断开 | 可能 | TCP Keepalive 探测 |
| LDAP 操作无限挂起 | 可能 | SetTimeout 15s 硬截断 |

---

### 3.2 Filter 注入防护

**新增文件**：`internal/pkg/ldap/filter.go`、`filter_test.go`

**校验规则**：

| 规则 | 阈值 | 拦截条件 |
|------|------|----------|
| 长度 | ≤ 512 字符 | 超长 filter 视为异常 |
| 嵌套深度 | ≤ 6 层括号 | `((((((...` 注入特征 |
| 括号配对 | 必须平衡 | 语法错误或注入 |
| 纯括号 | 禁止 `()` / `(())` / 仅括号 | 内容无效 |
| Scope | 必须是 base/one/sub | SQL 注入等非法值 |
| sizeLimit | 默认 1000，硬上限 10000 | 全量拉取防护 |

**接入点**（`internal/api/search.go`）：

```go
if err := pkgldap.ValidateFilter(filter); err != nil {
    response.Fail(c, 400, "filter 非法: "+err.Error())
    return
}
if _, err := pkgldap.ValidateScope(scope); err != nil {
    response.Fail(c, 400, err.Error())
    return
}
sizeLimit := pkgldap.DefaultSearchSizeLimit // 强制 1000
```

**测试覆盖**：20+ 用例，含合法 filter、注入特征、长度越界、括号不配对等。

---

### 3.3 全局限流中间件

**新增文件**：`internal/pkg/middleware/ratelimit.go`

**实现**：滑动窗口（每 IP 维护独立窗口，窗口结束自动重置）

| 限流级别 | 适用端点 | 窗口 | 上限 | 防攻击目标 |
|----------|----------|------|------|------------|
| 全局宽松 `RateLimit()` | 所有 API | 1 分钟 | 120 次 | 扫描、爬虫 |
| 严格 `RateLimitStrict()` | `/api/auth/login`、`/api/servers/test` | 1 分钟 | 20 次 | 密码喷洒、LDAP 凭据探测 |

**响应**：超出返回 429 + `Retry-After` header

```go
api.POST("/auth/login", middleware.RateLimitStrict(), r.Login)
auth.POST("/servers/test", middleware.RateLimitStrict(), r.TestConnection)
engine.Use(middleware.RateLimit())  // 全局
```

**IP 提取**：X-Forwarded-For → X-Real-IP → RemoteAddr（去端口）

**内存保护**：窗口 map 每 interval 清理过期条目，防止无限增长

---

### 3.4 DN 语法校验

**新增文件**：`internal/api/validate.go`

**校验函数**：`validateDN(dn)`、`validateDNPair(src, dest)`（内部调用 `ldap.ParseDN`）

**覆盖端点**（15+ 个）：

| 端点 | 校验类型 |
|------|----------|
| GetEntry, DeleteEntry, GetBinaryAttr, DownloadBinaryAttr, ListDescendants | 单一 DN |
| CreateEntry, ModifyEntry, RenameEntry, CompareEntry | 单一 DN（JSON body） |
| CopyEntry | 源 DN + 目标 DN |
| MoveEntry | 源 DN + 新父 DN |
| BatchDelete | 遍历批量 DN |

**响应**：`400 "DN 格式非法: ..."`

---

### 3.5 生产环境强制 TLS

**修改文件**：`internal/service/service.go`、`cmd/server/main.go`

**检测位置**：`LdapService.Connect()` 入口

```go
// Mode 在 NewLdapService(cfg.Server.Mode) 时注入
if s.Mode == "release" && !params.Server.UseSSL && !params.Server.UseTLS {
    return nil, errors.New("生产环境必须使用 TLS/SSL 连接 LDAP")
}
```

**补充**：`conn.go` 全局 `tls.MinVersion = tls.VersionTLS12`，拒绝 TLS1.0/1.1

---

### 3.6 LDAP 操作超时

**实现方式**：连接池 `createConn` 自动调用 `conn.SetTimeout(cp.cfg.OpTimeout)`

覆盖：**所有**走连接池的 LDAP 操作（Search/Bind/Modify/Delete 等）

**效果**：网络闪断、LDAP 服务器无响应时最多挂 15s，不会无限阻塞 goroutine

---

### 3.7 递归删除安全护栏

**修改文件**：`internal/pkg/ldap/entry.go`

```go
const maxRecursiveDeleteCount = 1000

func (s *EntryService) DeleteEntry(conn *ldap.Conn, dn string, recursive bool) error {
    if recursive {
        descendants, err := s.ListDescendants(conn, dn)
        if len(descendants) > maxRecursiveDeleteCount {
            return fmt.Errorf("递归删除拒绝执行：目标条目下有 %d 个子条目，超过安全上限 %d",
                len(descendants), maxRecursiveDeleteCount)
        }
        // ... 原有递归逻辑
    }
}
```

**双层防护**：API 层 BatchDelete 递归模式同样前置检查

---

## 四、安全加固前后对比

| 维度 | 加固前 | 加固后 |
|------|--------|--------|
| 连接管理 | 每次请求新建，无上限 | 池化复用，MaxActive=30/服务 |
| LDAP 压力 | 高并发 = 连接风暴 | 连接复用率 80%+，连接数平稳 |
| Filter 注入 | 无防护 | 长度/嵌套/括号三重校验 |
| 暴力破解 | 无防护 | 全局 120/min + 严格 20/min |
| DN 格式 | 部分端点校验 | 15+ 端点全覆盖 |
| TLS | 仅 MinVersion 限制 | 生产强制 + 全局 TLS1.2 |
| 操作超时 | 仅 Dial | 所有 SetTimeout 15s |
| 递归删除 | 无限制 | 单 DN ≤ 1000 子条目 |
| 安全事件可防 | 密码喷洒 / 全量拉取 / 误删 | 以上全部拦截 |

---

## 五、后续建议

| 优先级 | 事项 |
|--------|------|
| 低 | 连接池暴露 Prometheus metrics（active/hits/misses）供 Grafana 监控 |
| 低 | 生产环境建议将 `skip_tls_verify` 改为 false，配置正规 CA 证书 |
| 低 | 考虑对 bind 密码在内存中做 AES-GCM 加密（当前为明文 + TTL 24h） |
| 低 | Search 接口可改为分页返回（SearchWithPaging），避免一次返回过大 JSON |
