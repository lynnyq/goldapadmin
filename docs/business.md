# GoLDAPAdmin 业务说明文档

> 一款替代 phpLDAPadmin 的现代化 LDAP 目录管理工具。基于 Go + Vue3，单二进制部署，零外部运行时依赖。

---

## 一、产品定位

### 解决的问题

| 痛点 | 现状 | GoLDAPAdmin 方案 |
|---|---|---|
| **部署繁琐** | phpLDAPadmin 需要 Apache + PHP + Composer 全套环境 | 单二进制 + 内嵌前端，**一个文件启动** |
| **安全风险** | PHP 老旧、漏洞多、需定期 patch、CGI 注入风险 | Go 静态编译、内存安全、依赖精简、JWT 鉴权 |
| **性能差** | PHP 每请求重启动态解释、连接开销大 | Go 原生并发、单进程常驻、QPS 高出 10-100x |
| **UI 落后** | jQuery + PHP 模板渲染、交互生硬、无 SPA 体验 | Vue3 + Element Plus、SPA 响应式、三栏经典布局 |
| **监控缺失** | 无审计、无统计、操作黑盒、出问题无法追溯 | 操作日志全记录、监控面板、全链路 TraceID |
| **多服务器不便** | phpLDAPadmin 配置多台 LDAP 麻烦、需频繁切换配置文件 | 内置 SQLite 持久化管理多台、顶部栏快速切换 |
| **LDAP 密码存储** | phpLDAPadmin 可能将密码存 cookie / session | GoLDAPAdmin **完全不持久化密码**，前端内存态、每次请求传入 |

### 目标用户画像

| 用户 | 核心诉求 | 使用场景 |
|---|---|---|
| **运维工程师** | 日常 LDAP 目录维护 | 增删改查用户/组/OU、批量导入导出、密码重置 |
| **系统管理员** | 多 LDAP 服务器集中管理 | 切换 OpenLDAP / AD、统一监控、审计追溯 |
| **开发工程师** | Schema 查阅、LDAP 调试 | 浏览 objectClass/attributeType、构造 LDAP 搜索过滤器 |
| **安全审计** | 操作追溯、合规审计 | 查询操作日志、追踪敏感操作（删除/密码修改） |
| **DevOps** | 部署简单、运维友好 | 单文件二进制、无外部依赖、systemd 管理 |

### 与 phpLDAPadmin 功能对照

| phpLDAPadmin 功能 | GoLDAPAdmin 实现 | 实现位置 | 状态 |
|---|---|---|---|
| 目录树浏览 | ✅ el-tree 懒加载 + 图标区分 | `views/layout/ServerTree.vue` | 已实现 |
| 条目属性编辑 | ✅ 行内表格 + 弹窗编辑 | `views/entry/EntryView.vue` | 已实现 |
| 新建条目（模板） | ✅ inetOrgPerson/groupOfNames/OU/自定义 | `components/entry/CreateEntryDialog.vue` | 已实现 |
| 复制条目 | ✅ 支持递归 | `EntryService.CopyEntry` | 已实现 |
| 移动条目 | ✅ ModifyDN + newParentDN | `EntryService.MoveEntry` | 已实现 |
| 重命名条目 | ✅ ModifyDN | `EntryService.RenameEntry` | 已实现 |
| 删除（级联） | ✅ recursive=true | `EntryService.DeleteEntry` | 已实现 |
| LDAP 搜索 | ✅ 过滤器编辑器 | `views/search/AdvancedSearch.vue` | 已实现 |
| LDIF 导入 | ✅ 批量解析 + 逐条执行 + 结果汇总 | `LdifService.Import` | 已实现 |
| LDIF 导出 | ✅ 递归导出 + Content-Disposition 下载 | `LdifService.Export` | 已实现 |
| Schema 浏览 | ✅ objectClass + attributeType 双 Tab | `views/schema/SchemaBrowser.vue` | 已实现 |
| Schema 参考面板 | ✅ MUST/MAY 属性 + 可添加 | `views/layout/SidePanel.vue` | 已实现 |
| 用户密码修改 | ✅ SSHA/SHA/MD5 预哈希 | `HashPassword` | 已实现 |
| 多服务器管理 | ✅ SQLite 持久化 + 顶部栏切换 | `views/servers/ServerManager.vue` | 已实现 |
| 服务器连接测试 | ✅ 独立测试接口 | `TestConnection` | 已实现 |
| 服务器监控 | ✅ 连接状态/操作统计/响应时间序列 | `views/monitor/ServerMonitor.vue` | 已实现 |
| 操作日志审计 | ✅ 分页查询 + 操作类型标签 | `views/logs/OperationLogs.vue` | 已实现 |
| 书签收藏 | ✅ 收藏常用 DN、快速跳转 | `views/bookmarks/BookmarkList.vue` | 已实现 |
| SASL 认证 | Kerberos/GSSAPI | — | 规划中 |
| 目录树右键菜单 | 新建/复制/移动/删除/书签 | — | 规划中 |
| 多语言 | zh-CN / en-US | — | 规划中 |
| 暗黑模式 | 主题切换 | — | 规划中 |

