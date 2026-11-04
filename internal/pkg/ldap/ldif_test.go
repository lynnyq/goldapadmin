package ldap

import (
	"strings"
	"testing"
)

// ========== parseLdif 测试 ==========

func TestParseLdif_Simple(t *testing.T) {
	input := `dn: cn=admin,dc=example,dc=com
objectClass: inetOrgPerson
cn: admin
sn: Administrator
mail: admin@example.com

dn: ou=Users,dc=example,dc=com
objectClass: organizationalUnit
ou: Users
`

	entries, err := parseLdif(input)
	if err != nil {
		t.Fatalf("parseLdif 失败: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("解析出 %d 条, 期望 2 条", len(entries))
	}

	// 验证第一条
	e1 := entries[0]
	if e1["dn"] != "cn=admin,dc=example,dc=com" {
		t.Errorf("第一条 dn = %q", e1["dn"])
	}
	if e1["cn"] != "admin" {
		t.Errorf("第一条 cn = %q", e1["cn"])
	}
	if e1["sn"] != "Administrator" {
		t.Errorf("第一条 sn = %q", e1["sn"])
	}

	// 验证第二条
	e2 := entries[1]
	if e2["dn"] != "ou=Users,dc=example,dc=com" {
		t.Errorf("第二条 dn = %q", e2["dn"])
	}
	if e2["ou"] != "Users" {
		t.Errorf("第二条 ou = %q", e2["ou"])
	}
}

func TestParseLdif_MultipleValues(t *testing.T) {
	input := `dn: cn=admin,dc=example,dc=com
objectClass: inetOrgPerson
objectClass: organizationalPerson
objectClass: person
objectClass: top
mail: admin@example.com
mail: admin2@example.com
`

	entries, err := parseLdif(input)
	if err != nil {
		t.Fatalf("parseLdif 失败: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("解析出 %d 条, 期望 1 条", len(entries))
	}

	e := entries[0]
	// 多值属性应该合并为 \n 分隔
	if !strings.Contains(e["objectClass"], "inetOrgPerson") {
		t.Errorf("objectClass 应该包含 inetOrgPerson, 得到 %q", e["objectClass"])
	}
	if !strings.Contains(e["objectClass"], "top") {
		t.Errorf("objectClass 应该包含 top, 得到 %q", e["objectClass"])
	}
	if !strings.Contains(e["mail"], "admin@example.com") || !strings.Contains(e["mail"], "admin2@example.com") {
		t.Errorf("多值 mail 应该被合并, 得到 %q", e["mail"])
	}
}

func TestParseLdif_Comments(t *testing.T) {
	input := `# 这是一个注释
dn: cn=user1,dc=example,dc=com
# 另一个注释
objectClass: person
cn: user1
sn: Test
`

	entries, err := parseLdif(input)
	if err != nil {
		t.Fatalf("parseLdif 失败: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("解析出 %d 条, 期望 1 条", len(entries))
	}
	if entries[0]["dn"] != "cn=user1,dc=example,dc=com" {
		t.Errorf("dn = %q", entries[0]["dn"])
	}
}

func TestParseLdif_Empty(t *testing.T) {
	entries, err := parseLdif("")
	if err != nil {
		t.Fatalf("parseLdif 失败: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("空输入应该返回 0 条, 得到 %d", len(entries))
	}
}

func TestParseLdif_Base64Value(t *testing.T) {
	// 带冒号的 base64 编码值
	input := `dn: cn=test,dc=example,dc=com
objectClass: person
userPassword:: cGFzc3dvcmQxMjM=
cn: test
sn: user
`

	entries, err := parseLdif(input)
	if err != nil {
		t.Fatalf("parseLdif 失败: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("解析出 %d 条, 期望 1 条", len(entries))
	}
	// base64 以 :: 开头时取第三个 ":" 之后的内容
	pwd := entries[0]["userPassword"]
	if !strings.Contains(pwd, "cGFzc3dvcmQxMjM=") {
		t.Errorf("base64 值未正确处理, 得到 %q", pwd)
	}
}

func TestParseLdif_Changetype(t *testing.T) {
	input := `dn: cn=admin,dc=example,dc=com
changetype: delete

dn: cn=newuser,dc=example,dc=com
changetype: add
objectClass: person
cn: newuser
sn: User
`

	entries, err := parseLdif(input)
	if err != nil {
		t.Fatalf("parseLdif 失败: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("解析出 %d 条, 期望 2 条", len(entries))
	}
	if entries[0]["changetype"] != "delete" {
		t.Errorf("第一条 changetype = %q, 期望 'delete'", entries[0]["changetype"])
	}
	if entries[1]["changetype"] != "add" {
		t.Errorf("第二条 changetype = %q, 期望 'add'", entries[1]["changetype"])
	}
}

func TestNewLdifService(t *testing.T) {
	svc := NewLdifService()
	if svc == nil {
		t.Error("NewLdifService 不应返回 nil")
	}
}
