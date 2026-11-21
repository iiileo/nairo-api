package middleware

import (
	"net/http"
	"strings"

	"nairo/api_service/internal/admin"

	"github.com/gin-gonic/gin"
)

const CtxAdminID = "admin_id"
const CtxAdminUsername = "admin_username"

func AdminAuth(tm *admin.TokenManager) gin.HandlerFunc {
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
		c.Set(CtxAdminID, claims.AdminID)
		c.Set(CtxAdminUsername, claims.Username)
		c.Next()
	}
}
