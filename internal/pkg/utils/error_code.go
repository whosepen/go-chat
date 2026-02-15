package utils

type ResultCode int

const (
	SuccessCode ResultCode = 0
	FailCode    ResultCode = -1

	// Business Errors 10000+
	ErrCodeInvalidParam  ResultCode = 10001
	ErrCodeUserExist     ResultCode = 10002
	ErrCodeUserNotExist  ResultCode = 10003
	ErrCodePasswdWrong   ResultCode = 10004
	ErrCodeTokenInvalid  ResultCode = 10005
	ErrCodeGroupNotExist ResultCode = 20001
	ErrCodeNotMember     ResultCode = 20002
)

var codeMsg = map[ResultCode]string{
	SuccessCode:          "success",
	FailCode:             "failed",
	ErrCodeInvalidParam:  "参数错误",
	ErrCodeUserExist:     "用户已存在",
	ErrCodeUserNotExist:  "用户不存在",
	ErrCodePasswdWrong:   "密码错误",
	ErrCodeTokenInvalid:  "Token无效",
	ErrCodeGroupNotExist: "群组不存在",
	ErrCodeNotMember:     "非群成员",
}

func GetMsg(code ResultCode) string {
	msg, ok := codeMsg[code]
	if ok {
		return msg
	}
	return "unknown error"
}
