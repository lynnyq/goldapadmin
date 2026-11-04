# =============================================================================
# GoLDAPAdmin RPM Spec
#
# 构建方式：
#   (A) 交叉编译预编译包（推荐，deploy/rpm/build-rpm.sh 一键完成）
#       外部先编好二进制，再调 rpmbuild：
#         rpmbuild --target x86_64 \
#                  --define "_bin_arch amd64" -bb SPECS/goldapadmin.spec
#         rpmbuild --target aarch64 \
#                  --define "_bin_arch arm64" -bb SPECS/goldapadmin.spec
#
#   (B) 源码构建（需要 golang + nodejs + npm）
#       rpmbuild --define "_buildmode source" -bs SPECS/goldapadmin.spec
#
# 关键设计：
#   %ifarch 求值的是【构建机】架构，不是 --target 指定的目标架构；
#   所以本 spec 绝对不在内部用 %ifarch 判断二进制名，
#   二进制架构一律由外部 _bin_arch 参数注入。
# =============================================================================

%define debug_package %{nil}

Name:           goldapadmin
Version:        0.1.0
Release:        1%{?dist}
Summary:        GoLDAPAdmin - Lightweight web-based LDAP directory management tool

License:        MIT
URL:            https://github.com/lynnyq/goldapadmin

# -----------------------------------------------------------------------------
# 构建模式
#   prebuilt: 外部已编好二进制（默认，build-rpm.sh 用）
#   source:    rpmbuild 在构建机上自己 go build + npm build
# -----------------------------------------------------------------------------
%global _buildmode  %{?_buildmode: %{_buildmode}}%{!?_buildmode: prebuilt}
%global _bin_arch   %{?_bin_arch:  %{_bin_arch}}%{!?_bin_arch:  amd64}

# -----------------------------------------------------------------------------
# Source 声明
# -----------------------------------------------------------------------------
%if "%{_buildmode}" == "source"
Source0:        %{name}-%{version}.tar.gz
BuildRequires:  golang >= 1.22
BuildRequires:  nodejs >= 18
BuildRequires:  npm
BuildRequires:  systemd-rpm-macros
%endif

# prebuilt 模式的 SOURCES 目录里需已存在:
#   goldapadmin.amd64.bin   ← x86_64 目标用
#   goldapadmin.arm64.bin   ← aarch64 目标用
#   goldapadmin.service.rpm
#   config.yaml.example

Source1:        goldapadmin.service.rpm
Source2:        config.yaml.example

Requires(pre):  shadow-utils
Requires:       systemd

# 纯 Go 静态二进制（modernc.org/sqlite，无 CGO 依赖）
AutoReqProv:    yes

# -----------------------------------------------------------------------------
# description
# -----------------------------------------------------------------------------
%description
GoLDAPAdmin is a lightweight, single-binary, web-based LDAP directory
management tool written in Go. It provides an intuitive Vue UI for
browsing, searching, and managing entries on OpenLDAP / Active Directory
servers. SQLite-only stores connection profiles, bookmarks, and audit logs.

# -----------------------------------------------------------------------------
# %prep — 仅源码构建需要解压
# -----------------------------------------------------------------------------
%prep
%if "%{_buildmode}" == "source"
%setup -q
%endif

# -----------------------------------------------------------------------------
# %build — 仅源码构建模式
# -----------------------------------------------------------------------------
%build
%if "%{_buildmode}" == "source"
set -euo pipefail

# 1. 前端
cd web
npm install --silent
npm run build
cd ..

# 2. 拷贝到 embed 目录，让 Go 编译时打包进去
rm -rf internal/web/dist
cp -R web/dist/. internal/web/dist/

# 3. Go 后端（纯静态 — CGO_ENABLED=0）
#    源码构建时 GOOS/GOARCH 不指定，直接编构建机本机架构
#    如果要在 x86_64 构建机上交叉编 aarch64，外部传 --define "_target_cpu aarch64"
export CGO_ENABLED=0
export GOOS=linux
go build \
    -ldflags "-s -w -X main.version=%{version}" \
    -o %{name} ./cmd/server
%endif

# -----------------------------------------------------------------------------
# %install — 两种模式都要做
# -----------------------------------------------------------------------------
%install
set -euo pipefail
rm -rf %{buildroot}

# ---- 目录结构 ----
install -d -m 0755 %{buildroot}/appdata/goldapadmin/config
install -d -m 0755 %{buildroot}/appdata/goldapadmin/data
install -d -m 0755 %{buildroot}/var/log/goldapadmin
install -d -m 0755 %{buildroot}/%{_unitdir}

# ---- 二进制 ----
# prebuilt 模式: 从 SOURCES 取外部编好的
# source   模式: 从 %build 产物取
%if "%{_buildmode}" == "source"
install -m 0755 %{name} %{buildroot}/appdata/goldapadmin/goldapadmin
%else
install -m 0755 %{_sourcedir}/goldapadmin.%{_bin_arch}.bin \
    %{buildroot}/appdata/goldapadmin/goldapadmin
%endif

