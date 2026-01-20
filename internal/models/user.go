package models

import "time"

type User struct {
	Model
	Username  string     `gorm:"size:64;uniqueIndex" json:"username"` // 用户名，唯一
	Password  string     `gorm:"size:255" json:"-"`                   // 密码，json化时不返回
	Nickname  string     `gorm:"size:64" json:"nickname"`             // 昵称
	Avatar    string     `gorm:"size:255" json:"avatar"`              // 头像URL
	Email     string     `gorm:"size:128" json:"email"`               // 邮箱
	Phone     string     `gorm:"size:20;index" json:"phone"`          // 手机号
	Status    int        `gorm:"default:1" json:"status"`             // 1:正常 2:禁用 (后台管理用)
	LastLogin *time.Time `json:"last_login"`                          // 最后登录时间
}

// TableName 指定表名
func (User) TableName() string {
	return "users"
}
