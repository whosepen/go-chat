package service

import (
	"context"
	"errors"
	"go-chat/global"
	"go-chat/internal/models"
	"go-chat/internal/repository"

	"gorm.io/gorm"
)

var (
	ErrRequestExist     = errors.New("已发送过申请，请等待对方处理")
	ErrAlreadyFriend    = errors.New("你们已经是好友了")
	ErrAddYourself      = errors.New("不能添加自己为好友")
	ErrRequestNotFound  = errors.New("申请记录不存在")
	ErrRequestHandled   = errors.New("该申请已被处理")
	ErrPermissionDenied = errors.New("无权处理此申请")
	ErrBlockedByTarget  = errors.New("已被该用户拉黑")
	ErrBlockTarget      = errors.New("你已拉黑对方")
	ErrIsNotFriend      = errors.New("你们不是好友")
)

// --------------------------
// 发送好友申请
// --------------------------
func SendFriendRequest(ctx context.Context, userID uint, req SendFriendRequestReq) error {
	// 基本校验
	if userID == req.TargetID {
		return ErrAddYourself
	}

	// 检查目标用户是否存在
	var target models.User
	if err := global.DB.WithContext(ctx).First(&target, req.TargetID).Error; err != nil {
		return errors.New("目标用户不存在")
	}

	// 检查是否已经是好友 (查询 relation 表)
	var rel models.Relation
	err := global.DB.WithContext(ctx).
		Where("owner_id = ? AND target_id = ? AND type = 1", userID, req.TargetID).
		First(&rel).Error
	if err == nil {
		return ErrAlreadyFriend // 查到了记录，说明已经是好友
	}

	// 检查是否被对方拉黑
	err = global.DB.WithContext(ctx).Unscoped().
		Where("owner_id = ? AND target_id = ? AND type = 2", req.TargetID, userID).
		First(&rel).Error
	if err == nil {
		return ErrBlockedByTarget // 查到了记录，说明被拉黑
	}

	err = global.DB.WithContext(ctx).Unscoped().
		Where("owner_id = ? AND target_id = ? AND type = 2", userID, req.TargetID).
		First(&rel).Error
	if err == nil {
		return ErrBlockTarget // 查到了记录，说明拉黑拉黑对方
	}

	// 检查是否重复发送申请 (查询 friend_requests 表，状态为 0-待处理)
	var existReq models.FriendRequest
	err = global.DB.WithContext(ctx).
		Where("sender_id = ? AND receiver_id = ? AND status = 0", userID, req.TargetID).
		First(&existReq).Error
	if err == nil {
		return ErrRequestExist // 查到了记录，说明申请还在排队
	}

	// 4. 创建申请记录
	friendReq := models.FriendRequest{
		SenderID:   userID,
		ReceiverID: req.TargetID,
		Remark:     req.Remark,
		Status:     0, // 0: 待处理
	}

	return global.DB.WithContext(ctx).Create(&friendReq).Error
}

