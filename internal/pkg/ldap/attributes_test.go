package ldap

import (
	"testing"

	"github.com/lynnyq/goldapadmin/internal/model"
)

func TestMaskSensitiveAttributes(t *testing.T) {
	attrs := []model.LdapAttribute{
		{Name: "userPassword", Values: []string{"{SSHA}secret-hash"}},
		{Name: "cn", Values: []string{"admin"}},
		{Name: "unicodePwd", Values: []string{"binary"}},
	}
	MaskSensitiveAttributes(attrs)

	if attrs[0].Values[0] != maskedValue {
		t.Errorf("userPassword 应被脱敏, 得到 %q", attrs[0].Values[0])
	}
	if !attrs[0].Masked {
		t.Error("userPassword 的 Masked 标记应为 true")
	}
	if attrs[1].Values[0] != "admin" {
		t.Error("普通属性不应被脱敏")
	}
	if attrs[2].Values[0] != maskedValue {
		t.Error("unicodePwd 应被脱敏")
	}
}

func TestIsSensitiveAttribute(t *testing.T) {
	if !IsSensitiveAttribute("userPassword") {
		t.Error("userPassword 应识别为敏感属性")
	}
	if !IsSensitiveAttribute("UserPassword") {
		t.Error("敏感属性判断应大小写不敏感")
	}
	if IsSensitiveAttribute("cn") {
		t.Error("cn 不应识别为敏感属性")
	}
}

func TestValidateAttributeName(t *testing.T) {
	valid := []string{"cn", "objectClass", "1.2.840.113556.1.4.8", "sambaNTPassword", "cn;lang-en"}
	for _, name := range valid {
		if err := ValidateAttributeName(name); err != nil {
			t.Errorf("合法属性名 %q 不应报错: %v", name, err)
		}
	}

	invalid := []string{"", "*", "+", "dn", "att/name", "attr name", "attr(x)"}
	for _, name := range invalid {
		if err := ValidateAttributeName(name); err == nil {
			t.Errorf("非法属性名 %q 应报错", name)
		}
	}
}

func TestValidateAttributeNames(t *testing.T) {
	if err := ValidateAttributeNames([]string{"cn", "sn"}); err != nil {
		t.Fatalf("合法属性列表不应报错: %v", err)
	}
	if err := ValidateAttributeNames([]string{"cn", "bad name"}); err == nil {
		t.Error("包含非法属性名时应报错")
	}
}
