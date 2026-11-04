package ldap

import (
	"regexp"
	"strings"

	"github.com/go-ldap/ldap/v3"
	"github.com/lynnyq/goldapadmin/internal/model"
)

// SchemaService Schema 服务
type SchemaService struct{}

// NewSchemaService 创建 Schema 服务
func NewSchemaService() *SchemaService {
	return &SchemaService{}
}

// GetObjectClasses 获取所有 objectClass
func (s *SchemaService) GetObjectClasses(conn *ldap.Conn) ([]model.ObjectClassDef, error) {
	result, err := conn.Search(ldap.NewSearchRequest(
		"cn=schema,cn=config",
		ldap.ScopeBaseObject,
		ldap.NeverDerefAliases, 0, 0, false,
		"(objectClass=*)",
		[]string{"objectClasses"},
		nil,
	))
	if err != nil {
		// 尝试从 subschema 子条目获取
		result, err = conn.Search(ldap.NewSearchRequest(
			"cn=subschema",
			ldap.ScopeBaseObject,
			ldap.NeverDerefAliases, 0, 0, false,
			"(objectClass=*)",
			[]string{"objectClasses"},
			nil,
		))
		if err != nil {
			return defaultObjectClasses(), nil
		}
	}

	if len(result.Entries) == 0 {
		return defaultObjectClasses(), nil
	}

	defs := result.Entries[0].GetAttributeValues("objectClasses")
	list := make([]model.ObjectClassDef, 0, len(defs))
	for _, def := range defs {
		oc := parseObjectClass(def)
		list = append(list, oc)
	}
	if len(list) == 0 {
		return defaultObjectClasses(), nil
	}
	return list, nil
}

// GetAttributeTypes 获取所有 attributeType
func (s *SchemaService) GetAttributeTypes(conn *ldap.Conn) ([]model.AttributeTypeDef, error) {
	result, err := conn.Search(ldap.NewSearchRequest(
		"cn=schema,cn=config",
		ldap.ScopeBaseObject,
		ldap.NeverDerefAliases, 0, 0, false,
		"(objectClass=*)",
		[]string{"attributeTypes"},
		nil,
	))
	if err != nil {
		result, err = conn.Search(ldap.NewSearchRequest(
			"cn=subschema",
			ldap.ScopeBaseObject,
			ldap.NeverDerefAliases, 0, 0, false,
			"(objectClass=*)",
			[]string{"attributeTypes"},
			nil,
		))
		if err != nil {
			return defaultAttributeTypes(), nil
		}
	}

	if len(result.Entries) == 0 {
		return defaultAttributeTypes(), nil
	}

	defs := result.Entries[0].GetAttributeValues("attributeTypes")
	list := make([]model.AttributeTypeDef, 0, len(defs))
	for _, def := range defs {
		at := parseAttributeType(def)
		list = append(list, at)
	}
	if len(list) == 0 {
		return defaultAttributeTypes(), nil
	}
	return list, nil
}

// GetObjectClassByName 获取指定 objectClass
func (s *SchemaService) GetObjectClassByName(conn *ldap.Conn, name string) (*model.ObjectClassDef, error) {
	list, err := s.GetObjectClasses(conn)
	if err != nil {
		return nil, err
	}
	for _, oc := range list {
		if strings.EqualFold(oc.Name, name) {
			return &oc, nil
		}
		for _, n := range oc.Names {
			if strings.EqualFold(n, name) {
				return &oc, nil
			}
		}
	}
	return nil, nil
}

// GetAttributeByName 获取指定属性
func (s *SchemaService) GetAttributeByName(conn *ldap.Conn, name string) (*model.AttributeTypeDef, error) {
	list, err := s.GetAttributeTypes(conn)
	if err != nil {
		return nil, err
	}
	for _, at := range list {
		if strings.EqualFold(at.Name, name) {
			return &at, nil
		}
		for _, n := range at.Names {
			if strings.EqualFold(n, name) {
				return &at, nil
			}
		}
	}
	return nil, nil
}

