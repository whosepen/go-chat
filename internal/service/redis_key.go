package service

import (
	"fmt"
)

// 在线状态 Key
func onlineStatusKey(userID uint) string {
	return fmt.Sprintf("user:online:%d", userID)
}

// 群id Key
func groupIDKey(groupCode string) string {
	return fmt.Sprintf("group:id:code:%s", groupCode)
}

// 生成 Redis Key：保证顺序一致 (small_id:big_id)
// 群消息key id1 为群ID
func generateKey(id1 uint, id2 uint, isGroup bool) string {
	var key string
	if isGroup {
		return fmt.Sprintf("chat:msg:group:%d", id1)
	}
	if id1 < id2 {
		key = fmt.Sprintf("msg:history:%d:%d", id1, id2)
	} else {
		key = fmt.Sprintf("msg:history:%d:%d", id2, id1)
	}
	return key
}
