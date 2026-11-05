package service

import (
	"errors"
	"fmt"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lynnyq/goldapadmin/internal/dao"
	"github.com/lynnyq/goldapadmin/internal/model"
)

// setupServiceTestDB 初始化服务层测试所需的数据库
func setupServiceTestDB(t *testing.T) {
	t.Helper()
	if err := dao.Init(filepath.Join(t.TempDir(), "service_test.db")); err != nil {
		t.Fatalf("Init DB 失败: %v", err)
	}
	t.Cleanup(func() {
		dao.GetDB().Close()
	})
}

// newTestContext 构造带鉴权信息的 Gin 上下文
func newTestContext(token string, serverID int, userDN string) *gin.Context {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Set("token", token)
	c.Set("server_id", serverID)
	c.Set("user_dn", userDN)
	return c
}

// TestCacheCredentialsAndFetch 验证会话缓存的基本存取
func TestCacheCredentialsAndFetch(t *testing.T) {
	token := "test-token-basic"
	password := "secret-password"

	CacheSession(token, nil, password)

	got := getCachedPassword(token)
	if got != password {
		t.Errorf("getCachedPassword() = %q, want %q", got, password)
	}
}

// TestCacheCredentialsEmptyToken 验证空 token 不写入缓存
func TestCacheCredentialsEmptyToken(t *testing.T) {
	CacheSession("", nil, "password")

	if v, ok := credCache.Load(""); ok {
		t.Errorf("空 token 不应写入缓存，但读到了 %v", v)
	}
}

// TestGetCachedPasswordEmptyToken 验证空 token 读取返回空
func TestGetCachedPasswordEmptyToken(t *testing.T) {
	if got := getCachedPassword(""); got != "" {
		t.Errorf("getCachedPassword(\"\") = %q, want empty", got)
	}
}

// TestGetCachedPasswordUnknownToken 验证未知 token 读取返回空
func TestGetCachedPasswordUnknownToken(t *testing.T) {
	if got := getCachedPassword("nonexistent-token"); got != "" {
		t.Errorf("未知 token 应返回空，实际 %q", got)
	}
}

// TestGetCachedPasswordExpired 验证过期条目被惰性清除并返回空
func TestGetCachedPasswordExpired(t *testing.T) {
	token := "test-token-expired"
	credCache.Store(token, credEntry{
		password:  "old-secret",
		expiresAt: time.Now().Add(-time.Minute), // 已过期
	})

	if got := getCachedPassword(token); got != "" {
		t.Errorf("过期凭据应返回空，实际 %q", got)
	}

	// 过期条目应被删除
	if _, ok := credCache.Load(token); ok {
		t.Error("过期条目应被惰性清除")
	}
}

// TestCacheCredentialsOverwrite 验证同一 token 重复缓存时后者覆盖前者
func TestCacheCredentialsOverwrite(t *testing.T) {
	token := "test-token-overwrite"

	CacheSession(token, nil, "first")
	CacheSession(token, nil, "second")

	if got := getCachedPassword(token); got != "second" {
		t.Errorf("重复缓存应覆盖，实际 %q", got)
	}
}

// TestCacheCredentialsConcurrent 验证并发读写安全
func TestCacheCredentialsConcurrent(t *testing.T) {
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			token := "concurrent-token"
			CacheSession(token, nil, "pwd")
			_ = getCachedPassword(token)
		}(i)
	}
	wg.Wait()
}

// TestErrCredentialsExpiredIsSentinel 验证 ErrCredentialsExpired 哨兵错误可被 errors.Is 识别
func TestErrCredentialsExpiredIsSentinel(t *testing.T) {
	if !errors.Is(ErrCredentialsExpired, ErrCredentialsExpired) {
		t.Error("errors.Is 应匹配哨兵错误")
	}
	// 包装后仍可识别
	wrapped := fmt.Errorf("get conn params: %w", ErrCredentialsExpired)
	if !errors.Is(wrapped, ErrCredentialsExpired) {
		t.Error("errors.Is 应识别包装后的哨兵错误")
	}
}

// TestCacheSessionStoresServerCopy 验证缓存的是服务器配置副本，外部修改不影响会话
func TestCacheSessionStoresServerCopy(t *testing.T) {
	token := "test-token-server-copy"
	server := &model.LdapServer{Host: "192.168.19.140", Port: 389, BaseDN: "dc=example,dc=com"}

	CacheSession(token, server, "pwd")
	server.Host = "changed-host"

	session, ok := getCachedSession(token)
	if !ok {
		t.Fatal("会话缓存应存在")
	}
	if session.server == nil {
		t.Fatal("会话缓存应包含服务器配置")
	}
	if session.server.Host != "192.168.19.140" {
		t.Errorf("缓存应保存副本，实际 Host = %q", session.server.Host)
	}
}

// TestClearSession 验证登出清除会话缓存
func TestClearSession(t *testing.T) {
	token := "test-token-clear"
	CacheSession(token, &model.LdapServer{Host: "127.0.0.1"}, "pwd")

	ClearSession(token)

	if _, ok := getCachedSession(token); ok {
		t.Error("ClearSession 后不应再读到会话缓存")
	}
}

