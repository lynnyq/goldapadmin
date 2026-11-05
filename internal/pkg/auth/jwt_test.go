package auth

import (
	"os"
	"testing"
	"time"
)

// ========== JWT 生成与解析测试 ==========

func TestMain(m *testing.M) {
	SetSecret("unit-test-secret-0123456789abcdefghijklmn")
	os.Exit(m.Run())
}

func TestGenerateAndParseToken(t *testing.T) {
	serverID := 1
	userDN := "cn=admin,dc=example,dc=com"
	ttl := time.Hour

	token, err := GenerateToken(serverID, userDN, ttl)
	if err != nil {
		t.Fatalf("GenerateToken 失败: %v", err)
	}
	if token == "" {
		t.Fatal("生成的 token 不应为空")
	}

	claims, err := ParseToken(token)
	if err != nil {
		t.Fatalf("ParseToken 失败: %v", err)
	}

	if claims.ServerID != serverID {
		t.Errorf("ServerID = %d, 期望 %d", claims.ServerID, serverID)
	}
	if claims.UserDN != userDN {
		t.Errorf("UserDN = %q, 期望 %q", claims.UserDN, userDN)
	}
	if claims.Issuer != "goldapadmin" {
		t.Errorf("Issuer = %q, 期望 %q", claims.Issuer, "goldapadmin")
	}
}

func TestParseToken_InvalidToken(t *testing.T) {
	_, err := ParseToken("invalid.token.here")
	if err == nil {
		t.Error("无效 token 应该返回错误")
	}
}

func TestParseToken_EmptyToken(t *testing.T) {
	_, err := ParseToken("")
	if err == nil {
		t.Error("空 token 应该返回错误")
	}
}

func TestParseToken_TamperedToken(t *testing.T) {
	token, _ := GenerateToken(1, "cn=user,dc=example,dc=com", time.Hour)
	// 篡改 token 中间部分
	tampered := token[:len(token)/2] + "X" + token[len(token)/2+1:]
	_, err := ParseToken(tampered)
	if err == nil {
		t.Error("篡改后的 token 应该返回错误")
	}
}

func TestGenerateToken_MultipleTokens(t *testing.T) {
	// 验证多次生成可以正确解析
	for i := 0; i < 5; i++ {
		token, err := GenerateToken(i, "cn=test,dc=example,dc=com", time.Hour)
		if err != nil {
			t.Fatalf("第 %d 次 GenerateToken 失败: %v", i, err)
		}
		claims, err := ParseToken(token)
		if err != nil {
			t.Fatalf("第 %d 次 ParseToken 失败: %v", i, err)
		}
		if claims.ServerID != i {
			t.Errorf("第 %d 次 ServerID = %d, 期望 %d", i, claims.ServerID, i)
		}
	}
}

func TestSetSecret(t *testing.T) {
	oldSecret := string(getSecret())
	defer func() { SetSecret(oldSecret) }()

	newSecret := "my-new-secret-key-1234567890abcdef"
	SetSecret(newSecret)

	if string(getSecret()) != newSecret {
		t.Errorf("SetSecret 应该更新密钥")
	}

	// 使用新密钥生成并验证
	token, err := GenerateToken(5, "cn=admin,dc=example,dc=com", time.Hour)
	if err != nil {
		t.Fatalf("GenerateToken 失败: %v", err)
	}
	claims, err := ParseToken(token)
	if err != nil {
		t.Fatalf("ParseToken 使用新密钥应该成功: %v", err)
	}
	if claims.ServerID != 5 {
		t.Errorf("ServerID = %d, 期望 5", claims.ServerID)
	}
}

func TestSetSecret_EmptyDoesNotChange(t *testing.T) {
	oldSecret := string(getSecret())
	SetSecret("")
	if string(getSecret()) != oldSecret {
		t.Error("空密钥不应该改变已有密钥")
	}
}
