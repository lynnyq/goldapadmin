package ldap

import (
	"bufio"
	"encoding/base64"
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

// maxImportEntries 单次 LDIF 导入的条目上限，防止超大文件打爆进程与 LDAP 服务器
const maxImportEntries = 1000

// maxImportValueLength 单条属性值长度上限（字节）
const maxImportValueLength = 1 << 20

// Import 导入 LDIF 内容
//
// 安全限制：最多 maxImportEntries 条；每条必须包含合法 dn；若 baseDN 非空，则要求
// 条目 DN 位于该子树内；拒绝 changetype=delete/modify/modrdn（避免误删或语义错乱）。
func (s *LdifService) Import(conn *ldap.Conn, content, baseDN string) (*ImportResult, error) {
	result := &ImportResult{Errors: []string{}}

	entries, err := parseLdif(content)
	if err != nil {
		return nil, err
	}
	if len(entries) > maxImportEntries {
		return nil, fmt.Errorf("LDIF 条目数 %d 超过单次导入上限 %d", len(entries), maxImportEntries)
	}

	baseDN = strings.TrimSpace(baseDN)

	for _, entry := range entries {
		dn := strings.TrimSpace(entry["dn"])
		if dn == "" {
			result.Failed++
			result.Errors = append(result.Errors, "缺少 dn")
			continue
		}
		if _, err := ldap.ParseDN(dn); err != nil {
			result.Failed++
			result.Errors = append(result.Errors, fmt.Sprintf("dn 语法非法 %q: %v", dn, err))
			continue
		}
		if baseDN != "" && !isDNWithinBase(dn, baseDN) {
			result.Failed++
			result.Errors = append(result.Errors, fmt.Sprintf("dn %q 不在允许导入的 BaseDN %q 范围内", dn, baseDN))
			continue
		}

		delete(entry, "dn")

		changetype := strings.ToLower(strings.TrimSpace(entry["changetype"]))
		delete(entry, "changetype")

		if changetype != "" && changetype != "add" {
			// 明确拒绝删除/修改类指令：导入接口只做新增，避免误删与语义歧义
			result.Failed++
			result.Errors = append(result.Errors, fmt.Sprintf("dn %q 的 changetype=%s 不被支持（仅支持新增）", dn, changetype))
			continue
		}

		addReq := ldap.NewAddRequest(dn, nil)
		tooLong := false
		for attr, values := range entry {
			if strings.TrimSpace(attr) == "" {
				continue
			}
			if len(values) > maxImportValueLength {
				result.Failed++
				result.Errors = append(result.Errors, fmt.Sprintf("dn %q 属性 %s 的值过长", dn, attr))
				tooLong = true
				break
			}
			vals := strings.Split(values, "\n")
			addReq.Attribute(attr, vals)
		}
		if tooLong {
			continue
		}
		if err := conn.Add(addReq); err != nil {
			result.Failed++
			result.Errors = append(result.Errors, fmt.Sprintf("add %s: %v", dn, err))
		} else {
			result.Success++
		}
	}

	return result, nil
}

// isDNWithinBase 判断 dn 是否等于 baseDN 或位于其子树内（大小写不敏感）
func isDNWithinBase(dn, baseDN string) bool {
	if strings.EqualFold(strings.TrimSpace(dn), strings.TrimSpace(baseDN)) {
		return true
	}
	return strings.HasSuffix(strings.ToLower(strings.TrimSpace(dn)), ","+strings.ToLower(strings.TrimSpace(baseDN)))
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
		MaxSubtreeEntries, 0, false,
		"(objectClass=*)",
		[]string{"*", "+"},
		nil,
	)
	result, err := conn.Search(req)
	if err != nil {
		var ldapErr *ldap.Error
		if errors.As(err, &ldapErr) && ldapErr.ResultCode == ldap.LDAPResultSizeLimitExceeded {
			return "", fmt.Errorf("Export: 条目数超过单次导出上限 %d（已拉 %d 条被截断），请缩小导出范围", MaxSubtreeEntries, len(result.Entries))
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
//
// 支持：注释、空行分隔、续行折叠（以空格开头）、base64 值（attr:: base64）。
// 空属性值（attr:）会被保留为空串；非法 base64 直接返回错误，避免把编码串当明文写入。
func parseLdif(content string) ([]map[string]string, error) {
	var entries []map[string]string
	current := make(map[string]string)
	lastAttr := ""

	flush := func() {
		if len(current) > 0 {
			entries = append(entries, current)
			current = make(map[string]string)
			lastAttr = ""
		}
	}

	scanner := bufio.NewScanner(strings.NewReader(content))
	scanner.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	for scanner.Scan() {
		line := scanner.Text()

		if strings.HasPrefix(line, "#") {
			continue
		}
		if strings.TrimSpace(line) == "" {
			flush()
			continue
		}

		// 续行：LDIF 折叠以单个空格开头，直接拼接到上一个属性值
		if strings.HasPrefix(line, " ") {
			if lastAttr == "" {
				return nil, fmt.Errorf("非法的 LDIF 续行（缺少前序属性）")
			}
			current[lastAttr] += line[1:]
			continue
		}

		idx := strings.Index(line, ":")
		if idx < 0 {
			return nil, fmt.Errorf("非法的 LDIF 行: %q", line)
		}

		attr := strings.TrimSpace(line[:idx])
		if attr == "" {
			return nil, fmt.Errorf("非法的 LDIF 行（属性为空）: %q", line)
		}
		rest := line[idx+1:]

		// base64 编码值：attr:: <base64>
		if strings.HasPrefix(rest, ":") {
			encoded := strings.TrimSpace(rest[1:])
			decoded, err := base64.StdEncoding.DecodeString(encoded)
			if err != nil {
				return nil, fmt.Errorf("属性 %s 的 base64 值解码失败: %w", attr, err)
			}
			rest = string(decoded)
			if len(rest) > maxImportValueLength {
				return nil, fmt.Errorf("属性 %s 的值超过长度上限", attr)
			}
		} else {
			rest = strings.TrimPrefix(rest, " ")
		}

		if _, ok := current[attr]; ok {
			current[attr] += "\n" + rest
		} else {
			current[attr] = rest
		}
		lastAttr = attr
	}

	flush()
	return entries, scanner.Err()
}
