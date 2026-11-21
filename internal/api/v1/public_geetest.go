package v1

import (
	"net/http"
	"strings"

	"nairo/api_service/internal/runtimeconfig"

	"github.com/gin-gonic/gin"
)

type PublicGeetestHandler struct {
	rt *runtimeconfig.RuntimeConfig
}

func NewPublicGeetestHandler(rt *runtimeconfig.RuntimeConfig) *PublicGeetestHandler {
	return &PublicGeetestHandler{rt: rt}
}

// Get 返回前端初始化用的 captcha_id（不含 Key）以及是否开放注册。
func (h *PublicGeetestHandler) Get(c *gin.Context) {
	g := h.rt.Geetest()
	id := strings.TrimSpace(g.CaptchaID)
	enabled := h.rt.GeetestEnabled()
	c.JSON(http.StatusOK, gin.H{
		"enabled":          enabled,
		"captcha_id":       id,
		"register_enabled": h.rt.RegisterEnabled(),
	})
}