---

## 二、核心业务流程

### 2.1 用户登录

```
用户打开浏览器 → http://your-server:8080
       │
       ▼
┌──────────────────────────────────┐
│  登录页 Login.vue                │
│                                  │
│  ○ 选择已保存的服务器             │
│  ● 手动输入                       │
│                                  │
│  Host:     ldap.example.com      │
│  Port:     389                   │
│  Base DN:  dc=example,dc=com     │
│  Bind DN:  cn=admin,dc=...       │
│  密码:     ●●●●●●●●              │
│  ☐ Use TLS  ☐ Use SSL            │
│  ☑ 记住此服务器                  │
│                                  │
│  [登录]                           │
└──────────────────────────────────┘
       │
       ▼
后端用 BindDN + 密码 对目标 LDAP 做 Bind 验证
       │
       ├── Bind 失败 → 显示 "LDAP 认证失败: invalid credentials"
       │
       └── Bind 成功
              │
              ▼
       签发 JWT（含 server_id + user_dn）
       如勾选"记住" → 保存服务器配置到 SQLite
              │
              ▼
       返回 { token, server, user_dn }
              │
              ▼
       前端: token 存 Pinia + localStorage
             密码存内存态（Pinia store）
              │
              ▼
       跳转主布局 MainLayout.vue
```

### 2.2 目录浏览（三栏经典布局）

```
┌──────────────────────────────────────────────────────────────────┐
│ 顶部栏                                                            │
│  [GoLDAPAdmin]  │ 服务器下拉 [ldap.example.com ▾]  │ 🔍  🔔 ⚙️  👤 │
├──────────┬───────────────────────────────┬──────────────────────┤
│ 左栏     │ 中栏                            │ 右栏                 │
│ ServerTree│ EntryView                     │ SidePanel            │
│ (280px)  │ 自适应                          │ (320px, 可收起)      │
│          │                                │                      │
│ 🗄️ 服务器 │ 面包屑: dc=example ▸ dc=com ▸ ou=users │ Schema 参考    │
│   ▼ dc   │ ┌───────────────────────────┐  │ - objectClass 列表   │
│   ▼ ou   │ │ [+ 新建] [📋 复制] [➡ 移动]│  │ - MUST 属性          │
│    ▶ cn  │ │ [✏️ 重命名] [🗑 删除] [📥导出]│  │ - MAY 属性 (可添加)  │
│   📁 📁  │ ├───────────────────────────┤  │                      │
│   👤 👥  │ │ 属性表格 (el-table)         │  │                      │
│   🏢 🌐  │ │ ┌──────┬────────┬──────┐  │  │ 操作历史时间线       │
│          │ │ │ cn   │ zhang  │ ✏️   │  │  │ ────────────       │
│ 懒加载    │ │ │ sn   │ san    │ ✏️   │  │  │ ✅ 10:30 create user│
│ 点击展开 │ │ │ mail │ z@xx.c │ ✏️   │  │  │ ✅ 10:28 modify grp │
│ 树内搜索  │ │ │ ✨密码│ •••••• │ 🔑   │  │  │ ❌ 10:25 delete ou │
│          │ │ └──────┴────────┴──────┘  │  │ ✅ 10:20 login      │
├──────────┴───────────────────────────────┴──────────────────────┤
│ 状态栏: 🟢 已连接 | user: cn=admin,dc=... | 当前 DN: ou=users,...  │
└──────────────────────────────────────────────────────────────────┘
```

