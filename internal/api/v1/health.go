package v1

import (
	"net/http"

	"nairo/api_service/internal/mysql"
	"nairo/api_service/internal/version"

	"github.com/gin-gonic/gin"
)

type HealthHandler struct {
	db *mysql.Client
}

func NewHealthHandler(db *mysql.Client) *HealthHandler {
	return &HealthHandler{db: db}
}

func (h *HealthHandler) Health(c *gin.Context) {
	dbOK := false
	if h.db != nil && h.db.DB() != nil {
		if err := h.db.DB().Ping(); err == nil {
			dbOK = true
		}
	}
	status := http.StatusOK
	if !dbOK {
		status = http.StatusServiceUnavailable
	}
	c.JSON(status, gin.H{
		"ok":         dbOK,
		"version":    version.Version,
		"commit":     version.Commit,
		"build_time": version.BuildTime,
	})
}

func (h *HealthHandler) Version(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"version":    version.Version,
		"commit":     version.Commit,
		"build_time": version.BuildTime,
	})
}
