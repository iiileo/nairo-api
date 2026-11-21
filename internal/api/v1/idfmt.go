package v1

import (
	"strconv"

	"nairo/api_service/internal/api/middleware"

	"github.com/gin-gonic/gin"
)

func formatInt64(id int64) string {
	return strconv.FormatInt(id, 10)
}

func parseID(s string) (int64, error) {
	return strconv.ParseInt(s, 10, 64)
}

func consoleUserID(c *gin.Context) int64 {
	uid, _ := c.Get(middleware.CtxConsoleUserID)
	id, _ := uid.(int64)
	return id
}
