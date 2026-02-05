package utils

import "github.com/speps/go-hashids/v2"

// 基于DB自增id生成唯一code
func GenGroupCode(id uint) string {
	hd := hashids.NewData()
	hd.Salt = "iWantAJob" // 为防止ID碰撞，尽量不要中途修改salt
	hd.MinLength = 6      // 设定最小长度

	// 为生成的群号设置字母表：
	hd.Alphabet = "0123456789ABCEFGHJKMNPQRTWXY"

	h, _ := hashids.NewWithData(hd)
	e, _ := h.Encode([]int{int(id)})
	return e
}
