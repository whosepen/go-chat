package initial

import (
	"fmt"
	"go-chat/global"

	"github.com/spf13/viper"
	"go.uber.org/zap"
)

// InitConfig 读取配置文件
func InitConfig() {
	viper.SetConfigName("config")   // 文件名 (不带后缀)
	viper.SetConfigType("yaml")     // 文件类型
	viper.AddConfigPath("./config") // 路径

	err := viper.ReadInConfig()
	if err != nil {
		panic(fmt.Errorf("Fatal error config file: %s \n", err))
	}
	global.Config = viper.GetViper()
	fmt.Println("Config loaded successfully")
}

// InitLogger 简单的日志初始化 (后续我们会优化)
func InitLogger() {
	logger, _ := zap.NewDevelopment()
	global.Log = logger
}
