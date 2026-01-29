package routers

import (
	_ "go-chat/docs"
	"go-chat/internal/api"
	"go-chat/internal/middleware"
	"go-chat/internal/service"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

func InitRouter() *gin.Engine {
	r := gin.Default()

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

			//WebSocket route
			protectGroup.GET("/ws", chatApi.Connect)
			protectGroup.GET("/chat/history", chatApi.GetHistory)

			// 搜索用户 (返回包含ID的DTO)
			protectGroup.GET("/user/search", api.SearchUser)

			// 好友相关
			protectGroup.POST("/friend/request", api.SendFriendRequest)  // 发送申请
			protectGroup.POST("/friend/handle", api.HandleFriendRequest) // 同意/拒绝
			protectGroup.GET("/friend/requests", api.GetPendingRequests) // 查看列表
			protectGroup.GET("/friend/list", api.GetFriendList)          // 查看好友列表

		}

	}

	return r
}
