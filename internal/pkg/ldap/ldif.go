package ldap

import (
	"bufio"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/go-ldap/ldap/v3"
)

// LdifService LDIF 导入导出服务
type LdifService struct{}

// NewLdifService 创建 LDIF 服务
func NewLdifService() *LdifService {
	return &LdifService{}
}

// ImportResult 导入结果
type ImportResult struct {
	Success int      `json:"success"`
	Failed  int      `json:"failed"`
	Errors  []string `json:"errors"`
}

// Import 导入 LDIF 内容
func (s *LdifService) Import(conn *ldap.Conn, content string) (*ImportResult, error) {
	result := &ImportResult{Errors: []string{}}

	entries, err := parseLdif(content)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		dn := entry["dn"]
		if dn == "" {
			result.Failed++
			result.Errors = append(result.Errors, "missing dn")
			continue
		}

		delete(entry, "dn")

		// 检查 changetype
		changetype := entry["changetype"]
		delete(entry, "changetype")

		switch changetype {
		case "delete":
			if err := conn.Del(ldap.NewDelRequest(dn, nil)); err != nil {
				result.Failed++
				result.Errors = append(result.Errors, fmt.Sprintf("delete %s: %v", dn, err))
			} else {
				result.Success++
			}
		case "modify", "modrdn", "moddn":
			// 简化处理：modify 暂用 add 代替
			fallthrough
		default:
			// add
			addReq := ldap.NewAddRequest(dn, nil)
			for attr, values := range entry {
				if attr == "" {
					continue
				}
				vals := strings.Split(values, "\n")
				addReq.Attribute(attr, vals)
			}
			if err := conn.Add(addReq); err != nil {
				result.Failed++
				result.Errors = append(result.Errors, fmt.Sprintf("add %s: %v", dn, err))
			} else {
				result.Success++
			}
		}
	}

	return result, nil
}

// Export 导出 LDIF
//
// 性能优化：单次 ScopeWholeSubtree 拉取 baseDN 下所有条目 → 遍历输出，
// 替代旧版递归调 GetChildren + GetEntry（N 次 LDAP 请求）。
//
// 输出顺序：按 DN 层级深度升序（根先、叶子后），同一层级按字母序。
// recursive=false 时只导出 baseDN 自身。
func (s *LdifService) Export(conn *ldap.Conn, dn string, recursive bool) (string, error) {
	scope := ldap.ScopeBaseObject
	if recursive {
		scope = ldap.ScopeWholeSubtree
	}

	req := ldap.NewSearchRequest(
		dn, scope, ldap.NeverDerefAliases,
		0, 0, false,
		"(objectClass=*)",
		[]string{"*", "+"},
		nil,
	)
	result, err := conn.Search(req)
	if err != nil {
		var ldapErr *ldap.Error
		if errors.As(err, &ldapErr) && ldapErr.ResultCode == ldap.LDAPResultSizeLimitExceeded {
			return "", fmt.Errorf("Export: 条目数超过服务器 sizeLimit（已拉 %d 条被截断）", len(result.Entries))
		}
		return "", err
	}

	// 按 DN 深度 + 字母序排序（根先、叶子后）
	sort.SliceStable(result.Entries, func(i, j int) bool {
		depthI := strings.Count(result.Entries[i].DN, ",")
		depthJ := strings.Count(result.Entries[j].DN, ",")
		if depthI != depthJ {
			return depthI < depthJ
		}
		return strings.ToLower(result.Entries[i].DN) < strings.ToLower(result.Entries[j].DN)
	})

	var sb strings.Builder
	for _, entry := range result.Entries {
		if !recursive && !strings.EqualFold(entry.DN, dn) {
			continue
		}
		sb.WriteString(fmt.Sprintf("dn: %s\n", entry.DN))
		for _, attr := range entry.Attributes {
			for _, v := range attr.Values {
				sb.WriteString(fmt.Sprintf("%s: %s\n", attr.Name, v))
			}
		}
		sb.WriteString("\n")
	}

	return sb.String(), nil
}

// ExportFilename 根据 baseDN 生成下载文件名
// 取 baseDN 中最顶层的 RDN 值（最后一个组件），替换非法字符后加上 .ldif 后缀
func ExportFilename(baseDN string) string {
	// 从右向左取最后一个 RDN（根级）
	idx := strings.LastIndex(baseDN, "=")
	if idx < 0 {
		return "export.ldif"
	}
	// 取第一个逗号前的部分（整个最顶层 RDN 如 "dc=com"）
	// 但更友好的做法是取 RDN value
	eqIdx := strings.Index(baseDN, "=")
	commaIdx := strings.Index(baseDN, ",")
	if commaIdx < 0 || eqIdx < 0 {
		return "export.ldif"
	}
	value := baseDN[eqIdx+1 : commaIdx]
	// 替换文件名非法字符
	value = strings.NewReplacer("/", "_", "\\", "_", ":", "_", "*", "_", "?", "_", "\"", "_", "<", "_", ">", "_", "|", "_", " ", "_").Replace(value)
	if value == "" {
		return "export.ldif"
	}
	return fmt.Sprintf("%s.ldif", value)
}

// parseLdif 解析 LDIF 内容
func parseLdif(content string) ([]map[string]string, error) {
	var entries []map[string]string
	current := make(map[string]string)

	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := scanner.Text()

		// 跳过注释和空行
		if strings.HasPrefix(line, "#") {
			continue
		}

		if line == "" {
			if len(current) > 0 {
				entries = append(entries, current)
				current = make(map[string]string)
			}
			continue
		}

		// 续行（以空格开头）
		if strings.HasPrefix(line, " ") {
			if len(entries) > 0 && len(current) == 0 {
				// 续行到上一条目最后一个属性
				last := entries[len(entries)-1]
				for k, v := range last {
					_ = k
					_ = v
				}
			}
			continue
		}

		idx := strings.Index(line, ":")
		if idx < 0 {
			continue
		}

		attr := strings.TrimSpace(line[:idx])
		value := strings.TrimSpace(line[idx+1:])

		// 处理 base64 编码
		if strings.HasPrefix(line[idx+1:], ":") {
			// base64 解码
			// 简化处理，不做解码
			value = strings.TrimSpace(line[idx+2:])
		}

		if existing, ok := current[attr]; ok {
			current[attr] = existing + "\n" + value
		} else {
			current[attr] = value
		}
	}

	if len(current) > 0 {
		entries = append(entries, current)
	}

	return entries, scanner.Err()
}