// GetAllowedAttributes 获取 objectClass 允许的属性
// 对标 phpLDAPadmin rebuildTemplateAttrs：沿 SUP（父类）链递归收集 MUST/MAY，
// 因为 RFC 4512 规定 objectClass 定义的 MUST/MAY 不含父类属性。
func (s *SchemaService) GetAllowedAttributes(conn *ldap.Conn, objectClasses []string) (must, may []string, err error) {
	allOCs, err := s.GetObjectClasses(conn)
	if err != nil {
		return nil, nil, err
	}

	// 按名称（含别名）建立索引，便于沿 SUP 链查找
	ocIndex := make(map[string]*model.ObjectClassDef)
	for i := range allOCs {
		oc := &allOCs[i]
		for _, n := range append([]string{oc.Name}, oc.Names...) {
			ocIndex[strings.ToLower(n)] = oc
		}
	}

	mustSet := make(map[string]bool)
	maySet := make(map[string]bool)
	visited := make(map[string]bool)

	// findOC 按名称（含别名）查找 objectClass 定义
	findOC := func(name string) *model.ObjectClassDef {
		return ocIndex[strings.ToLower(name)]
	}

	// collectOC 递归收集某个 objectClass 及其全部父类的 MUST/MAY
	var collectOC func(oc *model.ObjectClassDef)
	collectOC = func(oc *model.ObjectClassDef) {
		if oc == nil {
			return
		}
		key := strings.ToLower(oc.Name)
		if visited[key] {
			return
		}
		visited[key] = true
		for _, m := range oc.Must {
			mustSet[m] = true
		}
		for _, m := range oc.May {
			maySet[m] = true
		}
		// 递归处理父类
		for _, sup := range oc.Superior {
			collectOC(findOC(sup))
		}
	}

	for _, ocName := range objectClasses {
		collectOC(findOC(ocName))
	}

	// 对标 phpLDAPadmin rebuildTemplateAttrs：objectClass 由条目本身维护，
	// 不作为可编辑属性出现在创建/编辑表单中（top 的 MUST 含 objectClass，需排除）
	for k := range mustSet {
		if strings.EqualFold(k, "objectClass") {
			continue
		}
		must = append(must, k)
	}
	for k := range maySet {
		if !mustSet[k] && !strings.EqualFold(k, "objectClass") {
			may = append(may, k)
		}
	}
	// 保证返回非 nil slice，避免 JSON 序列化为 null
	if must == nil {
		must = []string{}
	}
	if may == nil {
		may = []string{}
	}
	return must, may, nil
}

func containsCI(list []string, target string) bool {
	for _, s := range list {
		if strings.EqualFold(s, target) {
			return true
		}
	}
	return false
}

var (
	ocNameRegexp  = regexp.MustCompile(`NAME\s+'([^']+)'`)
	ocNamesRegexp = regexp.MustCompile(`NAME\s+\(([^)]+)\)`)
	ocDescRegexp  = regexp.MustCompile(`DESC\s+'([^']*)'`)
	// MUST/MAY 正则优先匹配括号包裹的多值列表，再匹配简单标识符
	ocMustRegexp  = regexp.MustCompile(`MUST\s+(?:\(([^)]+)\)|([A-Za-z][A-Za-z0-9_-]*))`)
	ocMayRegexp   = regexp.MustCompile(`MAY\s+(?:\(([^)]+)\)|([A-Za-z][A-Za-z0-9_-]*))`)
	// SUP 正则只匹配 objectClass 名称，遇到 STRUCTURAL/AUXILIARY/ABSTRACT/MUST/MAY 关键字时停止
	ocSupRegexp   = regexp.MustCompile(`SUP\s+([A-Za-z][A-Za-z0-9.$_-]*(?:\s+[A-Za-z][A-Za-z0-9.$_-]*)*)`)
	ocOIDRegexp   = regexp.MustCompile(`^\s*([\d.]+)`)
)

// ocSupStopWords SUP 匹配时的停止词，遇到这些词就不再继续匹配
var ocSupStopWords = []string{"STRUCTURAL", "AUXILIARY", "ABSTRACT", "MUST", "MAY", "DESC", "NAME", "OBSOLETE", "X-"}

