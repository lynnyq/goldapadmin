package service

import (
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

// TestCacheCredentialsAndFetch 验证凭据缓存的基本存取
func TestCacheCredentialsAndFetch(t *testing.T) {
	token := "test-token-basic"
	password := "secret-password"

	CacheCredentials(token, password)

	got := getCachedPassword(token)
	if got != password {
		t.Errorf("getCachedPassword() = %q, want %q", got, password)
	}
}

// TestCacheCredentialsEmptyToken 验证空 token 不写入缓存
func TestCacheCredentialsEmptyToken(t *testing.T) {
	CacheCredentials("", "password")

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

	CacheCredentials(token, "first")
	CacheCredentials(token, "second")

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
			CacheCredentials(token, "pwd")
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
