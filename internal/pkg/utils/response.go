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

// Success 成功响应 200;0
func Success(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Response{
		Code: 0,
		Msg:  "success",
		Data: data,
	})
}

// SuccessWithMsg 成功响应（带自定义消息）_;0
func SuccessWithMsg(c *gin.Context, msg string, data interface{}) {
	c.JSON(http.StatusOK, Response{
		Code: 0,
		Msg:  msg,
		Data: data,
	})
}

// Fail 预期内失败响应 200;-1
func Fail(c *gin.Context, msg string) {
	c.JSON(http.StatusOK, Response{
		Code: -1,
		Msg:  msg,
		Data: nil,
	})
}

// FailWithCode 失败响应（带状态码） _;_
func FailWithCode(c *gin.Context, httpCode int, msg string) {
	c.JSON(httpCode, Response{
		Code: httpCode,
		Msg:  msg,
		Data: nil,
	})
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
