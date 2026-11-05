package api

import (
	"fmt"

	"github.com/go-ldap/ldap/v3"
)

// validateDN 校验 DN 字符串的语法合法性
//
// LDAP DN 有严格的语法规则：每个 RDN 是 attr=value 对，多个 RDN 用逗号分隔，
// value 中的特殊字符（, + " \ < > ; = # 以及前导/尾随空格）需要转义。
// 非法 DN 会在服务端触发 "Invalid DN Syntax" 错误，不如在请求层提前拦截。
func validateDN(dn string) error {
	if dn == "" {
		return fmt.Errorf("DN 不能为空")
	}
	if _, err := ldap.ParseDN(dn); err != nil {
		return fmt.Errorf("DN 格式非法 %q: %w（提示：RDN 值中的逗号、加号、空格等需要转义）", dn, err)
	}
	return nil
}

// validateDNPair 校验源 DN 和目标 DN 格式，用于 Copy/Move 等需要两个 DN 的接口
func validateDNPair(srcDN, destDN string) error {
	if err := validateDN(srcDN); err != nil {
		return fmt.Errorf("源 DN %w", err)
	}
	if err := validateDN(destDN); err != nil {
		return fmt.Errorf("目标 DN %w", err)
	}
	return nil
}
