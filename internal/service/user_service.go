package service

import (
	"context"
	"go-chat/global"
	"go-chat/internal/models"
	"go-chat/internal/pkg/utils"
	"time"

	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

type UserService struct{}

func (s *UserService) Register(ctx context.Context, username, password, email string) error {
	var user models.User
	result := global.DB.Select("id").Where("username = ?", username).First(&user)
	if result.RowsAffected > 0 {
		return ErrOccupiedUsername
	}

	hashPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	newUser := models.User{
		Username: username,
		Password: string(hashPassword),
		Email:    email,
		Nickname: username,
		Status:   1,
	}
	if err := global.DB.Create(&newUser).Error; err != nil {
		return err
	}
	return nil
}

func (s *UserService) Login(ctx context.Context, username, password string) (*LoginResponseDTO, error) {
	var user models.User
	err := global.DB.Where("username = ?", username).First(&user).Error
	if err != nil {
		return nil, ErrInvalidPassword
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
		return nil, ErrInvalidPassword
	}
	token, err := utils.GenerateToken(user.ID, user.Username)
	if err != nil {
		return nil, ErrGenerateToken
	}
	dto := LoginResponseDTO{
		Token:    token,
		Username: user.Username,
		Nickname: user.Nickname,
	}
	go func() {
		err := global.DB.WithContext(context.Background()).
			Model(&user).
			Update("last_login", time.Now()).Error

		if err != nil {
			global.Log.Error("update last_login failed",
				zap.Error(err),
				zap.String("username", user.Username))
		}
	}()
	return &dto, nil
}
