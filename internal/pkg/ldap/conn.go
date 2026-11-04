package ldap

import (
	"crypto/tls"
	"fmt"
	"net"
	"time"

	"github.com/go-ldap/ldap/v3"
	"github.com/lynnyq/goldapadmin/internal/model"
)

// ConnInfo 连接信息
type ConnInfo struct {
	Host          string
	Port          int
	BindDN        string
	Password      string
	UseTLS        bool
	UseSSL        bool
	SkipTLSVerify bool
	Timeout       int
}

// Connect 建立 LDAP 连接并绑定
func Connect(info ConnInfo) (*ldap.Conn, error) {
	timeout := time.Duration(info.Timeout) * time.Second
	dialer := &net.Dialer{Timeout: timeout}

	tlsConfig := &tls.Config{
		InsecureSkipVerify: info.SkipTLSVerify,
		MinVersion:         tls.VersionTLS12,
	}

	var conn *ldap.Conn
	var err error

	if info.UseSSL {
		conn, err = ldap.DialURL(
			fmt.Sprintf("ldaps://%s:%d", info.Host, info.Port),
			ldap.DialWithDialer(dialer),
			ldap.DialWithTLSConfig(tlsConfig),
		)
	} else {
		conn, err = ldap.DialURL(
			fmt.Sprintf("ldap://%s:%d", info.Host, info.Port),
			ldap.DialWithDialer(dialer),
		)
	}
	if err != nil {
		return nil, fmt.Errorf("dial ldap: %w", err)
	}

	if info.UseTLS && !info.UseSSL {
		if err := conn.StartTLS(tlsConfig); err != nil {
			conn.Close()
			return nil, fmt.Errorf("starttls: %w", err)
		}
	}

	// 根据 BindDN / Password 组合选择绑定模式：
	//   1. BindDN 为空    → 匿名绑定（UnauthenticatedBind）
	//   2. BindDN 非空且密码非空 → SIMPLE 绑定
	//   3. BindDN 非空但密码为空 → 明确报错，拒绝执行
	switch {
	case info.BindDN == "":
		if err := conn.UnauthenticatedBind(""); err != nil {
			conn.Close()
			return nil, fmt.Errorf("anonymous bind: %w", err)
		}
	case info.Password == "":
		conn.Close()
		return nil, fmt.Errorf("bind: 已填写绑定 DN [%s]，但密码为空。SIMPLE 绑定需要密码；如需匿名连接请清空绑定 DN 字段", info.BindDN)
	default:
		if err := conn.Bind(info.BindDN, info.Password); err != nil {
			conn.Close()
			return nil, fmt.Errorf("bind: %w", err)
		}
	}

	return conn, nil
}

// ConnectWithServer 使用服务器配置 + 密码连接
func ConnectWithServer(server *model.LdapServer, bindDN, password string) (*ldap.Conn, error) {
	info := ConnInfo{
		Host:          server.Host,
		Port:          server.Port,
		BindDN:        bindDN,
		Password:      password,
		UseTLS:        server.UseTLS,
		UseSSL:        server.UseSSL,
		SkipTLSVerify: server.SkipTLSVerify,
		Timeout:       server.Timeout,
	}
	return Connect(info)
}

