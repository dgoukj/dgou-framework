package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// CORS 跨域中间件，支持配置允许的来源
// allowedOrigins: 允许的来源列表，若包含 "*" 或为空，则允许所有来源（并设置为当前请求的 Origin）
func CORS(allowedOrigins []string) gin.HandlerFunc {
	// 判断是否允许所有来源
	allowAll := false
	originMap := make(map[string]struct{})
	for _, o := range allowedOrigins {
		if o == "*" {
			allowAll = true
			break
		}
		originMap[o] = struct{}{}
	}

	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")
		if origin != "" {
			// 检查来源是否允许
			if allowAll {
				c.Header("Access-Control-Allow-Origin", origin)
			} else {
				if _, ok := originMap[origin]; ok {
					c.Header("Access-Control-Allow-Origin", origin)
				}
			}
			c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS, PATCH")
			c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Accept, Authorization, X-Request-ID")
			c.Header("Access-Control-Expose-Headers", "Content-Length, X-Request-ID")
			c.Header("Access-Control-Allow-Credentials", "true")
		}

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}
