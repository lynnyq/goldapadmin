package service

import (
	"errors"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-ldap/ldap/v3"
	"github.com/lynnyq/goldapadmin/internal/dao"
	"github.com/lynnyq/goldapadmin/internal/model"
	pkgldap "github.com/lynnyq/goldapadmin/internal/pkg/ldap"
	"github.com/lynnyq/goldapadmin/internal/pkg/middleware"
)

var errServerNotFound = errors.New("ldap server not found")

// ErrCredentialsExpired 登录凭据缓存失效（典型场景：服务重启后凭据缓存丢失，JWT 仍有效）
var ErrCredentialsExpired = errors.New("登录凭据已失效，请重新登录")

// LdapService LDAP 业务服务
type LdapService struct {
	serverDAO   *dao.ServerDAO
	bookmarkDAO *dao.BookmarkDAO
	logDAO      *dao.LogDAO
}

// NewLdapService 创建 LDAP 服务
func NewLdapService() *LdapService {
	return &LdapService{
		serverDAO:   dao.NewServerDAO(),
		bookmarkDAO: dao.NewBookmarkDAO(),
		logDAO:      dao.NewLogDAO(),
	}
}

// ConnParams 连接参数
type ConnParams struct {
	Server   *model.LdapServer
	BindDN   string
	Password string
}

// credEntry 凭据缓存条目
type credEntry struct {
	password  string
	expiresAt time.Time
}

// credTTL 凭据缓存有效期，与 JWT 有效期保持一致
const credTTL = 24 * time.Hour

// credCache 登录凭据内存缓存（key: JWT token）
// 密码仅在服务端内存中随 JWT 生命周期暂存，避免前端明文保存或在每个请求头中传输
var credCache sync.Map

// CacheCredentials 缓存登录成功后的绑定密码
func CacheCredentials(token, password string) {
	if token == "" {
		return
	}
	credCache.Store(token, credEntry{password: password, expiresAt: time.Now().Add(credTTL)})
}

// getCachedPassword 按 token 获取缓存的密码，过期条目惰性清除
func getCachedPassword(token string) string {
	if token == "" {
		return ""
	}
	v, ok := credCache.Load(token)
	if !ok {
		return ""
	}
	e := v.(credEntry)
	if time.Now().After(e.expiresAt) {
		credCache.Delete(token)
		return ""
	}
	return e.password
}

// ClearCredentials 清除指定 token 的凭据缓存（用于 Logout）
func ClearCredentials(token string) {
	if token == "" {
		return
	}
	credCache.Delete(token)
}

// GetConnParams 从请求上下文获取连接参数
func (s *LdapService) GetConnParams(c *gin.Context) (*ConnParams, error) {
	serverID := middleware.GetServerID(c)
	server, err := s.serverDAO.Get(serverID)
	if err != nil {
		return nil, err
	}
	if server == nil {
		return nil, errServerNotFound
	}
	// 只从登录凭据缓存取密码，不再接受 X-Ldap-Password header 绕过
	password := getCachedPassword(middleware.GetToken(c))
	bindDN := middleware.GetUserDN(c)
	// 带 BindDN 但无密码 → 凭据缓存丢失（如服务重启），要求重新登录
	if bindDN != "" && password == "" {
		return nil, ErrCredentialsExpired
	}
	return &ConnParams{
		Server:   server,
		BindDN:   bindDN,
		Password: password,
	}, nil
}

// Connect 建立 LDAP 连接
func (s *LdapService) Connect(params *ConnParams) (*ldap.Conn, error) {
	return pkgldap.ConnectWithServer(params.Server, params.BindDN, params.Password)
}

// LogOperation 记录操作日志
func (s *LdapService) LogOperation(serverID int, userDN, action, targetDN, detail, status, errorMsg string, duration time.Duration) {
	log := &model.OperationLog{
		ServerID:   serverID,
		UserDN:     userDN,
		Action:     action,
		TargetDN:   targetDN,
		Detail:     detail,
		Status:     status,
		ErrorMsg:   errorMsg,
		DurationMS: duration.Milliseconds(),
	}
	_ = s.logDAO.Create(log)
}

// GetServerDAO 获取服务器 DAO
func (s *LdapService) GetServerDAO() *dao.ServerDAO {
	return s.serverDAO
}

// GetBookmarkDAO 获取书签 DAO
func (s *LdapService) GetBookmarkDAO() *dao.BookmarkDAO {
	return s.bookmarkDAO
}

// GetLogDAO 获取日志 DAO
func (s *LdapService) GetLogDAO() *dao.LogDAO {
	return s.logDAO
}
