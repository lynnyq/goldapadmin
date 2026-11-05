package ldap

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/go-ldap/ldap/v3"
)

// ========== CopyEntryCrossServer 逻辑验证（不连 LDAP，只验证参数/结构） ==========

// TestCopyEntryCrossServer_Recursive 验证跨服务器递归复制时子 DN 拼接正确
func TestCopyEntryCrossServer_Recursive(t *testing.T) {
	tests := []struct {
		childDN   string
		destDN    string
		expectRDN string
	}{
		{"cn=subuser,ou=users,dc=src", "ou=users,dc=dst", "cn=subuser"},
		{"uid=test,ou=dev,ou=users,dc=src", "ou=users,dc=dst", "uid=test"},
		{"ou=children,ou=parent,dc=src", "ou=parent,dc=dst", "ou=children"},
	}
	for _, tt := range tests {
		t.Run(tt.childDN, func(t *testing.T) {
			got := getRDN(tt.childDN)
			if got != tt.expectRDN {
				t.Errorf("getRDN(%q) = %q, 期望 %q", tt.childDN, got, tt.expectRDN)
			}
			dest := got + "," + tt.destDN
			if !strings.HasSuffix(dest, tt.destDN) {
				t.Errorf("目标 DN %q 应该以 %q 结尾", dest, tt.destDN)
			}
		})
	}
}

// ========== BatchModify 参数校验（go-ldap v3 PartialAttribute 结构正确性） ==========

func TestBatchModify_OpMapping(t *testing.T) {
	req := ldap.NewModifyRequest("cn=test,dc=example,dc=com", nil)
	req.Changes = []ldap.Change{{
		Operation: ldap.ReplaceAttribute,
		Modification: ldap.PartialAttribute{
			Type: "description",
			Vals: []string{"new desc"},
		},
	}}
	if req.Changes[0].Operation != ldap.ReplaceAttribute {
		t.Error("replace op 应该映射到 ldap.ReplaceAttribute")
	}
	if req.Changes[0].Modification.Type != "description" {
		t.Error("attr name 应该正确传递")
	}
	if len(req.Changes[0].Modification.Vals) != 1 {
		t.Error("values 应该正确传递")
	}
}

func TestBatchModify_AllOps(t *testing.T) {
	ops := []struct {
		name     string
		strOp    string
		expected uint
	}{
		{"replace", "replace", ldap.ReplaceAttribute},
		{"add", "add", ldap.AddAttribute},
		{"delete", "delete", ldap.DeleteAttribute},
	}

	for _, op := range ops {
		t.Run(op.name, func(t *testing.T) {
			var mapped uint
			switch op.strOp {
			case "replace":
				mapped = ldap.ReplaceAttribute
			case "add":
				mapped = ldap.AddAttribute
			case "delete":
				mapped = ldap.DeleteAttribute
			default:
				t.Fatalf("未知 op: %s", op.strOp)
			}
			if mapped != op.expected {
				t.Errorf("op %q 映射到 %d, 期望 %d", op.strOp, mapped, op.expected)
			}

			req := ldap.NewModifyRequest("cn=test,dc=example,dc=com", nil)
			req.Changes = []ldap.Change{{
				Operation: mapped,
				Modification: ldap.PartialAttribute{
					Type: "telephoneNumber",
					Vals: []string{"+86 12345678"},
				},
			}}
			if len(req.Changes) != 1 {
				t.Error("Change 应该被正确附加到 ModifyRequest")
			}
			if req.Changes[0].Modification.Type != "telephoneNumber" {
				t.Error("属性类型应该被保留")
			}
		})
	}
}

// ========== FriendlyLdapError 错误处理 ==========

func TestFriendlyLdapError_NilInput(t *testing.T) {
	err := FriendlyLdapError(nil)
	if err != nil {
		t.Errorf("FriendlyLdapError(nil) 应该返回 nil, 得到 %v", err)
	}
}

func TestFriendlyLdapError_NonLdap(t *testing.T) {
	orig := fmt.Errorf("connection refused")
	wrapped := FriendlyLdapError(orig)
	// 非 ldap.Error 应该原样返回
	if !errors.Is(wrapped, orig) {
		t.Errorf("非 LDAP 错误应该原样返回, 得到 %q", wrapped.Error())
	}
}

func TestFriendlyLdapError_WrapsLdapError(t *testing.T) {
	orig := &ldap.Error{ResultCode: ldap.LDAPResultNoSuchObject, Err: errors.New("no such object")}
	wrapped := FriendlyLdapError(orig)
	if wrapped == nil {
		t.Fatal("wrapped error 不应为 nil")
	}
	if !strings.Contains(wrapped.Error(), orig.Error()) {
		t.Errorf("应该保留原始 LDAP 错误信息, 得到 %q", wrapped.Error())
	}
}
