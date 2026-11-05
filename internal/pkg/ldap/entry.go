package ldap

import (
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"errors"
	"fmt"
	"hash"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/go-ldap/ldap/v3"
	"github.com/lynnyq/goldapadmin/internal/model"
)

// BINARY_ATTRS 典型二进制属性（对标 phpLDAPadmin view_jpeg_photo.php / download_binary_attr.php）
// 这些属性前端会以 base64 字符串传值，后端需解码为 []byte 走 BytesVals 路径
var BINARY_ATTRS = map[string]struct{}{
	"jpegphoto":                          {},
	"thumbnailphoto":                     {},
	"usercertificate":                    {},
	"userpkcs12":                         {},
	"cacertificate":                      {},
	"crosscertificatepair":               {},
	"certificaterevocationlist":          {},
	"authoritycertificaterevocationlist": {},
	"audio":                              {},
	"supportedalgorithms":                {},
}

// ErrSizeLimitTruncated LDAP 搜索结果因服务器 sizelimit 被截断，
// 但 Search / GetNextID 等函数仍返回了部分数据。
// 调用方可用 errors.Is(err, ErrSizeLimitTruncated) 判断并决定是接受部分结果还是提示用户。
var ErrSizeLimitTruncated = errors.New("LDAP search truncated by server size limit")

// maxRecursiveDeleteCount 单次递归删除安全上限（条目数）
const maxRecursiveDeleteCount = 1000

// EntryService LDAP 条目操作服务
type EntryService struct{}

// NewEntryService 创建条目服务
func NewEntryService() *EntryService {
	return &EntryService{}
}

// handleSizeLimit 将 go-ldap Conn.Search 返回的 SizeLimitExceeded 标准化：
//   - 若 result.Entries 非空 → 返回部分结果 + ErrSizeLimitTruncated sentinel error
//   - 若 result.Entries 为空 → 返回原始错误
//   - 非 SizeLimitExceeded 错误 → 原样返回
//
// 对标 phpLDAPadmin：即使被截断也渲染可见的部分结果，而不是白屏报错。
func handleSizeLimit(result *ldap.SearchResult, err error) ([]*ldap.Entry, error) {
	if err == nil {
		return result.Entries, nil
	}
	var ldapErr *ldap.Error
	if errors.As(err, &ldapErr) && ldapErr.ResultCode == ldap.LDAPResultSizeLimitExceeded {
		if len(result.Entries) > 0 {
			return result.Entries, ErrSizeLimitTruncated
		}
	}
	return nil, err
}

