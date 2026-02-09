package service

type UserFullInfoDTO struct {
	ID       uint   `json:"id"`
	Username string `json:"username"` // 用户名，唯一
	Nickname string `json:"nickname"` // 昵称
	Avatar   string `json:"avatar"`   // 头像URL
	Email    string `json:"email"`    // 邮箱
	Phone    string `json:"phone"`    // 手机号
}

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
	ID          uint   `json:"id"`
	Username    string `json:"username"`
	Nickname    string `json:"nickname"`
	Avatar      string `json:"avatar"`
	Online      bool   `json:"online"`
	UnreadCount int    `json:"unread_count"`
	LastMsgTime int64  `json:"last_message_time,omitempty"`
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

// 入参：标记消息已读
type MarkMessagesReadReq struct {
	TargetID uint `json:"target_id" binding:"required"`
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

// 入群申请
type SendGroupRequestReq struct {
	GroupID int    `json:"group_id" binding:"required"`
	Remark  string `json:"remark"`
}

// ============ Group 请求 DTO ============
type CreateGroupReq struct {
	Name string `json:"name" binding:"required,min=1,max=50"`
	Desc string `json:"desc"` // 可选：群描述
	Icon string `json:"icon"` // 可选：群头像
}

type HandleGroupRequestReq struct {
	RequestID uint `json:"request_id" binding:"required"`
	Action    int  `json:"action" binding:"required,oneof=1 2"` // 1同意 2拒绝
}

type QuitGroupReq struct {
	GroupID uint `json:"group_id" binding:"required"`
}

type UpdateGroupInfoReq struct {
	GroupID uint   `json:"group_id" binding:"required"`
	Name    string `json:"name"`
	Desc    string `json:"desc"`
	Icon    string `json:"icon"`
}

type KickMemberReq struct {
	GroupID uint `json:"group_id" binding:"required"`
	UserID  uint `json:"user_id" binding:"required"`
}

// ============ Group 响应 DTO ============
type GroupInfoDTO struct {
	ID        uint   `json:"id"`
	Code      string `json:"code"`
	Name      string `json:"name"`
	Icon      string `json:"icon"`
	Desc      string `json:"desc"`
	OwnerID   uint   `json:"owner_id"`
	MemberCnt int    `json:"member_count"`
}

type GroupMemberDTO struct {
	UserID   uint   `json:"user_id"`
	Nickname string `json:"nickname"`
	Role     int    `json:"role"` // 1群主 2管理员 3普通成员
	Mute     int    `json:"mute"` // 0正常 1禁言
	JoinTime string `json:"join_time"`
}

type GroupRequestDTO struct {
	ID         uint   `json:"id"`
	GroupID    uint   `json:"group_id"`
	GroupName  string `json:"group_name"`
	SenderID   uint   `json:"sender_id"`
	SenderName string `json:"sender_name"`
	Avatar     string `json:"avatar"`
	Remark     string `json:"remark"`
	Status     int    `json:"status"`
	CreatedAt  string `json:"created_at"`
}

type GroupListReqDTO struct {
	ID          uint   `json:"id"`
	Code        string `json:"code"`
	Name        string `json:"name"`
	Icon        string `json:"icon"`
	UnreadCount int    `json:"unread_count"`
	LastMsgTime int64  `json:"last_message_time,omitempty"`
}
