package service

import (
	"context"
	"errors"
	"go-chat/global"
	"go-chat/internal/models"
	"go-chat/internal/pkg/utils"
	"go-chat/internal/repository"
	"time"

	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidPassword  = errors.New("username or password error")
	ErrGenerateToken    = errors.New("generate token failed")
	ErrOccupiedUsername = errors.New("username is already exists")
	ErrUserNotFound     = errors.New("用户不存在")
)

type UserService struct {
	repo *repository.UserRepository
}

func NewUserService() *UserService {
	return &UserService{
		repo: repository.NewUserRepository(),
	}
}

func (s *UserService) Register(ctx context.Context, username, password, email string) error {
	exists, err := s.repo.Exists(ctx, username)
	if err != nil {
		return err
	}
	if exists {
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
	if err := s.repo.Create(ctx, &newUser); err != nil {
		return err
	}
	return nil
}

func (s *UserService) Login(ctx context.Context, username, password string) (*LoginResponseDTO, error) {
	user, err := s.repo.FindByUsername(ctx, username)
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
		defer func() {
			if r := recover(); r != nil {
				global.Log.Error("Async DB panic", zap.Any("err", r))
			}
			cancel()
		}()
		err := s.repo.UpdateLastLogin(timeOut, user.ID)
		if err != nil {
			global.Log.Error("update last_login failed", //修改失败只写日志，不影响主要服务
				zap.Error(err),
				zap.String("username", user.Username))
		}
	}()
	return &dto, nil
}

func GetFullUserInfo(ctx context.Context, uid uint) (UserFullInfoDTO, error) {
	repo := repository.NewUserRepository()
	user, err := repo.FindByID(ctx, uid)
	if err != nil {
		return UserFullInfoDTO{}, err
	}
	return UserFullInfoDTO{
		ID:       user.ID,
		Username: user.Username,
		Nickname: user.Nickname,
		Avatar:   user.Avatar,
		Email:    user.Email,
	}, nil
}

// 更新用户信息
func UpdateUserInfo(ctx context.Context, userID uint, req UpdateUserInfoReq) error {
	var user models.User
	if err := global.DB.WithContext(ctx).First(&user, userID).Error; err != nil {
		return ErrUserNotFound
	}

	// 3. 更新群信息
	updates := make(map[string]interface{})
	if req.Nickname != "" {
		updates["nickname"] = req.Nickname
	}
	if req.Avatar != "" {
		updates["avatar"] = req.Avatar
	}
	if req.Email != "" {
		updates["email"] = req.Email
	}
	if req.Phone != "" {
		updates["phone"] = req.Phone
	}

	if len(updates) == 0 {
		return nil
	}

	return global.DB.WithContext(ctx).Model(&user).Updates(updates).Error
}

// 验证密码
func PasswordIsRight(ctx context.Context, userID uint, password string) bool {
	var user *models.User
	user, err := repository.NewUserRepository().FindByID(ctx, userID)
	if err != nil {
		return false
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
		return false
	}
	return true
}

// 更改密码
func UpdateUserPassword(ctx context.Context, userID uint, req UpdatePasswordReq) error {
	if ok := PasswordIsRight(ctx, userID, req.OldPassword); !ok {
		return ErrInvalidPassword
	}
	hashPassword, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	return repository.NewUserRepository().UpdatePassword(ctx, userID, string(hashPassword))

}