func parseObjectClass(def string) model.ObjectClassDef {
	oc := model.ObjectClassDef{
		Type:      "structural",
		Names:     []string{},
		Must:      []string{},
		May:       []string{},
		Superior:  []string{},
	}

	if m := ocOIDRegexp.FindStringSubmatch(def); len(m) > 1 {
		oc.OID = m[1]
	}

	// 名称
	if m := ocNamesRegexp.FindStringSubmatch(def); len(m) > 1 {
		names := strings.Split(m[1], "'")
		for _, n := range names {
			n = strings.TrimSpace(n)
			if n != "" {
				oc.Names = append(oc.Names, n)
			}
		}
		if len(oc.Names) > 0 {
			oc.Name = oc.Names[0]
		}
	} else if m := ocNameRegexp.FindStringSubmatch(def); len(m) > 1 {
		oc.Name = m[1]
		oc.Names = []string{m[1]}
	}

	if m := ocDescRegexp.FindStringSubmatch(def); len(m) > 1 {
		oc.Description = m[1]
	}

	if m := ocMustRegexp.FindStringSubmatch(def); len(m) >= 3 {
		// group 1 = 括号内的值，group 2 = 简单标识符（二选一）
		val := m[1]
		if val == "" {
			val = m[2]
		}
		oc.Must = splitQDStrings(val)
	}

	if m := ocMayRegexp.FindStringSubmatch(def); len(m) >= 3 {
		val := m[1]
		if val == "" {
			val = m[2]
		}
		oc.May = splitQDStrings(val)
	}

	if m := ocSupRegexp.FindStringSubmatch(def); len(m) > 1 {
		// SUP 可能有多个 super 类，遇到关键字就停止
		parts := strings.Fields(m[1])
		var sups []string
		for _, p := range parts {
			upper := strings.ToUpper(p)
			isStop := false
			for _, sw := range ocSupStopWords {
				if strings.HasPrefix(upper, sw) {
					isStop = true
					break
				}
			}
			if isStop {
				break
			}
			sups = append(sups, p)
		}
		oc.Superior = sups
	}

	if strings.Contains(def, "AUXILIARY") {
		oc.Type = "auxiliary"
	} else if strings.Contains(def, "ABSTRACT") {
		oc.Type = "abstract"
	}

	return oc
}

var atNameRegexp = regexp.MustCompile(`NAME\s+'([^']+)'`)

func parseAttributeType(def string) model.AttributeTypeDef {
	at := model.AttributeTypeDef{}

	if m := ocOIDRegexp.FindStringSubmatch(def); len(m) > 1 {
		at.OID = m[1]
	}
	if m := atNameRegexp.FindStringSubmatch(def); len(m) > 1 {
		at.Name = m[1]
		at.Names = []string{m[1]}
	}
	if m := ocDescRegexp.FindStringSubmatch(def); len(m) > 1 {
		at.Description = m[1]
	}

	syntaxIdx := strings.Index(def, "SYNTAX")
	if syntaxIdx > -1 {
		rest := def[syntaxIdx+6:]
		parts := strings.Fields(rest)
		if len(parts) > 0 {
			at.Syntax = strings.Trim(parts[0], "'")
		}
	}

	if strings.Contains(def, "SINGLE-VALUE") {
		at.SingleValue = true
	}

	return at
}

func splitQDStrings(s string) []string {
	var result []string
	for _, part := range strings.Split(s, "$") {
		part = strings.Trim(strings.TrimSpace(part), "'")
		if part != "" {
			result = append(result, part)
		}
	}
	return result
}