// GetChildren 获取子节点列表（对标 phpLDAPadmin 树懒加载）
//
// 性能优化（消除 N+1 查询）：
//
//	单次 sizeLimit=0 的 ScopeWholeSubtree 拿到 baseDN 下所有条目 DN（不算 baseDN 自己），
//	构建 DN→是否有子孙 的倒排索引，O(1) 查出每个直接子节点的 HasChildren。
//	代价：大目录下会拉全量 DN；目录节点数 >1000 时触发后端 sizelimit 保护。
//
// 改进点：
//  1. 排序按类型分组 + RDN 字母序
//  2. 额外拉 userAccountControl — 识别 AD 禁用账户
//  3. classifyEntryType 统一判断 domain/gpo/alias/referral
//  4. 返回 is_leaf 字段，前端 el-tree 直接消费
func (s *EntryService) GetChildren(conn *ldap.Conn, baseDN string) ([]model.TreeNode, error) {
	// Step 1: 拉全量子孙 DN（只用 dn 和 objectClass，最小属性集）
	subtreeReq := ldap.NewSearchRequest(
		baseDN,
		ldap.ScopeWholeSubtree,
		ldap.NeverDerefAliases,
		MaxSubtreeEntries, 0, false,
		"(objectClass=*)",
		[]string{"dn", "objectClass", "userAccountControl"},
		nil,
	)
	subtreeResult, err := conn.Search(subtreeReq)
	if err != nil {
		var ldapErr *ldap.Error
		if errors.As(err, &ldapErr) && ldapErr.ResultCode == ldap.LDAPResultSizeLimitExceeded {
			return nil, fmt.Errorf("子条目超过服务器 sizeLimit，请缩小 baseDN 或调整后端 sizelimit 参数")
		}
		return nil, err
	}

	// 构建：所有 DN set + 直接子节点列表（排除 baseDN 自身）
	allDNs := make(map[string]struct{}, len(subtreeResult.Entries))
	var directChildren []*ldap.Entry
	for _, e := range subtreeResult.Entries {
		if strings.EqualFold(e.DN, baseDN) {
			continue
		}
		allDNs[strings.ToLower(e.DN)] = struct{}{}

		// 计算 parentDN = 去掉 rdn 后的剩余部分
		rdn := getRDN(e.DN)
		parentDN := strings.TrimPrefix(e.DN, rdn+",")
		if strings.EqualFold(parentDN, baseDN) {
			directChildren = append(directChildren, e)
		}
	}

	// Step 2: 对每个直接子节点，判断它在 allDNs 中是否有子孙
	// 方法：任何其他 DN 以 ",子节点DN" 为后缀的 → 子节点有子孙
	// 例如子节点 ou=People,dc=example,dc=com，
	//   子 DN uid=yangqian,ou=people,dc=example,dc=com
	//   HasSuffix(lowerSub, ","+lowerChild) → true
	hasChildren := make(map[string]bool, len(directChildren))
	for _, e := range directChildren {
		suffix := "," + strings.ToLower(e.DN)
		for dn := range allDNs {
			if strings.HasSuffix(dn, suffix) {
				hasChildren[e.DN] = true
				break
			}
		}
	}

	// Step 3: 构造 TreeNode 数组
	nodes := make([]model.TreeNode, 0, len(directChildren))
	for _, entry := range directChildren {
		oc := entry.GetAttributeValues("objectClass")
		entryType := classifyEntryType(oc)
		disabled := isADDisabled(entry)
		hc := hasChildren[entry.DN]

		nodes = append(nodes, model.TreeNode{
			DN:            entry.DN,
			RDN:           getRDN(entry.DN),
			ObjectClasses: oc,
			HasChildren:   hc,
			IsLeaf:        !hc,
			EntryType:     entryType,
			Disabled:      disabled,
		})
	}

	// Step 4: 排序（类型分组 + RDN 字母序）
	sort.SliceStable(nodes, func(i, j int) bool {
		rankI, rankJ := entryTypeRank(nodes[i].EntryType), entryTypeRank(nodes[j].EntryType)
		if rankI != rankJ {
			return rankI < rankJ
		}
		return strings.ToLower(nodes[i].RDN) < strings.ToLower(nodes[j].RDN)
	})

	return nodes, nil
}

// classifyEntryType 按 objectClass 推断条目类型（对标 phpLDAPadmin 的图标判断）
func classifyEntryType(oc []string) string {
	classes := make(map[string]bool, len(oc))
	for _, c := range oc {
		classes[strings.ToLower(c)] = true
	}
	switch {
	case classes["domain"] || classes["domainrelatedobject"]:
		return "domain"
	case classes["organization"]:
		return "org"
	case classes["organizationalunit"]:
		return "ou"
	case classes["grouppolicycontainer"] || classes["grouppolicyobject"]:
		return "gpo"
	case classes["alias"]:
		return "alias"
	case classes["referral"]:
		return "referral"
	case classes["posixgroup"] || classes["groupofnames"] || classes["groupofuniquenames"] || classes["group"]:
		return "group"
	case classes["inetorgperson"] || classes["organizationalperson"] || classes["person"] || classes["user"]:
		return "user"
	}
	return "unknown"
}

// entryTypeRank 条目类型排序优先级（数字越小越靠前）
func entryTypeRank(t string) int {
	ranks := map[string]int{
		"domain":   0,
		"org":      1,
		"ou":       2,
		"gpo":      3,
		"group":    4,
		"user":     5,
		"alias":    6,
		"referral": 7,
	}
	if r, ok := ranks[t]; ok {
		return r
	}
	return 99
}

// isADDisabled 判断 AD 账户是否被禁用
// userAccountControl bit 2 = ACCOUNTDISABLE
func isADDisabled(entry *ldap.Entry) bool {
	vals := entry.GetAttributeValues("userAccountControl")
	if len(vals) == 0 {
		return false
	}
	val, err := strconv.Atoi(vals[0])
	if err != nil {
		return false
	}
	return val&2 != 0
}

