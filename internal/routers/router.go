package routers

import (
	_ "go-chat/docs"
	"go-chat/internal/api"
	"go-chat/internal/middleware"
	"go-chat/internal/service"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

func InitRouter() *gin.Engine {
	r := gin.Default()

	//注册CORS中间件
	config := cors.Config{
		// 允许所有来源 (开发环境用，生产环境建议指定具体域名)
		AllowAllOrigins: true,

		// 允许的请求方法
		AllowMethods: []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"},

		// 允许的 Header (非常重要，必须包含 Authorization 和自定义 Header)
		AllowHeaders: []string{
			"Origin",
			"Content-Length",
			"Content-Type",
			"Authorization",
			"Accept",
			"token",
			"X-Requested-With",
		},

		// 暴露给前端的 Header
		ExposeHeaders: []string{"Content-Length", "Access-Control-Allow-Origin"},

		// 允许携带凭证 (Cookie)
		AllowCredentials: true,

		// 预检请求缓存时间 (12小时)，避免频繁发 OPTIONS
		MaxAge: 12 * time.Hour,
	}

	r.Use(cors.New(config))

	//Swagger
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	//WebSocket Manager
	go service.Manager.Start()

	userApi := api.UserApi{}
	chatApi := api.ChatApi{}

	apiGroup := r.Group("/api")
	{
		//open route (No login required)
		userGroup := apiGroup.Group("/user")
		{
			userGroup.POST("/register", userApi.Register)
			userGroup.POST("/login", userApi.Login)
		}

		//protected route (login reqired)
		protectGroup := apiGroup.Group("")
		protectGroup.Use(middleware.JWTAuth()) //need verification
		{
			//user service
			protectGroup.GET("/user/info", userApi.GetUserInfo)
			protectGroup.GET("/user/profile", userApi.GetFullUserInfo) // 获取完整用户信息

			//WebSocket route
			protectGroup.GET("/ws", chatApi.Connect)
			protectGroup.GET("/chat/history", chatApi.GetHistory)

			// 搜索用户 (返回包含ID的DTO)
			protectGroup.GET("/user/search", api.SearchUser)

			// 好友相关
			protectGroup.POST("/friend/request", api.SendFriendRequest)    // 发送申请
			protectGroup.POST("/friend/handle", api.HandleFriendRequest)   // 同意/拒绝
			protectGroup.GET("/friend/requests", api.GetPendingRequests)   // 查看列表
			protectGroup.GET("/friend/list", api.GetFriendList)            // 查看好友列表
			protectGroup.POST("/friend/mark-read", api.MarkMessagesRead)   // 标记消息已读

		}

	}

	return r
}