func defaultObjectClasses() []model.ObjectClassDef {
	return []model.ObjectClassDef{
		{OID: "2.5.6.0", Name: "top", Names: []string{"top"}, Description: "top", Type: "abstract", Must: []string{"objectClass"}},
		{OID: "2.5.6.5", Name: "organizationalUnit", Names: []string{"organizationalUnit", "ou"}, Description: "organizational unit", Type: "structural", Superior: []string{"top"}, Must: []string{"ou"}, May: []string{"businessCategory", "description", "facsimileTelephoneNumber", "l", "physicalDeliveryOfficeName", "postalAddress", "postalCode", "preferredDeliveryMethod", "registeredAddress", "searchGuide", "seeAlso", "st", "street", "telephoneNumber", "userPassword"}},
		{OID: "2.5.6.7", Name: "organization", Names: []string{"organization", "o"}, Description: "organization", Type: "structural", Superior: []string{"top"}, Must: []string{"o"}, May: []string{"businessCategory", "description", "facsimileTelephoneNumber", "internationaliSDNNumber", "l", "physicalDeliveryOfficeName", "postalAddress", "postalCode", "postOfficeBox", "preferredDeliveryMethod", "registeredAddress", "searchGuide", "seeAlso", "st", "street", "telephoneNumber", "teletexTerminalIdentifier", "telexNumber", "userPassword", "x121Address"}},
		{OID: "2.5.6.9", Name: "groupOfNames", Names: []string{"groupOfNames"}, Description: "group of names", Type: "structural", Superior: []string{"top"}, Must: []string{"cn", "member"}, May: []string{"businessCategory", "description", "facsimileTelephoneNumber", "l", "mail", "o", "ou", "owner", "phoneBook", "postalAddress", "postalCode", "preferredDeliveryMethod", "seeAlso", "st", "street", "telephoneNumber"}},
		{OID: "2.16.840.1.113730.3.2.2", Name: "inetOrgPerson", Names: []string{"inetOrgPerson"}, Description: "inetOrgPerson", Type: "structural", Superior: []string{"organizationalPerson"}, Must: []string{"cn", "sn"}, May: []string{"audio", "businessCategory", "carLicense", "departmentNumber", "displayName", "employeeNumber", "employeeType", "givenName", "homePhone", "homePostalAddress", "initials", "jpegPhoto", "labeledURI", "l", "mail", "manager", "mobile", "pager", "postalAddress", "postalCode", "preferredLanguage", "roomNumber", "secretary", "uid", "userCertificate", "x500UniqueIdentifier", "x121Address"}},
		{OID: "2.5.6.6", Name: "person", Names: []string{"person"}, Description: "person", Type: "structural", Superior: []string{"top"}, Must: []string{"cn", "sn"}, May: []string{"description", "seeAlso", "telephoneNumber", "userPassword"}},
		{OID: "2.5.6.7", Name: "organizationalPerson", Names: []string{"organizationalPerson"}, Description: "organizational person", Type: "structural", Superior: []string{"person"}, Must: []string{"cn", "sn"}, May: []string{"facsimileTelephoneNumber", "internationaliSDNNumber", "l", "physicalDeliveryOfficeName", "postalAddress", "postalCode", "postOfficeBox", "preferredDeliveryMethod", "registeredAddress", "st", "street", "teletexTerminalIdentifier", "telexNumber", "title", "x121Address"}},
	}
}

func defaultAttributeTypes() []model.AttributeTypeDef {
	attrs := []string{"cn", "sn", "uid", "mail", "ou", "o", "dc", "telephoneNumber", "mobile", "facsimileTelephoneNumber", "description", "seeAlso", "userPassword", "l", "st", "street", "postalAddress", "postalCode", "postOfficeBox", "title", "givenName", "initials", "jpegPhoto", "labeledURI", "manager", "pager", "roomNumber", "employeeNumber", "employeeType", "departmentNumber", "displayName", "carLicense", "homePhone", "homePostalAddress", "preferredLanguage", "secretary", "userCertificate", "x500UniqueIdentifier", "audio", "businessCategory", "objectClass", "member", "owner", "registeredAddress", "searchGuide", "physicalDeliveryOfficeName", "preferredDeliveryMethod", "phoneBook", "internationaliSDNNumber", "teletexTerminalIdentifier", "telexNumber", "x121Address", "uidNumber", "gidNumber", "homeDirectory", "loginShell", "gecos"}
	list := make([]model.AttributeTypeDef, len(attrs))
	for i, a := range attrs {
		list[i] = model.AttributeTypeDef{Name: a, Names: []string{a}, Syntax: "1.3.6.1.4.1.1466.115.121.1.15", SingleValue: a == "uid" || a == "uidNumber" || a == "gidNumber"}
	}
	return list
}
