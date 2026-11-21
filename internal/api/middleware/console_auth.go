package middleware

import (
	"net/http"
	"strings"

	"nairo/api_service/internal/console"

	"github.com/gin-gonic/gin"
)

const CtxConsoleUserID = "console_user_id"

func ConsoleAuth(tm *console.TokenManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.GetHeader("Authorization")
		if !strings.HasPrefix(h, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		claims, err := tm.ParseAccess(strings.TrimPrefix(h, "Bearer "))
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		c.Set(CtxConsoleUserID, claims.UserID)
		c.Next()
	}
}
