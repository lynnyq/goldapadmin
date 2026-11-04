#!/usr/bin/env bash
# =============================================================================
# GoLDAPAdmin RPM 交叉编译构建脚本
#
# 功能:
#   1. 构建前端 (web/dist) 并 embed 到 Go
#   2. 交叉编译 Go 二进制 (linux/amd64 + linux/arm64)
#   3. 分别打包 x86_64 / aarch64 RPM
#
# 使用:
#   ./deploy/rpm/build-rpm.sh [--version 0.1.0] [--release 1] [--skip-frontend]
#
# 依赖:
#   - Go >= 1.22  (go.mod 声明 1.26.4)
#   - Node.js >= 18 + npm
#   - rpm-build (RHEL/CentOS/Fedora 自带; macOS 用 brew install rpm 或 Docker)
# =============================================================================

set -euo pipefail

# ---- 默认参数 ----
VERSION="0.1.0"
RELEASE="1"
SKIP_FRONTEND=false
SPEC_ARCHS=("x86_64" "aarch64")
BIN_ARCHS=("amd64"   "arm64")

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

# ---- 参数解析 ----
while [[ $# -gt 0 ]]; do
    case "$1" in
        --version)   VERSION="$2"; shift 2 ;;
        --release)   RELEASE="$2"; shift 2 ;;
        --skip-frontend) SKIP_FRONTEND=true; shift ;;
        -h|--help)
            sed -n '2,22p' "$0" | sed 's/^#//'
            exit 0 ;;
        *) echo "Unknown option: $1"; exit 1 ;;
    esac
done

echo "========================================"
echo " GoLDAPAdmin RPM Build"
echo "   Version : $VERSION"
echo "   Release : $RELEASE"
echo "   Skip FE : $SKIP_FRONTEND"
echo "   Project : $PROJECT_ROOT"
echo "========================================"
echo ""

# -----------------------------------------------------------------------------
# 0. 环境检查
# -----------------------------------------------------------------------------
command -v go >/dev/null 2>&1 || { echo "❌ go not found"; exit 1; }
command -v rpmbuild >/dev/null 2>&1 || {
    echo "❌ rpmbuild not found."
    echo "   macOS: brew install rpm"
    echo "   RHEL/CentOS: sudo dnf install rpm-build rpmdevtools"
    exit 1
}

if ! $SKIP_FRONTEND; then
    command -v node >/dev/null 2>&1 || { echo "❌ node not found"; exit 1; }
    command -v npm  >/dev/null 2>&1 || { echo "❌ npm not found";  exit 1; }
fi

# -----------------------------------------------------------------------------
# 1. 前端构建 + embed
# -----------------------------------------------------------------------------
if ! $SKIP_FRONTEND; then
    echo "==> [1/4] Building frontend ..."
    (cd "$PROJECT_ROOT/web" && npm install --silent && npm run build)
    rm -rf "$PROJECT_ROOT/internal/web/dist"
    cp -R "$PROJECT_ROOT/web/dist/." "$PROJECT_ROOT/internal/web/dist/"
    echo "    frontend embedded OK"
else
    echo "==> [1/4] Skipping frontend (--skip-frontend)"
fi

# -----------------------------------------------------------------------------
# 2. 交叉编译 Go 二进制
# -----------------------------------------------------------------------------
echo "==> [2/4] Cross-compiling Go binaries (CGO_ENABLED=0) ..."

cd "$PROJECT_ROOT"
mkdir -p "$SCRIPT_DIR/output/bin"

LDFLAGS="-s -w -X main.version=${VERSION}"

for i in "${!BIN_ARCHS[@]}"; do
    GOARCH="${BIN_ARCHS[$i]}"
    BIN_OUT="$SCRIPT_DIR/output/bin/goldapadmin.${GOARCH}"
    echo "    -> linux/${GOARCH}"
    CGO_ENABLED=0 GOOS=linux GOARCH="$GOARCH" \
        go build -ldflags "$LDFLAGS" -o "$BIN_OUT" ./cmd/server
    chmod 0755 "$BIN_OUT"
    echo "       $(ls -lh "$BIN_OUT" | awk '{print $5}')"
done

echo "    done."
echo ""

# -----------------------------------------------------------------------------
# 3. 准备 rpmbuild 目录
# -----------------------------------------------------------------------------
echo "==> [3/4] Preparing rpmbuild tree ..."

RPM_BUILDROOT="$SCRIPT_DIR/output/rpmbuild"
rm -rf "$RPM_BUILDROOT"
mkdir -p "$RPM_BUILDROOT"/{BUILD,RPMS,SOURCES,SPECS,tmp}

cp "$SCRIPT_DIR/goldapadmin.spec"             "$RPM_BUILDROOT/SPECS/"
cp "$SCRIPT_DIR/goldapadmin.service.rpm"       "$RPM_BUILDROOT/SOURCES/goldapadmin.service.rpm"

# 把 config.example.prod.yaml 当示例配置拷过去
cp "$PROJECT_ROOT/config/config.example.prod.yaml" \
   "$RPM_BUILDROOT/SOURCES/config.yaml.example"

# 把编好的二进制按 rpm spec 里 %{_bin_arch} 命名放 SOURCES
cp "$SCRIPT_DIR/output/bin/goldapadmin.amd64" \
   "$RPM_BUILDROOT/SOURCES/goldapadmin.amd64.bin"
cp "$SCRIPT_DIR/output/bin/goldapadmin.arm64" \
   "$RPM_BUILDROOT/SOURCES/goldapadmin.arm64.bin"

echo "    layout:"
ls -R "$RPM_BUILDROOT"
echo ""

# -----------------------------------------------------------------------------
# 4. 分别打包两个架构
# -----------------------------------------------------------------------------
echo "==> [4/4] Building RPMs ..."

RPM_OUT="$SCRIPT_DIR/output/rpms"
mkdir -p "$RPM_OUT"

cd "$RPM_BUILDROOT"

for arch in "${SPEC_ARCHS[@]}"; do
    echo ""
    echo "----- target: $arch -----"

    rpmbuild \
        --define "_topdir $RPM_BUILDROOT" \
        --define "_buildmode prebuilt" \
        --define "version $VERSION" \
        --define "release $RELEASE" \
        --target "$arch" \
        -bb SPECS/goldapadmin.spec

    # 把产物拷到统一目录
    find "$RPM_BUILDROOT/RPMS/$arch" -name "*.rpm" -print0 \
        | xargs -0 -I{} cp -v {} "$RPM_OUT/"
done

echo ""
echo "========================================"
echo " ✅ 构建完成"
echo " RPM 产物:"
ls -lh "$RPM_OUT"/
echo "========================================"
echo ""
echo "安装示例:"
echo "  sudo rpm -ivh goldapadmin-${VERSION}-${RELEASE}.el9.x86_64.rpm"
echo "  sudo rpm -ivh goldapadmin-${VERSION}-${RELEASE}.el9.aarch64.rpm"
echo "  sudo systemctl enable --now goldapadmin"
echo ""
echo "配置文件位置: /appdata/goldapadmin/config/config.yaml"
echo "数据目录:     /appdata/goldapadmin/data"
echo "日志:         journalctl -u goldapadmin -f"
