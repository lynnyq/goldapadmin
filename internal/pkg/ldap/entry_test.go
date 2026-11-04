package ldap

import (
	"crypto/sha1"
	"encoding/base64"
	"errors"
	"strings"
	"testing"

	"github.com/go-ldap/ldap/v3"
)

// ========== handleSizeLimit 测试 ==========

func TestHandleSizeLimit_NoError(t *testing.T) {
	result := &ldap.SearchResult{
		Entries: []*ldap.Entry{{DN: "cn=a,dc=com"}},
	}
	entries, err := handleSizeLimit(result, nil)
	if err != nil {
		t.Errorf("无错误时不应返回 sentinel error, 得到 %v", err)
	}
	if len(entries) != 1 {
		t.Errorf("期望 1 条 entries, 得到 %d", len(entries))
	}
}

func TestHandleSizeLimit_SizeLimitWithPartialData(t *testing.T) {
	result := &ldap.SearchResult{
		Entries: []*ldap.Entry{
			{DN: "cn=a,dc=com"},
			{DN: "cn=b,dc=com"},
		},
	}
	ldapErr := &ldap.Error{ResultCode: ldap.LDAPResultSizeLimitExceeded}
	entries, err := handleSizeLimit(result, ldapErr)
	if !errors.Is(err, ErrSizeLimitTruncated) {
		t.Errorf("SizeLimitExceeded + 部分数据 应返回 ErrSizeLimitTruncated, 得到 %v", err)
	}
	if len(entries) != 2 {
		t.Errorf("期望返回 2 条部分数据, 得到 %d", len(entries))
	}
}

func TestHandleSizeLimit_SizeLimitNoData(t *testing.T) {
	result := &ldap.SearchResult{Entries: nil}
	ldapErr := &ldap.Error{ResultCode: ldap.LDAPResultSizeLimitExceeded}
	entries, err := handleSizeLimit(result, ldapErr)
	if !errors.Is(err, ldapErr) {
		t.Errorf("SizeLimitExceeded + 无数据 应返回原始错误, 得到 %v", err)
	}
	if entries != nil {
		t.Errorf("无数据时 entries 应为 nil, 得到 %v", entries)
	}
}

func TestHandleSizeLimit_OtherError(t *testing.T) {
	otherErr := errors.New("connection refused")
	result := &ldap.SearchResult{Entries: []*ldap.Entry{{DN: "cn=a,dc=com"}}}
	entries, err := handleSizeLimit(result, otherErr)
	if !errors.Is(err, otherErr) {
		t.Errorf("非 SizeLimitExceeded 错误应原样返回, 得到 %v", err)
	}
	if entries != nil {
		t.Errorf("其他错误时 entries 应为 nil, 得到 %v", entries)
	}
}

// ========== DN 解析测试 ==========

func TestSplitDN(t *testing.T) {
	tests := []struct {
		name  string
		dn    string
		parts []string
	}{
		{
			name:  "简单 DN",
			dn:    "cn=admin,dc=example,dc=com",
			parts: []string{"cn=admin", "dc=example", "dc=com"},
		},
		{
			name:  "带转义逗号",
			dn:    "cn=Smith\\, John,ou=Users,dc=example,dc=com",
			parts: []string{"cn=Smith\\, John", "ou=Users", "dc=example", "dc=com"},
		},
		{
			name:  "空 DN",
			dn:    "",
			parts: nil,
		},
		{
			name:  "单值 DN",
			dn:    "dc=com",
			parts: []string{"dc=com"},
		},
		{
			name:  "多层级 DN",
			dn:    "uid=zhangsan,ou=dev,ou=users,dc=example,dc=com",
			parts: []string{"uid=zhangsan", "ou=dev", "ou=users", "dc=example", "dc=com"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := splitDN(tt.dn)
			if len(got) != len(tt.parts) {
				t.Errorf("splitDN(%q) 返回 %d 个 part, 期望 %d 个", tt.dn, len(got), len(tt.parts))
				return
			}
			for i := range got {
				if got[i] != tt.parts[i] {
					t.Errorf("splitDN(%q)[%d] = %q, 期望 %q", tt.dn, i, got[i], tt.parts[i])
				}
			}
		})
	}
}

