package service

import (
	"context"
	"encoding/json"
	"errors"
	"go-chat/global"
	"go-chat/internal/models"
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
	// [Refactor] 私聊不再查询 Redis，直接走 DB (因为私聊 Redis 缓存已被移除)
	// 无论是否翻页，统一查 DB，前端自行缓存

	query := global.DB.Where(
		"((from_user_id = ? AND to_user_id = ?) OR (from_user_id = ? AND to_user_id = ?)) AND type = ?",
		userID, targetID, targetID, userID, 2,
	)

	if lastMsgID > 0 {
		query = query.Where("id < ?", lastMsgID)
	}

	var messages []models.Message
	if err := query.Order("id desc").Limit(100).Find(&messages).Error; err != nil {
		return nil, err
	}

	// DB查出来是倒序的 [New -> Old]，需要反转为正序 [Old -> New]
	return ToMessageDTOs(messages), nil
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

	// 2. 尝试读 Redis Stream (XRevRange)
	// 如果 lastMsgID=0，拉取最新的 100 条
	// 如果 lastMsgID>0，暂不走 Stream 索引 (因为 Stream ID 是时间戳格式，跟 DB ID 不兼容，需要映射)
	// 简单起见：只有拉最新消息走 Stream，翻页走 DB
	if lastMsgID == 0 {
		cached, err := fetchFromStream(ctx, key, 100)
		if err == nil && len(cached) > 0 {
			// Stream 本身是正序的，fetchFromStream 返回正序
			return cached, nil
		}
	}

	// 3. 读 DB (Cache Warming)
	var messages []models.Message
	query := global.DB.Where("to_user_id = ? AND type = ?", groupID, 3)
	if lastMsgID > 0 {
		query = query.Where("id < ?", lastMsgID)
	}

	if err := query.Order("id desc").Limit(100).Find(&messages).Error; err != nil {
		return nil, err
	}

	dtos := ToMessageDTOs(messages) // 反转为正序

	// 只有当拉取最新消息且 Redis 为空时，才回写 (Cache Warming)
	// 注意：回写 Stream 比较麻烦，因为需要保证顺序和 ID，
	// 这里简化处理：如果不命中，就不回写了，等待新消息自然流入 Stream。
	// 或者全量回写比较耗时，暂略。

	return dtos, nil
}

// ------ 提取公共 Redis 操作 -------------------------------------------------------------

// 从 Redis Stream 拉取消息 (XRevRange)
// 返回正序 [Old -> New]
func fetchFromStream(ctx context.Context, key string, limit int) ([]MessageDTO, error) {
	// XRevRange key + - COUNT limit
	// 从最新(+)到最旧(-)
	cmd := global.RDB.XRevRangeN(ctx, key, "+", "-", int64(limit))
	streams, err := cmd.Result()

	if err != nil {
		return nil, err
	}
	if len(streams) == 0 {
		return nil, errors.New("cache miss")
	}

	n := len(streams)
	dtos := make([]MessageDTO, n)

	// Stream 返回的是 [New -> Old]，我们需要反转为 [Old -> New]
	for i, msg := range streams {
		val, ok := msg.Values["data"].(string)
		if !ok {
			continue
		}
		var dto MessageDTO
		if json.Unmarshal([]byte(val), &dto) == nil {
			dtos[n-1-i] = dto
		}
	}
	return dtos, nil
}
