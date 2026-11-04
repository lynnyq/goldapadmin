package api

import (
	"github.com/gin-gonic/gin"
	"github.com/lynnyq/goldapadmin/internal/pkg/response"
	pkgldap "github.com/lynnyq/goldapadmin/internal/pkg/ldap"
)

// GetObjectClasses 获取所有 objectClass
func (r *Router) GetObjectClasses(c *gin.Context) {
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

	svc := pkgldap.NewSchemaService()
	list, err := svc.GetObjectClasses(conn)
	if err != nil {
		response.Fail(c, 500, err.Error())
		return
	}
	response.Success(c, list)
}

// GetAttributeTypes 获取所有 attributeType
func (r *Router) GetAttributeTypes(c *gin.Context) {
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

	svc := pkgldap.NewSchemaService()
	list, err := svc.GetAttributeTypes(conn)
	if err != nil {
		response.Fail(c, 500, err.Error())
		return
	}
	response.Success(c, list)
}

// GetObjectClass 获取指定 objectClass
func (r *Router) GetObjectClass(c *gin.Context) {
	name := c.Param("name")
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

	svc := pkgldap.NewSchemaService()
	oc, err := svc.GetObjectClassByName(conn, name)
	if err != nil {
		response.Fail(c, 500, err.Error())
		return
	}
	response.Success(c, oc)
}

// GetAttribute 获取指定属性
func (r *Router) GetAttribute(c *gin.Context) {
	name := c.Param("name")
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

	svc := pkgldap.NewSchemaService()
	at, err := svc.GetAttributeByName(conn, name)
	if err != nil {
		response.Fail(c, 500, err.Error())
		return
	}
	response.Success(c, at)
}

// GetAllowedAttributes 获取允许的属性
func (r *Router) GetAllowedAttributes(c *gin.Context) {
	var req struct {
		ObjectClasses []string `json:"object_classes"`
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
		response.Fail(c, 500, "连接 LDAP 失败: "+err.Error())
		return
	}
	defer conn.Close()

	svc := pkgldap.NewSchemaService()
	must, may, err := svc.GetAllowedAttributes(conn, req.ObjectClasses)
	if err != nil {
		response.Fail(c, 500, err.Error())
		return
	}
	response.Success(c, gin.H{"must": must, "may": may})
}
