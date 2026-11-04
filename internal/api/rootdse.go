package api

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-ldap/ldap/v3"
	"github.com/lynnyq/goldapadmin/internal/model"
	"github.com/lynnyq/goldapadmin/internal/pkg/response"
)

// GetRootDSE 获取 LDAP 服务器 rootDSE 元信息（对标 phpLDAPadmin server_info.php）
// 用于命名上下文、支持的 SASL 机制、服务器厂商信息等展示
// GET /api/rootdse
func (r *Router) GetRootDSE(c *gin.Context) {
	params := r.mustConnParams(c)
	if params == nil {
		return
	}
	conn, err := r.svc.Connect(params)
	if err != nil {
		response.Fail(c, 500, "连接 LDAP 失败: "+err.Error())
		return
	}
	defer conn.Close()

	start := time.Now()
	req := ldap.NewSearchRequest(
		"",
		ldap.ScopeBaseObject, ldap.NeverDerefAliases,
		0, 0, false,
		"(objectClass=*)",
		[]string{
			"namingContexts", "supportedLDAPVersion",
			"supportedSASLMechanisms", "vendorName", "vendorVersion",
			"serverName", "subschemaSubentry", "supportedBindMethods",
			"defaultNamingContext", "configurationNamingContext",
			"rootDomainNamingContext", "schemaNamingContext",
		},
		nil,
	)
	result, err := conn.Search(req)
	duration := time.Since(start)
	if err != nil {
		r.svc.LogOperation(params.Server.ID, params.BindDN, "rootdse", "", "", "failed", err.Error(), duration)
		response.Fail(c, 500, "rootDSE 查询失败: "+err.Error())
		return
	}
	if len(result.Entries) == 0 {
		r.svc.LogOperation(params.Server.ID, params.BindDN, "rootdse", "", "", "failed", "no rootDSE", duration)
		response.Fail(c, 404, "未找到 rootDSE 条目")
		return
	}

	e := result.Entries[0]
	info := model.RootDSEInfo{
		NamingContexts:          e.GetAttributeValues("namingContexts"),
		SupportedLDAPVersion:    e.GetAttributeValues("supportedLDAPVersion"),
		SupportedSASLMechanisms: e.GetAttributeValues("supportedSASLMechanisms"),
		SupportedBindMethods:    e.GetAttributeValues("supportedBindMethods"),
		VendorName:              e.GetAttributeValue("vendorName"),
		VendorVersion:           e.GetAttributeValue("vendorVersion"),
		ServerName:              e.GetAttributeValue("serverName"),
		SubschemaSubentry:       e.GetAttributeValue("subschemaSubentry"),
	}

	// AD 特有字段：defaultNamingContext / configurationNamingContext / rootDomainNamingContext / schemaNamingContext
	if v := e.GetAttributeValue("defaultNamingContext"); v != "" {
		info.VendorName = v // 占位，前端会把它识别为 AD
	}

	r.svc.LogOperation(params.Server.ID, params.BindDN, "rootdse", "", "", "success", "", duration)
	response.Success(c, info)
}
