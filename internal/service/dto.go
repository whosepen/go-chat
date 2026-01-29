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

type UserResponseDTO struct {
	ID       uint   `json:"id"`
	Username string `json:"username"`
	Nickname string `json:"nickname"`
	Avatar   string `json:"avatar"`
}

// 入参：发送申请
type SendFriendRequestReq struct {
	TargetID uint   `json:"target_id" binding:"required"`
	Remark   string `json:"remark"`
}

// 入参：处理申请
type HandleFriendRequestReq struct {
	RequestID uint `json:"request_id" binding:"required"`
	Action    int  `json:"action" binding:"required,oneof=1 2"` // 只能传 1 或 2
}

// 出参：申请列表项
type FriendRequestDTO struct {
	ID         uint   `json:"id"`          // 申请记录ID
	SenderID   uint   `json:"sender_id"`   // 发送人ID
	SenderName string `json:"sender_name"` // 发送人用户名
	Avatar     string `json:"avatar"`      // 发送人头像
	Remark     string `json:"remark"`      // 附言
	Status     int    `json:"status"`      // 状态
	CreatedAt  string `json:"created_at"`  // 时间
}
