package main

import (
	"fmt"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lynnyq/goldapadmin/internal/api"
	"github.com/lynnyq/goldapadmin/internal/config"
	"github.com/lynnyq/goldapadmin/internal/dao"
	"github.com/lynnyq/goldapadmin/internal/pkg/auth"
	"github.com/lynnyq/goldapadmin/internal/pkg/middleware"
	"github.com/lynnyq/goldapadmin/internal/service"
	"github.com/lynnyq/goldapadmin/internal/web"
)

func main() {
	// 加载配置（release 模式下会拒绝弱 JWT 密钥与非法配置）
	configPath := "config/config.yaml"
	if len(os.Args) > 1 {
		configPath = os.Args[1]
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Printf("load config failed: %v\n", err)
		os.Exit(1)
	}

	// 设置 JWT 密钥与黑名单持久化
	auth.SetSecret(cfg.JWT.Secret)
	auth.SetBlacklistStore(dao.NewJWTBlacklistDAO())

	// 设置可信反向代理（仅来自这些地址的请求才解析 X-Forwarded-For）
	if err := middleware.SetTrustedProxies(cfg.Security.TrustedProxies); err != nil {
		fmt.Printf("invalid trusted_proxies: %v\n", err)
		os.Exit(1)
	}

	// 初始化数据库
	if err := dao.Init(cfg.Database.Path); err != nil {
		fmt.Printf("init database failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("database initialized: %s\n", cfg.Database.Path)

	// 设置 Gin 模式
	gin.SetMode(cfg.Server.Mode)

	// 创建引擎
	engine := gin.New()
	engine.Use(gin.Recovery())
	engine.Use(middleware.TraceID())
	engine.Use(middleware.Logger())
	engine.Use(middleware.SecurityHeaders())
	// CORS 白名单化：从配置读取允许的 origins（release 禁止通配符）
	engine.Use(middleware.CORS(cfg.Security.CORS.Origins, cfg.Server.Mode == "release"))

	// 注册 API 路由
	svc := service.NewLdapServiceWithOptions(service.SecurityOptions{
		RequireLDAPTLS:     cfg.Security.RequireLDAPTLS,
		AllowAnonymousBind: cfg.Security.AllowAnonymousBind,
		AdminDNs:           cfg.Security.AdminDNs,
	})
	router := api.NewRouter(svc, api.RouterOptions{
		JWTTTL:       time.Duration(cfg.JWT.TTL) * time.Hour,
		MaxBodyBytes: cfg.Security.MaxBodyBytes,
		ReadOnly:     cfg.Security.ReadOnly,
	})
	router.Register(engine)

	// 注册前端静态资源
	web.Register(engine)

	// 周期性清理已过期的 JWT 黑名单（内存 + DB）
	go func() {
		ticker := time.NewTicker(time.Hour)
		defer ticker.Stop()
		for now := range ticker.C {
			_ = now
			auth.CleanExpiredBlacklist()
		}
	}()

	// 启动服务
	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	fmt.Printf("GoLDAPAdmin starting on http://localhost%s\n", addr)
	if err := engine.Run(addr); err != nil {
		fmt.Printf("server failed: %v\n", err)
		os.Exit(1)
	}
}
