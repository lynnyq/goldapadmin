# =============================================================================
# GoLDAPAdmin RPM Spec  (prebuilt mode)
#
# Usage:
#   rpmbuild --target x86_64   --define "_bin_arch amd64"  -bb SPECS/goldapadmin.spec
#   rpmbuild --target aarch64  --define "_bin_arch arm64"  -bb SPECS/goldapadmin.spec
#
# SOURCES dir must contain (deploy/rpm/build-rpm.sh prepares these):
#   goldapadmin.amd64.bin     goldapadmin.arm64.bin
#   goldapadmin.service.rpm   config.yaml.example
#
# Design notes (rpm 4.14 pitfalls):
#   * Binary arch injected externally via --define "_bin_arch ...".
#     DO NOT use %%ifarch inside spec — %%ifarch evaluates BUILD host arch,
#     NOT the --target; mixing them produces wrong-arch RPMs that
#     immediately fail with "exec format error".
#   * NO %%if/%%endif blocks inside ANY section (%%prep, %%build, %%install,
#     %%files, etc.).  Old rpm (Kylin V10 / openEuler 4.14) has a parser
#     bug where %%endif after a section does not properly pop the section
#     stack, so the next section gets flagged as "duplicate %%install".
#     This is why %%prep and %%build are left empty here.
# =============================================================================

%define debug_package %{nil}

Name:           goldapadmin
Version:        0.1.0
Release:        1%{?dist}
Summary:        GoLDAPAdmin - Lightweight web-based LDAP directory management tool

License:        MIT
URL:            https://github.com/lynnyq/goldapadmin

Source1:        goldapadmin.service.rpm
Source2:        config.yaml.example

Requires(pre):  shadow-utils
Requires:       systemd

AutoReqProv:    yes

%description
GoLDAPAdmin is a lightweight, single-binary, web-based LDAP directory
management tool written in Go. It provides an intuitive Vue UI for
browsing, searching, and managing entries on OpenLDAP / Active Directory
servers. SQLite-only stores connection profiles, bookmarks, and audit logs.

# -----------------------------------------------------------------------------
# Empty sections — prebuilt mode, no source to extract or compile.
# -----------------------------------------------------------------------------
%prep

%build

# -----------------------------------------------------------------------------
# %%install
# -----------------------------------------------------------------------------
%install
set -euo pipefail
rm -rf %{buildroot}

install -d -m 0755 %{buildroot}/appdata/goldapadmin/config
install -d -m 0755 %{buildroot}/appdata/goldapadmin/data
install -d -m 0755 %{buildroot}/var/log/goldapadmin
install -d -m 0755 %{buildroot}/%{_unitdir}

install -m 0755 %{_sourcedir}/goldapadmin.%{_bin_arch}.bin \
    %{buildroot}/appdata/goldapadmin/goldapadmin

install -m 0644 %{SOURCE2} %{buildroot}/appdata/goldapadmin/config/config.yaml.example
install -m 0640 %{SOURCE2} %{buildroot}/appdata/goldapadmin/config/config.yaml

install -m 0644 %{SOURCE1} %{buildroot}/%{_unitdir}/goldapadmin.service

# -----------------------------------------------------------------------------
# %%pre — ensure system user ldap exists
#   ldap is a system account on RHEL / Kylin (created automatically with
#   openldap-server), so we do NOT hard-code UID/GID — let the distro pick.
# -----------------------------------------------------------------------------
%pre
set -euo pipefail

if ! getent group ldap >/dev/null 2>&1; then
    groupadd --system ldap
fi

if ! getent passwd ldap >/dev/null 2>&1; then
    useradd --system \
            --gid ldap \
            --no-create-home \
            --shell /usr/sbin/nologin \
            ldap
fi

exit 0

# -----------------------------------------------------------------------------
# %%post
# -----------------------------------------------------------------------------
%post
set -euo pipefail

# RPM %%files 里的 %%attr 已指定权限，%%post 这里做防御性 chmod/chown，
# 确保升级场景下旧版本遗留的不一致权限被修正。
chown -R ldap:ldap /appdata/goldapadmin /var/log/goldapadmin 2>/dev/null || true
chmod 0755 /appdata/goldapadmin
chmod 0750 /appdata/goldapadmin/data
chmod 0640 /appdata/goldapadmin/config/config.yaml 2>/dev/null || true

%systemd_post goldapadmin.service

# -----------------------------------------------------------------------------
# %%preun
# -----------------------------------------------------------------------------
%preun
%systemd_preun goldapadmin.service

# -----------------------------------------------------------------------------
# %%postun
#   $1 == 0 : final uninstall
#   $1 == 1 : upgrade
# -----------------------------------------------------------------------------
%postun
set -euo pipefail

%systemd_postun_with_restart goldapadmin.service

if [ "$1" -eq 0 ]; then
    userdel ldap 2>/dev/null || true
    groupdel ldap 2>/dev/null || true
    echo "GoLDAPAdmin data preserved at /appdata/goldapadmin"
fi

# -----------------------------------------------------------------------------
# %%files
#   %%verify MUST appear inside %%files; rpm rejects it as a standalone section.
# -----------------------------------------------------------------------------
%files
%defattr(-, root, root, -)

/appdata/goldapadmin/goldapadmin
%verify(mode owner group) /appdata/goldapadmin/goldapadmin

/appdata/goldapadmin/config/config.yaml.example

%config(noreplace) %attr(0640, ldap, ldap) /appdata/goldapadmin/config/config.yaml
%verify(mode owner group) /appdata/goldapadmin/config/config.yaml

%dir %attr(0755, ldap, ldap) /appdata/goldapadmin
%dir %attr(0750, ldap, ldap) /appdata/goldapadmin/data
%dir %attr(0755, ldap, ldap) /var/log/goldapadmin

%{_unitdir}/goldapadmin.service
%verify(mode owner group) %{_unitdir}/goldapadmin.service

# -----------------------------------------------------------------------------
%changelog
* Wed Nov 04 2026 GoLDAPAdmin Team <dev@goldapadmin.local> - 0.1.0-1
- Initial RPM package
- Deploy to /appdata/goldapadmin, run as system user ldap
- systemd service file bundled
- Cross-compiled for x86_64 (amd64) and aarch64 (arm64) via CGO_ENABLED=0
- Prebuilt mode: external binary via --define "_bin_arch amd64|arm64"