func TestGetRDN(t *testing.T) {
	tests := []struct {
		dn     string
		expect string
	}{
		{"cn=admin,dc=example,dc=com", "cn=admin"},
		{"ou=Users,dc=example,dc=com", "ou=Users"},
		{"dc=com", "dc=com"},
		{"uid=testuser,ou=dev,dc=example,dc=com", "uid=testuser"},
	}

	for _, tt := range tests {
		t.Run(tt.dn, func(t *testing.T) {
			got := getRDN(tt.dn)
			if got != tt.expect {
				t.Errorf("getRDN(%q) = %q, 期望 %q", tt.dn, got, tt.expect)
			}
		})
	}
}

// ========== 密码哈希测试 ==========

func TestHashPassword_Plain(t *testing.T) {
	result := HashPassword("", "mypassword")
	if result != "mypassword" {
		t.Errorf("plain password 应该原样返回, 得到 %q", result)
	}
}

func TestHashPassword_SHA(t *testing.T) {
	password := "test123"
	result := HashPassword("SHA", password)

	if !strings.HasPrefix(result, "{SHA}") {
		t.Errorf("SHA 密码应该以 {SHA} 开头, 得到 %q", result)
	}

	// 手动计算 SHA1 验证
	h := sha1.New()
	h.Write([]byte(password))
	expected := "{SHA}" + base64.StdEncoding.EncodeToString(h.Sum(nil))
	if result != expected {
		t.Errorf("SHA 哈希不匹配, 得到 %q, 期望 %q", result, expected)
	}
}

func TestHashPassword_SSHA(t *testing.T) {
	password := "securepass"
	result := HashPassword("SSHA", password)

	if !strings.HasPrefix(result, "{SSHA}") {
		t.Errorf("SSHA 密码应该以 {SSHA} 开头, 得到 %q", result)
	}

	// SSHA 格式: {SSHA}base64(sha1(password+salt)+salt)
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(result, "{SSHA}"))
	if err != nil {
		t.Errorf("SSHA base64 解码失败: %v", err)
		return
	}

	// SHA1 输出为 20 字节，后面跟 4 字节盐
	if len(decoded) <= 20 {
		t.Errorf("SSHA 解码后长度应该 > 20, 实际 %d", len(decoded))
	}

	salt := decoded[20:]
	h := sha1.New()
	h.Write([]byte(password))
	h.Write(salt)
	expectedHash := h.Sum(nil)

	for i := 0; i < 20; i++ {
		if decoded[i] != expectedHash[i] {
			t.Errorf("SSHA 哈希验证失败: 第 %d 字节不匹配", i)
			break
		}
	}
}

func TestHashPassword_MD5(t *testing.T) {
	password := "md5test"
	result := HashPassword("MD5", password)

	if !strings.HasPrefix(result, "{MD5}") {
		t.Errorf("MD5 密码应该以 {MD5} 开头, 得到 %q", result)
	}
}

func TestHashPassword_AlgoCaseInsensitive(t *testing.T) {
	password := "caseTest"
	r1 := HashPassword("sha", password)
	r2 := HashPassword("SHA", password)
	r3 := HashPassword("SHA}", password)

	if r1 != r2 {
		t.Errorf("sha 和 SHA 应该产生相同结果, %q != %q", r1, r2)
	}
	if r2 != r3 {
		t.Errorf("SHA 和 SHA} 应该产生相同结果, %q != %q", r2, r3)
	}
}

// ========== EntryService 构造测试 ==========

func TestNewEntryService(t *testing.T) {
	svc := NewEntryService()
	if svc == nil {
		t.Error("NewEntryService 不应返回 nil")
	}
}
