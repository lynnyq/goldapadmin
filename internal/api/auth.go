package api

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lynnyq/goldapadmin/internal/dao"
	"github.com/lynnyq/goldapadmin/internal/model"
	"github.com/lynnyq/goldapadmin/internal/pkg/auth"
	pkgldap "github.com/lynnyq/goldapadmin/internal/pkg/ldap"
	"github.com/lynnyq/goldapadmin/internal/pkg/middleware"
	"github.com/lynnyq/goldapadmin/internal/pkg/response"
	"github.com/lynnyq/goldapadmin/internal/service"
)

// LoginRequest 登录请求
type LoginRequest struct {
	ServerID int    `json:"server_id"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	BaseDN   string `json:"base_dn"`
	BindDN   string `json:"bind_dn"`
	Password string `json:"password"`
	UseTLS   bool   `json:"use_tls"`
	UseSSL   bool   `json:"use_ssl"`
	Remember bool   `json:"remember"`
}

// Login 登录
func (r *Router) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, 400, "参数错误: "+err.Error())
		return
	}

	// 参数组合校验：
	//   - bind_dn 非空但密码为空 → 拒绝（SIMPLE 绑定必须有密码）
	//   - bind_dn 为空且密码也为空 → 匿名绑定，允许
	if req.BindDN != "" && req.Password == "" {
		response.Fail(c, 400, "已填写绑定 DN，但密码为空。SIMPLE 绑定需要密码；如需匿名连接请清空绑定 DN 字段")
		return
	}

	var server *model.LdapServer
	if req.ServerID > 0 {
		s, err := r.svc.GetServerDAO().Get(req.ServerID)
		if err != nil || s == nil {
			response.Fail(c, 404, "服务器不存在")
			return
		}
		server = s
	} else {
		// 使用临时服务器配置
		server = &model.LdapServer{
			Host:    req.Host,
			Port:    req.Port,
			BaseDN:  req.BaseDN,
			UseTLS:  req.UseTLS,
			UseSSL:  req.UseSSL,
			Timeout: 30,
		}
	}

	// 尝试 LDAP Bind 验证
	conn, err := pkgldap.ConnectWithServer(server, req.BindDN, req.Password)
	if err != nil {
		response.Fail(c, 401, "LDAP 认证失败: "+err.Error())
		return
	}
	defer conn.Close()

	// 如果是记住服务器，保存到数据库
	if req.Remember && req.ServerID == 0 {
		server.Name = req.Host
		server.BindDN = req.BindDN
		if err := r.svc.GetServerDAO().Create(server); err != nil {
			// 保存失败不影响登录
		}
	}

	// 生成 JWT
	ttl := time.Duration(24) * time.Hour
	token, err := auth.GenerateToken(server.ID, req.BindDN, ttl)
	if err != nil {
		response.Fail(c, 500, "生成 token 失败")
		return
	}

	// 缓存绑定凭据，供登录后的业务接口建立 LDAP 连接使用
	service.CacheCredentials(token, req.Password)

	response.Success(c, gin.H{
		"token":   token,
		"server":  server,
		"user_dn": req.BindDN,
	})
}

// Logout 登出：让 JWT 立即失效并清除服务端凭据缓存
//
// Logout 位于公开路由组（不需要 Auth 中间件），所以自行解析 Authorization header。
// 同时支持携带或不携带 token 的情况（客户端可能已清理本地 token 后再调用）。
func (r *Router) Logout(c *gin.Context) {
	tokenString := ""
	claims := (*auth.Claims)(nil)

	header := c.GetHeader("Authorization")
	if header != "" {
		parts := strings.SplitN(header, " ", 2)
		if len(parts) == 2 && parts[0] == "Bearer" {
			tokenString = parts[1]
			// 尝试解析以拿 expire 时间；解析失败时 blacklist 也会用默认 24h
			if cl, err := auth.ParseToken(tokenString); err == nil {
				claims = cl
			}
		}
	}

	if tokenString != "" {
		auth.AddToBlacklist(tokenString, claims)
		service.ClearCredentials(tokenString)
	}

	response.Success(c, nil)
}

// GetServerDAO 获取服务器 DAO
func (r *Router) GetServerDAO() *dao.ServerDAO {
	return r.svc.GetServerDAO()
}

// 确保 middleware 包被引用
var _ = middleware.GetTraceID
var _ = http.StatusOK
