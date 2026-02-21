package repository

import (
	"context"
	"go-chat/global"
	"go-chat/internal/models"
	"time"
)

type UserRepository struct{}

func NewUserRepository() *UserRepository {
	return &UserRepository{}
}

func (r *UserRepository) FindByUsername(ctx context.Context, username string) (*models.User, error) {
	var user models.User
	err := global.DB.WithContext(ctx).Where("username = ?", username).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *UserRepository) FindByID(ctx context.Context, id uint) (*models.User, error) {
	var user models.User
	err := global.DB.WithContext(ctx).First(&user, id).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *UserRepository) Create(ctx context.Context, user *models.User) error {
	return global.DB.WithContext(ctx).Create(user).Error
}

func (r *UserRepository) UpdateLastLogin(ctx context.Context, userID uint) error {
	return global.DB.WithContext(ctx).Model(&models.User{}).Where("id = ?", userID).Update("last_login", time.Now()).Error
}

func (r *UserRepository) Exists(ctx context.Context, username string) (bool, error) {
	var count int64
	err := global.DB.WithContext(ctx).Model(&models.User{}).Where("username = ?", username).Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *UserRepository) UpdatePassword(ctx context.Context, userID uint, password string) error {
	return global.DB.WithContext(ctx).Model(&models.User{}).Where("id = ?", userID).Update("password", password).Error
}
