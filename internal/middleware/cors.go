package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Cors 跨域处理中间件
func Cors() gin.HandlerFunc {
	return func(c *gin.Context) {
		method := c.Request.Method
		origin := c.Request.Header.Get("Origin")

		// 如果有 Origin 请求头，说明是跨域请求
		if origin != "" {
			// 允许的源
			// 生产环境建议将 "*" 替换为具体的域名，如 "http://localhost:5500"
			c.Header("Access-Control-Allow-Origin", "*")

			// 允许的方法
			c.Header("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE, UPDATE")

			// 允许的 Header
			// 重点：这里必须包含你自定义的 Authorization，否则带 Token 的请求会失败
			c.Header("Access-Control-Allow-Headers", "Authorization, Content-Length, X-CSRF-Token, Token, session, X_Requested_With, Accept, Origin, Host, Connection, Accept-Encoding, Accept-Language, DNT, X-CustomHeader, Keep-Alive, User-Agent, If-Modified-Since, Cache-Control, Content-Type, Pragma")

			// 暴露给前端的 Header (例如前端需要读取后端返回的某些 Header)
			c.Header("Access-Control-Expose-Headers", "Content-Length, Access-Control-Allow-Origin, Access-Control-Allow-Headers, Cache-Control, Content-Language, Content-Type")

			// 允许携带凭证 (Cookie)
			// 如果 Allow-Origin 是 "*", 这里通常不能设为 true，除非 Origin 是具体域名
			// 这里我们为了方便先设为 true，但在 Chrome 新版中如果 Origin=* 且 Credential=true 会报错
			// 稳妥做法：如果不强制用 Cookie，这里可以不设置或者设为 false
			// c.Header("Access-Control-Allow-Credentials", "true")
		}

		// 处理 OPTIONS 预检请求
		// 浏览器会在发送 POST/GET 前先发一个 OPTIONS 探测，后端必须返回 200 或 204
		if method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent) // 204 No Content
			return
		}

		// 处理请求
		c.Next()
	}
}
