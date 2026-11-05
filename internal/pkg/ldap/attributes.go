package ldap

import (
	"fmt"
	"strings"

	"github.com/lynnyq/goldapadmin/internal/model"
)

// MaxSubtreeEntries 单次子树/全量查询的条目上限
//
// 用于替代 sizeLimit=0（无限）查询，避免大目录下把 LDAP 服务器与本地进程拖垮。
const MaxSubtreeEntries = 3000

// sensitiveAttributes 需要在 API 响应中脱敏的属性（密码哈希 / 凭据类）
//
// 对标 phpLDAPadmin 默认隐藏 userPassword 的行为：即使 LDAP 服务器的 ACL 允许
// 当前绑定账户读取哈希，也不应把哈希原样返回给前端（客户端可离线爆破）。
var sensitiveAttributes = map[string]struct{}{
	"userpassword":    {},
	"unicodepwd":      {},
	"lmhash":          {},
	"nthash":          {},
	"ntpassword":      {},
	"sambalmpassword": {},
	"sambantpassword": {},
	"ms-mcs-admpwd":   {},
	"krbprincipalkey": {},
	"userpkcs12":      {},
}

// maskedValue 脱敏后的占位值（前端仍能看到属性存在，但拿不到真实内容）
const maskedValue = "******"

// IsSensitiveAttribute 判断属性是否属于敏感属性
func IsSensitiveAttribute(name string) bool {
	_, ok := sensitiveAttributes[strings.ToLower(strings.TrimSpace(name))]
	return ok
}

// MaskSensitiveAttributes 将敏感属性值替换为占位符（原地修改）
func MaskSensitiveAttributes(attrs []model.LdapAttribute) {
	for i := range attrs {
		if !IsSensitiveAttribute(attrs[i].Name) {
			continue
		}
		values := make([]string, len(attrs[i].Values))
		for j := range values {
			values[j] = maskedValue
		}
		attrs[i].Values = values
		attrs[i].Masked = true
	}
}

// ValidateAttributeName 校验单个属性名（RFC 4512 属性描述符）
//
// 允许字母、数字、连字符、点、下划线、分号与 OID；拒绝 "*"、"+" 等通配符，
// 防止调用方通过 attributes 参数索取全部属性（进而拿到密码哈希）。
func ValidateAttributeName(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("属性名不能为空")
	}
	if len(name) > 128 {
		return fmt.Errorf("属性名过长")
	}
	if name == "*" || name == "+" || strings.EqualFold(name, "dn") {
		return fmt.Errorf("不支持在属性列表中显式指定 %q", name)
	}
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		case r == '-', r == '_', r == '.', r == ';':
		default:
			return fmt.Errorf("属性名包含非法字符 %q", string(r))
		}
	}
	return nil
}

// ValidateAttributeNames 批量校验属性名
func ValidateAttributeNames(names []string) error {
	for _, name := range names {
		if err := ValidateAttributeName(name); err != nil {
			return err
		}
	}
	return nil
}
