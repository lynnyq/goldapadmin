package model

import "time"

// LdapServer LDAP 服务器配置
type LdapServer struct {
	ID             int       `json:"id" db:"id"`
	Name           string    `json:"name" db:"name"`
	Host           string    `json:"host" db:"host"`
	Port           int       `json:"port" db:"port"`
	BaseDN         string    `json:"base_dn" db:"base_dn"`
	BindDN         string    `json:"bind_dn" db:"bind_dn"`
	UseTLS         bool      `json:"use_tls" db:"use_tls"`
	UseSSL         bool      `json:"use_ssl" db:"use_ssl"`
	SkipTLSVerify  bool      `json:"skip_tls_verify" db:"skip_tls_verify"` // 是否跳过 TLS 证书校验（生产默认 false）
	Timeout        int       `json:"timeout" db:"timeout"`
	CreatedAt      time.Time `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time `json:"updated_at" db:"updated_at"`
}

// Bookmark 书签
type Bookmark struct {
	ID        int       `json:"id" db:"id"`
	ServerID  int       `json:"server_id" db:"server_id"`
	Name      string    `json:"name" db:"name"`
	DN        string    `json:"dn" db:"dn"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

// OperationLog 操作日志
type OperationLog struct {
	ID         int       `json:"id" db:"id"`
	ServerID   int       `json:"server_id" db:"server_id"`
	UserDN     string    `json:"user_dn" db:"user_dn"`
	Action     string    `json:"action" db:"action"`
	TargetDN   string    `json:"target_dn" db:"target_dn"`
	Detail     string    `json:"detail" db:"detail"`
	Status     string    `json:"status" db:"status"` // success / failed
	ErrorMsg   string    `json:"error_msg" db:"error_msg"`
	DurationMS int64     `json:"duration_ms" db:"duration_ms"`
	CreatedAt  time.Time `json:"created_at" db:"created_at"`
}

// LdapAttribute LDAP 属性
type LdapAttribute struct {
	Name   string   `json:"name"`
	Values []string `json:"values"`
}

// LdapEntry LDAP 条目
type LdapEntry struct {
	DN            string          `json:"dn"`
	ObjectClasses []string        `json:"object_classes"`
	Attributes    []LdapAttribute `json:"attributes"`
	HasChildren   bool            `json:"has_children,omitempty"`
}

// TreeNode 目录树节点
type TreeNode struct {
	DN            string     `json:"dn"`
	RDN           string     `json:"rdn"`
	ObjectClasses []string   `json:"object_classes"`
	HasChildren   bool       `json:"has_children"`
	IsLeaf        bool       `json:"is_leaf"`              // true 表示无子节点（前端 el-tree leaf 字段）
	ChildCount    int        `json:"child_count,omitempty"`  // 直接子条目数（省略表示未查询）
	EntryType     string     `json:"entry_type,omitempty"`  // user/group/ou/org/domain/gpo/alias/referral/unknown
	Disabled      bool       `json:"disabled,omitempty"`     // AD userAccountControl 禁用账户
	Children      []TreeNode `json:"children,omitempty"`
}

// ObjectClassDef Schema objectClass 定义
type ObjectClassDef struct {
	OID         string   `json:"oid"`
	Name        string   `json:"name"`
	Names       []string `json:"names"`
	Description string   `json:"description"`
	Type        string   `json:"type"` // structural / auxiliary / abstract
	Must        []string `json:"must"`
	May         []string `json:"may"`
	Superior    []string `json:"superior"`
}

// AttributeTypeDef Schema 属性类型定义
type AttributeTypeDef struct {
	OID         string   `json:"oid"`
	Name        string   `json:"name"`
	Names       []string `json:"names"`
	Description string   `json:"description"`
	Syntax      string   `json:"syntax"`
	SingleValue bool     `json:"single_value"`
	Mandatory   bool     `json:"mandatory"`
}

// ModifyRequest 修改条目请求
type ModifyRequest struct {
	DN      string          `json:"dn"`
	Changes []ModifyChange  `json:"changes"`
	// PasswordAlgos 密码属性哈希算法（对标 phpLDAPadmin password helper）
	// key=属性名（如 userPassword），value=SSHA/SHA/MD5，空或缺省为明文
	PasswordAlgos map[string]string `json:"password_algos,omitempty"`
}

// ModifyChange 修改操作
type ModifyChange struct {
	Op     string   `json:"op"` // add / replace / delete
	Attr   string   `json:"attr"`
	Values []string `json:"values,omitempty"`
}

// BatchResultItem 批量操作单条结果（对标 phpLDAPadmin mass_delete/mass_edit 汇总）
type BatchResultItem struct {
	DN       string        `json:"dn"`
	Success  bool          `json:"success"`
	Error    string        `json:"error,omitempty"`
	Duration time.Duration `json:"duration"`
}

// RootDSEInfo LDAP 服务器 rootDSE 元信息（对标 phpLDAPadmin server_info.php）
type RootDSEInfo struct {
	NamingContexts          []string `json:"namingContexts"`
	SupportedLDAPVersion    []string `json:"supportedLDAPVersion"`
	SupportedSASLMechanisms []string `json:"supportedSASLMechanisms,omitempty"`
	SupportedBindMethods    []string `json:"supportedBindMethods,omitempty"`
	VendorName              string   `json:"vendorName,omitempty"`
	VendorVersion           string   `json:"vendorVersion,omitempty"`
	ServerName              string   `json:"serverName,omitempty"`
	SubschemaSubentry       string   `json:"subschemaSubentry,omitempty"`
}
