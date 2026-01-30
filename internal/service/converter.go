package service

import (
	"go-chat/internal/models"
)

// ToMessageDTO 将DB实体转换为DTO
// DATETIME转化为int64时间戳
func ToMessageDTO(m *models.Message) MessageDTO {
	return MessageDTO{
		ID:         m.ID,
		FromUserID: m.FromUserID,
		ToUserID:   m.ToUserID,
		Content:    m.Content,
		Type:       m.Type,
		Media:      m.Media,
		CreatedAt:  m.CreatedAt.UnixMilli(),
	}
}

// ToMessageDTOs 批量转换
func ToMessageDTOs(msgs []models.Message) []MessageDTO {
	n := len(msgs)
	if n == 0 {
		return []MessageDTO{}
	}
	dtos := make([]MessageDTO, n)
	for i, m := range msgs {
		dtos[n-1-i] = ToMessageDTO(&m)
	}
	return dtos
}

// ToUserDTO 将User实体转换为DTO
func ToUserDTO(u models.User) UserResponseDTO {
	return UserResponseDTO{
		ID:       u.ID,
		Username: u.Username,
		Nickname: u.Nickname,
		Avatar:   u.Avatar,
	}
}

// ToUserDTOWithOnline 将User实体转换为DTO（带在线状态）
func ToUserDTOWithOnline(u models.User, online bool) UserResponseDTO {
	return UserResponseDTO{
		ID:       u.ID,
		Username: u.Username,
		Nickname: u.Nickname,
		Avatar:   u.Avatar,
		Online:   online,
	}
}

// ToUserDTOs 批量转换
func ToUserDTOs(users []models.User) []UserResponseDTO {
	n := len(users)
	if n == 0 {
		return []UserResponseDTO{}
	}
	dtos := make([]UserResponseDTO, n)
	for i, u := range users {
		dtos[i] = ToUserDTO(u)
	}
	return dtos
}
