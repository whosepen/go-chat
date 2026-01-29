package models

// Relation 好友关系
// 我们采用简单设计：两条记录代表双向好友
// UserID=1, TargetID=2 (1的好友是2)
// UserID=2, TargetID=1 (2的好友是1)
type Relation struct {
	Model
	OwnerID  uint   `gorm:"index;not null" json:"owner_id"`  // 谁的关系
	TargetID uint   `gorm:"index;not null" json:"target_id"` // 对应的好友ID
	Type     int    `json:"type"`                            // 1=好友, 2=拉黑
	Desc     string `json:"desc"`                            // 备注名
}

func (Relation) TableName() string {
	return "relations"
}