// TestGetConnParamsFallsBackToSessionServer 验证临时登录（server_id=0）回退到会话中的服务器配置
func TestGetConnParamsFallsBackToSessionServer(t *testing.T) {
	setupServiceTestDB(t)
	svc := NewLdapService(false)
	defer svc.Close()

	token := "test-token-temp-session"
	server := &model.LdapServer{
		Host:    "192.168.19.140",
		Port:    389,
		BaseDN:  "dc=example,dc=com",
		Timeout: 30,
	}
	CacheSession(token, server, "yangqian@123.com")

	params, err := svc.GetConnParams(newTestContext(token, 0, "cn=Manager,dc=example,dc=com"))
	if err != nil {
		t.Fatalf("临时会话应能获取连接参数，实际错误: %v", err)
	}
	if params.Server.ID != 0 {
		t.Errorf("临时会话 Server.ID = %d, want 0", params.Server.ID)
	}
	if params.Server.Host != "192.168.19.140" {
		t.Errorf("Server.Host = %q, want 192.168.19.140", params.Server.Host)
	}
	if params.Password != "yangqian@123.com" {
		t.Errorf("Password = %q, want 缓存的密码", params.Password)
	}
}

// TestGetConnParamsUsesSavedServer 验证 server_id 命中数据库时优先使用数据库配置
func TestGetConnParamsUsesSavedServer(t *testing.T) {
	setupServiceTestDB(t)
	svc := NewLdapService(false)
	defer svc.Close()

	saved := &model.LdapServer{
		Name:    "saved",
		Host:    "10.0.0.1",
		Port:    389,
		BaseDN:  "dc=saved,dc=com",
		Timeout: 30,
	}
	if err := svc.GetServerDAO().Create(saved); err != nil {
		t.Fatalf("创建服务器记录失败: %v", err)
	}

	token := "test-token-saved-server"
	CacheSession(token, &model.LdapServer{Host: "10.0.0.2"}, "pwd")

	params, err := svc.GetConnParams(newTestContext(token, saved.ID, "cn=admin,dc=saved,dc=com"))
	if err != nil {
		t.Fatalf("已保存服务器应能获取连接参数，实际错误: %v", err)
	}
	if params.Server.Host != "10.0.0.1" {
		t.Errorf("应优先使用数据库配置，实际 Host = %q", params.Server.Host)
	}
	if params.Server.ID != saved.ID {
		t.Errorf("Server.ID = %d, want %d", params.Server.ID, saved.ID)
	}
}

// TestGetConnParamsExpiredSessionWithoutRecord 验证会话过期且无服务器记录时返回 server not found
func TestGetConnParamsExpiredSessionWithoutRecord(t *testing.T) {
	setupServiceTestDB(t)
	svc := NewLdapService(false)
	defer svc.Close()

	token := "test-token-expired-session"
	credCache.Store(token, credEntry{
		password:  "pwd",
		server:    &model.LdapServer{Host: "127.0.0.1"},
		expiresAt: time.Now().Add(-time.Minute),
	})

	if _, err := svc.GetConnParams(newTestContext(token, 0, "cn=admin")); err == nil {
		t.Error("会话过期且无服务器记录时应返回错误")
	}
}

// TestGetConnParamsExpiredSessionWithRecord 验证会话过期但服务器记录存在时返回凭据失效
func TestGetConnParamsExpiredSessionWithRecord(t *testing.T) {
	setupServiceTestDB(t)
	svc := NewLdapService(false)
	defer svc.Close()

	saved := &model.LdapServer{Name: "saved", Host: "10.0.0.1", Port: 389, Timeout: 30}
	if err := svc.GetServerDAO().Create(saved); err != nil {
		t.Fatalf("创建服务器记录失败: %v", err)
	}

	_, err := svc.GetConnParams(newTestContext("unknown-token", saved.ID, "cn=admin"))
	if !errors.Is(err, ErrCredentialsExpired) {
		t.Errorf("应返回 ErrCredentialsExpired，实际: %v", err)
	}
}

// TestValidateConnectionSecurity 验证 security.require_ldap_tls 开关行为
func TestValidateConnectionSecurity(t *testing.T) {
	plainServer := &model.LdapServer{Host: "127.0.0.1", Port: 389}
	tlsServer := &model.LdapServer{Host: "127.0.0.1", Port: 389, UseTLS: true}
	sslServer := &model.LdapServer{Host: "127.0.0.1", Port: 636, UseSSL: true}

	t.Run("默认允许明文连接", func(t *testing.T) {
		svc := NewLdapService(false)
		defer svc.Close()
		if err := svc.validateConnectionSecurity(plainServer); err != nil {
			t.Errorf("require_ldap_tls=false 时应允许明文连接，实际: %v", err)
		}
	})

	t.Run("开启后拒绝明文连接", func(t *testing.T) {
		svc := NewLdapService(true)
		defer svc.Close()
		err := svc.validateConnectionSecurity(plainServer)
		if err == nil {
			t.Fatal("require_ldap_tls=true 时应拒绝明文连接")
		}
		if !strings.Contains(err.Error(), "require_ldap_tls") {
			t.Errorf("错误信息应提示配置项，实际: %v", err)
		}
	})

	t.Run("开启后允许 TLS 与 SSL", func(t *testing.T) {
		svc := NewLdapService(true)
		defer svc.Close()
		if err := svc.validateConnectionSecurity(tlsServer); err != nil {
			t.Errorf("UseTLS 连接不应被拒绝，实际: %v", err)
		}
		if err := svc.validateConnectionSecurity(sslServer); err != nil {
			t.Errorf("UseSSL 连接不应被拒绝，实际: %v", err)
		}
	})
}