**左栏 ServerTree.vue — 目录树**：

| 功能 | 实现 | 说明 |
|---|---|---|
| 懒加载 | `@node-expand` → `GET /api/entry/children?base_dn=` | 只加载当前展开层，减少请求 |
| 图标区分 | 根据 `objectClass` 决定 | 👤 person / 👥 group / 🏢 OU / 📁 组织 / 🌐 域 |
| 树内搜索 | `el-input` + `filter-node-method` | 纯前端，不发请求，实时过滤 |
| 多服务器 | 顶部栏服务器切换 → 重置 store → 刷新树 | 切换即生效 |
| 默认根节点 | 登录时的 Base DN | 直接展开 Base DN 子树 |

**中栏 EntryView.vue — 条目详情**：

| 功能 | 实现 | 说明 |
|---|---|---|
| 面包屑 | DN 解析 + `getRDNValue` + 可点击跳转 | 每层都可点击，回到任意层级 |
| 工具栏 | 6 个按钮：新建 / 复制 / 移动 / 重命名 / 删除 / 导出 / 导入 | 每个操作有独立弹窗确认 |
| 属性表格 | `el-table` + 行内值 + 右侧编辑按钮 | 多值属性换行显示；密码属性用 🔑 图标 |
| 编辑弹窗 | `el-dialog` + 动态表单 | 编辑完调 `POST /api/entry/modify` |
| 新建弹窗 | `CreateEntryDialog.vue` | 见 2.3 节 |

**右栏 SidePanel.vue — Schema + 操作历史**：

| 功能 | 实现 | 说明 |
|---|---|---|
| Schema 参考 | 选中条目 → 取 objectClass → `POST /api/schema/allowed-attributes` | 实时联动，显示 MUST 和 MAY |
| MAY 属性 | 可点击"添加到条目"按钮 | 直接触发 EntryView 编辑 |
| 操作历史 | `GET /api/logs` + 时间线 UI | 最新 10 条，显示操作类型 + 状态 + 耗时 |

### 2.3 新建条目

```
中栏工具栏 [+ 新建] 按钮
       │
       ▼
┌────────────────────────────────────────┐
│  CreateEntryDialog.vue                 │
│                                        │
│  父 DN:  ou=users,dc=example,dc=com   │
│                                        │
│  选择模板:                              │
│  ┌───────────────┐                     │
│  │ ○ 用户         │ inetOrgPerson      │
│  │ ○ 组          │ groupOfNames        │
│  │ ○ 组织单位     │ organizationalUnit  │
│  │ ○ 自定义       │ 手动选 objectClass  │
│  └───────────────┘                     │
│                                        │
│  系统自动加载 Schema:                   │
│  ┌────────────────────────────────┐   │
│  │ MUST 属性 (必填)                │   │
│  │ cn: [____________] *            │   │
│  │ sn: [____________] *            │   │
│  ├────────────────────────────────┤   │
│  │ MAY 属性 (可选, 点击添加)       │   │
│  │ [+] mail                        │   │
│  │ [+] telephoneNumber             │   │
│  │ [+] uid                         │   │
│  └────────────────────────────────┘   │
│                                        │
│  ✅ 生成 DN: cn=zhangsan,ou=users,... │
│                                        │
│  [取消]  [创建]                         │
└────────────────────────────────────────┘
       │
       ▼
POST /api/entry/create
{
  "dn": "cn=zhangsan,ou=users,dc=example,dc=com",
  "attributes": {
    "objectClass": ["top", "person", "inetOrgPerson"],
    "cn": ["zhangsan"],
    "sn": ["zhang"],
    "mail": ["zhangsan@example.com"]
  }
}
       │
       ▼
Go → EntryService.CreateEntry → LDAP Add
       │
       ▼
✅ 成功: 刷新目录树 + 中栏自动加载新条目
❌ 失败: 弹窗提示错误 + 操作日志记录
```

### 2.4 高级搜索

