package service

import (
	"context"
	"errors"
	"go-chat/global"
	"go-chat/internal/models"
	"go-chat/internal/pkg/protocol"
	"go-chat/internal/pkg/utils"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

var (
	ErrGroupNotFound     = errors.New("群组不存在")
	ErrAlreadyInGroup    = errors.New("你已在群中")
	ErrNotInGroup        = errors.New("你不在群中")
	ErrNotGroupAdmin     = errors.New("只有群主或管理员可以执行此操作")
	ErrNotGroupOwner     = errors.New("只有群主可以执行此操作")
	ErrCannotKickOwner   = errors.New("不能移除群主")
	ErrCannotQuitAsOwner = errors.New("群主不能直接退出，请先转让群或解散群")
	ErrUnknownForGroup   = errors.New("群聊业务未知错误")
	ErrYouRNotMember     = errors.New("你不在该群中")
)

// CreateGroup 创建群组
func CreateGroup(ctx context.Context, ownerID uint, req CreateGroupReq) (*GroupInfoDTO, error) {
	group := models.Group{
		Name:    req.Name,
		OwnerID: ownerID,
		Desc:    req.Desc,
		Icon:    req.Icon,
	}

	// 事务：建群 + 把自己加进去 + 生成群号
	err := global.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&group).Error; err != nil {
			return err
		}

		group.Code = utils.GenGroupCode(group.ID)
		if err := tx.Model(&group).Update("code", group.Code).Error; err != nil {
			return err
		}

		member := models.GroupMember{
			GroupID: group.ID,
			UserID:  ownerID,
			Role:    1, // 群主
		}
		if err := tx.Create(&member).Error; err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	// 返回 DTO
	return &GroupInfoDTO{
		ID:        group.ID,
		Code:      group.Code,
		Name:      group.Name,
		Icon:      group.Icon,
		Desc:      group.Desc,
		OwnerID:   group.OwnerID,
		MemberCnt: 1,
	}, nil
}

// JoinGroup 加入群（发送入群申请）
func JoinGroup(ctx context.Context, userID int, req SendGroupRequestReq) error {
	// 1. 检查群是否存在
	var group models.Group
	if err := global.DB.WithContext(ctx).First(&group, req.GroupID).Error; err != nil {
		return ErrGroupNotFound
	}

	// 2. 检查是否已在群里
	var member models.GroupMember
	if err := global.DB.WithContext(ctx).
		Where("group_id = ? AND user_id = ?", req.GroupID, userID).
		First(&member).Error; err == nil {
		return ErrAlreadyInGroup
	}

	// 3. 检查是否重复发送申请 (查询 group_requests 表，状态为 0-待处理)
	var existReq models.GroupRequest
	if err := global.DB.WithContext(ctx).
		Where("sender_id = ? AND group_id = ? AND status = 0", userID, req.GroupID).
		First(&existReq).Error; err == nil {
		return ErrRequestExist // 查到了记录，说明申请还在排队
	}

	// 4. 创建申请记录
	groupReq := models.GroupRequest{
		SenderID: userID,
		GroupID:  req.GroupID,
		Remark:   req.Remark,
		Status:   0, // 0: 待处理
	}

	return global.DB.WithContext(ctx).Create(&groupReq).Error
}

// SearchGroupByCode 通过group code查找群
func SearchGroupByCode(ctx context.Context, groupCode string) (*GroupInfoDTO, error) {
	key := groupIDKey(groupCode)
	if cached, err := global.RDB.Get(ctx, key).Int(); err == nil {
		return GetGroupInfo(ctx, uint(cached))
	}
	var group models.Group
	if err := global.DB.WithContext(ctx).Model(&models.Group{}).
		Where("code = ?", groupCode).
		First(&group).Error; err != nil {
		return nil, ErrGroupNotFound
	}

	// 统计成员数量
	var memberCount int64
	err := global.DB.WithContext(ctx).
		Model(&models.GroupMember{}).Where("group_id = ?", group.ID).Count(&memberCount).Error

	if err != nil {
		global.Log.Warn("Get group members count failed", zap.Error(err))
		return nil, err
	}

	// 异步存code-id到redis
	go func(key string, id uint) {
		tctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer func() {
			if r := recover(); r != nil {
				global.Log.Warn("set redis panic", zap.Any("err", r))
			}
			cancel()
		}()
		if err := global.RDB.Set(tctx, key, id, 6*time.Hour).Err(); err != nil {
			global.Log.Debug("set redis error", zap.Error(err))
		}
	}(key, group.ID)

	return &GroupInfoDTO{
		ID:        group.ID,
		Code:      group.Code,
		Name:      group.Name,
		Icon:      group.Icon,
		Desc:      group.Desc,
		OwnerID:   group.OwnerID,
		MemberCnt: int(memberCount),
	}, nil
}