// --------------------------
// 处理好友申请 (同意/拒绝)
// --------------------------
func HandleFriendRequest(ctx context.Context, userID uint, req HandleFriendRequestReq) error {
	// 1. 查找申请记录
	var friendReq models.FriendRequest
	if err := global.DB.WithContext(ctx).First(&friendReq, req.RequestID).Error; err != nil {
		return ErrRequestNotFound
	}

	// 2. 校验权限：只有接收者才能处理申请
	if friendReq.ReceiverID != userID {
		return ErrPermissionDenied
	}

	// 3. 校验状态：防止重复处理
	if friendReq.Status != 0 {
		return ErrRequestHandled
	}

	// 4. 处理逻辑
	return global.DB.Transaction(func(tx *gorm.DB) error {
		// 4.1 如果是同意操作，先清理可能存在的反向申请
		if req.Action == 1 {
			// 检查是否已经是好友
			var existingRel models.Relation
			err := tx.Where("owner_id = ? AND target_id = ? AND type = 1",
				friendReq.SenderID, friendReq.ReceiverID).First(&existingRel).Error
			if err == nil {
				// 已经是好友关系，只更新申请状态为已同意，不重复创建relation
				friendReq.Status = 1
				if err := tx.Save(&friendReq).Error; err != nil {
					return err
				}
				// 缓存失效
				repository.NewRelationRepository().InvalidateRelationCache(ctx, friendReq.SenderID, friendReq.ReceiverID)
				return nil
			}

			// 删除用户发给对方的好友申请（反向申请）
			if err := tx.Where("sender_id = ? AND receiver_id = ? AND status = 0",
				friendReq.ReceiverID, friendReq.SenderID).Delete(&models.FriendRequest{}).Error; err != nil {
				return err
			}
		}

		// 4.2 更新申请状态 (1:同意, 2:拒绝)
		friendReq.Status = req.Action
		if err := tx.Save(&friendReq).Error; err != nil {
			return err
		}

		// 如果是拒绝，到这里就结束了
		if req.Action == 2 {
			return nil
		}

		var rel models.Relation
		// 如果曾经加过好友，后被删除了，直接恢复旧relation
		if err := tx.Unscoped().Where("owner_id = ? AND target_id = ? AND type = 1",
			friendReq.SenderID, friendReq.ReceiverID).First(&rel).Error; err == nil {
			if err := tx.Model(&rel).Update("delete_at", nil).Error; err != nil {
				return err
			}
		} else {
			rel = models.Relation{
				OwnerID:  friendReq.SenderID,
				TargetID: friendReq.ReceiverID,
				Type:     1,
			}
			if err := tx.Create(&rel).Error; err != nil {
				return err
			}
		}

		if err := tx.Unscoped().Where("owner_id = ? AND target_id = ? AND type = 1",
			friendReq.ReceiverID, friendReq.Sender).First(&rel).Error; err == nil {
			if err := tx.Model(&rel).Update("delete_at", nil).Error; err != nil {
				return err
			}
		} else {
			rel = models.Relation{
				OwnerID:  friendReq.ReceiverID,
				TargetID: friendReq.SenderID,
				Type:     1,
			}
			if err := tx.Create(&rel).Error; err != nil {
				return err
			}
		}

		// 缓存失效
		repository.NewRelationRepository().InvalidateRelationCache(ctx, friendReq.SenderID, friendReq.ReceiverID)
		return nil
	})
}

// --------------------------
// 获取待处理的申请列表
// --------------------------
// 返回 DTO 列表，避免暴露 Sender 的敏感信息
func GetPendingRequests(ctx context.Context, userID uint) ([]FriendRequestDTO, error) {
	var requests []models.FriendRequest

	// 1. Where: 查发给我的(receiver_id = userID) 且 没处理的(status = 0)
	// 2. Preload("Sender"): 告诉 GORM "顺便把 Sender 字段对应的 User 信息也给我查出来"
	// 3. Order: 按时间倒序
	err := global.DB.WithContext(ctx).
		Preload("Sender").
		Where("receiver_id = ? AND status = 0", userID).
		Order("created_at desc").
		Find(&requests).Error

	if err != nil {
		return nil, err
	}

	// 转换 DTO
	dtos := make([]FriendRequestDTO, 0, len(requests))
	for _, req := range requests {
		// 因为用了 Preload，这里可以直接通过 req.Sender 拿到用户信息
		// 如果不使用 Preload，req.SenderID虽然有值，但是取出来时不会为req.Sender填充user,
		// 如果没查到 Sender (比如用户注销了)，req.Sender.ID 会是零值，不会 panic
		if req.Sender.ID == 0 {
			continue
		}

		dtos = append(dtos, FriendRequestDTO{
			ID:         req.ID,
			SenderID:   req.SenderID,
			SenderName: req.Sender.Username, // 直接取值！
			Avatar:     req.Sender.Avatar,   // 直接取值！
			Remark:     req.Remark,
			Status:     req.Status,
			CreatedAt:  req.CreatedAt.Format("2006-01-02 15:04:05"),
		})
	}

	return dtos, nil
}

// SearchUserByUsername 根据用户名搜索用户
func SearchUserByUsername(ctx context.Context, username string) (*UserResponseDTO, error) {
	var user models.User
	err := global.DB.WithContext(ctx).
		Where("username = ?", username).
		First(&user).Error

	if err != nil {
		return nil, errors.New("用户不存在")
	}

	dto := ToUserDTO(user)

	return &dto, nil
}