```
┌──────────────────────────────────────────┐
│  AdvancedSearch.vue                      │
│                                          │
│  🔍 搜索条件:                             │
│                                          │
│  Base DN:  [dc=example,dc=com ▾]         │
│  过滤器:   (&(objectClass=inetOrgPerson) │
│           (|(cn=*zhang*)(sn=*zhang*)))   │
│  范围:     ○ 基础  ○ 单层  ● 整个子树     │
│  大小限制: [100]                          │
│                                          │
│  [🔍 开始搜索]  [🔧 可视化编辑器]         │
│                                          │
│  结果表格:                                │
│  ┌────┬──────────────────────────┬──────┐│
│  │ ☐  │ DN                        │ 对象 ││
│  ├────┼──────────────────────────┼──────┤│
│  │ ☐  │ cn=zhangsan,ou=users,...  │ 👤   ││
│  │ ☐  │ cn=zhangwei,ou=users,...  │ 👤   ││
│  │ ☐  │ cn=zhangli,ou=devs,...    │ 👤   ││
│  └────┴──────────────────────────┴──────┘│
│                                          │
│  批量操作: [🗑 删除选中] [📥 导出选中]     │
└──────────────────────────────────────────┘
       │
       ▼
GET /api/search
  ?base_dn=dc=example,dc=com
  &filter=(&(objectClass=inetOrgPerson)(cn=*zhang*))
  &scope=sub
  &size_limit=100
       │
       ▼
Go → EntryService.Search → LDAP Search
       │
       ▼
返回条目列表 → 结果表格展示
```

### 2.5 LDIF 批量导入

```
┌──────────────────────────────────────┐
│  LdifImport.vue                      │
│                                      │
│  方式一: 拖拽上传文件                  │
│  ┌──────────────────────────────┐   │
│  │   📁 拖拽 .ldif 文件到这里    │   │
│  └──────────────────────────────┘   │
│                                      │
│  方式二: 直接粘贴内容                  │
│  ┌──────────────────────────────┐   │
│  │ dn: cn=zhangsan,ou=users,... │   │
│  │ objectClass: inetOrgPerson   │   │
│  │ cn: zhangsan                 │   │
│  │ sn: zhang                    │   │
│  │                              │   │
│  │ dn: cn=zhangwei,ou=users,... │   │
│  │ ...                          │   │
│  └──────────────────────────────┘   │
│                                      │
│  [执行导入]                            │
│                                      │
│  执行结果:                             │
│  ┌──────────────────────────────┐   │
│  │ ✅ 成功: 48                   │   │
│  │ ❌ 失败:  2                   │   │
│  │                              │   │
│  │ 失败详情:                     │   │
│  │ - add cn=exists,...:          │   │
│  │   Already exists              │   │
│  │ - add cn=invalid,...:         │   │
│  │   Missing attribute 'sn'      │   │
│  └──────────────────────────────┘   │
└──────────────────────────────────────┘
```

### 2.6 密码修改

```
中栏属性表格 → 点击 userPassword 属性的 ✏️ 按钮
       │
       ▼
┌──────────────────────────────────┐
│  修改密码弹窗                     │
│                                  │
│  DN: cn=zhangsan,ou=users,...   │
│                                  │
│  新密码:     [••••••••]           │
│  确认密码:   [••••••••]           │
│                                  │
│  哈希算法:                         │
│  ┌───────────────────────┐       │
│  │ ● {SSHA} (推荐)        │       │
│  │ ○ {SHA}                │       │
│  │ ○ {MD5}                │       │
│  │ ○ 明文 (LDAP 服务器处理)│       │
│  └───────────────────────┘       │
│                                  │
│  [取消]  [确认修改]                │
└──────────────────────────────────┘
       │
       ▼
前端根据选择的算法调 HashPassword() 预哈希
       │
       ▼
POST /api/entry/modify
{
  "dn": "cn=zhangsan,...",
  "changes": [{
    "op": "replace",
    "attr": "userPassword",
    "values": ["{SSHA}base64(sha1(pwd+salt)+salt)"]
  }]
}
       │
       ▼
Go → EntryService.ModifyEntry → LDAP ModifyRequest
       │
       ▼
✅ 成功: 刷新属性表格 + 日志记录
❌ 失败: 显示 LDAP 错误 (如 password policy violation)
```

### 2.7 服务器监控面板

