package ldap

import (
	"strings"
	"testing"

	"github.com/lynnyq/goldapadmin/internal/model"
)

// ========== Schema 解析测试 ==========

// 注意：LDAP subschema 存储格式没有开头的 '(' 包裹，OID 直接在最前面
// 例如 OpenLDAP 的 cn=subschema,cn=config 中 objectClasses 字段的值
func TestParseObjectClass_Simple(t *testing.T) {
	def := `2.5.6.5 NAME 'organizationalUnit' DESC 'RFC2256: an organizational unit' SUP top STRUCTURAL MUST ou MAY ( businessCategory $ description $ telephoneNumber )`
	oc := parseObjectClass(def)

	if oc.OID != "2.5.6.5" {
		t.Errorf("OID = %q, 期望 %q", oc.OID, "2.5.6.5")
	}
	if oc.Name != "organizationalUnit" {
		t.Errorf("Name = %q, 期望 %q", oc.Name, "organizationalUnit")
	}
	if oc.Type != "structural" {
		t.Errorf("Type = %q, 期望 %q", oc.Type, "structural")
	}
	if oc.Description != "RFC2256: an organizational unit" {
		t.Errorf("Description = %q", oc.Description)
	}
	if len(oc.Must) != 1 || oc.Must[0] != "ou" {
		t.Errorf("Must = %v, 期望 [ou]", oc.Must)
	}
	if len(oc.May) != 3 {
		t.Errorf("May 数量 = %d, 期望 3, 实际 %v", len(oc.May), oc.May)
	}
	if len(oc.Superior) != 1 || oc.Superior[0] != "top" {
		t.Errorf("Superior = %v, 期望 [top]", oc.Superior)
	}
}

func TestParseObjectClass_MultiName(t *testing.T) {
	def := `2.5.6.6 NAME ( 'person' 'Person' ) DESC 'RFC2256: a person' SUP top STRUCTURAL MUST ( cn $ sn ) MAY ( userPassword $ telephoneNumber )`
	oc := parseObjectClass(def)

	if oc.Name != "person" {
		t.Errorf("Name = %q, 期望 %q", oc.Name, "person")
	}
	if len(oc.Names) != 2 {
		t.Errorf("Names 数量 = %d, 期望 2", len(oc.Names))
	}
	if !contains(oc.Must, "cn") {
		t.Errorf("Must = %v, 应该包含 cn", oc.Must)
	}
	if !contains(oc.Must, "sn") {
		t.Errorf("Must = %v, 应该包含 sn", oc.Must)
	}
}

func TestParseObjectClass_Auxiliary(t *testing.T) {
	def := `1.2.840.113556.1.3.24 NAME 'member' DESC 'Helper class' AUXILIARY MUST member`
	oc := parseObjectClass(def)

	if oc.Type != "auxiliary" {
		t.Errorf("Type = %q, 期望 %q", oc.Type, "auxiliary")
	}
}

func TestParseObjectClass_Abstract(t *testing.T) {
	def := `2.5.6.0 NAME 'top' DESC 'top of hierarchy' ABSTRACT MUST objectClass`
	oc := parseObjectClass(def)

	if oc.Type != "abstract" {
		t.Errorf("Type = %q, 期望 %q", oc.Type, "abstract")
	}
}

func TestParseObjectClass_NoMust(t *testing.T) {
	def := `2.5.6.9 NAME 'groupOfNames' DESC 'group of names' SUP top STRUCTURAL MUST ( cn $ member ) MAY description`
	oc := parseObjectClass(def)

	if !contains(oc.Must, "cn") {
		t.Errorf("Must 应该包含 cn, 实际 %v", oc.Must)
	}
	if !contains(oc.Must, "member") {
		t.Errorf("Must 应该包含 member, 实际 %v", oc.Must)
	}
}