// GetGroupInfo 获取群信息
func GetGroupInfo(ctx context.Context, groupID uint) (*GroupInfoDTO, error) {
	var group models.Group
	if err := global.DB.WithContext(ctx).First(&group, groupID).Error; err != nil {
		return nil, ErrGroupNotFound
	}

	// 统计成员数量
	var memberCount int64
	err := global.DB.WithContext(ctx).
		Model(&models.GroupMember{}).Where("group_id = ?", groupID).Count(&memberCount).Error

	if err != nil {
		global.Log.Warn("Get group members count failed", zap.Error(err))
		return nil, err
	}

	return &GroupInfoDTO{
		ID:        group.ID,
		Code:      group.Code,
		Name:      group.Name,
		Icon:      group.Icon,
		Desc:      group.Desc,
		OwnerID:   group.OwnerID,
		MemberCnt: int(memberCount),
	}, nil
}

// GetGroupMembers 获取群成员列表
func GetGroupMembers(ctx context.Context, groupID uint) ([]GroupMemberDTO, error) {
	// 检查群是否存在
	var group models.Group
	if err := global.DB.WithContext(ctx).First(&group, groupID).Error; err != nil {
		return nil, ErrGroupNotFound
	}

	var members []models.GroupMember
	if err := global.DB.WithContext(ctx).
		Where("group_id = ?", groupID).
		Order("role ASC, id ASC").
		Find(&members).Error; err != nil {
		return nil, err
	}

	dtos := make([]GroupMemberDTO, 0, len(members))
	for _, m := range members {
		dtos = append(dtos, GroupMemberDTO{
			UserID:   m.UserID,
			Username: m.Username,
			Nickname: m.Nickname,
			Role:     m.Role,
			Mute:     m.Mute,
			JoinTime: m.CreatedAt.Format("2006-01-02 15:04:05"),
		})
	}
	return dtos, nil
}

// HandleGroupRequest 处理入群申请
func HandleGroupRequest(ctx context.Context, userID uint, req HandleGroupRequestReq) error {
	// 1. 查找申请记录
	var groupReq models.GroupRequest
	if err := global.DB.WithContext(ctx).
		Preload("Sender").
		First(&groupReq, req.RequestID).Error; err != nil {
		return ErrRequestNotFound
	}

	// 2. 校验状态：防止重复处理
	if groupReq.Status != 0 {
		return ErrRequestHandled
	}

	// 3. 检查处理者是否是群主或管理员
	var operator models.GroupMember
	if err := global.DB.WithContext(ctx).
		Where("group_id = ? AND user_id = ?", groupReq.GroupID, userID).
		First(&operator).Error; err != nil {
		return ErrNotInGroup
	}

	if operator.Role != 1 && operator.Role != 2 {
		return ErrNotGroupAdmin
	}

	// 4. 事务处理
	return global.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 更新申请状态
		groupReq.Status = req.Action
		if err := tx.Save(&groupReq).Error; err != nil {
			return err
		}

		// 如果申请者已注销，直接结束
		if groupReq.Sender.ID == 0 {
			return nil
		}

		// 如果是拒绝，到这里就结束了
		if req.Action == 2 {
			return nil
		}

		var lastMsg models.Message
		var lastMsgID uint

		// 如果是同意，添加申请者为群成员
		// 先将已读消息置为群中最新消息，避免入群时消息爆炸
		global.DB.WithContext(ctx).Model(&lastMsg).
			Where("to_user_id = ? AND type = 3", groupReq.GroupID).
			Order("created_at DESC").Limit(1).Select("id").Find(&lastMsgID)

		member := models.GroupMember{
			GroupID:       uint(groupReq.GroupID),
			UserID:        uint(groupReq.SenderID),
			Username:      groupReq.Sender.Username,
			Nickname:      groupReq.Sender.Nickname,
			Role:          3, // 普通成员
			LastReadMsgID: lastMsgID,
		}
		return tx.Create(&member).Error
	})
}

