package service

import (
	"context"
	"encoding/json"
	"fmt"
	"go-chat/global"
	"go-chat/internal/models"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// 拉取到message列表
func GetHistoryMsg(ctx context.Context, userID uint, targetIDStr string) ([]MessageDTO, error) {
	// 1. 转换 targetID 为 uint 以便排序生成 Key
	key, err := generateKeyForStr(targetIDStr, userID)
	if err != nil {
		return nil, err
	}
	// 3. 尝试从 Redis 获取
	val, err := global.RDB.Get(ctx, key).Result()

	if err == nil {
		var cachedDTOs []MessageDTO
		// 反序列化 JSON 到 DTO 切片
		if jsonErr := json.Unmarshal([]byte(val), &cachedDTOs); jsonErr == nil {
			// 成功获取缓存，直接返回，无需查库
			return cachedDTOs, nil
			// 如果 Redis 里取出来的数据解不开，说明缓存脏了/格式错了
		} else {
			global.Log.Error("redis data unmarshal failed", zap.String("key", key), zap.Error(jsonErr))
		}
		// 如果反序列化失败，视同未命中，继续走下面逻辑查库
	} else if err != redis.Nil {
		// Redis 报错 (连接超时等)，记录日志但不崩溃，降级查数据库
		fmt.Println("Redis error:", err)
	}

	// === Redis 未命中或出错，执行数据库查询 ===

	var messages []models.Message
	err = global.DB.Where(
		"(from_user_id = ? AND to_user_id = ?) OR (from_user_id = ? AND to_user_id = ?)",
		userID, targetIDStr, targetIDStr, userID,
	).Order("created_at desc").Limit(100).Find(&messages).Error
	// 倒序desc，最新100条消息
	if err != nil {
		return nil, err
	}

	dtos := ToMessageDTOs(messages)

	// 回写 Redis (设置过期时间，例如 10 分钟)
	// 注意：缓存的是转换后的 DTO 数据
	if len(dtos) > 0 { // 只有有数据才缓存，防止缓存空值(看业务需求)
		jsonBytes, _ := json.Marshal(dtos)
		// 这里的过期时间根据业务定，比如 10 分钟
		// 注意：如果用户发送新消息，记得要清除这个 Key，否则用户 10 分钟内看不到新消息
		global.RDB.Set(ctx, key, jsonBytes, 10*time.Minute)
	}

	return dtos, nil
}
