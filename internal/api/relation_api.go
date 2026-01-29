package api

import (
	"go-chat/internal/pkg/utils"
	"go-chat/internal/service"
	"net/http"

	"github.com/gin-gonic/gin"
)

// SendFriendRequest 发送好友申请接口
func SendFriendRequest(c *gin.Context) {
	var req service.SendFriendRequestReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.FailWithCode(c, http.StatusBadRequest, "参数错误")
		return
	}

	userID := c.GetUint("userID")

	if err := service.SendFriendRequest(c.Request.Context(), userID, req); err != nil {
		utils.ServerError(c, err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{"msg": "好友申请已发送"})
}

// HandleFriendRequest 处理好友申请接口
func HandleFriendRequest(c *gin.Context) {
	var req service.HandleFriendRequestReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}

	userID := c.GetUint("userID")

	if err := service.HandleFriendRequest(c.Request.Context(), userID, req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"msg": "处理成功"})
}

func SearchUser(c *gin.Context) {
	username := c.Query("username")
	if username == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请输入用户名"})
		return
	}

	userDTO, err := service.SearchUserByUsername(c.Request.Context(), username)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 404, "msg": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "data": userDTO})
}

// GetFriendList 获取好友列表
func GetFriendList(c *gin.Context) {
	userID := c.GetUint("userID") // 假设中间件设置了

	// Service 返回的直接就是 []UserDTO
	friendList, err := service.GetFriendList(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "data": friendList})
}

// GetPendingRequests 获取申请列表
func GetPendingRequests(c *gin.Context) {
	userID := c.GetUint("userID")

	// Service 返回的直接就是 []FriendRequestDTO
	requests, err := service.GetPendingRequests(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "data": requests})
}
