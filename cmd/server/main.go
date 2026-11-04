package main

import (
	"fmt"
	"os"

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
	// 加载配置
	configPath := "config/config.yaml"
	if len(os.Args) > 1 {
		configPath = os.Args[1]
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Printf("load config failed: %v\n", err)
		os.Exit(1)
	}

	// 设置 JWT 密钥
	auth.SetSecret(cfg.JWT.Secret)

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
	// CORS 白名单化：从配置读取允许的 origins
	engine.Use(middleware.CORS(cfg.Security.CORS.Origins))

	// 注册 API 路由
	svc := service.NewLdapService()
	router := api.NewRouter(svc)
	router.Register(engine)

	// 注册前端静态资源
	web.Register(engine)

	// 启动服务
	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	fmt.Printf("GoLDAPAdmin starting on http://localhost%s\n", addr)
	if err := engine.Run(addr); err != nil {
		fmt.Printf("server failed: %v\n", err)
		os.Exit(1)
	}
}