// GetMyGroups 获取我的群聊列表
func GetMyGroups(ctx context.Context, userID uint) ([]GroupListReqDTO, error) {
	var members []models.GroupMember
	if err := global.DB.WithContext(ctx).
		Where("user_id = ?", userID).
		Find(&members).Error; err != nil {
		return nil, err
	}

	dtos := make([]GroupListReqDTO, 0, len(members))
	for _, m := range members {
		var group models.Group
		if err := global.DB.WithContext(ctx).First(&group, m.GroupID).Error; err != nil {
			continue
		}

		// 计算未读消息数
		var unreadCount int64
		unreadCount, err := GetGroupUnreadCount(ctx, userID, m.GroupID)
		if err != nil {
			unreadCount = 0
		}
		// 最新消息时间
		var lastMsg models.Message
		lastMsgTime := int64(0)
		global.DB.WithContext(ctx).
			Where("to_user_id = ? AND type = ?", group.ID, protocol.TypeGroupMsg).
			Order("id DESC").
			First(&lastMsg)
		if lastMsg.ID > 0 {
			lastMsgTime = lastMsg.CreatedAt.UnixMilli()
		}

		dtos = append(dtos, GroupListReqDTO{
			ID:          group.ID,
			Code:        group.Code,
			Name:        group.Name,
			Icon:        group.Icon,
			UnreadCount: int(unreadCount),
			LastMsgTime: lastMsgTime,
		})
	}
	return dtos, nil
}

// GetGroupUnreadCount 获取群聊未读消息数量
func GetGroupUnreadCount(ctx context.Context, userID, groupID uint) (int64, error) {
	// 1. 查找该群成员记录
	var member models.GroupMember
	if err := global.DB.WithContext(ctx).
		Where("group_id = ? AND user_id = ?", groupID, userID).
		First(&member).Error; err != nil {
		return 0, err
	}

	// 2. 查询该群中 last_read_msg_id 之后的消息数量
	var unreadCount int64
	err := global.DB.WithContext(ctx).
		Model(&models.Message{}).
		Where("to_user_id = ? AND type = ? AND id > ?", groupID, protocol.TypeGroupMsg, member.LastReadMsgID).
		Count(&unreadCount).Error

	return unreadCount, err
}

// MarkGroupMessagesAsRead 标记群聊消息为已读
func MarkGroupMessagesAsRead(ctx context.Context, userID, groupID uint) error {
	// 查找该群成员记录
	var member models.GroupMember
	if err := global.DB.WithContext(ctx).
		Where("group_id = ? AND user_id = ?", groupID, userID).
		First(&member).Error; err != nil {
		return ErrYouRNotMember
	}

	var lastMsg models.Message
	var lastMsgID uint

	if err := global.DB.WithContext(ctx).Model(&lastMsg).Where("to_user_id = ? AND type = 3", groupID).
		Order("created_at DESC").Limit(1).Select("id").Find(&lastMsgID).Error; err != nil {
		return nil // 无群消息
	}

	// 更新 last_read_msg_id
	return global.DB.WithContext(ctx).
		Model(&member).
		Where("id = ?", member.ID).
		Update("last_read_msg_id", lastMsgID).Error
}

// QuitGroup 退出群聊
func QuitGroup(ctx context.Context, userID uint, groupID uint) error {
	// 1. 检查是否是群主
	var group models.Group
	if err := global.DB.WithContext(ctx).First(&group, groupID).Error; err != nil {
		return ErrGroupNotFound
	}

	if group.OwnerID == userID {
		return ErrCannotQuitAsOwner
	}

	// 2. 检查是否在群里
	var member models.GroupMember
	if err := global.DB.WithContext(ctx).
		Where("group_id = ? AND user_id = ?", groupID, userID).
		First(&member).Error; err != nil {
		return ErrNotInGroup
	}

	// 3. 删除成员记录
	return global.DB.WithContext(ctx).Delete(&member).Error
}

