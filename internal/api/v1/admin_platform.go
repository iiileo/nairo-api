package v1

import (
	"net/http"

	"nairo/api_service/internal/runtimeconfig"
	"nairo/api_service/internal/service"

	"github.com/gin-gonic/gin"
)

type AdminPlatformHandler struct {
	platform *service.PlatformService
	rt       *runtimeconfig.RuntimeConfig
}

func NewAdminPlatformHandler(platform *service.PlatformService, rt *runtimeconfig.RuntimeConfig) *AdminPlatformHandler {
	return &AdminPlatformHandler{platform: platform, rt: rt}
}

func (h *AdminPlatformHandler) GetSettings(c *gin.Context) {
	view, err := h.platform.GetSettings(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "server_error"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"pay_user_id":    view.PayUserID,
		"pay_user_email": view.PayUserEmail,
		"pay_user_phone": view.PayUserPhone,
		"pay_user_ok":    view.PayUserOK,
		"hint":           "指定一个商户作为平台收款方。其他商户在线充值或测试支付时，款项打到该商户收款码，成功后记入付款方手续费余额。",
	})
}

type setPlatformPayUserReq struct {
	UserID string `json:"user_id"`
}

func (h *AdminPlatformHandler) GetMatchRules(c *gin.Context) {
	rules, err := h.platform.GetAppMatchRules(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "server_error"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"rules": rules,
		"hint":  "App 监听收款通知时按此规则识别渠道与金额；保存后 App 会在心跳周期内拉取生效。",
	})
}

func (h *AdminPlatformHandler) SetMatchRules(c *gin.Context) {
	var req service.AppMatchRules
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_body"})
		return
	}
	rules, err := h.platform.SetAppMatchRules(c.Request.Context(), req)
	if err != nil {
		mapConsoleErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "rules": rules})
}

func (h *AdminPlatformHandler) ResetMatchRules(c *gin.Context) {
	rules, err := h.platform.ResetAppMatchRules(c.Request.Context())
	if err != nil {
		mapConsoleErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "rules": rules})
}

func (h *AdminPlatformHandler) GetRuntimeConfig(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"config": h.rt.View(),
		"hint":   "SMTP / 支付公网地址 / 验证码与默认费率可在此修改；保存后立即热加载，无需重启。",
	})
}

func (h *AdminPlatformHandler) SetRuntimeConfig(c *gin.Context) {
	var patch runtimeconfig.RuntimeConfigPatch
	if err := c.ShouldBindJSON(&patch); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_body"})
		return
	}
	view, err := h.rt.Update(c.Request.Context(), patch)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "server_error"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "config": view})
}

func (h *AdminPlatformHandler) ReloadRuntimeConfig(c *gin.Context) {
	if err := h.rt.Reload(c.Request.Context()); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "server_error"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "config": h.rt.View()})
}

func (h *AdminPlatformHandler) SetPayUser(c *gin.Context) {
	var req setPlatformPayUserReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_body"})
		return
	}
	var uid int64
	if req.UserID != "" {
		id, err := parseID(req.UserID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_user_id"})
			return
		}
		uid = id
	}
	if err := h.platform.SetPlatformPayUserID(c.Request.Context(), uid); err != nil {
		mapConsoleErr(c, err)
		return
	}
	view, err := h.platform.GetSettings(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"ok": true, "pay_user_id": req.UserID})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"ok":             true,
		"pay_user_id":    view.PayUserID,
		"pay_user_email": view.PayUserEmail,
		"pay_user_phone": view.PayUserPhone,
		"pay_user_ok":    view.PayUserOK,
	})
}
