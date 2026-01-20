package initial

import (
	"context"
	"fmt"
	"go-chat/global"

	"github.com/redis/go-redis/v9"
)

func InitRedis() {
	addr := global.Config.GetString("redis.addr")
	password := global.Config.GetString("redis.password")
	db := global.Config.GetInt("redis.db")

	rdb := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password, // no password set
		DB:       db,       // use default DB
	})

	// 测试连接
	ctx := context.Background()
	_, err := rdb.Ping(ctx).Result()
	if err != nil {
		panic(fmt.Sprintf("redis connect failed: %s", err))
	}

	global.RDB = rdb
	global.Log.Info("Redis connected successfully")
}