```
┌────────────────────────────────────────────────────────────┐
│  ServerMonitor.vue                                         │
│                                                            │
│  ┌──────────────┐ ┌──────────────┐ ┌────────────┐ ┌───────┐│
│  │  连接状态     │ │  活跃连接     │ │  总操作     │ │平均响应││
│  │  🟢 已连接    │ │     1        │ │   1,234    │ │  12ms ││
│  │  last check:  │ │              │ │            │ │       ││
│  │  2 秒前       │ │              │ │            │ │       ││
│  └──────────────┘ └──────────────┘ └────────────┘ └───────┘│
│                                                            │
│  ┌────── 操作统计 ──────────────────────────────────────┐ │
│  │ success: 1,200 | failed: 34 | uptime: 2h 35m         │ │
│  └─────────────────────────────────────────────────────┘ │
│                                                            │
│  ┌── 操作类型分布 (饼图) ──┐  ┌── 响应时间趋势 (折线图) ──┐│
│  │    🥧 ECharts            │  │    📈 ECharts              ││
│  │    list 80% (987)       │  │    ms:                     ││
│  │    modify 12% (148)     │  │    15 ────────              ││
│  │    create 5% (62)       │  │    10 ────────              ││
│  │    delete 3% (37)       │  │     8 ──── 7 ────          ││
│  └─────────────────────────┘  └───────────────────────────┘│
└────────────────────────────────────────────────────────────┘
```

### 2.8 Schema 浏览

```
┌────────────────────────────────────────┐
│  SchemaBrowser.vue                     │
│                                        │
│  [ objectClass ]  [ attributeType ]    │
│                                        │
│  🔍 搜索: [inetOrgPerson____________]  │
│                                        │
│  ┌────────────────────────────────┐   │
│  │ OC: inetOrgPerson              │   │
│  │ OID: 2.16.840.1.113730.3.2.2  │   │
│  │ Type: structural               │   │
│  │ Superior: organizationalPerson │   │
│  │                                │   │
│  │ MUST: [cn] [sn]                │   │
│  │ MAY:  [audio] [businessCat...] │   │
│  │       [carLicense] [deptNum..] │   │
│  │       [departmentNumber] ...   │   │
│  └────────────────────────────────┘   │
│                                        │
│  ⚠️ 某些 AD 不暴露 schema 子树         │
│     → 自动降级使用内置默认列表           │
└────────────────────────────────────────┘
```

---

## 三、页面清单与导航

### 路由表

| 路径 | 页面组件 | 布局 | 说明 |
|---|---|---|---|
| `/login` | `Login.vue` | 独立登录页 | 服务器选择/手动输入/TLS 配置 |
| `/entry` | `MainLayout.vue` + `EntryView.vue` | 三栏主布局 | **默认首页**，目录浏览 |
| `/servers` | `ServerManager.vue` | 三栏主布局 | 服务器配置 CRUD |
| `/search` | `AdvancedSearch.vue` | 三栏主布局 | 高级搜索 + 批量操作 |
| `/schema` | `SchemaBrowser.vue` | 三栏主布局 | Schema 浏览 |
| `/bookmarks` | `BookmarkList.vue` | 三栏主布局 | 书签管理 |
| `/monitor` | `ServerMonitor.vue` | 三栏主布局 | 监控面板 |
| `/import` | `LdifImport.vue` | 三栏主布局 | LDIF 批量导入 |
| `/logs` | `OperationLogs.vue` | 三栏主布局 | 操作审计日志 |

### 顶部栏导航

```
┌────────────────────────────────────────────────────────────────┐
│ [GoLDAPAdmin] │ 📌 [ldap.example.com ▾] │ 🔍 │ ⭐ │ 📊 │ ⚙️ │ 👤 │
│               │ 服务器下拉                  │搜索│书签│监控│设置│用户│
└────────────────────────────────────────────────────────────────┘
```

| 元素 | 功能 |
|---|---|
| Logo | 点击回到首页 `/entry` |
| 服务器下拉 | 切换已保存的 LDAP 服务器 → 重置 store → 刷新目录树 |
| 搜索 | 快速跳转 `/search` |
| 书签 | 跳转 `/bookmarks` |
| 监控 | 跳转 `/monitor` |
| 设置 | 跳转 `/servers` |
| 用户 | 显示 user_dn + 退出登录 |

### 页面组件总览

