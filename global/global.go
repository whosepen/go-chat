package global

import (
	"github.com/redis/go-redis/v9"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

var (
	Config *viper.Viper
	DB     *gorm.DB
	Log    *zap.Logger
	RDB    *redis.Client
)
