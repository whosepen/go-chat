package cmd

import (
	"go-chat/global"
	"go-chat/internal/pkg/initial"
	"go-chat/internal/service"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
)

var consumerCmd = &cobra.Command{
	Use:   "consumer",
	Short: "Start Kafka Consumer Microservice",
	Run: func(cmd *cobra.Command, args []string) {
		initial.InitLogger()
		initial.InitConfig()
		initial.InitDB()
		initial.InitRedis()
		initial.InitKafka()

		global.Log.Info("Starting Kafka Consumer Service...")

		// Start Consumer Logic
		service.StartConsumer()

		// Block until signal
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		<-quit
		global.Log.Info("Shutting down consumer service...")
	},
}

func init() {
	rootCmd.AddCommand(consumerCmd)
}