| 模块 | 页面组件 | 公共组件 | Store |
|---|---|---|---|
| 登录 | `Login.vue` | — | `auth` |
| 主布局 | `MainLayout.vue`、`ServerTree.vue`、`SidePanel.vue` | — | `entry`、`server` |
| 条目管理 | `EntryView.vue` | `CreateEntryDialog.vue` | `entry` |
| 服务器 | `ServerManager.vue` | — | `server` |
| 搜索 | `AdvancedSearch.vue` | — | — |
| Schema | `SchemaBrowser.vue` | — | — |
| 书签 | `BookmarkList.vue` | — | — |
| 监控 | `ServerMonitor.vue` | — | — |
| 导入 | `LdifImport.vue` | — | — |
| 日志 | `OperationLogs.vue` | — | — |

---

## 四、业务规则

| # | 规则 | 说明 |
|---|---|---|
| 1 | **密码不持久化** | SQLite 只存服务器连接配置（host/port/base_dn），**绝不存** BindDN 密码。密码每次请求从前端 header 传入 |
| 2 | **密码前端内存态** | 密码存 Pinia store（内存），关闭浏览器即丢失；**不写 localStorage** |
| 3 | **单 LDAP 服务器绑定** | 一次登录绑定一个 server_id，后续所有 API 操作基于该服务器；顶部栏切换服务器会重置整个 store |
| 4 | **JWT 只存 ID 不存密码** | `Claims{server_id, user_dn}` — JWT payload 可读，不能存敏感信息 |
| 5 | **级联删除需显式 recursive=true** | 删除非叶子节点时弹窗警告，用户勾选"级联"才会递归删除子条目 |
| 6 | **删除幂等** | 递归删除遇到已删除的子条目不会中断（LDAP 返回 NO_SUCH_OBJECT 会被捕获） |
| 7 | **密码哈希由客户端决定** | 前端可选 SSHA/SHA/MD5 预哈希，或明文让 LDAP 服务器按自身密码策略处理 |
| 8 | **Schema 自动降级** | AD 等有些服务器不暴露 schema 子树（cn=schema,cn=config 或 cn=subschema），系统自动使用内置默认列表 |
| 9 | **SQLite 单连接串行化** | `SetMaxOpenConns(1)` 保证并发安全，避免 SQLite 写锁冲突 |
| 10 | **CORS 开发全开** | 开发方便。生产反向代理到同域即可避免跨域问题 |
| 11 | **TraceID 全链路** | 每个请求自动生成唯一 TraceID（可前端传入），贯穿日志和响应，方便问题排查 |
| 12 | **操作失败也记录日志** | 任何 LDAP 操作（成功或失败）都会写入 operation_logs，便于审计 |
| 13 | **批量导入逐条执行** | LDIF Import 不是一个事务，而是逐条 AddRequest。一条失败不影响其他条目，最终返回 success/failed/errors 汇总 |
| 14 | **目录树懒加载** | 初始只加载登录时 Base DN 的直接子节点。展开某节点时才请求其子节点 |
| 15 | **SPA 回退机制** | 所有非 `/api/` 的 HTTP 404 请求返回 index.html，前端 Vue Router 处理路由 |

---

## 五、部署架构

### 场景一：单节点（最简部署）

```
┌──────────────┐
│   浏览器      │
│   Chrome     │
└──────┬───────┘
       │ HTTP :8080
       ▼
┌─────────────────────┐
│  GoLDAPAdmin        │
│  单二进制: :8080     │
│  ┌───────────────┐  │
│  │ SQLite        │  │
│  │ ./data/db     │  │
│  └───────┬───────┘  │
└──────────┼──────────┘
           │ TCP :389 / :636
           ▼
┌─────────────────────┐
│  LDAP Server         │
│  OpenLDAP / AD       │
└─────────────────────┘
```

**适用**：开发环境、小型团队、测试验证

### 场景二：Nginx 反向代理（生产推荐）

```
┌──────────────┐     HTTPS :443     ┌──────────────┐
│   浏览器      │────────────────────►│    Nginx      │
│   Chrome     │                     │              │
└──────────────┘                     │ ┌──────────┐ │
                                     │ │ SSL 终止  │ │
                                     │ │ 静态缓存  │ │
                                     │ │ Gzip 压缩 │ │
                                     │ └────┬─────┘ │
                                     └──────┼───────┘
                                            │ HTTP :8080
                                            ▼
                                     ┌─────────────────────┐
                                     │  GoLDAPAdmin        │
                                     └────────┬────────────┘
                                              │ TCP :389 / :636
                                              ▼
                                     ┌─────────────────────┐
                                     │  LDAP Server         │
                                     └─────────────────────┘
```

