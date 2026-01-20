package service

import (
	"go-chat/internal/models"
)

// ToMessageDTO将DB实体转换为DTO
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
	// 预分配切片容量，避免扩容带来的性能损耗
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
