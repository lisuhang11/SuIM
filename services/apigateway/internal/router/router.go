// Package router 负责注册所有 REST 路由并装配中间件。
package router

import (
	"apigateway/internal/config"
	"apigateway/internal/grpc"
	"apigateway/internal/handler"
	"apigateway/internal/middleware"

	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
)

// NewEngine 创建并配置 gin 引擎，注入所有中间件和路由。
func NewEngine(cfg *config.GatewayConfig, clients *grpc.Clients, authMW *middleware.AuthMiddleware, rateLimiter *middleware.RateLimiter) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	engine := gin.New()

	// ---- 全局中间件（按优先级排列）----
	engine.Use(
		middleware.Recovery(),              // 1. panic 恢复
		middleware.RequestID(),             // 2. 请求 ID
		middleware.MetricsInFlight(),       // 3. 并发计数
		middleware.Metrics(),               // 4. Prometheus 指标
		middleware.Logging(),               // 5. 结构化日志
		middleware.CompressBody(),          // 6. 缓冲请求体（供限流/日志复用）
		gzip.Gzip(gzip.DefaultCompression), // 7. gzip 压缩
		middleware.CORS(cfg.CORSOrigins),   // 8. CORS
		rateLimiter.Handler(),              // 9. 限流
		authMW.Handler(),                   // 10. JWT 鉴权
		middleware.BodyLogger(),            // 11. 调试请求体日志
	)

	// ---- 指标端点 ----
	engine.GET("/metrics", middleware.MetricsHandler())
	engine.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	// ---- 注册所有业务路由 ----
	api := engine.Group("/api/v1")

	// user 服务 — 公开路由：register、login 无需鉴权
	userHandler := handler.NewUserHandler(clients.User)
	usersGroup := api.Group("/users")
	userHandler.RegisterRoutes(usersGroup)
	if clients.MsgGateway != nil {
		handler.NewPresenceHandler(clients.MsgGateway).RegisterRoutes(usersGroup)
	}

	// relation 服务
	if clients.Relation != nil {
		relHandler := handler.NewRelationHandler(clients.Relation, clients.Conversation)
		relHandler.RegisterRoutes(api.Group("/relations"))
	}

	// group 服务
	if clients.Group != nil {
		groupHandler := handler.NewGroupHandler(clients.Group)
		groupHandler.RegisterRoutes(api.Group("/groups"))
	}

	// conversation 服务
	if clients.Conversation != nil {
		convHandler := handler.NewConversationHandler(clients.Conversation)
		convHandler.RegisterRoutes(api.Group("/conversations"))
	}

	// message 服务
	if clients.Message != nil {
		msgHandler := handler.NewMessageHandler(clients.Message, clients.File)
		msgHandler.RegisterRoutes(api.Group("/messages"))
	}

	if clients.File != nil {
		fileHandler := handler.NewFileHandler(clients.File)
		fileHandler.RegisterRoutes(api.Group("/files"))
	}

	if clients.Rtc != nil {
		callHandler := handler.NewCallHandler(clients.Rtc)
		callHandler.RegisterRoutes(api.Group("/calls"))
	}

	// BFF 聚合（对齐 OpenIM jssdk）
	if clients.Conversation != nil && clients.Message != nil {
		bffHandler := handler.NewBFFHandler(
			clients.Conversation,
			clients.Message,
			clients.User,
			clients.Relation,
			clients.Group,
		)
		bffHandler.RegisterRoutes(api.Group("/bff"))
	}

	return engine
}
