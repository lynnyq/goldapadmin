# =============================================================================
# GoLDAPAdmin 生产部署文档（手动 install 脚本方案）
#
# ⚠️ 如果你的环境是 RHEL / 麒麟 / openEuler / Rocky / AlmaLinux，
#    强烈建议改用 deploy/rpm/README.md 的 **RPM 方案**：
#    - 交叉编译 x86_64 + aarch64 一个命令搞定
#    - 标准 rpm 生命周期（install / upgrade / erase）
#    - systemd 服务自动注册
# =============================================================================

## 快速部署（三步骤）

```bash
# 1. 构建
make build

# 2. 一键安装
sudo bash deploy/install-systemd.sh
# → 会提示你是否立即启动

# 3. （可选）配 Nginx HTTPS 反代
sudo cp deploy/goldapadmin-nginx.conf /etc/nginx/conf.d/goldapadmin.conf
# 改 server_name / SSL 证书路径后
sudo nginx -t && sudo systemctl reload nginx
```

---

## 部署架构

```
浏览器
  │ HTTPS :443
  ▼
Nginx (SSL 终止 + gzip + 静态缓存)
  │ HTTP :8080
  ▼
GoLDAPAdmin (systemd 守护)
  │ TCP :389 / :636
  ▼
LDAP Server (OpenLDAP / AD)
```

---

## 目录清单

```
deploy/
├── goldapadmin.service          # systemd unit 文件（带完整沙箱加固）
├── install-systemd.sh           # 一键安装脚本
├── goldapadmin-nginx.conf      # Nginx HTTPS 反代配置
└── README.md                    # 本文档
```

---

## 一、systemd 服务详解

### 1.1 文件内容

