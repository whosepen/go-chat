package middleware

import (
	"go-chat/internal/pkg/utils"
	"strings"

	"github.com/gin-gonic/gin"
)

// JWTAuth 鉴权中间件
func JWTAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := ""
		//try to get token from Header
		authHeader := c.Request.Header.Get("Authorization")
		if len(authHeader) > 7 && strings.HasPrefix(authHeader, "Bearer ") {
			token = authHeader[7:]
		}

		//if get nothing in Header,try Query (mainly be used of WebSocket)
		if token == "" {
			token = c.Query("token")
		}

		//still nothing be found,try Cookie
		if token == "" {
			token, _ = c.Cookie("token")
		}

		if token == "" {
			utils.Unauthorized(c, "未登录，请提供 Token")
			c.Abort()
			return
		}

		claims, err := utils.ParseToken(token)
		if err != nil {
			utils.Unauthorized(c, "Token 无效或已过期")
			c.Abort()
			return
		}

		// 3. 将用户信息存入上下文，供后续 API 使用
		c.Set("userID", claims.UserID)
		c.Set("username", claims.Username)

		c.Next() // 放行
	}
}
