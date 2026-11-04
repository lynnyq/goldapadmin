package api

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/lynnyq/goldapadmin/internal/model"
	"github.com/lynnyq/goldapadmin/internal/pkg/middleware"
	"github.com/lynnyq/goldapadmin/internal/pkg/response"
	pkgldap "github.com/lynnyq/goldapadmin/internal/pkg/ldap"
)

// ListServers 获取服务器列表
func (r *Router) ListServers(c *gin.Context) {
	list, err := r.svc.GetServerDAO().List()
	if err != nil {
		response.Fail(c, 500, err.Error())
		return
	}
	if list == nil {
		list = []model.LdapServer{}
	}
	response.Success(c, list)
}

// CreateServer 创建服务器
func (r *Router) CreateServer(c *gin.Context) {
	var s model.LdapServer
	if err := c.ShouldBindJSON(&s); err != nil {
		response.Fail(c, 400, "参数错误: "+err.Error())
		return
	}
	if s.Port == 0 {
		s.Port = 389
	}
	if s.Timeout == 0 {
		s.Timeout = 30
	}
	if err := r.svc.GetServerDAO().Create(&s); err != nil {
		response.Fail(c, 500, err.Error())
		return
	}
	response.Success(c, s)
}

// UpdateServer 更新服务器
func (r *Router) UpdateServer(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var s model.LdapServer
	if err := c.ShouldBindJSON(&s); err != nil {
		response.Fail(c, 400, "参数错误: "+err.Error())
		return
	}
	s.ID = id
	if err := r.svc.GetServerDAO().Update(&s); err != nil {
		response.Fail(c, 500, err.Error())
		return
	}
	response.Success(c, s)
}

// DeleteServer 删除服务器
func (r *Router) DeleteServer(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if err := r.svc.GetServerDAO().Delete(id); err != nil {
		response.Fail(c, 500, err.Error())
		return
	}
	response.Success(c, nil)
}

// TestConnection 测试连接
func (r *Router) TestConnection(c *gin.Context) {
	var req struct {
		Host          string `json:"host"`
		Port          int    `json:"port"`
		BindDN        string `json:"bind_dn"`
		Password      string `json:"password"`
		UseTLS        bool   `json:"use_tls"`
		UseSSL        bool   `json:"use_ssl"`
		SkipTLSVerify bool   `json:"skip_tls_verify"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, 400, "参数错误")
		return
	}
	server := &model.LdapServer{
		Host:          req.Host,
		Port:          req.Port,
		UseTLS:        req.UseTLS,
		UseSSL:        req.UseSSL,
		SkipTLSVerify: req.SkipTLSVerify,
	}
	conn, err := pkgldap.ConnectWithServer(server, req.BindDN, req.Password)
	if err != nil {
		response.Success(c, gin.H{"connected": false, "message": err.Error()})
		return
	}
	defer conn.Close()
	response.Success(c, gin.H{"connected": true, "message": "连接成功"})
}

// ensure import used
var _ = middleware.GetServerID
