package service

import (
	"context"
	"errors"
	"go-chat/global"
	"go-chat/internal/models"

	"gorm.io/gorm"
)

var (
	ErrRequestExist     = errors.New("已发送过申请，请等待对方处理")
	ErrAlreadyFriend    = errors.New("你们已经是好友了")
	ErrAddYourself      = errors.New("不能添加自己为好友")
	ErrRequestNotFound  = errors.New("申请记录不存在")
	ErrRequestHandled   = errors.New("该申请已被处理")
	ErrPermissionDenied = errors.New("无权处理此申请")
)

// --------------------------
// 1. 发送好友申请
// --------------------------
func SendFriendRequest(ctx context.Context, userID uint, req SendFriendRequestReq) error {
	// 0. 基本校验
	if userID == req.TargetID {
		return ErrAddYourself
	}

	// 1. 检查目标用户是否存在
	var target models.User
	if err := global.DB.WithContext(ctx).First(&target, req.TargetID).Error; err != nil {
		return errors.New("目标用户不存在")
	}

	// 2. 检查是否已经是好友 (查询 relation 表)
	var rel models.Relation
	err := global.DB.WithContext(ctx).
		Where("owner_id = ? AND target_id = ? AND type = 1", userID, req.TargetID).
		First(&rel).Error
	if err == nil {
		return ErrAlreadyFriend // 查到了记录，说明已经是好友
	}

	// 3. 检查是否重复发送申请 (查询 friend_requests 表，状态为 0-待处理)
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
// 2. 处理好友申请 (同意/拒绝)
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
		// 4.1 更新申请状态 (1:同意, 2:拒绝)
		friendReq.Status = req.Action
		if err := tx.Save(&friendReq).Error; err != nil {
			return err
		}

		// 如果是拒绝，到这里就结束了
		if req.Action == 2 {
			return nil
		}

		// 4.2 如果是同意 (Action == 1)，需要在 relations 表创建双向好友关系
		// 关系 A -> B
		r1 := models.Relation{
			OwnerID:  friendReq.SenderID,
			TargetID: friendReq.ReceiverID,
			Type:     1,
		}
		if err := tx.Create(&r1).Error; err != nil {
			return err
		}

		// 关系 B -> A
		r2 := models.Relation{
			OwnerID:  friendReq.ReceiverID,
			TargetID: friendReq.SenderID,
			Type:     1,
		}
		if err := tx.Create(&r2).Error; err != nil {
			return err
		}

		return nil
	})
}

// --------------------------
// 3. 获取待处理的申请列表
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
		// 如果没查到 Sender (比如用户注销了)，req.Sender 会是零值，不会 panic
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

func toUserDTO(u models.User) UserResponseDTO {
	return UserResponseDTO{
		ID:       u.ID,
		Username: u.Username,
		Nickname: u.Nickname,
		Avatar:   u.Avatar,
	}
}

func SearchUserByUsername(ctx context.Context, username string) (*UserResponseDTO, error) {
	var user models.User
	err := global.DB.WithContext(ctx).
		Where("username = ?", username).
		First(&user).Error

	if err != nil {
		// 这里可以判断如果是 gorm.ErrRecordNotFound 则返回自定义的“用户不存在”错误
		return nil, errors.New("用户不存在")
	}

	dto := toUserDTO(user)

	return &dto, nil
}

// GetFriendList 获取我的好友列表
func GetFriendList(ctx context.Context, userID uint) ([]UserResponseDTO, error) {
	var friends []models.User

	// 使用 JOIN 查询：
	// 从 relations 表出发，找到 owner_id 是我，且 type=1 (是好友) 的记录
	// 然后关联 users 表，取出 users 表的所有字段
	// 这样只需要一次数据库交互
	err := global.DB.WithContext(ctx).
		Table("users").
		Select("users.*").
		Joins("JOIN relations ON relations.target_id = users.id").
		Where("relations.owner_id = ? AND relations.type = 1", userID).
		Scan(&friends).Error

	if err != nil {
		return nil, err
	}

	dtos := make([]UserResponseDTO, 0, len(friends))
	for _, f := range friends {
		dtos = append(dtos, toUserDTO(f))
	}

	return dtos, nil
}
