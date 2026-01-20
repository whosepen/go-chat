package service

// MessageDTO 消息数据传输对象（用于 API 响应）
type MessageDTO struct {
	ID         uint   `json:"id"`
	FromUserID uint   `json:"from_user_id"`
	ToUserID   uint   `json:"to_user_id"`
	Content    string `json:"content"`
	Type       int    `json:"type"`
	Media      int    `json:"media"`
	CreatedAt  int64  `json:"created_at"`
}

type LoginResponseDTO struct {
	Token    string `json:"token"`
	Username string `json:"username"`
	Nickname string `json:"nickname"`
}
