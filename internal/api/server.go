package api

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lynnyq/goldapadmin/internal/model"
	"github.com/lynnyq/goldapadmin/internal/pkg/response"
)

// PublicServerInfo 登录页可展示的最小服务器信息（不含 bind_dn/base_dn 等敏感字段）
type PublicServerInfo struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Host   string `json:"host"`
	Port   int    `json:"port"`
	UseTLS bool   `json:"use_tls"`
	UseSSL bool   `json:"use_ssl"`
}

// ListPublicServers 获取登录页可用的服务器列表
//
// 该接口无需认证，只返回连接目标与名称，避免泄漏管理员绑定 DN / BaseDN。
func (r *Router) ListPublicServers(c *gin.Context) {
	list, err := r.svc.GetServerDAO().List()
	if err != nil {
		response.Fail(c, 500, "获取服务器列表失败")
		return
	}
	result := make([]PublicServerInfo, 0, len(list))
	for _, s := range list {
		result = append(result, PublicServerInfo{
			ID:     s.ID,
			Name:   s.Name,
			Host:   s.Host,
			Port:   s.Port,
			UseTLS: s.UseTLS,
			UseSSL: s.UseSSL,
		})
	}
	response.Success(c, result)
}

// ListServers 获取服务器列表（管理员，含 bind_dn/base_dn）
func (r *Router) ListServers(c *gin.Context) {
	list, err := r.svc.GetServerDAO().List()
	if err != nil {
		response.Fail(c, 500, "获取服务器列表失败")
		return
	}
	if list == nil {
		list = []model.LdapServer{}
	}
	response.Success(c, list)
}

// normalizeServerInput 校验并规范化服务器配置
func normalizeServerInput(s *model.LdapServer) error {
	s.Host = strings.TrimSpace(s.Host)
	s.Name = strings.TrimSpace(s.Name)
	s.BaseDN = strings.TrimSpace(s.BaseDN)
	if s.Host == "" {
		return fmt.Errorf("服务器地址不能为空")
	}
	if len(s.Host) > 255 {
		return fmt.Errorf("服务器地址过长")
	}
	if s.Port == 0 {
		s.Port = 389
	}
	if s.Port < 1 || s.Port > 65535 {
		return fmt.Errorf("服务器端口非法")
	}
	if s.Timeout == 0 {
		s.Timeout = 30
	}
	if s.Timeout < 1 || s.Timeout > 120 {
		return fmt.Errorf("连接超时必须在 1-120 秒之间")
	}
	if s.Name == "" {
		s.Name = s.Host
	}
	if s.BaseDN != "" {
		if err := validateDN(s.BaseDN); err != nil {
			return fmt.Errorf("BaseDN 非法: %w", err)
		}
	}
	return nil
}

// CreateServer 创建服务器（管理员）
func (r *Router) CreateServer(c *gin.Context) {
	var s model.LdapServer
	if err := c.ShouldBindJSON(&s); err != nil {
		response.Fail(c, 400, "参数错误")
		return
	}
	if err := normalizeServerInput(&s); err != nil {
		response.Fail(c, 400, err.Error())
		return
	}
	if err := r.svc.GetServerDAO().Create(&s); err != nil {
		response.Fail(c, 500, "保存服务器配置失败")
		return
	}
	response.Success(c, s)
}

// UpdateServer 更新服务器（管理员）
func (r *Router) UpdateServer(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		response.Fail(c, 400, "服务器 ID 非法")
		return
	}
	var s model.LdapServer
	if err := c.ShouldBindJSON(&s); err != nil {
		response.Fail(c, 400, "参数错误")
		return
	}
	// 未提供的 ID 采用路径参数；避免将记录改到其它 ID
	s.ID = id
	if err := normalizeServerInput(&s); err != nil {
		response.Fail(c, 400, err.Error())
		return
	}
	if err := r.svc.GetServerDAO().Update(&s); err != nil {
		response.Fail(c, 500, "更新服务器配置失败")
		return
	}
	response.Success(c, s)
}

// DeleteServer 删除服务器（管理员）
func (r *Router) DeleteServer(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		response.Fail(c, 400, "服务器 ID 非法")
		return
	}
	if err := r.svc.GetServerDAO().Delete(id); err != nil {
		response.Fail(c, 500, "删除服务器配置失败")
		return
	}
	response.Success(c, nil)
}

// TestConnection 测试连接（管理员）
//
// 仅用于运维验证服务器配置；错误详情只记录服务端日志，响应返回通用提示，
// 避免被用于内网拓扑探测与错误信息收集。
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
		Host:          strings.TrimSpace(req.Host),
		Port:          req.Port,
		UseTLS:        req.UseTLS,
		UseSSL:        req.UseSSL,
		SkipTLSVerify: req.SkipTLSVerify,
		Timeout:       30,
	}
	if err := normalizeServerInput(server); err != nil {
		response.Fail(c, 400, err.Error())
		return
	}

	start := time.Now()
	conn, err := r.svc.VerifyBind(server, req.BindDN, req.Password)
	if err != nil {
		fmt.Printf("[TEST-LDAP] failed host=%s:%d dn=%q err=%v\n", server.Host, server.Port, req.BindDN, err)
		response.Success(c, gin.H{"connected": false, "message": "连接失败，请检查地址、端口、TLS 配置与绑定凭据"})
		return
	}
	defer conn.Close()
	response.Success(c, gin.H{"connected": true, "message": "连接成功", "elapsed_ms": time.Since(start).Milliseconds()})
}