// hasSubEntries 判断指定 DN 是否有直接子条目（用于懒加载叶子节点判定）
// 使用 sizeLimit=1 查询避免返回大量数据导致性能问题
// 注意：LDAP 协议中 sizeLimit 触发时服务器返回 ResultCode=4 (SizeLimitExceeded)，
// 这恰恰说明存在子条目，需要特殊处理
func (s *EntryService) hasSubEntries(conn *ldap.Conn, dn string) bool {
	searchRequest := ldap.NewSearchRequest(
		dn,
		ldap.ScopeSingleLevel,
		ldap.NeverDerefAliases,
		1, 0, false, // sizeLimit=1，只要有一条就够了
		"(objectClass=*)",
		[]string{"dn"},
		nil,
	)
	result, err := conn.Search(searchRequest)
	if err != nil {
		// sizeLimit 触发 — 有超过 1 条子条目，说明有子节点
		var ldapErr *ldap.Error
		if errors.As(err, &ldapErr) && ldapErr.ResultCode == ldap.LDAPResultSizeLimitExceeded {
			return true
		}
		// 其他错误，保守返回 false 让节点显示为叶子
		return false
	}
	return len(result.Entries) > 0
}

// GetTree 递归获取指定 baseDN 下所有子孙条目并构建嵌套树结构
// 使用 ScopeWholeSubtree 一次性拉全量数据，适合节点数较少（<500）的目录
//
// 隐患修复：
//   - DN 层级关系用 HasPrefix 判断（DN 中有转义逗号时 TrimPrefix 会出错）
//   - sizeLimit 触发时返回明确错误提示
//   - 每个节点同时填充 has_children / is_leaf
func (s *EntryService) GetTree(conn *ldap.Conn, baseDN string) ([]model.TreeNode, error) {
	searchRequest := ldap.NewSearchRequest(
		baseDN,
		ldap.ScopeWholeSubtree,
		ldap.NeverDerefAliases,
		MaxSubtreeEntries, 0, false,
		"(objectClass=*)",
		[]string{"dn", "objectClass"},
		nil,
	)

	result, err := conn.Search(searchRequest)
	if err != nil {
		var ldapErr *ldap.Error
		if errors.As(err, &ldapErr) && ldapErr.ResultCode == ldap.LDAPResultSizeLimitExceeded {
			return nil, fmt.Errorf("GetTree: 条目数超过服务器 sizeLimit（拉取了 %d 条被截断），请缩小 baseDN 或改走懒加载接口", len(result.Entries))
		}
		return nil, err
	}

	// 构建 DN 集合（Lowercase 用于不区分大小写比较）+ 所有条目
	type flatNode struct {
		dn string
		oc []string
	}
	flatList := make([]flatNode, 0, len(result.Entries))
	dnSet := make(map[string]struct{}, len(result.Entries))
	for _, e := range result.Entries {
		if strings.EqualFold(e.DN, baseDN) {
			continue
		}
		flatList = append(flatList, flatNode{dn: e.DN, oc: e.GetAttributeValues("objectClass")})
		dnSet[strings.ToLower(e.DN)] = struct{}{}
	}

	// 构建 childrenMap：parentDN → []TreeNode
	// 安全找父节点：对每个节点 dn，找 flatList 中所有候选，若节点 DN 以 ",候选DN" 为后缀则候选是祖先
	// 取"最短"的那个祖先（离自己最近的）作为直接父节点
	childrenMap := make(map[string][]model.TreeNode)

	for _, node := range flatList {
		nodeLower := strings.ToLower(node.dn)
		var parentDN string
		parentLen := 1 << 30
		for _, other := range flatList {
			if other.dn == node.dn {
				continue
			}
			// other 候选是 node 的祖先 ⟺ node.dn 以 ","+other.dn 为后缀
			suffix := "," + strings.ToLower(other.dn)
			if strings.HasSuffix(nodeLower, suffix) {
				if len(other.dn) < parentLen {
					parentDN = other.dn
					parentLen = len(other.dn)
				}
			}
		}

		entryType := classifyEntryType(node.oc)
		treeNode := model.TreeNode{
			DN:            node.dn,
			RDN:           getRDN(node.dn),
			ObjectClasses: node.oc,
			EntryType:     entryType,
		}

		if parentDN == "" {
			parentDN = baseDN
		}
		childrenMap[parentDN] = append(childrenMap[parentDN], treeNode)
	}

	// 递归构建嵌套 + 填充 HasChildren / IsLeaf
	var build func(dn string) []model.TreeNode
	build = func(dn string) []model.TreeNode {
		kids := childrenMap[dn]
		// 按类型 + RDN 排序保持一致
		sort.SliceStable(kids, func(i, j int) bool {
			rankI, rankJ := entryTypeRank(kids[i].EntryType), entryTypeRank(kids[j].EntryType)
			if rankI != rankJ {
				return rankI < rankJ
			}
			return strings.ToLower(kids[i].RDN) < strings.ToLower(kids[j].RDN)
		})
		out := make([]model.TreeNode, len(kids))
		for i, kid := range kids {
			sub := build(kid.DN)
			kid.HasChildren = len(sub) > 0
			kid.IsLeaf = !kid.HasChildren
			kid.Children = sub
			out[i] = kid
		}
		return out
	}

	return build(baseDN), nil
}

