package models

type Group struct {
	Model
	Name    string `json:"name"`
	OwnerID uint   `json:"owner_id"` // 群主ID
	Icon    string `json:"icon"`     // 群头像
	Type    int    `json:"type"`     // 1=普通群, 2=企业群
	Desc    string `json:"desc"`     // 群描述
}

type GroupMember struct {
	Model
	GroupID  uint   `gorm:"index" json:"group_id"`
	UserID   uint   `gorm:"index" json:"user_id"`
	Nickname string `json:"nickname"` // 在群里的昵称
	Role     int    `json:"role"`     // 1=群主, 2=管理员, 3=普通成员
	Mute     int    `json:"mute"`     // 0=正常, 1=禁言
}
