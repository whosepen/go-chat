package api

import (
	"go-chat/internal/pkg/utils"
	"go-chat/internal/service"
	"net/http"

	"github.com/gin-gonic/gin"
)

// SendFriendRequest 发送好友申请接口
// @Summary 发送好友申请
// @Description 向指定用户发送好友申请，支持附加备注
// @Tags 好友模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param request body service.SendFriendRequestReq true "好友申请参数"
// @Success 200 {object} utils.Response
// @Router /friend/request [post]
func SendFriendRequest(c *gin.Context) {
	var req service.SendFriendRequestReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.FailWithCode(c, http.StatusBadRequest, "参数错误")
		return
	}

	userID := c.GetUint("userID")

	if err := service.SendFriendRequest(c.Request.Context(), userID, req); err != nil {
		utils.Fail(c, err.Error())
		return
	}

	utils.SuccessWithMsg(c, "好友申请已发送", nil)
}

// HandleFriendRequest 处理好友申请接口
// @Summary 处理好友申请
// @Description 同意或拒绝收到的好友申请
// @Tags 好友模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param request body service.HandleFriendRequestReq true "处理参数"
// @Success 200 {object} utils.Response
// @Router /friend/handle [post]
func HandleFriendRequest(c *gin.Context) {
	var req service.HandleFriendRequestReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.FailWithCode(c, http.StatusBadRequest, "参数错误")
		return
	}

	userID := c.GetUint("userID")

	if err := service.HandleFriendRequest(c.Request.Context(), userID, req); err != nil {
		utils.Fail(c, err.Error())
		return
	}

	utils.SuccessWithMsg(c, "处理成功", nil)
}

// SearchUser 搜索用户
// @Summary 根据用户名搜索用户
// @Description 通过用户名精确搜索其他用户
// @Tags 用户模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param username query string true "用户名"
// @Success 200 {object} utils.Response{data=service.UserResponseDTO}
// @Router /user/search [get]
func SearchUser(c *gin.Context) {
	username := c.Query("username")
	if username == "" {
		utils.FailWithCode(c, http.StatusBadRequest, "请输入用户名")
		return
	}

	userDTO, err := service.SearchUserByUsername(c.Request.Context(), username)
	if err != nil {
		utils.Fail(c, err.Error())
		return
	}

	utils.Success(c, userDTO)
}

// GetFriendList 获取好友列表
// @Summary 获取好友列表
// @Description 获取当前用户的所有好友列表
// @Tags 好友模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Success 200 {object} utils.Response{data=[]service.UserResponseDTO}
// @Router /friend/list [get]
func GetFriendList(c *gin.Context) {
	userID := c.GetUint("userID")

	friendList, err := service.GetFriendList(c.Request.Context(), userID)
	if err != nil {
		utils.ServerError(c, "获取好友列表失败")
		return
	}

	utils.Success(c, friendList)
}

// GetPendingRequests 获取申请列表
// @Summary 获取待处理的好友申请
// @Description 获取当前用户收到的待处理好友申请列表
// @Tags 好友模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Success 200 {object} utils.Response{data=[]service.FriendRequestDTO}
// @Router /friend/requests [get]
func GetPendingRequests(c *gin.Context) {
	userID := c.GetUint("userID")

	requests, err := service.GetPendingRequests(c.Request.Context(), userID)
	if err != nil {
		utils.ServerError(c, "获取申请列表失败")
		return
	}

	utils.Success(c, requests)
}