**Nginx 配置示例**：

```nginx
server {
    listen 443 ssl;
    server_name ldap-admin.example.com;

    ssl_certificate     /etc/nginx/ssl/cert.pem;
    ssl_certificate_key /etc/nginx/ssl/key.pem;

    # 全量反代到 GoLDAPAdmin
    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;

        # 静态资源缓存
        location ~* \.(js|css|svg|png|jpg|ico)$ {
            proxy_pass http://127.0.0.1:8080;
            expires 7d;
            add_header Cache-Control "public, immutable";
        }
    }
}
```

### 场景三：Docker Compose 测试环境

```yaml
# docker-compose.yml
services:
  # GoLDAPAdmin
  app:
    build: .
    ports:
      - "8080:8080"
    volumes:
      - ./data:/app/data
    depends_on:
      - openldap

  # OpenLDAP 测试服务器
  openldap:
    image: osixia/openldap:1.5.0
    environment:
      LDAP_ORGANISATION: Example
      LDAP_DOMAIN: example.com
      LDAP_ADMIN_PASSWORD: admin123
    ports:
      - "389:389"
    volumes:
      - ldap-data:/var/lib/ldap

volumes:
  ldap-data:
```

```bash
docker compose up -d
# 打开 http://localhost:8080
# 登录: Bind DN=cn=admin,dc=example,dc=com, Password=admin123
```

---

## 六、快速上手

### 1. 构建

```bash
git clone github.com/lynnyq/goldapadmin
cd goldapadmin
make build
```

### 2. 启动

```bash
# 方式一：使用默认 config/config.yaml
./goldapadmin

# 方式二：指定配置文件路径
./goldapadmin /opt/goldapadmin/config/config.yaml
```

### 3. 访问

```
浏览器打开 → http://localhost:8080

登录页填写:
  ┌─────────────────────────────────────────┐
  │ 服务器: [留空，手动输入]                  │
  │ Host:   ldap.example.com               │
  │ Port:   389                            │
  │ Base DN: dc=example,dc=com             │
  │ Bind DN: cn=admin,dc=example,dc=com    │
  │ 密码:    admin123                       │
  │ ☐ Use TLS                               │
  │ ☑ 记住此服务器                          │
  └─────────────────────────────────────────┘
  [登录]

→ 进入主界面 → 左栏展开目录树 → 点击条目查看/编辑
```

### 4. 开发模式

```bash
# 后端开发（API）
make build-go          # 仅编译 Go（跳过前端，开发时快编）
./goldapadmin          # 启动后端 :8080

# 前端热更新
cd web
npm install            # 安装依赖（首次）
npm run dev            # Vite dev server :5173
                       # 已配置 proxy /api → localhost:8080

# 完整开发链路
# 前端改代码 → HMR 自动刷新
# 后端改代码 → make build-go && make run
```

### 5. 配置详解

```yaml
# config/config.yaml
server:
  port: 8080          # HTTP 监听端口
  mode: release       # Gin 运行模式: debug/release/test

database:
  path: ./data/goldapadmin.db   # SQLite 文件位置（自动创建目录）

jwt:
  secret: goldapadmin-secret-key-change-me  # ⚠️ 生产务必修改！
  ttl: 24                                     # Token 有效期（小时）

log:
  level: info                                  # debug/info/warn/error
```

---

## 七、常见问题 FAQ

### Q1: 为什么登录后左栏是空的？

**A**: 检查以下几点：

1. 登录时的 **Base DN** 是否正确 — 左栏只展开 Base DN 及其子树
2. 登录时 Bind DN 是否有搜索该子树的权限
3. 打开浏览器 DevTools → Network → 找到 `GET /api/entry/children` 请求，检查返回值

如果 Bind DN 只有特定 OU 的权限（如只能看 `ou=users,dc=example,dc=com`），Base DN 应该设为这个具体值。

### Q2: TLS 和 SSL 怎么选？

**A**:

