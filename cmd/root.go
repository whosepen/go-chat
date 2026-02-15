package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// rootCmd 代表没有参数时的基础命令
var rootCmd = &cobra.Command{
	Use:   "chat-app",
	Short: "A distributed chat application",
	Long:  `这是我的聊天室微服务项目，包含聊天服务和文件服务。`,
}

// Execute 是 main.go 调用的入口
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