func TestParseAttributeType(t *testing.T) {
	def := `2.5.4.3 NAME 'cn' DESC 'common name' SYNTAX 1.3.6.1.4.1.1466.115.121.1.15`
	at := parseAttributeType(def)

	if at.OID != "2.5.4.3" {
		t.Errorf("OID = %q, 期望 %q", at.OID, "2.5.4.3")
	}
	if at.Name != "cn" {
		t.Errorf("Name = %q, 期望 %q", at.Name, "cn")
	}
	if at.Syntax != "1.3.6.1.4.1.1466.115.121.1.15" {
		t.Errorf("Syntax = %q", at.Syntax)
	}
}

func TestParseAttributeType_SingleValue(t *testing.T) {
	def := `2.5.4.4 NAME 'sn' DESC 'surname' SYNTAX 1.3.6.1.4.1.1466.115.121.1.15 SINGLE-VALUE`
	at := parseAttributeType(def)

	if !at.SingleValue {
		t.Errorf("SINGLE-VALUE 属性应该标记为 SingleValue")
	}
}

// ========== 默认 Schema 测试 ==========

func TestDefaultObjectClasses(t *testing.T) {
	list := defaultObjectClasses()

	// 验证必要的 objectClass 存在
	requiredOCs := []string{"top", "organizationalUnit", "organization", "groupOfNames", "inetOrgPerson", "person", "organizationalPerson"}
	for _, expected := range requiredOCs {
		found := false
		for _, oc := range list {
			if oc.Name == expected || contains(oc.Names, expected) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("defaultObjectClasses 缺少 %q", expected)
		}
	}

	// 验证 top 存在且 MUST objectClass
	var top *model.ObjectClassDef
	for i := range list {
		if list[i].Name == "top" {
			top = &list[i]
			break
		}
	}
	if top == nil {
		t.Fatal("top objectClass 未找到")
	}
	if !contains(top.Must, "objectClass") {
		t.Error("top 的 MUST 应该包含 objectClass")
	}
}

func TestDefaultAttributeTypes(t *testing.T) {
	list := defaultAttributeTypes()

	requiredAttrs := []string{"cn", "sn", "uid", "mail", "ou", "o", "dc", "telephoneNumber", "userPassword", "objectClass"}
	for _, expected := range requiredAttrs {
		found := false
		for _, at := range list {
			if at.Name == expected || contains(at.Names, expected) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("defaultAttributeTypes 缺少 %q", expected)
		}
	}
}

// ========== 辅助函数测试 ==========

func TestContainsCI(t *testing.T) {
	list := []string{"CN", "uid", "Mail"}

	if !containsCI(list, "cn") {
		t.Error("应该不区分大小写匹配 'cn' 和 'CN'")
	}
	if !containsCI(list, "UID") {
		t.Error("应该不区分大小写匹配 'UID' 和 'uid'")
	}
	if !containsCI(list, "mail") {
		t.Error("应该不区分大小写匹配 'mail' 和 'Mail'")
	}
	if containsCI(list, "unknown") {
		t.Error("不应该匹配不存在的元素")
	}
}

func TestSplitQDStrings(t *testing.T) {
	// splitQDStrings 按 $ 分割并去除外层单引号，是 MUST/MAY 值的辅助函数
	// 输入已经是从正则中提取好的属性列表片段
	tests := []struct {
		input  string
		expect []string
	}{
		{"cn $ sn", []string{"cn", "sn"}},
		{"businessCategory $ description $ telephoneNumber", []string{"businessCategory", "description", "telephoneNumber"}},
		{"ou", []string{"ou"}},
		{"userPassword", []string{"userPassword"}},
		{"", nil},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := splitQDStrings(tt.input)
			if len(got) != len(tt.expect) {
				t.Errorf("splitQDStrings(%q) = %v, 期望 %v", tt.input, got, tt.expect)
				return
			}
			for i := range got {
				if got[i] != tt.expect[i] {
					t.Errorf("splitQDStrings(%q)[%d] = %q, 期望 %q", tt.input, i, got[i], tt.expect[i])
				}
			}
		})
	}
}

func contains(list []string, target string) bool {
	for _, s := range list {
		if strings.EqualFold(s, target) {
			return true
		}
	}
	return false
}

func TestNewSchemaService(t *testing.T) {
	svc := NewSchemaService()
	if svc == nil {
		t.Error("NewSchemaService 不应返回 nil")
	}
}
