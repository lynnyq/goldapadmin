package service

import (
	"testing"

	"github.com/lynnyq/goldapadmin/internal/model"
)

func TestIsAdmin(t *testing.T) {
	svc := NewLdapServiceWithOptions(SecurityOptions{
		AdminDNs: []string{"cn=admin,dc=example,dc=com", "uid=ops,ou=admins,dc=example,dc=com"},
	})

	if !svc.IsAdmin("cn=admin,dc=example,dc=com") {
		t.Error("白名单内 DN 应判定为管理员")
	}
	if !svc.IsAdmin("  CN=ADMIN,DC=EXAMPLE,DC=COM  ") {
		t.Error("管理员判定应忽略大小写与首尾空白")
	}
	if svc.IsAdmin("cn=user,dc=example,dc=com") {
		t.Error("非白名单 DN 不应判定为管理员")
	}
	if svc.IsAdmin("") {
		t.Error("空 DN 不应判定为管理员")
	}
}

func TestIsAdmin_EmptyWhitelistFailClosed(t *testing.T) {
	svc := NewLdapServiceWithOptions(SecurityOptions{})
	if svc.IsAdmin("cn=admin,dc=example,dc=com") {
		t.Error("admin_dns 为空时必须 fail-closed，任何人都不应成为管理员")
	}
}

func TestVerifyBind_RequireTLSRejectsPlaintext(t *testing.T) {
	svc := NewLdapServiceWithOptions(SecurityOptions{RequireLDAPTLS: true})
	_, err := svc.VerifyBind(&model.LdapServer{Host: "127.0.0.1", Port: 389, Timeout: 1}, "cn=admin,dc=example,dc=com", "secret")
	if err == nil {
		t.Fatal("开启 require_ldap_tls 后明文连接应被拒绝")
	}
}

func TestVerifyBind_AnonymousRejectedByDefault(t *testing.T) {
	svc := NewLdapServiceWithOptions(SecurityOptions{})
	_, err := svc.VerifyBind(&model.LdapServer{Host: "127.0.0.1", Port: 389, Timeout: 1}, "", "")
	if err == nil {
		t.Fatal("默认应拒绝匿名绑定")
	}
}

func TestNormalizeDN(t *testing.T) {
	if got := normalizeDN("  CN=Admin,DC=Example  "); got != "cn=admin,dc=example" {
		t.Errorf("normalizeDN 结果 = %q", got)
	}
	if got := normalizeDN(""); got != "" {
		t.Errorf("空 DN 规范化结果应为空, 得到 %q", got)
	}
}