// GetEntry 获取条目详情
func (s *EntryService) GetEntry(conn *ldap.Conn, dn string) (*model.LdapEntry, error) {
	searchRequest := ldap.NewSearchRequest(
		dn,
		ldap.ScopeBaseObject,
		ldap.NeverDerefAliases,
		0, 0, false,
		"(objectClass=*)",
		[]string{"*", "+"},
		nil,
	)

	result, err := conn.Search(searchRequest)
	if err != nil {
		return nil, err
	}

	if len(result.Entries) == 0 {
		return nil, fmt.Errorf("entry not found: %s", dn)
	}

	entry := result.Entries[0]
	attributes := make([]model.LdapAttribute, 0, len(entry.Attributes))
	for _, attr := range entry.Attributes {
		attributes = append(attributes, model.LdapAttribute{
			Name:   attr.Name,
			Values: attr.Values,
		})
	}
	// 密码哈希等敏感属性在返回前脱敏，避免客户端拿到可离线爆破的凭据
	MaskSensitiveAttributes(attributes)

	return &model.LdapEntry{
		DN:            entry.DN,
		ObjectClasses: entry.GetAttributeValues("objectClass"),
		Attributes:    attributes,
	}, nil
}

// CreateEntry 创建条目
// 先用 ldap.ParseDN 预检 DN 合法性，非法 DN（如含未转义的 , + " \ < > ;）
// 返回明确的错误提示，而不是依赖服务端的 Invalid DN Syntax
// passwordAlgos 指定密码属性的哈希算法（对标 phpLDAPadmin 的 password helper），
// key 为属性名（如 userPassword），value 为 SSHA/SHA/MD5/空(明文)
func (s *EntryService) CreateEntry(conn *ldap.Conn, dn string, attrs map[string][]string, passwordAlgos map[string]string) error {
	if _, err := ldap.ParseDN(dn); err != nil {
		return fmt.Errorf("DN 格式非法 %q: %w（提示：RDN 值中的逗号、加号等特殊字符需要转义）", dn, err)
	}
	// 密码属性按指定算法哈希，对标 phpLDAPadmin 的 password helper
	for attr, algo := range passwordAlgos {
		if values, ok := attrs[attr]; ok && algo != "" && !strings.EqualFold(algo, "plain") {
			hashed := make([]string, 0, len(values))
			for _, v := range values {
				h, hashErr := HashPassword(algo, v)
				if hashErr != nil {
					return fmt.Errorf("密码哈希失败: %w", hashErr)
				}
				hashed = append(hashed, h)
			}
			attrs[attr] = hashed
		}
	}
	addRequest := ldap.NewAddRequest(dn, nil)
	for attr, values := range attrs {
		vals := values
		if isBinaryAttr(attr) {
			// 二进制属性：base64 解码后强转 string（Go string 底层是 []byte）
			// go-ldap 序列化时会按原始字节发 OCTET STRING — PLA 后端同逻辑
			vals = make([]string, 0, len(values))
			for _, v := range values {
				raw, err := base64.StdEncoding.DecodeString(v)
				if err != nil {
					return fmt.Errorf("二进制属性 %s base64 解码失败: %w", attr, err)
				}
				vals = append(vals, string(raw))
			}
		}
		addRequest.Attribute(attr, vals)
	}
	if err := conn.Add(addRequest); err != nil {
		return FriendlyLdapError(err)
	}
	return nil
}

