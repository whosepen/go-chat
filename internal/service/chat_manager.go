package service

import (
	"context"
	"encoding/json"
	"go-chat/global"
	"go-chat/internal/models"
	"go-chat/internal/pkg/protocol"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

// ChatManager 管理所有 WebSocket 连接
// 这是一个单例模式，全局只有一个 manager
type ChatManager struct {
	// Clients 记录所有在线用户: map[UserID] -> *Client
	// 使用 sync.RWMutex 保护并发读写安全
	Clients map[uint]*Client
	Lock    sync.RWMutex

	// Register 注册连接通道
	Register chan *Client

	// Unregister 注销连接通道
	Unregister chan *Client
}

// Client 代表一个 WebSocket 连接
type Client struct {
	UserID uint
	Socket *websocket.Conn
	Send   chan []byte // 待发送的数据管道
}

// 全局 Manager 实例
var Manager = ChatManager{
	Clients:    make(map[uint]*Client),
	Register:   make(chan *Client),
	Unregister: make(chan *Client),
}

// Start 启动管理器 (在 main.go 中调用)
func (manager *ChatManager) Start() {
	for {
		select {
		case conn := <-manager.Register:
			// 建立连接
			manager.Lock.Lock()
			manager.Clients[conn.UserID] = conn
			manager.Lock.Unlock()
			// global.Log.Info("User connected: " + strconv.Itoa(int(conn.UserID)))

		case conn := <-manager.Unregister:
			// 断开连接
			manager.Lock.Lock()
			if _, ok := manager.Clients[conn.UserID]; ok {
				close(conn.Send) // 关闭发送通道
				delete(manager.Clients, conn.UserID)
			}
			manager.Lock.Unlock()
		}
	}
}

// Send 向客户端发送数据
func (c *Client) Write() {
	defer func() {
		c.Socket.Close()
	}()

	for {
		select {
		case message, ok := <-c.Send:
			if !ok {
				c.Socket.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			c.Socket.WriteMessage(websocket.TextMessage, message)
		}
	}
}

// Read 从客户端读取数据
func (c *Client) Read() {
	defer func() {
		Manager.Unregister <- c
		c.Socket.Close()
	}()

	for {
		// 读取消息
		_, messageBytes, err := c.Socket.ReadMessage()
		if err != nil {
			Manager.Unregister <- c
			c.Socket.Close()
			break
		}

		var msg protocol.Message
		if err := json.Unmarshal(messageBytes, &msg); err != nil {
			global.Log.Error("json unmarshal error", zap.Error(err))
			continue
		}

		// 处理消息
		c.HandleMessage(msg)
	}
}

func (c *Client) HandleMessage(msg protocol.Message) {
	switch msg.Type {
	case protocol.TypeSingleMsg:
		c.sendSingleMessage(msg)

	case protocol.TypeHeartbeat:

	}
}

func (c *Client) sendSingleMessage(msg protocol.Message) {
	dbMsg := models.Message{
		FromUserID: c.UserID,
		ToUserID:   msg.TargetID,
		Content:    msg.Content,
		Type:       msg.Type,
		Media:      1,
	}

	if err := global.DB.Create(&dbMsg).Error; err != nil {
		global.Log.Error("save message failed", zap.Error(err))
		return
	}

	key := generateKey(dbMsg.ToUserID, dbMsg.FromUserID)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel() // 设置缓存删除timeout

	if err := global.RDB.Del(ctx, key).Err(); err != nil {
		// 缓存删除失败只记录日志，不影响消息发送流程
		global.Log.Error("redis del failed", zap.Error(err))
	}

	Manager.Lock.RLock()
	targetClient, ok := Manager.Clients[msg.TargetID]
	Manager.Lock.RUnlock()

	if ok {
		reply := protocol.Reply{
			FromID:   c.UserID,
			Content:  msg.Content,
			Type:     protocol.TypeSingleMsg,
			SendTime: time.Now().Unix(),
		}

		replyBytes, _ := json.Marshal(reply)
		targetClient.Send <- replyBytes
	}

}