// GetFriendList 获取我的好友列表
func GetFriendList(ctx context.Context, userID uint) ([]UserResponseDTO, error) {
	// 1. 查询用户的所有好友关系记录
	var relations []models.Relation
	err := global.DB.WithContext(ctx).
		Where("owner_id = ? AND type = 1", userID).
		Find(&relations).Error
	if err != nil {
		return nil, err
	}

	dtos := make([]UserResponseDTO, 0, len(relations))

	for _, rel := range relations {
		// 2. 获取好友信息
		var user models.User
		if err := global.DB.WithContext(ctx).First(&user, rel.TargetID).Error; err != nil {
			continue // 好友不存在，跳过
		}

		// 3. 查询在线状态
		online := global.RDB.Exists(ctx, onlineStatusKey(user.ID)).Val() > 0

		// 4. 计算未读消息数量：查询该好友发给我的消息中，msg_id > last_read_msg_id 的数量
		var unreadCount int64
		global.DB.WithContext(ctx).
			Model(&models.Message{}).
			Where("from_user_id = ? AND to_user_id = ? AND id > ?", rel.TargetID, userID, rel.LastReadMsgID).
			Count(&unreadCount)

		// 5. 获取最后一条消息时间
		var lastMsg models.Message
		lastMsgTime := int64(0)
		global.DB.WithContext(ctx).
			Where("(from_user_id = ? AND to_user_id = ?) OR (from_user_id = ? AND to_user_id = ?)",
				userID, rel.TargetID, rel.TargetID, userID).
			Order("id DESC").
			First(&lastMsg)
		if lastMsg.ID > 0 {
			lastMsgTime = lastMsg.CreatedAt.UnixMilli()
		}

		dtos = append(dtos, UserResponseDTO{
			ID:          user.ID,
			Username:    user.Username,
			Nickname:    user.Nickname,
			Avatar:      user.Avatar,
			Online:      online,
			Email:       user.Email,
			UnreadCount: int(unreadCount),
			LastMsgTime: lastMsgTime,
		})
	}

	return dtos, nil
}

// MarkMessagesAsRead 标记与某好友的消息为已读
func MarkMessagesAsRead(ctx context.Context, userID uint, req MarkMessagesReadReq) error {
	// 1. 查找该好友关系记录
	var rel models.Relation
	if err := global.DB.WithContext(ctx).
		Where("owner_id = ? AND target_id = ? AND type = 1", userID, req.TargetID).
		First(&rel).Error; err != nil {
		return errors.New("好友关系不存在")
	}

	// 2. 获取该好友发给我的最后一条消息ID
	var lastMsg models.Message
	if err := global.DB.WithContext(ctx).
		Where("from_user_id = ? AND to_user_id = ?", req.TargetID, userID).
		Order("id DESC").
		First(&lastMsg).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// 没有消息，无需更新
			return nil
		}
		return err
	}

	// 3. 更新 last_read_msg_id
	return global.DB.WithContext(ctx).
		Model(&rel).
		Where("id = ?", rel.ID).
		Update("last_read_msg_id", lastMsg.ID).Error
}

// DeleteFriend 删除好友关系
func DeleteFriend(ctx context.Context, userID uint, targetID uint) error {
	var rel models.Relation
	// 查有无好友关系(包含未删除的拉黑)
	if err := global.DB.WithContext(ctx).
		Where("owner_id = ? AND target_id = ? ", userID, targetID).Delete(&rel).Error; err != nil {
		if err := global.DB.WithContext(ctx).Model(&rel).
			Where("owner_id = ? AND target_id = ?", targetID, userID).Delete(&rel).Error; err != nil {
			return ErrIsNotFriend
		}
		return nil
	}
	global.DB.WithContext(ctx).Model(&rel).
		Where("owner_id = ? AND target_id = ?", targetID, userID).Delete(&rel)
	global.DB.WithContext(ctx).Model(&rel).
		Where("owner_id = ? AND target_id = ?", userID, targetID).Delete(&rel)

	// 缓存失效
	repository.NewRelationRepository().InvalidateRelationCache(ctx, userID, targetID)
	return nil
}