[goldapadmin.service](file:///Users/yangqian/go/src/github.com/lynnyq/goldapadmin/deploy/goldapadmin.service)

### 1.2 关键配置说明

```ini
[Unit]
Description=GoLDAPAdmin - LDAP Directory Management Tool
Documentation=https://github.com/lynnyq/goldapadmin

# 只在网络就绪后启动，避免启动时 LDAP 不可达
After=network-online.target
Wants=network-online.target
```

```ini
[Service]
# 非 root 用户运行 — 生产强制
User=goldapadmin
Group=goldapadmin

# SQLite 相对路径的基准目录
WorkingDirectory=/opt/goldapadmin

# 启动命令（传配置文件路径，支持多实例）
ExecStart=/opt/goldapadmin/goldapadmin /opt/goldapadmin/config/config.yaml

# 进程异常退出自动重启，最多 60 秒内 3 次
Restart=on-failure
RestartSec=5
StartLimitIntervalSec=60
StartLimitBurst=3
```

### 1.3 沙箱加固（生产安全）

| 选项 | 作用 | 说明 |
|---|---|---|
| `NoNewPrivileges=true` | 禁止进程通过 exec 获取更高权限 | 防止 setuid 二进制滥用 |
| `ProtectSystem=strict` | 只读挂载 `/usr` `/etc` `/boot` | 进程无法篡改系统文件 |
| `ReadWritePaths` | 仅允许写指定目录 | `data/` + 日志目录可写，其他全只读 |
| `ProtectHome=true` | 隔离 home 目录 | 防止读取用户私钥等 |
| `ProtectKernelTunables=true` | 禁止改内核参数 | `/proc/sys/` 只读 |
| `ProtectKernelModules=true` | 禁止加载内核模块 | 彻底的模块加载隔离 |
| `ProtectKernelLogs=true` | 禁止读取内核日志 | `/dev/kmsg` 不可访问 |
| `ProtectControlGroups=true` | 禁止访问 Cgroup 层级 | 防容器逃逸 |
| `ProtectMounts=true` | 禁止挂载新文件系统 | 防挂载攻击 |
| `PrivateTmp=true` | 独立 `/tmp` | 进程间临时文件隔离 |
| `PrivateDevices=true` | 不共享设备节点 | 无法访问 `/dev/sda` 等 |
| `MemoryDenyWriteExecute=true` | 禁止 JIT 执行 | 防代码注入（Go 原生无 JIT） |
| `LimitNOFILE=65536` | 文件描述符上限 | SQLite + 多 LDAP 连接需要足够 fd |
| `LimitCORE=0` | 禁止 core dump | 避免泄露内存中的 LDAP 密码 |

### 1.4 版本兼容

部分沙箱选项需要较新的 systemd：

| 选项 | 最低 systemd 版本 |
|---|---|
| `ProtectClock` | 245 |
| `ProtectHostname` | 245 |
| `MemoryDenyWriteExecute` | 251 |
| `RestrictRealtime` | 241 |
| `ProtectControlGroups` | 232 |
| `ProtectMounts` | 232 |

老版本 CentOS 7 (systemd 219) / Ubuntu 18.04 (systemd 237) 可**删除**这些不支持的选项，核心的 `NoNewPrivileges` / `ProtectSystem` / `ProtectHome` / `PrivateTmp` 依然有效。

### 1.5 常用命令

```bash
# 启停
sudo systemctl start goldapadmin
sudo systemctl stop goldapadmin
sudo systemctl restart goldapadmin

# 查看状态
sudo systemctl status goldapadmin
sudo systemctl status goldapadmin --no-pager -l   # 完整输出

# 开机自启
sudo systemctl enable goldapadmin
sudo systemctl disable goldapadmin

# 日志
journalctl -u goldapadmin -f                     # 实时跟随
journalctl -u goldapadmin --since "10 minutes ago"
journalctl -u goldapadmin -p err                 # 只看 error 级别
journalctl -u goldapadmin --no-pager | grep trace-abc12345   # 按 TraceID 过滤

# 重新加载 unit 文件
sudo systemctl daemon-reload
sudo systemctl restart goldapadmin
```

---

## 二、Nginx 反代配置详解

### 2.1 为什么需要 Nginx？

| 问题 | 解决 |
|---|---|
| **HTTPS** | GoLDAPAdmin 自身是 HTTP，Nginx 做 TLS 终止 |
| **证书管理** | Let's Encrypt / certbot 标准实践 |
| **安全头** | HSTS / X-Frame-Options / CSP 统一配置 |
| **静态资源缓存** | Go embed 的 JS/CSS 可长缓存 7 天 |
| **Gzip 压缩** | 减少传输体积 |
| **统一入口** | 多个服务同域部署时的路径区分 |

### 2.2 关键配置片段

**SSL 现代安全**（Qualys A+ 评分）：
```nginx
ssl_protocols TLSv1.2 TLSv1.3;
ssl_ciphers ECDHE-ECDSA-AES128-GCM-SHA256:ECDHE-RSA-AES128-GCM-SHA256:...;
ssl_session_tickets off;    # 前向加密
add_header Strict-Transport-Security "max-age=63072000; includeSubDomains; preload";
```

**反代头**（GoLDAPAdmin 后端需要）：
```nginx
proxy_set_header Host              $host;
proxy_set_header X-Real-IP         $remote_addr;   # 日志里能看到真实客户端 IP
proxy_set_header X-Forwarded-For   $proxy_add_x_forwarded_for;
proxy_set_header X-Forwarded-Proto $scheme;
```

**静态资源缓存**（前端 Vite 产物带 hash）：
```nginx
location ~* ^/assets/.*\.(js|css|svg|png)$ {
    expires 7d;
    add_header Cache-Control "public, immutable";
}
```

### 2.3 完整部署流程

```bash
# 1. 安装 Nginx
sudo apt install nginx -y          # Debian/Ubuntu
sudo yum install nginx -y          # CentOS/RHEL
sudo dnf install nginx -y          # Fedora/Rocky 9+

# 2. 获取域名证书（需要域名已 DNS 指向服务器）
sudo apt install certbot python3-certbot-nginx -y
sudo certbot certonly -d ldap-admin.example.com \
    --webroot -w /var/lib/certbot

# 3. 安装反代配置
sudo cp deploy/goldapadmin-nginx.conf /etc/nginx/conf.d/goldapadmin.conf
sudo sed -i 's/ldap-admin.example.com/你的域名/g' /etc/nginx/conf.d/goldapadmin.conf

# 4. 测试 + 重载
sudo nginx -t
sudo systemctl enable nginx
sudo systemctl reload nginx

# 5. 自动续期（certbot 已 systemd timer 托管）
sudo certbot renew --dry-run        # 验证续期是否正常
```

---

## 三、生产配置清单

### 3.1 config.yaml（必须修改的项）

```yaml
server:
  port: 8080
  mode: release                    # ⚠️ 必须是 release，不能 debug

database:
  path: /opt/goldapadmin/data/goldapadmin.db   # 绝对路径

jwt:
  secret: "openssl rand -base64 64 的输出"      # ⚠️ 必须改！
  ttl: 24

log:
  level: info                      # debug 仅开发用
```

### 3.2 安全检查清单

| # | 检查项 | 命令 |
|---|---|---|
| 1 | **非 root 运行** | `ps aux \| grep goldapadmin` 确认用户是 `goldapadmin` |
| 2 | **JWT 密钥已改** | `grep secret /opt/goldapadmin/config/config.yaml` 不是默认值 |
| 3 | **HTTPS 开启** | 浏览器访问 https://无证书警告 |
| 4 | **systemd 沙箱生效** | `systemd-analyze security goldapadmin` 检查 SCORE |
| 5 | **端口 8080 不对外** | Nginx 反代后 GoLDAPAdmin 只监听 127.0.0.1 |
| 6 | **SQLite 权限** | `ls -l /opt/goldapadmin/data/` 属于 `goldapadmin` 用户 |
| 7 | **日志轮转** | systemd journal 自动轮转，无需额外配置 |
| 8 | **文件描述符** | `cat /proc/$(pidof goldapadmin)/limits \| grep "open files"` ≥ 65536 |

---

## 四、多实例部署

如果需要同时管理**多台独立的 LDAP 后端**，GoLDAPAdmin 本身已在 SQLite 管理多服务器，但如果需要物理隔离可跑多实例：

```bash
# 实例 1: ldap1 专用
cp config/config.yaml config/ldap1.yaml
# → 修改 port: 8081, database.path: ./data/ldap1.db

# 实例 2: ldap2 专用
cp config/config.yaml config/ldap2.yaml
# → 修改 port: 8082, database.path: ./data/ldap2.db
```

systemd 可以用模板或多 unit：

```ini
# goldapadmin-ldap1.service
ExecStart=/opt/goldapadmin/goldapadmin /opt/goldapadmin/config/ldap1.yaml

# goldapadmin-ldap2.service
ExecStart=/opt/goldapadmin/goldapadmin /opt/goldapadmin/config/ldap2.yaml
```

---

## 五、监控与告警

### 5.1 systemd 自动监控

- `Restart=on-failure` 进程崩溃自动拉起
- `StartLimitBurst=3` 防止无限重启（有上限保护）

### 5.2 GoLDAPAdmin 健康端点

```bash
# 健康检查（Nginx + 监控系统可用）
curl -s http://127.0.0.1:8080/api/health
# → {"status":"ok"}

# LDAP 连接状态（需 JWT）
curl -s -H "Authorization: Bearer $TOKEN" \
  http://127.0.0.1:8080/api/monitor/stats | jq .data.connected
# → true / false
```

### 5.3 Prometheus + Alertmanager（可选）

```yaml
# prometheus.yml 抓取配置
scrape_configs:
  - job_name: 'goldapadmin'
    static_configs:
      - targets: ['127.0.0.1:8080']
    metrics_path: '/api/health'
```

---

## 六、备份与恢复

### 6.1 需要备份的文件

```
/opt/goldapadmin/config/config.yaml     # 配置
/opt/goldapadmin/data/goldapadmin.db    # SQLite（服务器配置、书签、操作日志）
```

LDAP 目录数据**不在 GoLDAPAdmin**，备份由 LDAP 服务器自身负责。

### 6.2 备份脚本

```bash
#!/bin/bash
# /etc/cron.daily/goldapadmin-backup

BACKUP_DIR="/var/backups/goldapadmin"
DATE=$(date +%Y%m%d-%H%M%S)
SOURCE="/opt/goldapadmin"

mkdir -p "$BACKUP_DIR"

# SQLite 安全备份（先停止进程或用 sqlite3 .backup）
sqlite3 "$SOURCE/data/goldapadmin.db" ".backup $BACKUP_DIR/goldapadmin-$DATE.db"

# 配置文件
cp "$SOURCE/config/config.yaml" "$BACKUP_DIR/config-$DATE.yaml"

# 保留最近 30 天
find "$BACKUP_DIR" -name "goldapadmin-*.db" -mtime +30 -delete
find "$BACKUP_DIR" -name "config-*.yaml" -mtime +30 -delete
```

---

## 七、升级流程

```bash
# 1. 拉新代码 / 下载新二进制
git pull origin main            # 或 wget 下载新版本二进制

# 2. 构建
make build

# 3. 停止 + 替换 + 启动（秒级中断）
sudo systemctl stop goldapadmin
sudo cp goldapadmin /opt/goldapadmin/goldapadmin
sudo chown goldapadmin:goldapadmin /opt/goldapadmin/goldapadmin
sudo systemctl start goldapadmin

# 4. 验证
sudo systemctl status goldapadmin --no-pager
curl -s http://127.0.0.1:8080/api/health
```

升级期间 SQLite 文件不变，操作日志不会丢。