// FriendlyLdapError 将常见 LDAP 错误码翻译为可操作的中文提示（对标 phpLDAPadmin 的错误提示体验）
func FriendlyLdapError(err error) error {
	var le *ldap.Error
	if !errors.As(err, &le) {
		return err
	}
	switch le.ResultCode {
	case ldap.LDAPResultInvalidAttributeSyntax:
		return fmt.Errorf("%w（提示：属性值语法非法；若报错属性为 objectClass，通常是服务器未加载对应 Schema，例如 posixGroup 需要加载 nis.schema）", err)
	case ldap.LDAPResultObjectClassViolation:
		return fmt.Errorf("%w（提示：违反 objectClass 约束，通常是缺少该 objectClass 的必填属性，或 STRUCTURAL objectClass 链不完整）", err)
	case ldap.LDAPResultEntryAlreadyExists:
		return fmt.Errorf("%w（提示：该 DN 的条目已存在）", err)
	case ldap.LDAPResultNoSuchObject:
		return fmt.Errorf("%w（提示：父 DN 不存在，请确认父条目已创建）", err)
	case ldap.LDAPResultNamingViolation:
		return fmt.Errorf("%w（提示：命名违规，RDN 属性值必须与条目属性一致，且需符合 Schema 定义）", err)
	case ldap.LDAPResultAttributeOrValueExists:
		return fmt.Errorf("%w（提示：属性或属性值已存在）", err)
	case ldap.LDAPResultConstraintViolation:
		return fmt.Errorf("%w（提示：属性值不符合约束（如长度、单值属性填了多个值、密码格式非法））", err)
	}
	return err
}

// GetNextID 获取下一个可用的 uidNumber/gidNumber（对标 phpLDAPadmin 的自增 ID 分配）
// 搜索 baseDN 下所有 posixAccount/posixGroup 条目，取现有最大值 +1；无记录时从 30000 开始
// kind: "uid" 查询 uidNumber，"gid" 查询 gidNumber
func (s *EntryService) GetNextID(conn *ldap.Conn, baseDN, kind string) (int, error) {
	filter, attr := "(objectClass=posixAccount)", "uidNumber"
	if kind == "gid" {
		filter, attr = "(objectClass=posixGroup)", "gidNumber"
	}

	searchRequest := ldap.NewSearchRequest(
		baseDN,
		ldap.ScopeWholeSubtree,
		ldap.NeverDerefAliases,
		MaxSubtreeEntries, 0, false,
		filter,
		[]string{attr},
		nil,
	)
	result, err := conn.Search(searchRequest)
	// SizeLimitExceeded 时用已拉取的部分数据算一个保守的 nextID（可能比真实值小，
	// 但不会导致重复创建后再被服务器拒绝），同时附带 sentinel 让调用方知道不精确
	rawEntries, slErr := handleSizeLimit(result, err)
	if slErr != nil && !errors.Is(slErr, ErrSizeLimitTruncated) {
		return 0, err
	}

	max := 29999 // 无现有记录时从 30000 开始
	for _, entry := range rawEntries {
		for _, v := range entry.GetAttributeValues(attr) {
			if n, convErr := strconv.Atoi(strings.TrimSpace(v)); convErr == nil && n > max {
				max = n
			}
		}
	}
	return max + 1, slErr
}

// ModifyEntry 修改条目（支持密码属性哈希 + 二进制属性 base64 解码，对标 phpLDAPadmin）
// passwordAlgos 指定密码属性的哈希算法，key=属性名，value=SSHA/SHA/MD5/空(明文)
// 二进制属性（见 BINARY_ATTRS）前端以 base64 字符串传值，后端解码为 []byte 走 BytesVals 路径
func (s *EntryService) ModifyEntry(conn *ldap.Conn, dn string, changes []model.ModifyChange, passwordAlgos map[string]string) error {
	modifyRequest := ldap.NewModifyRequest(dn, nil)
	for _, change := range changes {
		isBinary := isBinaryAttr(change.Attr)

		// 密码属性按指定算法哈希
		values := change.Values
		if !isBinary {
			if algo, ok := passwordAlgos[change.Attr]; ok && algo != "" && !strings.EqualFold(algo, "plain") {
				hashed := make([]string, 0, len(values))
				for _, v := range values {
					h, hashErr := HashPassword(algo, v)
					if hashErr != nil {
						return fmt.Errorf("密码哈希失败: %w", hashErr)
					}
					hashed = append(hashed, h)
				}
				values = hashed
			}
		}

		var ldapOp uint
		switch change.Op {
		case "add":
			ldapOp = ldap.AddAttribute
		case "replace":
			ldapOp = ldap.ReplaceAttribute
		case "delete":
			ldapOp = ldap.DeleteAttribute
		default:
			return fmt.Errorf("不支持的 op: %s", change.Op)
		}

		if isBinary {
			// 二进制属性：base64 解码后 string(rawBytes)，go-ldap 按原始字节发 OCTET STRING
			decoded := make([]string, 0, len(values))
			for _, v := range values {
				raw, err := base64.StdEncoding.DecodeString(v)
				if err != nil {
					return fmt.Errorf("二进制属性 %s base64 解码失败: %w", change.Attr, err)
				}
				decoded = append(decoded, string(raw))
			}
			modifyRequest.Changes = append(modifyRequest.Changes, ldap.Change{
				Operation: ldapOp,
				Modification: ldap.PartialAttribute{
					Type: change.Attr,
					Vals: decoded,
				},
			})
		} else {
			modifyRequest.Changes = append(modifyRequest.Changes, ldap.Change{
				Operation: ldapOp,
				Modification: ldap.PartialAttribute{
					Type: change.Attr,
					Vals: values,
				},
			})
		}
	}

	return FriendlyLdapError(conn.Modify(modifyRequest))
}

