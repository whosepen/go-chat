package service

import (
	"context"
	"encoding/json"
	"errors"
	"go-chat/global"
	"go-chat/internal/models"
	"time"

	"go.uber.org/zap"
)

var (
	ErrNotGroupMember     = errors.New("你不是该群成员，无法获取消息")
	ErrUnexpectedChatType = errors.New("message类型错误")
)

func GetHistoryMsg(ctx context.Context, userID uint, targetID uint, chatType uint, lastMsgID uint) ([]MessageDTO, error) {
	switch chatType {
	case 2:
		return getPrivateHistory(ctx, userID, targetID, lastMsgID)
	case 3:
		return getGroupHistory(ctx, userID, targetID, lastMsgID)
	}
	return []MessageDTO{}, ErrUnexpectedChatType
}

// ------ 拉取历史私聊消息 -----------------------------------------------------------------------
func getPrivateHistory(ctx context.Context, userID uint, targetID uint, lastMsgID uint) ([]MessageDTO, error) {

	// 1. 如果 lastMsgID > 0，说明是翻页拉取更早的消息，直接走 DB，不走 Redis
	if lastMsgID > 0 {
		var messages []models.Message
		err := global.DB.Where(
			"((from_user_id = ? AND to_user_id = ?) OR (from_user_id = ? AND to_user_id = ?)) AND type = ? AND id < ?",
			userID, targetID, targetID, userID, 2, lastMsgID,
		).Order("id desc").Limit(100).Find(&messages).Error
		if err != nil {
			return nil, err
		}
		return ToMessageDTOs(messages), nil
	}

	key := generateKey(targetID, userID, false)

	// 2. 尝试从 Redis 获取 (获取最新的 N 条)
	// 因为 Redis 里是 LRU 2000 条，所以直接取最后 100 条即可
	cached, err := fetchFromRedis(ctx, key)
	if err == nil && len(cached) > 0 {
		// Redis 里的顺序是 [旧 -> 新]，我们需要返回最新的 100 条
		// fetchFromRedis 返回的是完整 list，我们需要截取
		if len(cached) > 100 {
			cached = cached[len(cached)-100:]
		}
		return cached, nil
	}

	// 3. Redis 未命中或为空，执行数据库查询 (Cache Warming)
	var messages []models.Message
	err = global.DB.Where(
		"((from_user_id = ? AND to_user_id = ?) OR (from_user_id = ? AND to_user_id = ?)) AND type = ?",
		userID, targetID, targetID, userID, 2, // 2 = 私聊
	).Order("id desc").Limit(100).Find(&messages).Error // 按 ID 倒序查最新的 100 条
	
	if err != nil {
		return nil, err
	}
	if len(messages) == 0 {
		return []MessageDTO{}, nil
	}

	// DTO已反转list，dtos{[旧]->[新]}
	dtos := ToMessageDTOs(messages)

	// === 异步回写 Redis ===
	go func() {
		// 容错
		defer func() {
			if r := recover(); r != nil {
				global.Log.Error("Async redis panic", zap.Any("err", r))
			}
		}()

		// 使用新的 Context (防止主请求结束导致 Context Canceled)
		// 设置 5秒超时，防止 Goroutine 僵死
		asyncCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// 直接传入 dtos
		saveToRedis(asyncCtx, key, dtos)
	}()

	return dtos, nil
}

// ------ 拉取历史群聊消息 ----------------------------------------------------------------------
func getGroupHistory(ctx context.Context, userID, groupID, lastMsgID uint) ([]MessageDTO, error) {
	// [重要] 安全检查：用户必须是群成员才能看历史消息！
	// 避免有人随便猜一个 group_id 就能偷窥群聊
	var member models.GroupMember
	if err := global.DB.Where("group_id = ? AND user_id = ?", groupID, userID).First(&member).Error; err != nil {
		return nil, ErrNotGroupMember
	}

	// 1. 翻页逻辑：直接查 DB
	if lastMsgID > 0 {
		var messages []models.Message
		err := global.DB.Where(
			"to_user_id = ? AND type = ? AND id < ?",
			groupID, 3, lastMsgID,
		).Order("id desc").Limit(100).Find(&messages).Error
		if err != nil {
			return nil, err
		}
		return ToMessageDTOs(messages), nil
	}

	key := generateKey(groupID, 0, true) // true=群聊

	// 2. 尝试读 Redis
	cached, err := fetchFromRedis(ctx, key)
	if err == nil && len(cached) > 0 {
		if len(cached) > 100 {
			cached = cached[len(cached)-100:]
		}
		return cached, nil
	}

	// 3. 读 DB (Cache Warming)
	var messages []models.Message
	global.DB.Where(
		"to_user_id = ? AND type = ?",
		groupID, 3, // 3 = 群聊
	).Order("id desc").Limit(100).Find(&messages)

	dtos := ToMessageDTOs(messages)

	// === 异步回写 Redis ===
	go func() {
		defer func() {
			if r := recover(); r != nil {
				global.Log.Error("Async redis panic", zap.Any("err", r))
			}
		}()
		asyncCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		saveToRedis(asyncCtx, key, dtos)
	}()

	return dtos, nil
}

// ------ 提取公共 Redis 操作 -------------------------------------------------------------

// 从 Redis 拉取并反序列化
func fetchFromRedis(ctx context.Context, key string) ([]MessageDTO, error) {
	// 拉取全部 list
	resultList, err := global.RDB.LRange(ctx, key, 0, -1).Result()
	if err != nil || len(resultList) == 0 {
		return nil, errors.New("cache miss")
	}

	dtos := make([]MessageDTO, 0, len(resultList))
	for _, val := range resultList {
		var dto MessageDTO
		if json.Unmarshal([]byte(val), &dto) == nil {
			dtos = append(dtos, dto)
		}
	}
	return dtos, nil
}

// 写入 Redis
func saveToRedis(ctx context.Context, key string, dtos []MessageDTO) {
	if len(dtos) == 0 {
		return
	}

	pipe := global.RDB.Pipeline()
	// 先删掉旧的 Key 防止数据错乱
	pipe.Del(ctx, key)

	for _, dto := range dtos {
		jsonBytes, _ := json.Marshal(dto)
		pipe.RPush(ctx, key, jsonBytes)
	}
	pipe.Expire(ctx, key, time.Hour) // 设置过期时间

	if _, err := pipe.Exec(ctx); err != nil {
		global.Log.Error("async redis save failed", zap.Error(err))
	}

}
