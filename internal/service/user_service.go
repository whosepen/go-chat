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
	result := global.DB.WithContext(ctx).Select("id").Where("username = ?", username).First(&user)
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
	if err := global.DB.WithContext(ctx).Create(&newUser).Error; err != nil {
		return err
	}
	return nil
}

func (s *UserService) Login(ctx context.Context, username, password string) (*LoginResponseDTO, error) {
	var user models.User
	err := global.DB.WithContext(ctx).Where("username = ?", username).First(&user).Error
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
	go func() { // 修改最后登录时间不是什么重要的环节，在成功登录后单独开进程处理，设置3秒的timeout防止进程挂起占用资源
		timeOut, cancel := context.WithTimeout(context.Background(), 3*time.Second) //主进程结束返回后会取消ctx,所以需要挂在新的ctxBackground上
		defer cancel()
		err := global.DB.WithContext(timeOut).
			Model(&user).
			Update("last_login", time.Now()).Error
		if err != nil {
			global.Log.Error("update last_login failed", //修改失败只写日志，不影响主要服务
				zap.Error(err),
				zap.String("username", user.Username))
		}
	}()
	return &dto, nil
}
