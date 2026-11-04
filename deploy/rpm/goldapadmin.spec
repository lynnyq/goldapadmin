# =============================================================================
# GoLDAPAdmin RPM Spec
# 构建: rpmbuild --target x86_64 -bb SPECS/goldapadmin.spec
#      rpmbuild --target aarch64 -bb SPECS/goldapadmin.spec
#
# 也可通过 deploy/rpm/build-rpm.sh 一键交叉编译 + 打包
# =============================================================================

%define debug_package %{nil}

Name:           goldapadmin
Version:        0.1.0
Release:        1%{?dist}
Summary:        GoLDAPAdmin - LDAP Directory Management Tool

License:        MIT
URL:            https://github.com/lynnyq/goldapadmin

# -----------------------------------------------------------------------------
# Source: 两种模式二选一
#   模式 A（推荐，交叉编译）: build-rpm.sh 先编好二进制，放 %{_sourcedir}/
#       Source0: goldapadmin.%{_arch}.bin
#   模式 B（源码构建）: rpmbuild 从源码编译，需要 BuildRequires: golang nodejs
# -----------------------------------------------------------------------------
%global _buildmode %{?_buildmode: %{_buildmode}}%{!?_buildmode: prebuilt}

%if "%{_buildmode}" == "source"
Source0:        %{name}-%{version}.tar.gz
BuildRequires:  golang >= 1.22
BuildRequires:  nodejs >= 18
BuildRequires:  npm
%endif

# 预编译产物（交叉编译模式）
%ifarch x86_64
%global _bin_arch amd64
%endif
%ifarch aarch64
%global _bin_arch arm64
%endif

Source1:        goldapadmin.service.rpm
Source2:        config.yaml.example

Requires(pre):  shadow-utils
Requires:       systemd

%description
GoLDAPAdmin is a lightweight, web-based LDAP directory management tool
written in Go. It provides an intuitive UI for browsing, searching, and
managing entries on OpenLDAP / Active Directory servers.

# -----------------------------------------------------------------------------
# 准备阶段
# -----------------------------------------------------------------------------
%prep
%if "%{_buildmode}" == "source"
%setup -q
%endif

# -----------------------------------------------------------------------------
# 编译阶段
# -----------------------------------------------------------------------------
%build
%if "%{_buildmode}" == "source"
# 1. 前端
cd web
npm install --silent
npm run build
cd ..
rm -rf internal/web/dist
cp -R web/dist/. internal/web/dist/

# 2. Go 后端（CGO_ENABLED=0 → 纯静态，支持所有架构）
export CGO_ENABLED=0
export GOOS=linux
export GOARCH=%{_bin_arch}
go build -ldflags "-s -w -X main.version=%{version}" \
    -o %{name} ./cmd/server
%endif

# -----------------------------------------------------------------------------
# 安装阶段
# -----------------------------------------------------------------------------
%install
rm -rf %{buildroot}

# 目录结构
install -d %{buildroot}/appdata/goldapadmin
install -d %{buildroot}/appdata/goldapadmin/config
install -d %{buildroot}/appdata/goldapadmin/data
install -d %{buildroot}/var/log/goldapadmin
install -d %{buildroot}/%{_unitdir}

# 二进制
%if "%{_buildmode}" == "source"
install -m 0755 %{name} %{buildroot}/appdata/goldapadmin/goldapadmin
%else
install -m 0755 %{_sourcedir}/goldapadmin.%{_bin_arch}.bin \
    %{buildroot}/appdata/goldapadmin/goldapadmin
%endif

# 配置文件（.example 后缀，避免覆盖现有配置）
install -m 0644 %{SOURCE2} %{buildroot}/appdata/goldapadmin/config/config.yaml.example

# systemd unit
install -m 0644 %{SOURCE1} %{buildroot}/%{_unitdir}/goldapadmin.service

# -----------------------------------------------------------------------------
# 用户创建：%pre —— 安装前创建 ldap 系统用户
# -----------------------------------------------------------------------------
%pre
getent group ldap >/dev/null 2>&1 || \
    groupadd --system --gid 55 ldap || \
    groupadd --system ldap

getent passwd ldap >/dev/null 2>&1 || \
    useradd --system --uid 55 --gid ldap \
        --no-create-home --shell /usr/sbin/nologin ldap || \
    useradd --system --gid ldap \
        --no-create-home --shell /usr/sbin/nologin ldap

exit 0

# -----------------------------------------------------------------------------
# 安装后：权限 + daemon-reload + 提示启用
# -----------------------------------------------------------------------------
%post
# 目录归属
chown -R ldap:ldap /appdata/goldapadmin /var/log/goldapadmin 2>/dev/null || true
chmod 0750 /appdata/goldapadmin
chmod 0750 /appdata/goldapadmin/data

# 如果 config.yaml 不存在，从 example 拷贝
if [ ! -f /appdata/goldapadmin/config/config.yaml ] \
   && [ -f /appdata/goldapadmin/config/config.yaml.example ]; then
    cp /appdata/goldapadmin/config/config.yaml.example \
       /appdata/goldapadmin/config/config.yaml
    chown ldap:ldap /appdata/goldapadmin/config/config.yaml
    chmod 0640 /appdata/goldapadmin/config/config.yaml
fi

# systemd 重载
%systemd_post goldapadmin.service

# -----------------------------------------------------------------------------
# 升级前：停服务
# -----------------------------------------------------------------------------
%preun
%systemd_preun goldapadmin.service

# -----------------------------------------------------------------------------
# 升级后 / 卸载后
# -----------------------------------------------------------------------------
%postun
%systemd_postun_with_restart goldapadmin.service

# 卸载完成后清理用户（仅删除 RPM 安装的）
if [ $1 -eq 0 ]; then
    userdel --remove ldap 2>/dev/null || true
    groupdel ldap 2>/dev/null || true
    # 数据目录保留，用户确认后再删
    echo "GoLDAPAdmin data preserved at /appdata/goldapadmin"
fi

# -----------------------------------------------------------------------------
# 文件清单
# -----------------------------------------------------------------------------
%files
%defattr(-, root, root, -)
/appdata/goldapadmin/goldapadmin
/appdata/goldapadmin/config/config.yaml.example
%config(noreplace) /appdata/goldapadmin/config/config.yaml
%dir %attr(0750, ldap, ldap) /appdata/goldapadmin/data
%dir %attr(0755, ldap, ldap) /var/log/goldapadmin
%{_unitdir}/goldapadmin.service

# -----------------------------------------------------------------------------
# 变更日志
# -----------------------------------------------------------------------------
%changelog
* Tue Nov 04 2026 GoLDAPAdmin Team <dev@goldapadmin.local> - 0.1.0-1
- Initial RPM package
- Deploy to /appdata/goldapadmin, run as system user ldap
- systemd service file bundled
- Cross-compiled for x86_64 and aarch64