// BlockFriend 拉黑好友
func BlockFriend(ctx context.Context, userID uint, targetID uint) error {
	var rel models.Relation
	if err := global.DB.WithContext(ctx).Model(&rel).
		Where("owner_id = ? AND target_id = ? ", userID, targetID).Error; err == nil {
		if err := global.DB.WithContext(ctx).Model(&rel).
			Where("owner_id = ? AND target_id = ? ", userID, targetID).
			Update("type", 2).Error; err != nil {
			return err
		}
		// 缓存失效
		repository.NewRelationRepository().InvalidateRelationCache(ctx, userID, targetID)
		return nil
	}
	return ErrIsNotFriend
}

// UnblockFriend 将好友移出黑名单
func UnblockFriend(ctx context.Context, userID uint, targetID uint) error {
	var rel models.Relation
	if err := global.DB.WithContext(ctx).Model(&rel).Unscoped().
		Where("owner_id = ? AND target_id = ? AND type = 2", userID, targetID).Error; err == nil {
		if err := global.DB.WithContext(ctx).Model(&rel).Unscoped().
			Where("owner_id = ? AND target_id = ? AND type = 2", userID, targetID).
			Update("type", 1).Error; err != nil {
			return err
		}
		// 缓存失效
		repository.NewRelationRepository().InvalidateRelationCache(ctx, userID, targetID)
		return nil
	} else {
		return err
	}
}

// GetFriendInfo 获取好友详细信息
func GetFriendInfo(ctx context.Context, userID uint, targetID uint) (FriendInfoDTO, error) {
	if ok := repository.NewRelationRepository().IsFriend(ctx, userID, targetID); !ok {
		return FriendInfoDTO{}, ErrIsNotFriend
	}
	var repo *models.User

	repo, err := repository.NewUserRepository().FindByID(ctx, targetID)
	if err != nil {
		return FriendInfoDTO{}, err
	}
	return FriendInfoDTO{
		Username: repo.Username,
		Nickname: repo.Nickname,
		Avatar:   repo.Avatar,
		Email:    repo.Email,
	}, nil

}

// GetBlockList 获取拉黑列表
func GetBlockList(ctx context.Context, userID uint) ([]UserResponseDTO, error) {
	// 1. 查询用户的所有拉黑关系记录
	var relations []models.Relation
	err := global.DB.WithContext(ctx).
		Where("owner_id = ? AND type = 2", userID).
		Find(&relations).Error
	if err != nil {
		return nil, err
	}

	dtos := make([]UserResponseDTO, 0, len(relations))

	for _, rel := range relations {
		// 2. 获取好友信息
		var user models.User
		if err := global.DB.WithContext(ctx).First(&user, rel.TargetID).Error; err != nil {
			continue // 好友不存在，跳过
		}

		// 4. 计算未读消息数量：查询该好友发给我的消息中，msg_id > last_read_msg_id 的数量
		var unreadCount int64
		global.DB.WithContext(ctx).
			Model(&models.Message{}).
			Where("from_user_id = ? AND to_user_id = ? AND id > ?", rel.TargetID, userID, rel.LastReadMsgID).
			Count(&unreadCount)

		// 5. 获取最后一条消息时间
		var lastMsg models.Message
		lastMsgTime := int64(0)
		global.DB.WithContext(ctx).
			Where("(from_user_id = ? AND to_user_id = ?) OR (from_user_id = ? AND to_user_id = ?)",
				userID, rel.TargetID, rel.TargetID, userID).
			Order("id DESC").
			First(&lastMsg)
		if lastMsg.ID > 0 {
			lastMsgTime = lastMsg.CreatedAt.UnixMilli()
		}

		dtos = append(dtos, UserResponseDTO{
			ID:          user.ID,
			Username:    user.Username,
			Nickname:    user.Nickname,
			Avatar:      user.Avatar,
			Online:      false,
			UnreadCount: int(unreadCount),
			LastMsgTime: lastMsgTime,
		})
	}

	return dtos, nil
}
