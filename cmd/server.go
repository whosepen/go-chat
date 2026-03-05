package cmd

import (
	"go-chat/global"
	"go-chat/internal/models" // 引入 models
	"go-chat/internal/pkg/initial"
	"go-chat/internal/repository"
	"go-chat/internal/routers"
	"go-chat/internal/service"

	"github.com/spf13/cobra"
)

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Start mainSERVER Microservice",
	Run: func(cmd *cobra.Command, args []string) {
		initial.InitLogger()
		initial.InitConfig()

		initial.InitDB()
		initial.InitRedis()
		initial.InitKafka()
		initial.InitOssClient() // 初始化 OSS gRPC 客户端

		// service.StartConsumer() // Consumer moved to separate microservice
		global.Log.Info("Starting Redis Push Listener...")
		service.StartPushListener() // Start Redis Pub/Sub listener for push notifications
		repository.StartRelationEventListener() // Start Local Cache Invalidation Listener
		// service.StartBatchPersister()   // [Deprecated] 废弃 Redis Queue 模式，改用全量 Kafka

		// 自动迁移 (Auto Migrate)
		if err := global.DB.AutoMigrate(
			&models.User{},
			&models.Message{},
			&models.Relation{},
			&models.Group{},
			&models.GroupMember{},
			&models.GroupRequest{},
			&models.FriendRequest{}); err != nil {
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
	},
}

func init() {
	rootCmd.AddCommand(serverCmd)
}
