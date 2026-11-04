#!/usr/bin/env bash
# =============================================================================
# GoLDAPAdmin 生产环境一键部署脚本
# 使用方式: sudo bash deploy/install-systemd.sh
# =============================================================================

set -euo pipefail

APP_NAME="goldapadmin"
INSTALL_DIR="/opt/goldapadmin"
SERVICE_FILE="/etc/systemd/system/${APP_NAME}.service"
CONFIG_SRC="$(cd "$(dirname "$0")" && pwd)"   # deploy/ 目录
BINARY_SRC="$(cd "$(dirname "$0")/.." && pwd)/${APP_NAME}"

echo "==> GoLDAPAdmin 生产部署"
echo ""

# ---- 1. 检查二进制 ----
if [ ! -f "$BINARY_SRC" ]; then
    echo "❌ 未找到二进制: $BINARY_SRC"
    echo "   请先执行: make build"
    exit 1
fi
echo "✅ 二进制存在: $BINARY_SRC"

# ---- 2. 创建非 root 用户 ----
if ! id -u goldapadmin &>/dev/null; then
    echo "==> 创建系统用户 goldapadmin (无登录 shell)"
    useradd --system --no-create-home --shell /usr/sbin/nologin --user-group goldapadmin
else
    echo "✅ 用户 goldapadmin 已存在"
fi

# ---- 3. 创建目录 ----
echo "==> 创建安装目录: $INSTALL_DIR"
mkdir -p "$INSTALL_DIR"
mkdir -p "$INSTALL_DIR/config"
mkdir -p "$INSTALL_DIR/data"
mkdir -p /var/log/goldapadmin

# ---- 4. 拷贝文件 ----
echo "==> 拷贝二进制"
cp -v "$BINARY_SRC" "$INSTALL_DIR/goldapadmin"
chmod 755 "$INSTALL_DIR/goldapadmin"

echo "==> 拷贝配置文件"
if [ -f "$INSTALL_DIR/config/config.yaml" ]; then
    echo "   config.yaml 已存在，跳过（保留现有配置）"
else
    if [ -f "$(dirname "$0")/../config/config.yaml" ]; then
        cp -v "$(dirname "$0")/../config/config.yaml" "$INSTALL_DIR/config/config.yaml"
    else
        echo "   ⚠️ 未找到 config/config.yaml，请手动创建"
    fi
fi

# ---- 5. 设置权限 ----
echo "==> 设置目录权限"
chown -R goldapadmin:goldapadmin "$INSTALL_DIR"
chown -R goldapadmin:goldapadmin /var/log/goldapadmin
chmod 750 "$INSTALL_DIR"
chmod 750 "$INSTALL_DIR/data"
chmod 640 "$INSTALL_DIR/config/config.yaml" 2>/dev/null || true

# ---- 6. 安装 systemd unit ----
echo "==> 安装 systemd 服务"
cp -v "$CONFIG_SRC/goldapadmin.service" "$SERVICE_FILE"
systemctl daemon-reload

# ---- 7. 提示 JWT 密钥修改 ----
SECRET=$(grep -E '^\s*secret:' "$INSTALL_DIR/config/config.yaml" 2>/dev/null | awk '{print $2}' || echo "")
if [ "$SECRET" = "goldapadmin-secret-key-change-me" ]; then
    echo ""
    echo "⚠️  ⚠️  ⚠️  生产必须修改 JWT secret！"
    echo ""
    echo "   生成随机密钥: openssl rand -base64 32"
    echo "   编辑文件:     $INSTALL_DIR/config/config.yaml"
    echo ""
fi

# ---- 8. 启动 ----
echo ""
read -p "是否立即启动服务? [y/N] " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
    systemctl enable goldapadmin
    systemctl start goldapadmin
    sleep 1
    echo ""
    systemctl status goldapadmin --no-pager -l
else
    echo ""
    echo "手动启动:"
    echo "  systemctl enable goldapadmin   # 开机自启"
    echo "  systemctl start goldapadmin    # 立即启动"
    echo "  systemctl status goldapadmin   # 查看状态"
    echo ""
    echo "查看日志:"
    echo "  journalctl -u goldapadmin -f"
fi

echo ""
echo "✅ 部署完成"
