package api

import (
	"errors"
	"go-chat/internal/pkg/utils"
	"go-chat/internal/service"

	"github.com/gin-gonic/gin"
)

type UserApi struct{}

type RegisterRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required,min=6"`
	Email    string `json:"email" binding:"email"`
}

type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// UpdateUserRequest 更新用户信息请求
type UpdateUserRequest struct {
	Nickname string `json:"nickname" binding:"max=64"`
	Avatar   string `json:"avatar" binding:"max=255"`
	Email    string `json:"email" binding:"email,max=128"`
}

// Register godoc
// @Summary 用户注册
// @Description 用户通过账号密码注册
// @Tags 用户模块
// @Accept json
// @Produce json
// @Param request body RegisterRequest true "注册参数"
// @Success 200 {object} utils.Response{}
// @Router /user/register [post]
func (u *UserApi) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Fail(c, err.Error())
		return
	}
	userService := service.UserService{}
	if err := userService.Register(c.Request.Context(), req.Username, req.Password, req.Email); err != nil {
		utils.Fail(c, err.Error())
		return
	}
	utils.SuccessWithMsg(c, "注册成功", nil)
}

// Login godoc
// @Summary 用户登录
// @Description 用户通过账号密码登录
// @Tags 用户模块
// @Accept json
// @Produce json
// @Param request body LoginRequest true "登录参数"
// @Success 200 {object} utils.Response{data=service.LoginResponseDTO}
// @Router /user/login [post]
func (u *UserApi) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Fail(c, err.Error())
		return
	}
	userService := service.UserService{}
	resp, err := userService.Login(c.Request.Context(), req.Username, req.Password)
	if err != nil {
		if errors.Is(err, service.ErrInvalidPassword) {
			utils.Fail(c, "用户名或密码错误")
		} else {
			utils.Fail(c, err.Error())
		}
		return
	}
	utils.SuccessWithMsg(c, "登录成功", resp)
}

func (u *UserApi) GetUserInfo(c *gin.Context) {
	// 从上下文中取出中间件存入的值
	username, exists := c.Get("username")
	if !exists {
		utils.Unauthorized(c, "用户信息不存在")
		return
	}
	userID, exists := c.Get("userID")
	if !exists {
		utils.Unauthorized(c, "用户信息不存在")
		return
	}

	utils.Success(c, gin.H{
		"id":       userID,
		"username": username,
	})
}

func (u *UserApi) GetFullUserInfo(c *gin.Context) {
	userID := c.GetUint("userID")

	var user service.UserFullInfoDTO
	user, err := service.GetFullUserInfo(c.Request.Context(), userID)
	if err != nil {
		utils.Fail(c, "获取完整用户信息失败")
	} else {
		utils.Success(c, user)
	}
}
