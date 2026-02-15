package utils

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type Response struct {
	Code int         `json:"code"`
	Msg  string      `json:"msg"`
	Data interface{} `json:"data,omitempty"`
}

// Result 通用响应
func Result(c *gin.Context, code ResultCode, msg string, data interface{}) {
	c.JSON(http.StatusOK, Response{
		Code: int(code),
		Msg:  msg,
		Data: data,
	})
}

// Success 成功响应 200;0
func Success(c *gin.Context, data interface{}) {
	Result(c, SuccessCode, GetMsg(SuccessCode), data)
}

// SuccessWithMsg 成功响应（带自定义消息）_;0
func SuccessWithMsg(c *gin.Context, msg string, data interface{}) {
	Result(c, SuccessCode, msg, data)
}

// Fail 预期内失败响应 200;-1
func Fail(c *gin.Context, msg string) {
	Result(c, FailCode, msg, nil)
}

// FailWithCode 失败响应（带状态码） _;_
// Deprecated: Use Error instead if possible
func FailWithCode(c *gin.Context, httpCode int, msg string) {
	c.JSON(httpCode, Response{
		Code: httpCode,
		Msg:  msg,
		Data: nil,
	})
}

// Error 业务错误响应 200;code
func Error(c *gin.Context, code ResultCode) {
	Result(c, code, GetMsg(code), nil)
}

// ErrorWithMsg 业务错误响应（自定义消息） 200;code
func ErrorWithMsg(c *gin.Context, code ResultCode, msg string) {
	Result(c, code, msg, nil)
}

// Unauthorized 未授权响应 401;401
func Unauthorized(c *gin.Context, msg string) {
	c.JSON(http.StatusUnauthorized, Response{
		Code: 401,
		Msg:  msg,
		Data: nil,
	})
}

// ServerError 服务器错误响应 500;500
func ServerError(c *gin.Context, msg string) {
	c.JSON(http.StatusInternalServerError, Response{
		Code: 500,
		Msg:  msg,
		Data: nil,
	})
}
