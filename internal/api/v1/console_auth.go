package v1

import (
	"errors"
	"net/http"

	"nairo/api_service/ent"
	"nairo/api_service/internal/api/middleware"
	"nairo/api_service/internal/geetest"
	"nairo/api_service/internal/service"

	"github.com/gin-gonic/gin"
)

type ConsoleAuthHandler struct {
	auth *service.ConsoleAuthService
}

func NewConsoleAuthHandler(auth *service.ConsoleAuthService) *ConsoleAuthHandler {
	return &ConsoleAuthHandler{auth: auth}
}

type consoleLoginReq struct {
	Login    string `json:"login" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type consoleRegisterReq struct {
	Email           string `json:"email" binding:"required"`
	Code            string `json:"code" binding:"required"`
	Password        string `json:"password" binding:"required"`
	ConfirmPassword string `json:"confirm_password" binding:"required"`
}

type consoleSendCodeReq struct {
	Email         string `json:"email" binding:"required"`
	LotNumber     string `json:"lot_number"`
	CaptchaOutput string `json:"captcha_output"`
	PassToken     string `json:"pass_token"`
	GenTime       string `json:"gen_time"`
}

type consoleRefreshReq struct {
	Renew string `json:"renew" binding:"required"`
}

type consoleLogoutReq struct {
	Renew string `json:"renew"`
}

func (h *ConsoleAuthHandler) Login(c *gin.Context) {
	var req consoleLoginReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_body"})
		return
	}
	sess, err := h.auth.Login(c.Request.Context(), req.Login, req.Password, c.Request.UserAgent())
	if err != nil {
		writeConsoleAuthErr(c, err)
		return
	}
	c.JSON(http.StatusOK, sessionJSON(sess))
}

func (h *ConsoleAuthHandler) SendRegisterCode(c *gin.Context) {
	var req consoleSendCodeReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_body"})
		return
	}
	if err := h.auth.SendRegisterCode(c.Request.Context(), req.Email, geetest.Proof{
		LotNumber:     req.LotNumber,
		CaptchaOutput: req.CaptchaOutput,
		PassToken:     req.PassToken,
		GenTime:       req.GenTime,
	}); err != nil {
		writeConsoleAuthErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *ConsoleAuthHandler) Register(c *gin.Context) {
	var req consoleRegisterReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_body"})
		return
	}
	sess, err := h.auth.Register(c.Request.Context(), service.RegisterInput{
		Email:           req.Email,
		Code:            req.Code,
		Password:        req.Password,
		ConfirmPassword: req.ConfirmPassword,
		UserAgent:       c.Request.UserAgent(),
	})
	if err != nil {
		writeConsoleAuthErr(c, err)
		return
	}
	c.JSON(http.StatusOK, sessionJSON(sess))
}

func (h *ConsoleAuthHandler) Refresh(c *gin.Context) {
	var req consoleRefreshReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_body"})
		return
	}
	sess, err := h.auth.Refresh(c.Request.Context(), req.Renew, c.Request.UserAgent())
	if err != nil {
		writeConsoleAuthErr(c, err)
		return
	}
	c.JSON(http.StatusOK, sessionJSON(sess))
}

func (h *ConsoleAuthHandler) Logout(c *gin.Context) {
	var req consoleLogoutReq
	_ = c.ShouldBindJSON(&req)
	if err := h.auth.Logout(c.Request.Context(), req.Renew); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "server_error"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *ConsoleAuthHandler) Heartbeat(c *gin.Context) {
	uid, _ := c.Get(middleware.CtxConsoleUserID)
	userID, _ := uid.(int64)
	if err := h.auth.TouchHeartbeat(c.Request.Context(), userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "server_error"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *ConsoleAuthHandler) Me(c *gin.Context) {
	uid, _ := c.Get(middleware.CtxConsoleUserID)
	userID, _ := uid.(int64)
	p, err := h.auth.Profile(c.Request.Context(), userID)
	if err != nil {
		if errors.Is(err, service.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "not_found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "server_error"})
		return
	}
	u := p.User
	c.JSON(http.StatusOK, gin.H{
		"user":   userJSON(u),
		"wallet": gin.H{"balance_cents": p.BalanceCents},
	})
}

func sessionJSON(sess *service.ConsoleSession) gin.H {
	return gin.H{
		"token":      sess.Pair.AccessToken,
		"renew":      sess.Pair.RefreshToken,
		"expires_in": sess.Pair.AccessExpiresIn,
		"renew_in":   sess.Pair.RefreshExpiresIn,
		"user":       userJSON(sess.User),
	}
}

func userJSON(u *ent.User) gin.H {
	return gin.H{
		"id":                      formatInt64(u.ID),
		"email":                   u.Email,
		"phone":                   u.Phone,
		"fee_rate_bps":            u.FeeRateBps,
		"adjust_direction":        u.AdjustDirection,
		"adjust_enabled":          u.AdjustEnabled,
		"adjust_step_cents":       u.AdjustStepCents,
		"adjust_max_offset_cents": u.AdjustMaxOffsetCents,
		"console_last_seen_at":    u.ConsoleLastSeenAt,
	}
}

func writeConsoleAuthErr(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrUnauthorized):
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid_credentials"})
	case errors.Is(err, service.ErrConflict):
		c.JSON(http.StatusConflict, gin.H{"error": "email_taken"})
	case errors.Is(err, service.ErrRateLimited):
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "rate_limited"})
	case errors.Is(err, service.ErrCodeInvalid):
		c.JSON(http.StatusBadRequest, gin.H{"error": "code_invalid"})
	case errors.Is(err, service.ErrCaptchaFailed):
		c.JSON(http.StatusBadRequest, gin.H{"error": "captcha_failed"})
	case errors.Is(err, service.ErrRegisterDisabled):
		c.JSON(http.StatusForbidden, gin.H{"error": "register_disabled"})
	case errors.Is(err, service.ErrInvalid):
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request"})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "server_error"})
	}
}
