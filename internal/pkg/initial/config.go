package initial

import (
	"go-chat/global"

	"github.com/spf13/viper"
	"go.uber.org/zap"
)

// InitConfig 读取配置文件
func InitConfig() {
	viper.SetConfigName("config")   // 文件名 (不带后缀)
	viper.SetConfigType("yaml")     // 文件类型
	viper.AddConfigPath("./config") // 路径
	viper.AddConfigPath(".")        // 根目录

	// 允许环境变量覆盖
	viper.AutomaticEnv()

	err := viper.ReadInConfig()
	if err != nil {
		global.Log.Fatal("Fatal error config file", zap.Error(err))
	}
	global.Config = viper.GetViper()
	global.Log.Info("Config loaded successfully")
}
