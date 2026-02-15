package repository

import (
	"context"
	"go-chat/global"
	"go-chat/internal/models"
)

type GroupRepository struct{}

func NewGroupRepository() *GroupRepository {
	return &GroupRepository{}
}

func (r *GroupRepository) FindByID(ctx context.Context, id uint) (*models.Group, error) {
	var group models.Group
	err := global.DB.WithContext(ctx).First(&group, id).Error
	if err != nil {
		return nil, err
	}
	return &group, nil
}

func (r *GroupRepository) FindByCode(ctx context.Context, code string) (*models.Group, error) {
	var group models.Group
	err := global.DB.WithContext(ctx).Where("code = ?", code).First(&group).Error
	if err != nil {
		return nil, err
	}
	return &group, nil
}

func (r *GroupRepository) GetMembers(ctx context.Context, groupID uint) ([]models.GroupMember, error) {
	var members []models.GroupMember
	err := global.DB.WithContext(ctx).Where("group_id = ?", groupID).Find(&members).Error
	if err != nil {
		return nil, err
	}
	return members, nil
}

func (r *GroupRepository) IsMember(ctx context.Context, groupID, userID uint) (bool, error) {
	var count int64
	err := global.DB.WithContext(ctx).Model(&models.GroupMember{}).
		Where("group_id = ? AND user_id = ?", groupID, userID).
		Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *GroupRepository) CountMembers(ctx context.Context, groupID uint) (int64, error) {
	var count int64
	err := global.DB.WithContext(ctx).Model(&models.GroupMember{}).
		Where("group_id = ?", groupID).
		Count(&count).Error
	return count, err
}
