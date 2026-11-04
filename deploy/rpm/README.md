# GoLDAPAdmin — 生产打包与部署手册（RPM 方案）

> 版本：0.1.0 · 更新：2026-11-04
> 目标平台：RHEL 8 / RHEL 9 / CentOS Stream / Rocky / AlmaLinux / Fedora
> 架构：x86_64（amd64）+ aarch64（arm64，如鲲鹏 / 树莓派 4 / AWS Graviton）
> 运行账户：**ldap**（系统用户，UID 55，GID 55，无登录 shell）
> 部署目录：`/appdata/goldapadmin`

---

## 目录

1. [整体架构](#1-整体架构)
2. [部署目录清单](#2-部署目录清单)
3. [构建 RPM（开发 / CI 侧）](#3-构建-rpm开发--ci-侧)
4. [安装 RPM（生产侧）](#4-安装-rpm生产侧)
5. [配置生产环境](#5-配置生产环境)
6. [systemd 服务详解](#6-systemd-服务详解)
7. [Nginx 反代与 HTTPS](#7-nginx-反代与-https)
8. [升级流程](#8-升级流程)
9. [卸载](#9-卸载)
10. [备份与恢复](#10-备份与恢复)
11. [日志与监控](#11-日志与监控)
12. [常见故障排查](#12-常见故障排查)
13. [安全检查清单](#13-安全检查清单)
14. [RPM 打包原理（维护者参考）](#14-rpm-打包原理维护者参考)

---

## 1. 整体架构

```
浏览器
  │ HTTPS :443
  ▼
Nginx (SSL 终止 + gzip + 静态缓存)  ← 可选但强烈推荐
  │ HTTP 127.0.0.1:8080
  ▼
GoLDAPAdmin (systemd 守护)          ← 本 RPM 安装的主体
  │ TCP :389 / :636
  ▼
LDAP Server (OpenLDAP / Active Directory)
```

- **单二进制**：Go 后端 + Vue 前端（Vite 产物通过 `//go:embed` 嵌入），一条 `systemd` 守护一个进程。
- **SQLite 内嵌**：`/appdata/goldapadmin/data/goldapadmin.db`，仅管理 LDAP 连接配置、书签、操作日志，**LDAP 自身数据不在本应用中**。
- **安全沙箱**：systemd `ProtectSystem=strict` + `NoNewPrivileges=true`，进程无法写系统目录、无法提权。

---

## 2. 部署目录清单

RPM 安装后实际落盘结构如下（`rpm -ql goldapadmin` 查看）：

```
/appdata/goldapadmin/                  # 应用根目录
├── goldapadmin                        # Go 单二进制 (root:root, 0755)
└── config/
    ├── config.yaml.example            # 出厂示例配置 (root:root, 0644)
    └── config.yaml                    # 实际运行配置 (ldap:ldap, 0640)  ← %config(noreplace)
/appdata/goldapadmin/data/             # SQLite 数据目录 (ldap:ldap, 0750)
/var/log/goldapadmin/                  # 预留日志目录 (ldap:ldap, 0755)  ← 日志实际走 journal
/usr/lib/systemd/system/goldapadmin.service   # systemd unit (root:root, 0644)
```

**文件归属与权限原则**：

| 路径 | owner:group | mode | 说明 |
|---|---|---|---|
| `/appdata/goldapadmin/goldapadmin` | root:root | 0755 | 二进制需要 root 所有权，让 ldap 用户可执行但不可修改 |
| `/appdata/goldapadmin/config/config.yaml` | ldap:ldap | 0640 | 含 JWT 密钥，组不可读（ldap 组里只有 ldap 用户） |
| `/appdata/goldapadmin/data/` | ldap:ldap | 0750 | SQLite 只有 ldap 用户可以读写 |

---

## 3. 构建 RPM（开发 / CI 侧）

### 3.1 构建机环境准备

```bash
# RHEL / Rocky / AlmaLinux
sudo dnf install -y go nodejs npm rpm-build rpmdevtools

# Fedora
sudo dnf install -y golang nodejs npm rpm-build rpmdevtools

# macOS（本机调试，产物可用但建议在 Linux 上打最终包）
brew install go node rpm
```

> Go 版本 ≥ 1.22（项目 `go.mod` 声明 `go 1.26.4`）；Node.js ≥ 18。

### 3.2 一键构建（推荐）

```bash
cd goldapadmin

# 默认版本 0.1.0，同时打 x86_64 + aarch64
./deploy/rpm/build-rpm.sh

# 指定版本号（发布时用）
./deploy/rpm/build-rpm.sh --version 0.2.0 --release 1

# 跳过前端重构建（前端没改时可省 90% 时间）
./deploy/rpm/build-rpm.sh --skip-frontend
```

脚本干四件事：

1. 构建前端（`web/dist/`）并复制到 `internal/web/dist/` 让 Go embed。
2. `CGO_ENABLED=0 GOOS=linux GOARCH=amd64` / `arm64` 交叉编译 Go 二进制。
3. 准备 `rpmbuild/{BUILD,RPMS,SOURCES,SPECS}` 目录树。
4. 分别 `rpmbuild -bb --target x86_64` 和 `--target aarch64`，产物拷到 `deploy/rpm/output/rpms/`。

### 3.3 构建产物位置

```
deploy/rpm/output/rpms/
├── goldapadmin-0.1.0-1.el9.x86_64.rpm
└── goldapadmin-0.1.0-1.el9.aarch64.rpm
```

| 架构 | 适用机型 |
|---|---|
| x86_64 | Intel / AMD 服务器、大多数虚拟机 |
| aarch64 | 鲲鹏 920 / 916、AWS Graviton 2/3、树莓派 4/5、Apple Silicon（U 系列虚拟化） |

### 3.4 产物上传与分发

```bash
# 推到制品库（示意）
coscmd upload deploy/rpm/output/rpms/*.rpm  oss://your-bucket/rpms/
# 或直接 scp 到跳板机
scp deploy/rpm/output/rpms/*.rpm  bastion:/data/releases/goldapadmin/
```

---

## 4. 安装 RPM（生产侧）

### 4.1 新安装

```bash
# x86_64 机器
sudo rpm -ivh goldapadmin-0.1.0-1.el9.x86_64.rpm

# aarch64 机器
sudo rpm -ivh goldapadmin-0.1.0-1.el9.aarch64.rpm
```

RPM 在安装阶段会自动完成：

- `%pre`：创建系统用户 `ldap`（UID 55，GID 55，`/usr/sbin/nologin`）。
- `%install`：把二进制、unit、示例配置拷到位。
- `%post`：目录归属 + 如果 `config.yaml` 不存在则从 `.example` 复制 + `systemctl daemon-reload`。

### 4.2 安装后立即要做的事

```bash
# 1. 修改 JWT secret —— 必须！
openssl rand -base64 48
sudo vi /appdata/goldapadmin/config/config.yaml
# 把 secret: 这一行改成上面 openssl 的输出

# 2. 改 CORS origins（生产要列域名，不能 "*"）
sudo vi /appdata/goldapadmin/config/config.yaml

# 3. 启动 + 开机自启
sudo systemctl enable --now goldapadmin

# 4. 验证
curl -s http://127.0.0.1:8080/api/health
# => {"status":"ok"}

sudo systemctl status goldapadmin --no-pager -l
```

### 4.3 什么都不改直接起会怎样？

- 进程能起来，但 **JWT 用的是示例值**，**CORS origins 是占位域名**——**公网访问等于裸奔**。内网临时用可以凑合，公网必须过 [第 5 章](#5-配置生产环境) 的 checklist。

---

## 5. 配置生产环境

安装后编辑 `/appdata/goldapadmin/config/config.yaml`（RPM 的 `%config(noreplace)` 标记确保升级不会覆盖你的配置）。

**推荐生产配置**（重点字段）：

```yaml
server:
  port: 8080              # 只监听内网；公网访问走 Nginx 反代
  mode: release

database:
  # 与 RPM %files 一致；RPM 已把 data/ 归给 ldap 用户
  path: /appdata/goldapadmin/data/goldapadmin.db

jwt:
  # 必须替换为随机值：openssl rand -base64 48
  secret: <生成的随机串>
  ttl: 12                 # 小时；按安全合规策略调整

log:
  level: info             # debug 只在排障时临时打开

security:
  skip_tls_verify: false  # 生产必须校验 LDAP 证书
  cors:
    origins:
      - "https://ldapadmin.yourcompany.com"
      # 多域名在此继续追加；禁止 ["*"]
```

**改完后重载**：

```bash
sudo systemctl restart goldapadmin
journalctl -u goldapadmin --since "1 min ago"
```

---

## 6. systemd 服务详解

unit 文件位置：`/usr/lib/systemd/system/goldapadmin.service`。

### 6.1 核心字段速查

| 字段 | 值 | 作用 |
|---|---|---|
| `User=ldap` / `Group=ldap` | 系统用户 | 非 root 运行，最小权限 |
| `WorkingDirectory=/appdata/goldapadmin` | —— | SQLite 相对路径基准 |
| `ExecStart=/appdata/goldapadmin/goldapadmin /appdata/goldapadmin/config/config.yaml` | —— | 单二进制 + 配置路径参数 |
| `Restart=on-failure` | —— | 崩溃自动拉起 |
| `RestartSec=5` | —— | 重启前等 5 秒 |
| `StartLimitBurst=3` + `StartLimitIntervalSec=60` | —— | 60 秒内最多重启 3 次，防无限重启 |

### 6.2 安全沙箱（生产加固）

RPM 自带的 unit 已全部打开，最低 systemd 232（RHEL 8 自带 239，够用）：

| 选项 | 作用 |
|---|---|
| `NoNewPrivileges=true` | 禁止 setuid 提权 |
| `ProtectSystem=strict` | `/usr` `/etc` `/boot` 只读挂载 |
| `ProtectHome=true` | 隔离 home 目录 |
| `ProtectKernelTunables=true` | `/proc/sys/` 只读 |
| `ProtectKernelModules=true` | 禁止加载内核模块 |
| `ProtectKernelLogs=true` | 禁止读内核日志 |
| `ProtectControlGroups=true` | 禁止访问 Cgroup 层级 |
| `ProtectMounts=true` | 禁止挂载新文件系统 |
| `PrivateTmp=true` | 独立 `/tmp` |
| `PrivateDevices=true` | 不共享设备节点 |
| `MemoryDenyWriteExecute=true` | 禁止 JIT 代码执行 |

`ReadWritePaths=/appdata/goldapadmin/data /var/log/goldapadmin` 精确限定可写目录——其他位置（包括 `/appdata/goldapadmin/config/`，因为配置只读）一律拒绝写入。

### 6.3 生命周期命令

```bash
sudo systemctl enable --now goldapadmin   # 开机自启 + 立即启动
sudo systemctl start   goldapadmin
sudo systemctl stop    goldapadmin
sudo systemctl restart goldapadmin
sudo systemctl status  goldapadmin --no-pager -l
sudo systemctl daemon-reload              # unit 改过之后
```

---

## 7. Nginx 反代与 HTTPS

GoLDAPAdmin 自身只监听 HTTP，公网部署必须在前端加 Nginx 做 TLS 终止。

### 7.1 安装 Nginx + 配置

```bash
sudo dnf install -y nginx certbot python3-certbot-nginx

# 复制配置（deploy/goldapadmin-nginx.conf 已随源码提供）
sudo cp /path/to/goldapadmin/deploy/goldapadmin-nginx.conf \
        /etc/nginx/conf.d/goldapadmin.conf

# 改 server_name 和证书路径
sudo sed -i 's/ldap-admin.example.com/ldapadmin.yourcompany.com/g' \
        /etc/nginx/conf.d/goldapadmin.conf

# 申请证书（DNS 已提前配好）
sudo certbot certonly -d ldapadmin.yourcompany.com --webroot -w /var/lib/certbot

# 测试 + 启动
sudo nginx -t
sudo systemctl enable --now nginx
```

### 7.2 关键配置片段（为什么需要这些）

```nginx
# 强制 HTTPS
add_header Strict-Transport-Security "max-age=63072000; includeSubDomains; preload" always;

# 反代头（GoLDAPAdmin 日志里要 X-Real-IP）
proxy_set_header Host              $host;
proxy_set_header X-Real-IP         $remote_addr;
proxy_set_header X-Forwarded-For   $proxy_add_x_forwarded_for;
proxy_set_header X-Forwarded-Proto $scheme;

# Go embed 静态资源文件名带 hash → 长缓存 7 天
location ~* ^/assets/.*\.(js|css|svg|png)$ {
    expires 7d;
    add_header Cache-Control "public, immutable";
}
```

### 7.3 防火墙

```bash
sudo firewall-cmd --permanent --add-service=http
sudo firewall-cmd --permanent --add-service=https
sudo firewall-cmd --reload

# GoLDAPAdmin 8080 只给本机反代用，不对公网开放
sudo firewall-cmd --list-all   # 确认只有 80/443
```

---

## 8. 升级流程

### 8.1 从旧版本升级

RPM 的 `%preun` / `%postun` 钩子会自动停服务、替换二进制、daemon-reload、重启服务——**业务中断秒级**（SQLite 不迁移、不改动数据）。

```bash
# 直接 upgrade
sudo rpm -Uvh goldapadmin-0.2.0-1.el9.x86_64.rpm

# 确认版本变了
/opt/goldapadmin/goldapadmin --version   # 如果二进制实现了 version 参数
rpm -q goldapadmin                       # 或看 RPM 记录

# 健康检查
curl -s http://127.0.0.1:8080/api/health
sudo systemctl status goldapadmin --no-pager
```

### 8.2 配置文件升级策略

`config.yaml` 在 RPM 里标记为 `%config(noreplace)`，意味着：

- **旧文件已存在** → 升级时 **不会覆盖**（你的自定义配置保留）
- **新旧配置差异** → `rpm -Uvh` 会额外生成 `config.yaml.rpmnew`，运维人工 diff 合并

```bash
# 看有没有 rpmnew 文件
ls /appdata/goldapadmin/config/

# 对比差异
diff -u /appdata/goldapadmin/config/config.yaml \
       /appdata/goldapadmin/config/config.yaml.rpmnew

# 合并后删 rpmnew
sudo rm /appdata/goldapadmin/config/config.yaml.rpmnew
sudo systemctl restart goldapadmin
```

---

## 9. 卸载

```bash
# 停 + 卸载
sudo rpm -evh goldapadmin

# 或者只停服务、保留 RPM 不卸载
sudo systemctl disable --now goldapadmin
```

RPM `%postun` 钩子：

- `$1 == 0`（完全卸载）：`userdel --remove ldap` + `groupdel ldap`
- **数据目录 `/appdata/goldapadmin/` 不会被删**（rpm 不删 `%dir` 里的数据文件），防止误删生产 SQLite。想彻底清手动删：

```bash
# ⚠️ 确认数据不再需要再执行
sudo rm -rf /appdata/goldapadmin
```

---

## 10. 备份与恢复

### 10.1 需要备份的文件

```
/appdata/goldapadmin/config/config.yaml           # 配置（含 JWT secret 和 LDAP 连接信息）
/appdata/goldapadmin/data/goldapadmin.db          # SQLite（服务器配置、书签、操作日志）
```

LDAP 目录数据在 LDAP 服务器自身（OpenLDAP `slapcat` / AD `ntdsutil`），**不在 GoLDAPAdmin 里**。

### 10.2 自动备份脚本

放到 `/etc/cron.daily/goldapadmin-backup`：

```bash
#!/usr/bin/env bash
# 每天 02:17 备份（避开业务高峰）
BACKUP_DIR="/var/backups/goldapadmin"
DATE=$(date +%Y%m%d-%H%M%S)
SRC="/appdata/goldapadmin"

mkdir -p "$BACKUP_DIR"

# SQLite 安全热备份（不需要停服务）
sqlite3 "$SRC/data/goldapadmin.db" ".backup $BACKUP_DIR/goldapadmin-$DATE.db"

# 配置文件
cp "$SRC/config/config.yaml" "$BACKUP_DIR/config-$DATE.yaml"

# 保留 30 天
find "$BACKUP_DIR" -name "goldapadmin-*.db"  -mtime +30 -delete
find "$BACKUP_DIR" -name "config-*.yaml"     -mtime +30 -delete
```

```bash
sudo chmod +x /etc/cron.daily/goldapadmin-backup
```

### 10.3 恢复

```bash
sudo systemctl stop goldapadmin

# 恢复数据
sudo cp /var/backups/goldapadmin/goldapadmin-20261104-021700.db \
       /appdata/goldapadmin/data/goldapadmin.db
sudo chown ldap:ldap /appdata/goldapadmin/data/goldapadmin.db

# 恢复配置（如果需要）
sudo cp /var/backups/goldapadmin/config-20261104-021700.yaml \
       /appdata/goldapadmin/config/config.yaml
sudo chown ldap:ldap /appdata/goldapadmin/config/config.yaml
sudo chmod 0640 /appdata/goldapadmin/config/config.yaml

sudo systemctl start goldapadmin
```

---

## 11. 日志与监控

### 11.1 systemd journal（主要日志源）

```bash
journalctl -u goldapadmin -f                          # 实时跟随
journalctl -u goldapadmin --since "10 minutes ago"
journalctl -u goldapadmin -p err                      # 只看 error 及以上
journalctl -u goldapadmin --no-pager | grep <TraceID> # 按 TraceID 过滤
```

GoLDAPAdmin 每条日志自带 `trace_id`，从 HTTP 请求头 `X-Trace-ID` 继承或自动生成，可贯穿全链路。

### 11.2 systemd 自动自愈

- `Restart=on-failure` + `RestartSec=5`：进程崩溃 5 秒后拉起
- `StartLimitBurst=3` + `StartLimitIntervalSec=60`：60 秒内重启超过 3 次就放弃——防止配置错误导致无限拉起

### 11.3 健康检查端点

```bash
# 最小健康检查
curl -s http://127.0.0.1:8080/api/health
# {"status":"ok"}

# Nginx / 负载均衡器可以把这个路径当 upstream 健康检查
```

### 11.4 Prometheus 接入（可选）

```yaml
# prometheus.yml
scrape_configs:
  - job_name: 'goldapadmin'
    static_configs:
      - targets: ['127.0.0.1:8080']
    metrics_path: '/api/health'
    scrape_interval: 30s
```

---

## 12. 常见故障排查

### 12.1 服务起不来

```bash
# 1. 看 unit 本身有没有问题
sudo systemctl status goldapadmin
sudo systemd-analyze verify /usr/lib/systemd/system/goldapadmin.service

# 2. 看 journal 启动日志
journalctl -u goldapadmin -b -n 50 --no-pager

# 3. 手动跑一下二进制绕过 systemd
sudo -u ldap -g ldap -s /bin/bash
/appdata/goldapadmin/goldapadmin /appdata/goldapadmin/config/config.yaml
# → 看控制台输出什么错误
```

### 12.2 SQLite 权限问题

```
error="failed to open database: permission denied"
```

```bash
ls -la /appdata/goldapadmin/data/
sudo chown -R ldap:ldap /appdata/goldapadmin/data
sudo chmod 0750 /appdata/goldapadmin/data
```

### 12.3 端口被占用

```bash
ss -tlnp | grep 8080
sudo lsof -i :8080
```

### 12.4 LDAP 连不上

```
error="ldap: connection refused"
error="ldap: x509: certificate signed by unknown authority"
```

- 证书不自签 → `security.skip_tls_verify: false` 必须生效
- 自签证书临时用 → 在 Web UI 的 LdapServer 记录上勾选 skip TLS verify（生产 **不推荐**）
- 连通性测试：`nc -vz <ldap-host> 636`

### 12.5 CORS 错误

浏览器 Console 里有 `Access-Control-Allow-Origin` 报错 → 把实际访问域名加到 `config.yaml` 的 `cors.origins` 里，不能用 `*`。

### 12.6 systemd 沙箱导致权限被拒

如果日志出现 `Operation not permitted` 且定位到文件读写：

```bash
# 看沙箱安全分；分数越低越严格
systemd-analyze security goldapadmin --no-pager

# 临时关掉沙箱的某一项做对比测试
sudo systemctl edit goldapadmin
# 在 override 里加 ProtectSystem=no 试一下
sudo systemctl restart goldapadmin
```

---

## 13. 安全检查清单

每一条上线前都必须打勾：

| # | 检查项 | 验证命令 |
|---|---|---|
| 1 | **非 root 运行** | `ps aux | grep goldapadmin` → 用户 `ldap` |
| 2 | **JWT secret 已改** | `grep secret /appdata/goldapadmin/config/config.yaml` ≠ 示例值 |
| 3 | **CORS origins 不含 `*`** | `grep -A5 origins /appdata/goldapadmin/config/config.yaml` |
| 4 | **HTTPS 开启** | 浏览器访问，无证书警告，HSTS 生效 |
| 5 | **8080 不对公网** | `sudo firewall-cmd --list-all` → 只有 80/443 |
| 6 | **SQLite 权限** | `ls -l /appdata/goldapadmin/data/` → owner `ldap`，mode `0750` |
| 7 | **config.yaml 权限** | `ls -l /appdata/goldapadmin/config/config.yaml` → mode `0640` |
| 8 | **LDAP TLS 不跳过验证** | `grep skip_tls_verify /appdata/goldapadmin/config/config.yaml` → `false` |
| 9 | **日志级别 info 或 warn** | `grep level /appdata/goldapadmin/config/config.yaml` |
| 10 | **systemd 沙箱生效** | `systemd-analyze security goldapadmin` 确认各项不显示 OFF |
| 11 | **文件描述符充足** | `cat /proc/$(pidof goldapadmin)/limits \| grep "open files"` → ≥ 65536 |
| 12 | **Nginx 只暴露必要接口** | 防火墙 / AWS SG / NSG 只放 80/443 |

---

## 14. RPM 打包原理（维护者参考）

### 14.1 为什么能跨架构？

Go 后端依赖 `modernc.org/sqlite`（纯 Go 实现 SQLite），**不调用 CGO**。所以 `CGO_ENABLED=0` 下可以：

```bash
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build ...
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build ...
```

编出来的是纯静态 ELF，`file` 命令看：

```
goldapadmin.amd64:  ELF 64-bit LSB executable, x86-64, version 1 (SYSV), statically linked, ...
goldapadmin.arm64:  ELF 64-bit LSB executable, ARM aarch64, version 1 (SYSV), statically linked, ...
```

### 14.2 spec 的双模式

`deploy/rpm/goldapadmin.spec` 里 `%global _buildmode prebuilt` 是默认值，走交叉编译产物。CI 里也可以切到源码构建模式：

```bash
# 源码构建（rpmbuild 在构建机上自己跑 go build）
rpmbuild \
    --define "_buildmode source" \
    --define "version 0.1.0" \
    --target x86_64 \
    -bs SPECS/goldapadmin.spec    # 先打 src.rpm，再在构建机上 -bb
```

### 14.3 生命周期钩子

| 钩子 | 触发时机 | 作用 |
|---|---|---|
| `%pre` | 安装前 | 创建 `ldap` 用户 / 组（幂等，已存在就跳过） |
| `%install` | 构建阶段 | `install -m` 拷文件到 `%{buildroot}` |
| `%post` | 安装后 | `chown` / 首启时拷贝 config.yaml / `%systemd_post` |
| `%preun` | 卸载前 / 升级前 | `%systemd_preun` 停服务 |
| `%postun` | 卸载后 / 升级后 | `%systemd_postun_with_restart` 重启；完全卸载时 `userdel` |

`%systemd_post` / `%systemd_preun` / `%systemd_postun_with_restart` 是 RPM 标准宏，底层调 `systemctl daemon-reload`、`enable`、`disable`、`restart`，比自己写 systemctl 更稳（兼容 rpm 旧版本）。

### 14.4 相关源码文件速查

| 文件 | 说明 |
|---|---|
| [deploy/rpm/goldapadmin.spec](file:///Users/yangqian/go/src/github.com/lynnyq/goldapadmin/deploy/rpm/goldapadmin.spec) | RPM spec 主文件 |
| [deploy/rpm/goldapadmin.service.rpm](file:///Users/yangqian/go/src/github.com/lynnyq/goldapadmin/deploy/rpm/goldapadmin.service.rpm) | 给 RPM 专用的 systemd unit |
| [deploy/rpm/build-rpm.sh](file:///Users/yangqian/go/src/github.com/lynnyq/goldapadmin/deploy/rpm/build-rpm.sh) | 一键构建脚本 |
| [deploy/goldapadmin.service](file:///Users/yangqian/go/src/github.com/lynnyq/goldapadmin/deploy/goldapadmin.service) | 非 RPM 场景的 service（`/opt/goldapadmin` + 用户 `goldapadmin`） |
| [deploy/install-systemd.sh](file:///Users/yangqian/go/src/github.com/lynnyq/goldapadmin/deploy/install-systemd.sh) | 手动安装脚本 |
| [deploy/goldapadmin-nginx.conf](file:///Users/yangqian/go/src/github.com/lynnyq/goldapadmin/deploy/goldapadmin-nginx.conf) | Nginx 反代模板 |
| [deploy/README.md](file:///Users/yangqian/go/src/github.com/lynnyq/goldapadmin/deploy/README.md) | 手动部署完整文档 |
