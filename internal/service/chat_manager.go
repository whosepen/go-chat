package service

import (
	"context"
	"encoding/json"
	"go-chat/global"
	"go-chat/internal/models"
	"go-chat/internal/pkg/protocol"
	"go-chat/internal/repository"
	"sync"
	"time"

	"github.com/IBM/sarama"
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

// Start 启动管理器
func (manager *ChatManager) Start() {
	for {
		select {
		case conn := <-manager.Register:
			// 建立连接
			manager.Lock.Lock()
			manager.Clients[conn.UserID] = conn
			manager.Lock.Unlock()

			// 异步设置在线状态到 Redis
			// 设置为心跳间隔的 3倍（90秒）
			// 允许客户端连续丢失2个心跳包，服务器依然认为他在线
			// 只有连续丢3个，判定离线
			go func(uid uint) {
				ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
				defer cancel()
				if err := global.RDB.Set(ctx,
					onlineStatusKey(uid), "1",
					90*time.Second).Err(); err != nil {
					global.Log.Warn("failed to set online status", zap.Error(err))
				} // redis更新只记录日志，不影响主程序

			}(conn.UserID)

			global.Log.Info("user online", zap.Uint("user_id", conn.UserID))

		case conn := <-manager.Unregister:
			// 断开连接
			manager.Lock.Lock()
			if _, ok := manager.Clients[conn.UserID]; ok {
				close(conn.Send) // 关闭发送通道
				delete(manager.Clients, conn.UserID)
				global.Log.Info("user offline", zap.Uint("user_id", conn.UserID))
			}
			manager.Lock.Unlock()

			// 清除在线状态
			go func(uid uint) {
				ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
				defer cancel()

				if err := global.RDB.Del(ctx, onlineStatusKey(uid)).Err(); err != nil {
					global.Log.Warn("Failed to set offline status", zap.Error(err))
				}
			}(conn.UserID)
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

	case protocol.TypeGroupMsg:
		c.sendGroupMessage(msg)

	case protocol.TypeHeartbeat:
		// 前端心跳间隔设置为30s，每收到消息就将存活时间重置为90s，容许至多连续丢失2次心跳
		if err := global.RDB.Expire(context.Background(), onlineStatusKey(c.UserID), 90*time.Second).Err(); err != nil {

		}

	case protocol.TypeLogin:
		// 登录/上线通知，目前已在连接时处理

	case protocol.TypeWebRTC:
		c.sendSignalMessage(msg)

	default:
		global.Log.Warn("unknown message type", zap.Int("type", msg.Type))
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

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if ok := repository.NewRelationRepository().IsFriend(ctx, c.UserID, msg.TargetID); !ok {
		global.Log.Info("消息被对方拒收")
		return
	}

	// 1. 直接入库
	if err := global.DB.WithContext(ctx).Create(&dbMsg).Error; err != nil {
		global.Log.Error("save message failed", zap.Error(err))
		return
	}
	//
	//TODO：私聊目前改用直连DB模式，暂时停用Kafka消费者，保留代码作参考
	/*
		msgBytes, err := json.Marshal(dbMsg)
		if err != nil {
			global.Log.Error("marshal message failed", zap.Error(err))
			return
		}

		// 投递到 Kafka
		kafkaMsg := &sarama.ProducerMessage{
			Topic: global.KTopic.ChatMsg,
			Value: sarama.ByteEncoder(msgBytes),
		}
		_, _, err = global.KafkaProducer.SendMessage(kafkaMsg)
		if err != nil {
			global.Log.Error("send message to kafka failed", zap.Error(err))
		}
	*/

	// 2. 更新 Redis 缓存 (RPush + LTrim)
	key := generateKey(dbMsg.FromUserID, dbMsg.ToUserID, false)

	// 构造 DTO
	dto := ToMessageDTO(&dbMsg)
	jsonBytes, _ := json.Marshal(dto)

	// 使用 Pipeline 执行 RPush + LTrim
	pipe := global.RDB.Pipeline()
	pipe.RPush(ctx, key, jsonBytes)
	pipe.LTrim(ctx, key, -2000, -1) // 保留最后 2000 条
	pipe.Expire(ctx, key, 7*24*time.Hour)
	pipe.Exec(ctx)

	// 3. 只推送给接收方，不推送给发送者自己
	PushMessageToUser(dbMsg)
}

func (c *Client) sendGroupMessage(msg protocol.Message) {
	// 验证群权限
	repo := repository.NewGroupRepository()
	ctx := context.Background()

	// 1. 检查是否为群成员 (隐含了群存在的检查)
	isMember, err := repo.IsMember(ctx, msg.TargetID, c.UserID)
	if err != nil {
		global.Log.Error("check group member failed", zap.Error(err))
		return
	}
	if !isMember {
		global.Log.Warn("user not in group or group not found", zap.Uint("uid", c.UserID), zap.Uint("gid", msg.TargetID))
		return
	}

	dbMsg := models.Message{
		FromUserID: c.UserID,
		ToUserID:   msg.TargetID, // ToUserID 存储群ID
		Content:    msg.Content,
		Type:       msg.Type,
		Media:      1,
	}

	// 发送到 Kafka，由 Consumer 处理群发
	value, _ := json.Marshal(dbMsg)
	kafkaMsg := &sarama.ProducerMessage{
		Topic: global.KTopic.GroupMsg,
		Value: sarama.ByteEncoder(value),
	}
	global.KafkaProducer.SendMessage(kafkaMsg)
}

func (c *Client) sendSignalMessage(msg protocol.Message) {
	Manager.Lock.RLock()
	targetClient, ok := Manager.Clients[msg.TargetID]
	Manager.Lock.RUnlock()

	if ok {
		reply := protocol.Reply{
			FromID:   c.UserID,
			Content:  msg.Content, // 这里面是 JSON 格式的 SDP/Candidate
			Type:     protocol.TypeWebRTC,
			SendTime: time.Now().Unix(),
		}

		replyBytes, _ := json.Marshal(reply)
		targetClient.Send <- replyBytes
	}
}

func PushMessageToGroup(msg models.Message) {
	// 查询群成员
	repo := repository.NewGroupRepository()
	members, err := repo.GetMembers(context.Background(), msg.ToUserID)
	if err != nil {
		global.Log.Error("query group members failed", zap.Error(err))
		return
	}

	var sendTime int64
	if !msg.CreatedAt.IsZero() {
		sendTime = msg.CreatedAt.Unix()
	} else {
		sendTime = time.Now().Unix()
	}
	reply := protocol.Reply{
		FromID:   msg.FromUserID,
		Type:     protocol.TypeGroupMsg,
		Content:  msg.Content,
		SendTime: sendTime,
	}
	replyBytes, _ := json.Marshal(reply)

	Manager.Lock.RLock()
	defer Manager.Lock.RUnlock()

	for _, member := range members {
		// 不推送给发送者自己
		if member.UserID == msg.FromUserID {
			continue
		}
		if client, ok := Manager.Clients[member.UserID]; ok {
			client.Send <- replyBytes
		}
	}
}

func PushMessageToUser(msg models.Message) {
	Manager.Lock.RLock()
	targetClient, ok := Manager.Clients[msg.ToUserID]
	Manager.Lock.RUnlock()

	if ok {
		var sendTime int64
		if !msg.CreatedAt.IsZero() {
			sendTime = msg.CreatedAt.Unix()
		} else {
			sendTime = time.Now().Unix()
		}
		reply := protocol.Reply{
			FromID:   msg.FromUserID,
			Content:  msg.Content,
			Type:     protocol.TypeSingleMsg,
			SendTime: sendTime,
		}
		replyBytes, err := json.Marshal(reply)
		if err != nil {
			global.Log.Error("marshal reply failed", zap.Error(err))
			return
		}
		targetClient.Send <- replyBytes
	}
}
