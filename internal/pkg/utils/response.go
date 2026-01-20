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

// Success 成功响应
func Success(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Response{
		Code: 0,
		Msg:  "success",
		Data: data,
	})
}

// SuccessWithMsg 成功响应（带自定义消息）
func SuccessWithMsg(c *gin.Context, msg string, data interface{}) {
	c.JSON(http.StatusOK, Response{
		Code: 0,
		Msg:  msg,
		Data: data,
	})
}

// Fail 失败响应
func Fail(c *gin.Context, msg string) {
	c.JSON(http.StatusOK, Response{
		Code: -1,
		Msg:  msg,
		Data: nil,
	})
}

// FailWithCode 失败响应（带状态码）
func FailWithCode(c *gin.Context, httpCode int, msg string) {
	c.JSON(httpCode, Response{
		Code: httpCode,
		Msg:  msg,
		Data: nil,
	})
}

// Unauthorized 未授权响应
func Unauthorized(c *gin.Context, msg string) {
	c.JSON(http.StatusUnauthorized, Response{
		Code: 401,
		Msg:  msg,
		Data: nil,
	})
}

// ServerError 服务器错误响应
func ServerError(c *gin.Context, msg string) {
	c.JSON(http.StatusInternalServerError, Response{
		Code: 500,
		Msg:  msg,
		Data: nil,
	})
}
