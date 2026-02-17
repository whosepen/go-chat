package repository

import (
	"context"
	"go-chat/global"
	"go-chat/internal/models"
)

type RelationRepository struct{}

func NewRelationRepository() *RelationRepository {
	return &RelationRepository{}
}

func (r *RelationRepository) IsFriend(ctx context.Context, userId uint, targetId uint) bool {
	var relation models.Relation
	if err := global.DB.WithContext(ctx).
		Where("owner_id = ? AND target_id = ? AND type = 1", userId, targetId).
		First(&relation).Error; err != nil {
		return false
	}
	if err := global.DB.WithContext(ctx).
		Where("owner_id = ? AND target_id = ? AND type = 1", targetId, userId).
		First(&relation).Error; err != nil {
		return false
	}
	return true
}
