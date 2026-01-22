package main

import (
	"go-chat/global"
	"go-chat/internal/models" // 引入 models
	"go-chat/internal/pkg/initial"
	"go-chat/internal/routers"
	"go-chat/internal/service"
)

// @title GoChat API
// @version 1.0
// @description 这是一个 Go 语言开发的即时通讯系统后端 API 文档
// @host localhost:8080
// @BasePath /api
func main() {
	initial.InitLogger()
	initial.InitConfig()

	initial.InitDB()
	initial.InitRedis()
	initial.InitKafka()

	service.StartConsumer()

	// 自动迁移 (Auto Migrate)
	if err := global.DB.AutoMigrate(&models.User{}, &models.Message{}); err != nil {
		global.Log.Fatal("Database auto migration failed")
	}
	global.Log.Info("Database auto migration success")

	r := routers.InitRouter()

	port := global.Config.GetString("server.port")
	if port == "" {
		port = "8080"
	}
	global.Log.Info("Server starting on port " + port)
	r.Run(":" + port)
}
