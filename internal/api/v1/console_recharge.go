package v1

import (
	"net/http"

	"nairo/api_service/internal/api/middleware"
	"nairo/api_service/internal/service"

	"github.com/gin-gonic/gin"
)

type ConsoleRechargeHandler struct {
	recharge *service.RechargeService
	platform *service.PlatformService
}

func NewConsoleRechargeHandler(recharge *service.RechargeService, platform *service.PlatformService) *ConsoleRechargeHandler {
	return &ConsoleRechargeHandler{recharge: recharge, platform: platform}
}

type walletPayReq struct {
	AmountCents int    `json:"amount_cents" binding:"required"`
	Channel     string `json:"channel" binding:"required"`
	Purpose     string `json:"purpose"` // 仅 recharge；测试请用公开 /api/v1/public/demo-pay
}

func (h *ConsoleRechargeHandler) Create(c *gin.Context) {
	uid, _ := c.Get(middleware.CtxConsoleUserID)
	userID, _ := uid.(int64)
	var req walletPayReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_body"})
		return
	}
	purpose := req.Purpose
	if purpose == "" {
		purpose = service.PurposeRecharge
	}
	if purpose != service.PurposeRecharge {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_purpose", "detail": "test moved to public homepage"})
		return
	}
	res, err := h.recharge.Create(c.Request.Context(), service.CreateWalletPayInput{
		PayerUserID: userID,
		AmountCents: req.AmountCents,
		Channel:     req.Channel,
		Purpose:     purpose,
	})
	if err != nil {
		mapOrderErr(c, err)
		return
	}
	c.JSON(http.StatusOK, buildPayOrderView(res.Order, res.PayCode, payOrderOpts{
		PayPage: res.PayPagePath,
	}))
}

func (h *ConsoleRechargeHandler) Get(c *gin.Context) {
	uid, _ := c.Get(middleware.CtxConsoleUserID)
	userID, _ := uid.(int64)
	oid, err := parseID(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_id"})
		return
	}
	o, code, err := h.recharge.GetForPayer(c.Request.Context(), userID, oid)
	if err != nil {
		mapConsoleErr(c, err)
		return
	}
	c.JSON(http.StatusOK, buildPayOrderView(o, code, payOrderOpts{
		OnlyPendingCode: false,
	}))
}

func (h *ConsoleRechargeHandler) PlatformStatus(c *gin.Context) {
	uid, _ := c.Get(middleware.CtxConsoleUserID)
	userID, _ := uid.(int64)
	view, err := h.platform.GetSettings(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "server_error"})
		return
	}
	configured := view.PayUserID != "" && view.PayUserOK
	isSelf := configured && view.PayUserID == formatInt64(userID)
	c.JSON(http.StatusOK, gin.H{
		"configured":           configured,
		"is_platform_pay_user": isSelf,
		"pay_user_id":          view.PayUserID,
		"min_cents":            service.MinRechargeCents,
		"max_cents":            service.MaxRechargeCents,
	})
}
