package models

// FriendRequest 好友申请表
type FriendRequest struct {
	Model
	SenderID   uint `gorm:"index;not null" json:"sender_id"`   // 发起人 ID
	ReceiverID uint `gorm:"index;not null" json:"receiver_id"` // 接收人 ID

	// 关联关系 (数据库里没有这一列，这是给 GORM 用的)
	// gorm:"foreignKey:SenderID" 的意思是：
	// "Sender 这个字段对应的 User模型，是通过本表的 SenderID 字段关联的"
	Sender User `gorm:"foreignKey:SenderID" json:"-"`

	Remark string `gorm:"size:255" json:"remark"`  // 申请附言
	Status int    `gorm:"default:0" json:"status"` // 0:待处理, 1:已同意, 2:已拒绝
}

func (FriendRequest) TableName() string {
	return "friend_requests"
}
