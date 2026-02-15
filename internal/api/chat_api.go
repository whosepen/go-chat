package api

import (
	"go-chat/internal/pkg/utils"
	"go-chat/internal/service"
	"net/http"
	"strconv"

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
// @Summary 建立WebSocket连接
// @Description 用户通过JWT Token建立WebSocket实时通信连接
// @Tags 聊天模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Success 101 {string} string "切换协议到WebSocket"
// @Router /ws [get]
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

func (api *ChatApi) GetHistory(c *gin.Context) {
	v, exists := c.Get("userID")
	if !exists {
		utils.Unauthorized(c, "用户未登录")
		return
	}
	userID := v.(uint)

	// 解析参数，从 Query 拿 String 并转 Uint
	targetIDStr := c.Query("target_id")
	chatTypeStr := c.Query("chat_type")
	lastMsgIDStr := c.Query("last_msg_id")

	targetID, err1 := strconv.ParseUint(targetIDStr, 10, 64)
	chatType, err2 := strconv.ParseUint(chatTypeStr, 10, 64) // 前端传 2(私聊)  3(群聊)
	var lastMsgID uint64
	if lastMsgIDStr != "" {
		lastMsgID, _ = strconv.ParseUint(lastMsgIDStr, 10, 64)
	}

	if err1 != nil || err2 != nil {
		utils.Fail(c, "参数错误")
		return
	}

	messages, err := service.GetHistoryMsg(c.Request.Context(), userID, uint(targetID), uint(chatType), uint(lastMsgID))
	if err != nil {
		utils.Fail(c, "历史记录拉取失败")
		return
	}

	utils.SuccessWithMsg(c, "历史记录拉取成功", messages)
}
