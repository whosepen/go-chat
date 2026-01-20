package service

import (
	"fmt"
	"strconv"
)

// 生成 Redis Key：保证顺序一致 (small_id:big_id)
func generateKey(id1 uint, id2 uint) string {
	var key string
	if id1 < id2 {
		key = fmt.Sprintf("msg:history:%d:%d", id1, id2)
	} else {
		key = fmt.Sprintf("msg:history:%d:%d", id2, id1)
	}
	return key
}

func generateKeyForStr(id1Str string, id2 uint) (string, error) {
	// 解析为 uint64
	id1Uint64, err := strconv.ParseUint(id1Str, 10, 64)
	if err != nil {
		return "", ErrInvalidID
	}
	// 强转为 uint 传入核心函数
	return generateKey(uint(id1Uint64), id2), nil
}
