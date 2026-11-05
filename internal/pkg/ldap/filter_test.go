package ldap

import "testing"

func TestValidateFilter(t *testing.T) {
	tests := []struct {
		name    string
		filter  string
		wantErr bool
	}{
		// --- 合法 filter ---
		{"simple filter", "(cn=yangqian)", false},
		{"multi attr", "(&(cn=yangqian)(objectClass=inetOrgPerson))", false},
		{"complex OR", "(|(cn=a)(cn=b)(cn=c))", false},
		{"wildcard", "(uid=*)", false},
		{"nested AND", "(&(a=1)(&(b=2)(c=3)))", false},

		// --- 非法 / 可疑 filter ---
		{"empty", "", true},
		{"whitespace only", "   ", true},
		{"only parens", "()", true},
		{"nested only", "((()))", true},
		{"too deep nesting", "((((((((((objectClass=*))))))))))", true}, // 10 层 > 6
		{"length too long", func() string {
			// "(cn=" + 508 a's + ")" = 513 字符 > MaxFilterLength (512)
			s := "(cn="
			for i := 0; i < 508; i++ {
				s += "a"
			}
			s += ")"
			return s
		}(), true},
		{"unclosed left", "(cn=yangqian", true},
		{"extra right", "(cn=yangqian))", true},
		{"unclosed pair", "((cn=a)", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateFilter(tt.filter)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateFilter() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateScope(t *testing.T) {
	tests := []struct {
		name    string
		scope   string
		wantErr bool
	}{
		{"base", "base", false},
		{"one", "one", false},
		{"sub", "sub", false},
		{"empty", "", true},
		{"invalid", "tree", true},
		{"sql injection", "sub; DROP TABLE", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ValidateScope(tt.scope)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateScope() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