// UpdateGroupInfo 修改群信息
func UpdateGroupInfo(ctx context.Context, userID uint, req UpdateGroupInfoReq) error {
	// 1. 检查群是否存在
	var group models.Group
	if err := global.DB.WithContext(ctx).First(&group, req.GroupID).Error; err != nil {
		return ErrGroupNotFound
	}

	// 2. 检查是否是群主
	if group.OwnerID != userID {
		return ErrNotGroupOwner
	}

	// 3. 更新群信息
	updates := make(map[string]interface{})
	if req.Name != "" {
		updates["name"] = req.Name
	}
	if req.Desc != "" {
		updates["desc"] = req.Desc
	}
	if req.Icon != "" {
		updates["icon"] = req.Icon
	}

	if len(updates) == 0 {
		return nil
	}

	return global.DB.WithContext(ctx).Model(&group).Updates(updates).Error
}

// KickMember 踢出群成员
func KickMember(ctx context.Context, operatorID uint, targetUserID uint, groupID uint) error {
	// 1. 检查群是否存在
	var group models.Group
	if err := global.DB.WithContext(ctx).First(&group, groupID).Error; err != nil {
		return ErrGroupNotFound
	}

	// 2. 检查操作者身份
	var operator models.GroupMember
	if err := global.DB.WithContext(ctx).
		Where("group_id = ? AND user_id = ?", groupID, operatorID).
		First(&operator).Error; err != nil {
		return ErrNotInGroup
	}

	// 3. 检查目标成员
	var target models.GroupMember
	if err := global.DB.WithContext(ctx).
		Where("group_id = ? AND user_id = ?", groupID, targetUserID).
		First(&target).Error; err != nil {
		return ErrNotInGroup
	}

	// 4. 权限校验
	// 群主可以踢任何人（包括管理员）
	if operator.Role == 1 {
		// 可以踢
	} else if operator.Role == 2 {
		// 管理员只能踢普通成员
		if target.Role == 1 || target.Role == 2 {
			return ErrCannotKickOwner
		}
	} else {
		return ErrNotGroupAdmin
	}

	// 不能踢群主
	if target.Role == 1 {
		return ErrCannotKickOwner
	}

	// 5. 删除成员记录
	return global.DB.WithContext(ctx).Delete(&target).Error
}

// GetPendingGroupRequests 获取收到的入群申请列表
func GetPendingGroupRequests(ctx context.Context, userID uint) ([]GroupRequestDTO, error) {
	// 1. 找到用户管理的群组
	var myGroups []models.GroupMember
	if err := global.DB.WithContext(ctx).
		Where("user_id = ? AND role IN (1, 2)", userID).
		Find(&myGroups).Error; err != nil {
		return nil, err
	}

	if len(myGroups) == 0 {
		return []GroupRequestDTO{}, nil
	}

	// 2. 获取这些群的待处理申请
	groupIDs := make([]uint, 0, len(myGroups))
	for _, g := range myGroups {
		groupIDs = append(groupIDs, g.GroupID)
	}

	var requests []models.GroupRequest
	if err := global.DB.WithContext(ctx).
		Preload("Sender").
		Where("group_id IN ? AND status = 0", groupIDs).
		Order("created_at desc").
		Find(&requests).Error; err != nil {
		return nil, err
	}

	// 3. 转换为 DTO
	dtos := make([]GroupRequestDTO, 0, len(requests))
	for _, req := range requests {
		// 获取群名称
		if req.Sender.ID == 0 {
			continue
		}
		var group models.Group
		global.DB.WithContext(ctx).Select("name").First(&group, req.GroupID)

		dtos = append(dtos, GroupRequestDTO{
			ID:         req.ID,
			GroupID:    uint(req.GroupID),
			GroupName:  group.Name,
			SenderID:   uint(req.SenderID),
			SenderName: req.Sender.Username,
			Avatar:     req.Sender.Avatar,
			Remark:     req.Remark,
			Status:     req.Status,
			CreatedAt:  req.CreatedAt.Format("2006-01-02 15:04:05"),
		})
	}
	return dtos, nil
}