| 方案 | 端口 | 说明 |
|---|---|---|
| **SSL (ldaps://)** | 636 | 整条链路加密，直接 TLS 握手 |
| **TLS (StartTLS)** | 389 | 先明文握手，收到 "Start TLS" 指令后再升级加密 |
| **明文** | 389 | 开发调试用，生产**绝对不要** |

- Active Directory 通常要求 **TLS (StartTLS)**，端口 389
- OpenLDAP 两者都支持
- 勾选"Use SSL"时端口自动用 636；勾选"Use TLS"时端口仍为 389

### Q3: 为什么 AD 搜索很慢？

**A**: AD 默认 `(objectClass=*)` + `scope=sub` 范围巨大，可能返回数万条目。建议：

1. 搜索时加**过滤器缩小范围**：
   ```
   (&(|(cn=*zhang*)(sn=*zhang*))(objectClass=inetOrgPerson))
   ```
2. 设置 **size_limit**（如 100）
3. 如果只需要某个 OU，把 **base_dn** 设到具体 OU

### Q4: 密码修改不生效？

**A**: 多数 LDAP 服务器对密码有特殊处理：

- **AD**：密码策略（长度、复杂度、历史）严格，不合规会返回 `ldap_errConstraintViolation`
- **OpenLDAP**：需要后端配置 `ppolicy` 模块和 `olcPasswordHash`
- GoLDAPAdmin 支持三种预哈希：`{SSHA}`、`{SHA}`、`{MD5}`，也可传明文让 LDAP 按自身策略处理

如果选 SSHA 还是失败，试试**传明文**（不选任何哈希），让 LDAP 服务器自己处理。

### Q5: Schema 浏览显示不出来？

**A**: 不同 LDAP 服务器 schema 子树位置不同：

| 服务器 | Schema DN | GoLDAPAdmin 处理 |
|---|---|---|
| OpenLDAP | `cn=schema,cn=config` | ✅ 自动查找 |
| OpenLDAP (旧版) | `cn=subschema` | ✅ 自动查找 |
| Active Directory | 不暴露 | ⚠️ 自动降级用内置默认列表 |
| 389 Directory | `cn=schema,cn=config` | ✅ |

内置默认列表覆盖 7 个常用 objectClass + 50+ 常见 attributeType，日常使用足够。

### Q6: 支持哪些 LDAP 服务器？

**A**: 基于 `go-ldap/ldap/v3`，协议层面兼容所有 **LDAPv3** 服务器：

| 服务器 | 兼容性 | 备注 |
|---|---|---|
| OpenLDAP | ✅ 完全支持 | 最常用 |
| Active Directory | ✅ 支持 | Schema 需降级，密码策略严格 |
| Apache Directory Server | ✅ 支持 | 经典 Java 实现 |
| 389 Directory Server | ✅ 支持 | Red Hat 出品 |
| FreeIPA / IdM | ✅ 支持 | 基于 389 |
| Domino | ⚠️ 基本支持 | Schema 可能有差异 |

### Q7: 能在 Windows 运行吗？

**A**: 可以。`modernc.org/sqlite` 是**纯 Go 实现，不依赖 CGO**，所以无需 MinGW/MSVC。

```powershell
go build -o goldapadmin.exe ./cmd/server
./goldapadmin.exe .\config\config.yaml
```

### Q8: 数据会丢失吗？

**A**: 分两部分：

| 数据 | 存储位置 | 备份策略 |
|---|---|---|
| LDAP 目录数据 | **LDAP 服务器**（GoLDAPAdmin 不做持久化） | LDAP 自身备份机制 |
| 服务器配置 / 书签 / 操作日志 | `./data/goldapadmin.db`（SQLite） | 定期拷贝这个 .db 文件 |

### Q9: 如何修改默认端口 / 配置？

**A**: 直接编辑 `config/config.yaml` 或启动时传参：

```bash
# 方案一：改 config/config.yaml
server:
  port: 9090

# 方案二：传配置文件路径
./goldapadmin /path/to/my-config.yaml
```

### Q10: 如何重置 / 清空所有配置？

**A**: 删除 SQLite 文件（会丢失服务器配置、书签、操作日志）：

```bash
rm ./data/goldapadmin.db
# 下次启动会自动创建新的空数据库
```

---

## 八、版本历史

| 版本 | 日期 | 内容 |
|---|---|---|
| v0.1.0 | 2026-11-03 | 初始版本：Go 1.26 + Vue3.5 + Element Plus，单二进制，phpLDAPadmin 全核心功能覆盖 |

---

## 九、许可证

MIT License
