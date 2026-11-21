package v1

import (
	"net/http"

	"nairo/api_service/internal/service"

	"github.com/gin-gonic/gin"
)

type AppMatchRulesHandler struct {
	platform *service.PlatformService
}

func NewAppMatchRulesHandler(platform *service.PlatformService) *AppMatchRulesHandler {
	return &AppMatchRulesHandler{platform: platform}
}

// PublicGet App 拉取匹配规则（无需登录）。
func (h *AppMatchRulesHandler) PublicGet(c *gin.Context) {
	rules, err := h.platform.GetAppMatchRules(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "server_error"})
		return
	}
	c.JSON(http.StatusOK, rules)
}
