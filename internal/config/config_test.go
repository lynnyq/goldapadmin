package config

import (
	"os"
	"path/filepath"
	"testing"
)

// TestLoadRequireLDAPTLSDefault 验证未配置 require_ldap_tls 时默认为 false（允许明文连接）
func TestLoadRequireLDAPTLSDefault(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	content := []byte("server:\n  mode: debug\n")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("写入测试配置失败: %v", err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load 失败: %v", err)
	}
	if cfg.Security.RequireLDAPTLS {
		t.Error("未配置时 require_ldap_tls 应为 false")
	}
}

// TestLoadRequireLDAPTLSTrue 验证配置文件中显式开启 require_ldap_tls
func TestLoadRequireLDAPTLSTrue(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	content := []byte("server:\n  mode: debug\nsecurity:\n  require_ldap_tls: true\n")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("写入测试配置失败: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load 失败: %v", err)
	}
	if !cfg.Security.RequireLDAPTLS {
		t.Error("配置 require_ldap_tls: true 后应解析为 true")
	}
}

// TestValidateReleaseRejectsWeakSecret release 模式必须拒绝弱密钥/占位符
func TestValidateReleaseRejectsWeakSecret(t *testing.T) {
	cases := []string{
		"",
		"goldapadmin-secret-key-change-me",
		"change-me-please-0123456789abcdefghijklm",
		"short",
	}
	for _, secret := range cases {
		cfg := &Config{Server: ServerConfig{Mode: "release"}, JWT: JWTConfig{Secret: secret, TTL: 24}}
		if err := cfg.Validate(); err == nil {
			t.Errorf("release 模式密钥 %q 应被拒绝", secret)
		}
	}
}

// TestValidateReleaseAcceptsStrongSecret release 模式接受强随机密钥
func TestValidateReleaseAcceptsStrongSecret(t *testing.T) {
	cfg := &Config{
		Server: ServerConfig{Mode: "release"},
		JWT:    JWTConfig{Secret: "Kj8sP2mQv7Lx4Rt9Yb3Nc6Fd1Hg5Za0W", TTL: 24},
		Security: SecurityConfig{
			MaxBodyBytes:   2 << 20,
			TrustedProxies: []string{"127.0.0.1"},
		},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("强密钥应通过校验: %v", err)
	}
}

// TestValidateTrustedProxiesInvalid 非法可信代理地址必须 fail-fast
func TestValidateTrustedProxiesInvalid(t *testing.T) {
	cfg := &Config{
		Server:   ServerConfig{Mode: "debug"},
		Security: SecurityConfig{TrustedProxies: []string{"not-an-ip"}},
	}
	if err := cfg.Validate(); err == nil {
		t.Error("非法 trusted_proxies 应报错")
	}
}

// TestValidateCORSWildcard release 模式禁止 CORS 通配符
func TestValidateCORSWildcard(t *testing.T) {
	cfg := &Config{
		Server:   ServerConfig{Mode: "release"},
		JWT:      JWTConfig{Secret: "Kj8sP2mQv7Lx4Rt9Yb3Nc6Fd1Hg5Za0W", TTL: 24},
		Security: SecurityConfig{CORS: CORSConfig{Origins: []string{"*"}}, MaxBodyBytes: 2 << 20},
	}
	if err := cfg.Validate(); err == nil {
		t.Error("release 模式 CORS 通配符应被拒绝")
	}
}

// TestValidateMaxBodyBytes 请求体上限必须落在合理范围
func TestValidateMaxBodyBytes(t *testing.T) {
	cfg := &Config{
		Server:   ServerConfig{Mode: "debug"},
		Security: SecurityConfig{MaxBodyBytes: 64 << 20},
	}
	if err := cfg.Validate(); err == nil {
		t.Error("超过 32MiB 的 max_body_bytes 应被拒绝")
	}
}
