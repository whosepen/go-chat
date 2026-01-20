package protocol

// 消息类型
const (
	TypeHeartbeat = 0 // 心跳
	TypeLogin     = 1 // 登录/上线通知
	TypeSingleMsg = 2 // 单聊消息
	TypeGroupMsg  = 3 // 群聊消息
)

// Message 客户端发送给服务器的消息结构
type Message struct {
	Type     int    `json:"type"`      // 消息类型
	TargetID uint   `json:"target_id"` // 接收者ID (如果是群聊则是Group ID)
	Content  string `json:"content"`   // 消息内容
}

// Reply 服务器推送给客户端的消息结构
type Reply struct {
	FromID   uint   `json:"from_id"`   // 发送者ID
	Type     int    `json:"type"`      // 消息类型
	Content  string `json:"content"`   // 内容
	SendTime int64  `json:"send_time"` // 发送时间戳
}
