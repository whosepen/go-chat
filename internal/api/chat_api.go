package api

import (
	"go-chat/internal/pkg/utils"
	"go-chat/internal/service"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

type ChatApi struct{}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // 允许跨域
	},
}

// Connect WebSocket
func (api *ChatApi) Connect(c *gin.Context) {

	userIDRaw, exists := c.Get("userID")

	if !exists {
		utils.Unauthorized(c, "未登录")
		return
	}

	// 安全断言模式
	userID, ok := userIDRaw.(uint)
	if !ok {
		if f64Id, ok := userIDRaw.(float64); ok {
			userID = uint(f64Id)
		} else {
			utils.FailWithCode(c, http.StatusInternalServerError, "无效的用户ID类型")
			return
		}
	}

	// 升级 HTTP -> WebSocket
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		utils.FailWithCode(c, http.StatusInternalServerError, err.Error())
		return
	}

	// 创建 Client 对象
	client := &service.Client{
		UserID: userID,
		Socket: conn,
		Send:   make(chan []byte),
	}

	// 注册到 Manager
	service.Manager.Register <- client

	// 开启读写协程
	// 单开 goroutine:
	go client.Write()
	client.Read()
}

// GetHistory 获取聊天历史记录
// @Summary 获取聊天历史
// @Tags 聊天模块
// @Param target_id query int true "对方ID"
// @Router /chat/history [get]
func (api *ChatApi) GetHistory(c *gin.Context) {
	v, exists := c.Get("userID")
	if !exists {
		utils.Unauthorized(c, "用户未登录")
		return
	}
	userID := v.(uint)
	targetIDStr := c.Query("target_id")

	messages, err := service.GetHistoryMsg(c.Request.Context(), userID, targetIDStr)
	if err != nil {
		utils.Fail(c, "历史记录拉取失败")
		return
	}

	utils.SuccessWithMsg(c, "历史记录拉取成功", messages)
}
