package api

import (
	"github.com/gin-gonic/gin"
	pkgldap "github.com/lynnyq/goldapadmin/internal/pkg/ldap"
	"github.com/lynnyq/goldapadmin/internal/pkg/response"
	"net/url"
)

// ImportLdif 导入 LDIF
func (r *Router) ImportLdif(c *gin.Context) {
	var req struct {
		Content string `json:"content" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, 400, "参数错误")
		return
	}

	params := r.mustConnParams(c)
	if params == nil {
		return
	}
	conn, err := r.svc.Connect(params)
	if err != nil {
		response.Fail(c, 500, "连接 LDAP 失败")
		return
	}
	defer conn.Close()

	svc := pkgldap.NewLdifService()
	result, err := svc.Import(conn.Conn, req.Content, params.Server.BaseDN)
	if err != nil {
		response.Fail(c, 400, "导入失败: "+err.Error())
		return
	}
	response.Success(c, result)
}

// ExportLdif 导出 LDIF
// 默认 recursive=true，导出 baseDN 及其所有子孙节点；recursive=false 时只导出 baseDN 自身
// Content-Disposition 文件名取 baseDN 最顶层 RDN（如 example.ldif）
func (r *Router) ExportLdif(c *gin.Context) {
	dn := c.Query("dn")
	recursive := c.Query("recursive") != "false"
	if dn == "" {
		response.Fail(c, 400, "dn 不能为空")
		return
	}
	if err := validateDN(dn); err != nil {
		response.Fail(c, 400, err.Error())
		return
	}

	params := r.mustConnParams(c)
	if params == nil {
		return
	}
	conn, err := r.svc.Connect(params)
	if err != nil {
		response.Fail(c, 500, "连接 LDAP 失败")
		return
	}
	defer conn.Close()

	svc := pkgldap.NewLdifService()
	content, err := svc.Export(conn.Conn, dn, recursive)
	if err != nil {
		response.Fail(c, 500, "导出失败: "+err.Error())
		return
	}

	filename := pkgldap.ExportFilename(dn)
	// RFC 5987 编码（文件名含非 ASCII 字符时浏览器能正确识别）
	encodedFilename := url.QueryEscape(filename)
	c.Header("Content-Type", "text/plain; charset=utf-8")
	c.Header("Content-Disposition",
		"attachment; filename=\""+filename+"\"; filename*=UTF-8''"+encodedFilename)
	c.String(200, content)
}
