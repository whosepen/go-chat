package models

type GroupRequest struct {
	Model
	GroupID  int    `gorm:"index;not null" json:"group_id"`
	SenderID int    `gorm:"index;not null" json:"sender_id"`
	Sender   User   `gorm:"foreignKey:SenderID" json:"-"`
	Remark   string `gorm:"size:255" json:"remark"`  // 申请附言
	Status   int    `gorm:"default:0" json:"status"` // 0:待处理, 1:已同意, 2:已拒绝
}

func (GroupRequest) TableName() string {
	return "group_request"
}