// isBinaryAttr 判断属性名是否为二进制属性（对标 PLA 的二进制属性专门处理）
func isBinaryAttr(name string) bool {
	_, ok := BINARY_ATTRS[strings.ToLower(name)]
	return ok
}

// DeleteEntry 删除条目
func (s *EntryService) DeleteEntry(conn *ldap.Conn, dn string, recursive bool) error {
	if recursive {
		// 先预估将被删除的条目数，超过安全上限则拒绝执行
		dns, err := s.ListDescendants(conn, dn)
		if err == nil && len(dns) > maxRecursiveDeleteCount {
			return fmt.Errorf("递归删除拒绝执行：目标条目下有 %d 个子条目，超过安全上限 %d，请缩小范围或联系管理员", len(dns), maxRecursiveDeleteCount)
		}
		// 先删除所有子条目
		children, err := s.GetChildren(conn, dn)
		if err == nil {
			for _, child := range children {
				if err := s.DeleteEntry(conn, child.DN, true); err != nil {
					return fmt.Errorf("delete child %s: %w", child.DN, err)
				}
			}
		}
	}
	delRequest := ldap.NewDelRequest(dn, nil)
	return conn.Del(delRequest)
}

// CopyEntry 复制条目
func (s *EntryService) CopyEntry(conn *ldap.Conn, srcDN, destDN string, recursive bool) error {
	entry, err := s.GetEntry(conn, srcDN)
	if err != nil {
		return err
	}

	attrs := make(map[string][]string)
	for _, attr := range entry.Attributes {
		if strings.EqualFold(attr.Name, "dn") {
			continue
		}
		attrs[attr.Name] = attr.Values
	}

	if err := s.CreateEntry(conn, destDN, attrs, nil); err != nil {
		return err
	}

	if recursive {
		children, err := s.GetChildren(conn, srcDN)
		if err == nil {
			for _, child := range children {
				childDestDN := getRDN(child.DN) + "," + destDN
				if err := s.CopyEntry(conn, child.DN, childDestDN, true); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// CopyEntryCrossServer 跨服务器复制条目（对标 phpLDAPadmin copy_form.php 的 server 下拉选择）
// srcConn — 源服务器连接（读取条目），destConn — 目标服务器连接（写入条目）
func (s *EntryService) CopyEntryCrossServer(srcConn, destConn *ldap.Conn, srcDN, destDN string, recursive bool) error {
	entry, err := s.GetEntry(srcConn, srcDN)
	if err != nil {
		return fmt.Errorf("读取源条目失败: %w", err)
	}

	attrs := make(map[string][]string)
	for _, attr := range entry.Attributes {
		if strings.EqualFold(attr.Name, "dn") {
			continue
		}
		attrs[attr.Name] = attr.Values
	}

	if err := s.CreateEntry(destConn, destDN, attrs, nil); err != nil {
		return fmt.Errorf("写入目标条目失败: %w", err)
	}

	if recursive {
		children, err := s.GetChildren(srcConn, srcDN)
		if err != nil {
			return fmt.Errorf("读取子条目失败: %w", err)
		}
		for _, child := range children {
			childDestDN := getRDN(child.DN) + "," + destDN
			if err := s.CopyEntryCrossServer(srcConn, destConn, child.DN, childDestDN, true); err != nil {
				return err
			}
		}
	}
	return nil
}

// MoveEntry 移动条目
func (s *EntryService) MoveEntry(conn *ldap.Conn, srcDN, newParentDN, newRDN string) error {
	rdn := newRDN
	if rdn == "" {
		rdn = getRDN(srcDN)
	}
	modDNRequest := ldap.NewModifyDNRequest(srcDN, rdn, true, newParentDN)
	return conn.ModifyDN(modDNRequest)
}

// RenameEntry 重命名条目
func (s *EntryService) RenameEntry(conn *ldap.Conn, dn, newRDN string) error {
	modDNRequest := ldap.NewModifyDNRequest(dn, newRDN, true, "")
	return conn.ModifyDN(modDNRequest)
}

// Search 搜索条目
func (s *EntryService) Search(conn *ldap.Conn, baseDN, filter, scope string, attributes []string, sizeLimit int) ([]model.LdapEntry, error) {
	var searchScope int
	switch scope {
	case "base":
		searchScope = ldap.ScopeBaseObject
	case "one":
		searchScope = ldap.ScopeSingleLevel
	default:
		searchScope = ldap.ScopeWholeSubtree
	}

	if attributes == nil {
		attributes = []string{"dn", "objectClass", "cn", "uid", "mail"}
	}

	searchRequest := ldap.NewSearchRequest(
		baseDN,
		searchScope,
		ldap.NeverDerefAliases,
		sizeLimit, 0, false,
		filter,
		attributes,
		nil,
	)

	result, err := conn.Search(searchRequest)
	// SizeLimitExceeded 但有部分数据 → 返回部分结果 + sentinel error（对标 phpLDAPadmin 截断渲染）
	rawEntries, slErr := handleSizeLimit(result, err)
	if slErr != nil && !errors.Is(slErr, ErrSizeLimitTruncated) {
		return nil, slErr
	}

	entries := make([]model.LdapEntry, 0, len(rawEntries))
	for _, e := range rawEntries {
		attrs := make([]model.LdapAttribute, 0, len(e.Attributes))
		for _, a := range e.Attributes {
			attrs = append(attrs, model.LdapAttribute{Name: a.Name, Values: a.Values})
		}
		MaskSensitiveAttributes(attrs)
		entries = append(entries, model.LdapEntry{
			DN:            e.DN,
			ObjectClasses: e.GetAttributeValues("objectClass"),
			Attributes:    attrs,
		})
	}
	return entries, slErr
}

func getRDN(dn string) string {
	parts := splitDN(dn)
	if len(parts) > 0 {
		return parts[0]
	}
	return dn
}

func splitDN(dn string) []string {
	var parts []string
	var current strings.Builder
	escaping := false
	for _, ch := range dn {
		if escaping {
			current.WriteRune(ch)
			escaping = false
		} else if ch == '\\' {
			current.WriteRune(ch)
			escaping = true
		} else if ch == ',' {
			parts = append(parts, current.String())
			current.Reset()
		} else {
			current.WriteRune(ch)
		}
	}
	if current.Len() > 0 {
		parts = append(parts, current.String())
	}
	return parts
}

// CompareEntry 执行 LDAP Compare 操作（对标 phpLDAPadmin compare.php）
// 返回 true 表示属性值匹配，false 表示不匹配；错误由底层返回
func (s *EntryService) CompareEntry(conn *ldap.Conn, dn, attr, value string) (bool, error) {
	matched, err := conn.Compare(dn, attr, value)
	if err != nil {
		return false, FriendlyLdapError(err)
	}
	return matched, nil
}

// GetBinaryAttribute 获取指定条目上某个二进制属性的原始字节值
// 前端用 base64 编码展示 / 下载
func (s *EntryService) GetBinaryAttribute(conn *ldap.Conn, dn, attr string) ([]byte, error) {
	if IsSensitiveAttribute(attr) {
		return nil, fmt.Errorf("属性 %s 属于敏感凭据属性，禁止直接读取", attr)
	}
	searchReq := ldap.NewSearchRequest(
		dn,
		ldap.ScopeBaseObject, ldap.NeverDerefAliases,
		1, 0, false,
		"(objectClass=*)",
		[]string{attr},
		nil,
	)
	result, err := conn.Search(searchReq)
	if err != nil {
		return nil, FriendlyLdapError(err)
	}
	if len(result.Entries) == 0 {
		return nil, fmt.Errorf("条目不存在: %s", dn)
	}
	values := result.Entries[0].GetRawAttributeValues(attr)
	if len(values) == 0 {
		return nil, fmt.Errorf("属性 %s 不存在", attr)
	}
	return values[0], nil
}

// ListDescendants 列出指定 baseDN 下所有子孙条目（DN 列表）
// 用于递归删除预览等场景，对标 phpLDAPadmin delete_form.php 先列出将被删除的条目
func (s *EntryService) ListDescendants(conn *ldap.Conn, baseDN string) ([]string, error) {
	searchReq := ldap.NewSearchRequest(
		baseDN,
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases,
		MaxSubtreeEntries, 0, false,
		"(objectClass=*)",
		[]string{"dn"},
		nil,
	)
	result, err := conn.Search(searchReq)
	if err != nil {
		return nil, FriendlyLdapError(err)
	}
	var dns []string
	for _, e := range result.Entries {
		if !strings.EqualFold(e.DN, baseDN) {
			dns = append(dns, e.DN)
		}
	}
	return dns, nil
}

// BatchDelete 批量删除多个 DN（逐条执行，返回成功/失败汇总）
func (s *EntryService) BatchDelete(conn *ldap.Conn, dns []string, recursive bool) ([]model.BatchResultItem, error) {
	var results []model.BatchResultItem
	for _, dn := range dns {
		start := time.Now()
		err := s.DeleteEntry(conn, dn, recursive)
		duration := time.Since(start)
		results = append(results, model.BatchResultItem{
			DN:      dn,
			Success: err == nil,
			Error: func() string {
				if err != nil {
					return err.Error()
				}
				return ""
			}(),
			Duration: duration,
		})
	}
	return results, nil
}

// BatchModify 批量修改多条目同一属性（批量编辑/替换）
// 对标 phpLDAPadmin mass_edit.php / mass_update.php
func (s *EntryService) BatchModify(conn *ldap.Conn, dns []string, op string, attr string, values []string) ([]model.BatchResultItem, error) {
	var ldapOp uint
	switch op {
	case "replace":
		ldapOp = ldap.ReplaceAttribute
	case "add":
		ldapOp = ldap.AddAttribute
	case "delete":
		ldapOp = ldap.DeleteAttribute
	default:
		return nil, fmt.Errorf("不支持的 op: %s", op)
	}

	var results []model.BatchResultItem
	for _, dn := range dns {
		start := time.Now()
		modifyReq := ldap.NewModifyRequest(dn, nil)
		modifyReq.Changes = []ldap.Change{{
			Operation: ldapOp,
			Modification: ldap.PartialAttribute{
				Type: attr,
				Vals: values,
			},
		}}
		err := FriendlyLdapError(conn.Modify(modifyReq))
		duration := time.Since(start)
		errMsg := ""
		if err != nil {
			errMsg = err.Error()
		}
		results = append(results, model.BatchResultItem{
			DN:       dn,
			Success:  err == nil,
			Error:    errMsg,
			Duration: duration,
		})
	}
	return results, nil
}

// HashPassword 按指定算法哈希密码
//
// 参数：algo 算法名（SSHA / SSHA256 / SSHA512 / SHA / PLAIN）、password 明文密码
// 返回：LDAP 密码哈希字符串；不支持的算法返回错误
// 错误说明：MD5 已禁用（碰撞与离线爆破风险），盐使用 crypto/rand 生成
func HashPassword(algo, password string) (string, error) {
	algo = strings.ToUpper(strings.TrimPrefix(strings.TrimSuffix(strings.TrimSpace(algo), "}"), "{"))
	switch algo {
	case "", "PLAIN", "CLEAR":
		return password, nil
	case "SSHA":
		return saltedHash("{SSHA}", sha1.New, 8, password)
	case "SSHA256":
		return saltedHash("{SSHA256}", sha256.New, 8, password)
	case "SSHA512":
		return saltedHash("{SSHA512}", sha512.New, 16, password)
	case "SHA":
		h := sha1.New()
		h.Write([]byte(password))
		return "{SHA}" + base64.StdEncoding.EncodeToString(h.Sum(nil)), nil
	case "MD5":
		return "", errors.New("MD5 密码哈希已禁用，请使用 SSHA512/SSHA256")
	default:
		return "", fmt.Errorf("不支持的密码哈希算法: %s", algo)
	}
}

// saltedHash 生成加盐哈希（格式：{ALGO}base64(digest + salt)）
func saltedHash(prefix string, newHash func() hash.Hash, saltLen int, password string) (string, error) {
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("生成随机盐失败: %w", err)
	}
	h := newHash()
	h.Write([]byte(password))
	h.Write(salt)
	return prefix + base64.StdEncoding.EncodeToString(append(h.Sum(nil), salt...)), nil
}
