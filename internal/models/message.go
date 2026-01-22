package models

// Message 存储在数据库中的消息记录
type Message struct {
	Model
	FromUserID uint   `gorm:"index" json:"from_user_id"` // 发送者
	ToUserID   uint   `gorm:"index" json:"to_user_id"`   // 接收者
	Content    string `gorm:"type:text" json:"content"`  // 内容 (文本或文件URL)
	Type       int    `json:"type"`                      // TypeHeartbeat = 0 ,TypeLogin = 1 ,TypeSingleMsg = 2 ,TypeGroupMsg  = 3
	Media      int    `json:"media"`                     // 媒体类型: 1文本 2图片 3音频
}

func (*Message) TableName() string {
	return "messages"
}