# ---- 配置文件 ----
# .example 给运维参考；.yaml 作为 %config(noreplace) 首次落盘载体
# 升级时 RPM 自动处理：若本机已有 config.yaml → 保留旧的，新包重命名为 .rpmnew
install -m 0644 %{SOURCE2} %{buildroot}/appdata/goldapadmin/config/config.yaml.example
install -m 0640 %{SOURCE2} %{buildroot}/appdata/goldapadmin/config/config.yaml

# ---- systemd unit ----
install -m 0644 %{SOURCE1} %{buildroot}/%{_unitdir}/goldapadmin.service

# -----------------------------------------------------------------------------
# %pre — 安装/升级前
#   作用: 确保 ldap 系统用户/组存在
#   注意: ldap 是 RHEL/麒麟/centos 系统预置账户（装 openldap-server 会自动建），
#         所以绝不指定 UID/GID，避免和系统预置值冲突
# -----------------------------------------------------------------------------
%pre
set -euo pipefail

# 组存在即跳过；不存在才创建（让系统自分配 GID）
if ! getent group ldap >/dev/null 2>&1; then
    groupadd --system ldap
fi

# 用户存在即跳过；不存在才创建（不指定 UID/GID，绑定到已存在的 ldap 组）
if ! getent passwd ldap >/dev/null 2>&1; then
    useradd --system \
            --gid ldap \
            --no-create-home \
            --shell /usr/sbin/nologin \
            ldap
fi

exit 0

# -----------------------------------------------------------------------------
# %post — 首次安装后 / 升级后
# -----------------------------------------------------------------------------
%post
set -euo pipefail

# %files 里已通过 %attr 指定了 owner/group/mode，RPM 安装时原生会处理；
# 这里做防御性 chown -R，确保升级后旧版本遗留的不一致权限被修正。
chown -R ldap:ldap /appdata/goldapadmin /var/log/goldapadmin 2>/dev/null || true

%systemd_post goldapadmin.service

# -----------------------------------------------------------------------------
# %preun — 卸载前 / 升级前
# -----------------------------------------------------------------------------
%preun
%systemd_preun goldapadmin.service

# -----------------------------------------------------------------------------
# %postun — 卸载后 / 升级后
#
#   $1 = 0  : 完全卸载（最后一包）
#   $1 = 1  : 升级中（还有新版本替换）
# -----------------------------------------------------------------------------
%postun
set -euo pipefail

%systemd_postun_with_restart goldapadmin.service

# 完全卸载时：清理 rpm 创建的 ldap 用户/组
# - 我们只在 %pre 中创建不存在的 ldap，装了 openldap-server 的机器本来就有
# - 对系统预置的 ldap，userdel 会报错但我们 -f 忽略
if [ "$1" -eq 0 ]; then
    userdel ldap 2>/dev/null || true
    groupdel ldap 2>/dev/null || true
    # 数据目录故意不删，防止误删生产 SQLite
    echo "GoLDAPAdmin data preserved at /appdata/goldapadmin"
fi

# -----------------------------------------------------------------------------
# %files — 文件清单
#
# %defattr 设定默认 root:root，然后对需要特殊权限的逐项 %attr 覆盖
# -----------------------------------------------------------------------------
%files
# 默认属性：二进制和 unit 用 root 所有
%defattr(-, root, root, -)

# 二进制 — root 所有，ldap 组用户可执行但不可修改
/appdata/goldapadmin/goldapadmin

# 配置示例
/appdata/goldapadmin/config/config.yaml.example

# 主配置 — %config(noreplace) 升级时自动保留旧文件、新包写 .rpmnew
# owner/group: ldap:ldap  mode: 0640（组不可读，只给 ldap 自身）
%config(noreplace) %attr(0640, ldap, ldap) /appdata/goldapadmin/config/config.yaml

# 应用根目录 — 挂载点，ldap 用户可遍历
%dir %attr(0750, ldap, ldap) /appdata/goldapadmin

# SQLite 数据目录 — 只有 ldap 可读写
%dir %attr(0750, ldap, ldap) /appdata/goldapadmin/data

# 预留日志目录
%dir %attr(0755, ldap, ldap) /var/log/goldapadmin

# systemd unit
%{_unitdir}/goldapadmin.service

# -----------------------------------------------------------------------------
# %verify — 升级/卸载时跳过某些文件的校验
#   config.yaml 是 %config(noreplace)，但仍需校验文件归属
# -----------------------------------------------------------------------------
%verify(mode owner group size md5 mtime) /appdata/goldapadmin/goldapadmin
%verify(mode owner group)                /appdata/goldapadmin/config/config.yaml
%verify(mode owner group)                /usr/lib/systemd/system/goldapadmin.service

# -----------------------------------------------------------------------------
# %changelog
# -----------------------------------------------------------------------------
%changelog
* Wed Nov 04 2026 GoLDAPAdmin Team <dev@goldapadmin.local> - 0.1.0-1
- Initial RPM package
- Deploy to /appdata/goldapadmin, run as system user ldap
- systemd service file bundled
- Cross-compiled for x86_64 (amd64) and aarch64 (arm64) via CGO_ENABLED=0
- %prebuilt mode: external binaries via --define "_bin_arch amd64|arm64"
- %source mode: rpmbuild builds golang + frontend from tarball
