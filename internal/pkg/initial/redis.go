package initial

import (
	"context"
	"go-chat/global"

	"github.com/redis/go-redis/v9"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

func InitRedis() {
	addr := viper.GetString("redis.addr")
	password := viper.GetString("redis.password")
	db := viper.GetInt("redis.db")

	rdb := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password, // no password set
		DB:       db,       // use default DB
	})

	// 测试连接
	ctx := context.Background()
	_, err := rdb.Ping(ctx).Result()
	if err != nil {
		global.Log.Fatal("redis connect failed", zap.Error(err))
	}

	global.RDB = rdb
	global.Log.Info("Redis connected successfully")
}
