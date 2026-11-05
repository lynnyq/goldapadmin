package ldap

import (
	"fmt"
	"strings"
)

// Filter 校验常量
const (
	// MaxFilterLength filter 最大长度（超过则视为异常请求，直接拒绝）
	// 合理 filter 通常远低于此值，过高意味着注入或异常
	MaxFilterLength = 512

	// MaxFilterNesting filter 最大括号嵌套深度
	// 深度过大如 (((((((((((objectClass=*)))))))))))) 是注入特征
	MaxFilterNesting = 6

	// DefaultSearchSizeLimit 默认搜索 sizeLimit（服务端 sizeLimit=0 通常表示无限制，
	// 可能导致全量拉取压垮 LDAP）；我们强制给一个合理上限
	DefaultSearchSizeLimit = 1000
)

// ValidateFilter 校验 LDAP filter 字符串的基本语法与安全边界
//
// 检查项：
//  1. 长度不超过 MaxFilterLength
//  2. 括号嵌套深度不超过 MaxFilterNesting
//  3. 括号配对（不配对说明语法错误或注入）
//  4. filter 不能为空或为纯括号序列
//
// 注意：本函数做的是防御性粗校验，不是完整 ASN.1/BER 解析。
// 目的是拦截明显的注入尝试和异常长/深的 filter，真正的过滤能力在 LDAP 服务端。
func ValidateFilter(filter string) error {
	filter = strings.TrimSpace(filter)
	if filter == "" {
		return fmt.Errorf("filter 不能为空")
	}
	if len(filter) > MaxFilterLength {
		return fmt.Errorf("filter 过长（%d 字符 > %d），可能是异常请求", len(filter), MaxFilterLength)
	}

	depth := 0
	maxDepth := 0
	for _, ch := range filter {
		switch ch {
		case '(':
			depth++
			if depth > MaxFilterNesting {
				return fmt.Errorf("filter 括号嵌套过深（%d 层），可能是注入尝试", depth)
			}
			if depth > maxDepth {
				maxDepth = depth
			}
		case ')':
			depth--
			if depth < 0 {
				return fmt.Errorf("filter 括号不配对（多余的 ')'）")
			}
		}
	}
	if depth != 0 {
		return fmt.Errorf("filter 括号不配对（缺少 %d 个 ')'）", depth)
	}

	// 简单防空 filter：不能只有括号
	if maxDepth == 0 || filter == "()" || filter == "(())" || isOnlyParens(filter) {
		return fmt.Errorf("filter 内容为空或无效")
	}

	return nil
}

// isOnlyParens 判断字符串是否只包含括号（合法 filter 不能只有括号嵌套）
func isOnlyParens(s string) bool {
	for _, ch := range s {
		if ch != '(' && ch != ')' {
			return false
		}
	}
	return true
}

// ValidateScope 校验 LDAP scope 参数合法值
func ValidateScope(scope string) (string, error) {
	switch scope {
	case "base", "one", "sub":
		return scope, nil
	default:
		return "", fmt.Errorf("scope 必须是 base / one / sub，当前值 %q", scope)
	}
}
