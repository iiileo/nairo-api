package v1

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"nairo/api_service/internal/api/middleware"
	"nairo/api_service/internal/pkg/channels"
	"nairo/api_service/internal/service"

	"github.com/gin-gonic/gin"
)

type ConsoleMerchantHandler struct {
	accts    *service.AccountService
	merchant *service.ConsoleMerchantService
	auth     *service.ConsoleAuthService
	bot      *service.BotNotifyService
	orders   *service.OrderService
}

func NewConsoleMerchantHandler(
	accts *service.AccountService,
	merchant *service.ConsoleMerchantService,
	auth *service.ConsoleAuthService,
	bot *service.BotNotifyService,
	orders *service.OrderService,
) *ConsoleMerchantHandler {
	return &ConsoleMerchantHandler{accts: accts, merchant: merchant, auth: auth, bot: bot, orders: orders}
}

func (h *ConsoleMerchantHandler) Dashboard(c *gin.Context) {
	uid, _ := c.Get(middleware.CtxConsoleUserID)
	userID, _ := uid.(int64)
	p, err := h.auth.Profile(c.Request.Context(), userID)
	if err != nil {
		mapConsoleErr(c, err)
		return
	}
	online, err := h.accts.CountOnlineAccounts(c.Request.Context(), userID, service.AppOnlineWindow)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "server_error"})
		return
	}
	orders, err := h.listPaidTodayHint(c, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "server_error"})
		return
	}
	u := p.User
	c.JSON(http.StatusOK, gin.H{
		"user": gin.H{
			"id": formatInt64(u.ID), "email": u.Email, "phone": u.Phone,
			"fee_rate_bps": u.FeeRateBps, "adjust_enabled": u.AdjustEnabled,
			"adjust_direction": u.AdjustDirection, "adjust_step_cents": u.AdjustStepCents,
			"adjust_max_offset_cents": u.AdjustMaxOffsetCents,
		},
		"wallet":          gin.H{"balance_cents": p.BalanceCents},
		"online_accounts": online,
		"today_paid":      orders,
	})
}

func (h *ConsoleMerchantHandler) listPaidTodayHint(c *gin.Context, userID int64) (gin.H, error) {
	n, sum, err := h.merchant.CountPaidToday(c.Request.Context(), userID)
	if err != nil {
		return nil, err
	}
	return gin.H{"count": n, "amount_cents": sum}, nil
}

type adjustReq struct {
	Enabled        *bool  `json:"adjust_enabled"`
	Direction      string `json:"adjust_direction"`
	StepCents      *int   `json:"adjust_step_cents"`
	MaxOffsetCents *int   `json:"adjust_max_offset_cents"`
}

func (h *ConsoleMerchantHandler) UpdateAdjust(c *gin.Context) {
	uid, _ := c.Get(middleware.CtxConsoleUserID)
	userID, _ := uid.(int64)
	var req adjustReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_body"})
		return
	}
	u, err := h.merchant.UpdateAdjust(c.Request.Context(), userID, service.AdjustSettingsInput{
		Enabled: req.Enabled, Direction: req.Direction,
		StepCents: req.StepCents, MaxOffsetCents: req.MaxOffsetCents,
	})
	if err != nil {
		mapConsoleErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"adjust_enabled": u.AdjustEnabled, "adjust_direction": u.AdjustDirection,
		"adjust_step_cents": u.AdjustStepCents, "adjust_max_offset_cents": u.AdjustMaxOffsetCents,
	})
}

func (h *ConsoleMerchantHandler) ListAccounts(c *gin.Context) {
	uid, _ := c.Get(middleware.CtxConsoleUserID)
	userID, _ := uid.(int64)
	list, err := h.accts.ListAccounts(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "server_error"})
		return
	}
	items := make([]gin.H, 0, len(list))
	now := time.Now()
	for _, a := range list {
		online := false
		if a.AppLastSeenAt != nil && now.Sub(*a.AppLastSeenAt) <= service.AppOnlineWindow {
			online = true
		}
		chs := channels.Parse(a.Channel)
		items = append(items, gin.H{
			"id": formatInt64(a.ID), "channel": a.Channel, "channels": chs,
			"channel_label": channels.Label(a.Channel), "name": a.Name,
			"callback_id": a.CallbackID, "callback_url": h.accts.CallbackURL(a.CallbackID),
			"heartbeat_url": h.accts.HeartbeatURL(),
			"heartbeat_ws_url": h.accts.HeartbeatWSURL(),
			"status": a.Status, "app_last_seen_at": a.AppLastSeenAt,
			"app_conn_type": a.AppConnType, "online": online,
			"phone_bound": a.BoundDeviceID != "", "phone_name": a.BoundDeviceName, "phone_bound_at": a.BoundAt,
		})
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

type createAccountReqConsole struct {
	Channels []string `json:"channels"`
	Channel  string   `json:"channel"` // 兼容旧客户端
	Name     string   `json:"name"`
}

func (h *ConsoleMerchantHandler) CreateAccount(c *gin.Context) {
	uid, _ := c.Get(middleware.CtxConsoleUserID)
	userID, _ := uid.(int64)
	var req createAccountReqConsole
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_body"})
		return
	}
	res, err := h.accts.CreateAccount(c.Request.Context(), service.CreateAccountInput{
		UserID: userID, Channels: req.Channels, Channel: req.Channel, Name: req.Name,
	})
	if err != nil {
		mapConsoleErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"account": gin.H{
			"id": formatInt64(res.Account.ID), "channel": res.Account.Channel,
			"channels": res.Channels, "channel_label": channels.Label(res.Account.Channel),
			"name": res.Account.Name, "callback_id": res.Account.CallbackID,
			"callback_url": res.CallbackURL, "heartbeat_url": res.Bind.HeartbeatURL,
			"status": res.Account.Status,
		},
		"app_secret": res.AppSecret,
		"bind":       res.Bind,
	})
}

// GetBindConfig 不重置密钥，返回当前扫码配置（含密钥）。
func (h *ConsoleMerchantHandler) GetBindConfig(c *gin.Context) {
	uid, _ := c.Get(middleware.CtxConsoleUserID)
	userID, _ := uid.(int64)
	aid, err := parseID(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_id"})
		return
	}
	bind, err := h.accts.GetBindConfig(c.Request.Context(), userID, aid)
	if err != nil {
		mapConsoleErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"bind": bind})
}

func (h *ConsoleMerchantHandler) ResetAppSecret(c *gin.Context) {
	uid, _ := c.Get(middleware.CtxConsoleUserID)
	userID, _ := uid.(int64)
	aid, err := parseID(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_id"})
		return
	}
	secret, acc, bind, err := h.accts.ResetAppSecret(c.Request.Context(), userID, aid)
	if err != nil {
		mapConsoleErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"account_id":   formatInt64(acc.ID),
		"app_secret":   secret,
		"callback_url": h.accts.CallbackURL(acc.CallbackID),
		"bind":         bind,
	})
}

type updateChannelsReq struct {
	Channels []string `json:"channels" binding:"required"`
}

func (h *ConsoleMerchantHandler) UpdateChannels(c *gin.Context) {
	uid, _ := c.Get(middleware.CtxConsoleUserID)
	userID, _ := uid.(int64)
	aid, err := parseID(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_id"})
		return
	}
	var req updateChannelsReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_body"})
		return
	}
	acc, err := h.accts.UpdateChannels(c.Request.Context(), userID, aid, req.Channels)
	if err != nil {
		mapConsoleErr(c, err)
		return
	}
	chs := channels.Parse(acc.Channel)
	c.JSON(http.StatusOK, gin.H{
		"id": formatInt64(acc.ID), "channel": acc.Channel, "channels": chs,
		"channel_label": channels.Label(acc.Channel),
	})
}

func (h *ConsoleMerchantHandler) DeleteAccount(c *gin.Context) {
	uid, _ := c.Get(middleware.CtxConsoleUserID)
	userID, _ := uid.(int64)
	aid, err := parseID(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_id"})
		return
	}
	if err := h.accts.DeleteAccount(c.Request.Context(), userID, aid); err != nil {
		mapConsoleErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "id": formatInt64(aid)})
}

func (h *ConsoleMerchantHandler) ReleasePhone(c *gin.Context) {
	uid, _ := c.Get(middleware.CtxConsoleUserID)
	userID, _ := uid.(int64)
	aid, err := parseID(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_id"})
		return
	}
	if err := h.accts.ReleaseBoundDevice(c.Request.Context(), userID, aid); err != nil {
		mapConsoleErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "id": formatInt64(aid)})
}

func (h *ConsoleMerchantHandler) ListPayCodes(c *gin.Context) {
	uid, _ := c.Get(middleware.CtxConsoleUserID)
	userID, _ := uid.(int64)
	aid, err := parseID(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_id"})
		return
	}
	list, err := h.accts.ListPayCodesForUser(c.Request.Context(), userID, aid)
	if err != nil {
		mapConsoleErr(c, err)
		return
	}
	items := make([]gin.H, 0, len(list))
	for _, p := range list {
		items = append(items, gin.H{
			"id": formatInt64(p.ID), "account_id": formatInt64(p.AccountID),
			"channel": p.Channel, "code_type": p.CodeType, "amount_cents": p.AmountCents,
			"image_url": p.ImageURL, "payload": p.Payload, "status": p.Status,
		})
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

type createPayCodeReqConsole struct {
	AccountID   string `json:"account_id" binding:"required"`
	Channel     string `json:"channel" binding:"required"`
	CodeType    string `json:"code_type"`
	AmountCents *int   `json:"amount_cents"`
	ImageURL    string `json:"image_url"`
	Payload     string `json:"payload"`
}

func (h *ConsoleMerchantHandler) CreatePayCode(c *gin.Context) {
	uid, _ := c.Get(middleware.CtxConsoleUserID)
	userID, _ := uid.(int64)
	var req createPayCodeReqConsole
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_body"})
		return
	}
	aid, err := parseID(req.AccountID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_account_id"})
		return
	}
	code, err := h.accts.CreatePayCodeForUser(c.Request.Context(), userID, service.CreatePayCodeInput{
		AccountID: aid, Channel: req.Channel, CodeType: req.CodeType, AmountCents: req.AmountCents,
		ImageURL: req.ImageURL, Payload: req.Payload,
	})
	if err != nil {
		mapConsoleErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"id": formatInt64(code.ID), "account_id": formatInt64(code.AccountID),
		"channel": code.Channel, "code_type": code.CodeType, "amount_cents": code.AmountCents,
		"image_url": code.ImageURL, "payload": code.Payload, "status": code.Status,
	})
}

func (h *ConsoleMerchantHandler) DeletePayCode(c *gin.Context) {
	uid, _ := c.Get(middleware.CtxConsoleUserID)
	userID, _ := uid.(int64)
	id, err := parseID(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_id"})
		return
	}
	if err := h.accts.DeletePayCodeForUser(c.Request.Context(), userID, id); err != nil {
		mapConsoleErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "id": formatInt64(id)})
}

func (h *ConsoleMerchantHandler) ListLedgers(c *gin.Context) {
	uid, _ := c.Get(middleware.CtxConsoleUserID)
	userID, _ := uid.(int64)
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	list, err := h.merchant.ListLedgers(c.Request.Context(), userID, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "server_error"})
		return
	}
	items := make([]gin.H, 0, len(list))
	for _, l := range list {
		items = append(items, gin.H{
			"id": formatInt64(l.ID), "type": l.Type,
			"amount_cents": l.AmountCents, "balance_after_cents": l.BalanceAfterCents,
			"order_id": formatOptID(l.OrderID), "remark": l.Remark, "created_at": l.CreatedAt,
		})
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (h *ConsoleMerchantHandler) GetCredential(c *gin.Context) {
	uid, _ := c.Get(middleware.CtxConsoleUserID)
	userID, _ := uid.(int64)
	cred, err := h.merchant.EnsureCredential(c.Request.Context(), userID)
	if err != nil {
		mapConsoleErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"merchant_id": formatInt64(userID),
		"access_key":  cred.AccessKey,
		"has_secret":  true,
		"hint":        "商户 ID 与 Access Key 用于对接配置；API Secret 仅在重置时显示一次",
	})
}

func (h *ConsoleMerchantHandler) ResetCredential(c *gin.Context) {
	uid, _ := c.Get(middleware.CtxConsoleUserID)
	userID, _ := uid.(int64)
	key, secret, err := h.merchant.ResetAPISecret(c.Request.Context(), userID)
	if err != nil {
		mapConsoleErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"merchant_id": formatInt64(userID),
		"access_key":  key,
		"api_secret":  secret,
	})
}

func notifyChannelJSON(ch *service.NotifyChannelView) gin.H {
	out := gin.H{
		"id":                   ch.ID,
		"provider":             ch.Provider,
		"provider_label":       ch.ProviderLabel,
		"name":                 ch.Name,
		"webhook_url":          ch.WebhookURL,
		"has_secret":           ch.HasSecret,
		"enabled":              ch.Enabled,
		"event_order_paid":     ch.EventOrderPaid,
		"event_fee_low":        ch.EventFeeLow,
		"event_app_push":       ch.EventAppPush,
		"event_device_offline": ch.EventDeviceOffline,
		"created_at":           ch.CreatedAt,
	}
	if ch.AccountID != "" {
		out["account_id"] = ch.AccountID
	} else {
		out["account_id"] = nil
	}
	return out
}

func (h *ConsoleMerchantHandler) ListNotifyChannels(c *gin.Context) {
	uid, _ := c.Get(middleware.CtxConsoleUserID)
	userID, _ := uid.(int64)
	list, err := h.bot.ListChannels(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "server_error"})
		return
	}
	items := make([]gin.H, 0, len(list))
	for _, ch := range list {
		items = append(items, notifyChannelJSON(service.ViewNotifyChannel(ch)))
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

type notifyChannelReq struct {
	Provider           string  `json:"provider"`
	Name               string  `json:"name"`
	WebhookURL         string  `json:"webhook_url"`
	Secret             *string `json:"secret"`
	Enabled            *bool   `json:"enabled"`
	EventOrderPaid     *bool   `json:"event_order_paid"`
	EventFeeLow        *bool   `json:"event_fee_low"`
	EventAppPush       *bool   `json:"event_app_push"`
	EventDeviceOffline *bool   `json:"event_device_offline"`
	AccountID          *string `json:"account_id"`
}

func parseOptionalAccountID(raw *string) (*int64, error) {
	if raw == nil {
		return nil, nil
	}
	s := strings.TrimSpace(*raw)
	if s == "" || s == "0" || s == "all" {
		z := int64(0)
		return &z, nil
	}
	id, err := parseID(s)
	if err != nil {
		return nil, err
	}
	return &id, nil
}

func (h *ConsoleMerchantHandler) CreateNotifyChannel(c *gin.Context) {
	uid, _ := c.Get(middleware.CtxConsoleUserID)
	userID, _ := uid.(int64)
	var req notifyChannelReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_body"})
		return
	}
	accountID, err := parseOptionalAccountID(req.AccountID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_account_id"})
		return
	}
	ch, err := h.bot.CreateChannel(c.Request.Context(), userID, service.NotifyChannelInput{
		Provider: req.Provider, Name: req.Name, WebhookURL: req.WebhookURL,
		Secret: req.Secret, Enabled: req.Enabled,
		EventOrderPaid: req.EventOrderPaid, EventFeeLow: req.EventFeeLow, EventAppPush: req.EventAppPush,
		EventDeviceOffline: req.EventDeviceOffline, AccountID: accountID,
	})
	if err != nil {
		mapConsoleErr(c, err)
		return
	}
	c.JSON(http.StatusOK, notifyChannelJSON(service.ViewNotifyChannel(ch)))
}

func (h *ConsoleMerchantHandler) UpdateNotifyChannel(c *gin.Context) {
	uid, _ := c.Get(middleware.CtxConsoleUserID)
	userID, _ := uid.(int64)
	id, err := parseID(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_id"})
		return
	}
	var req notifyChannelReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_body"})
		return
	}
	accountID, err := parseOptionalAccountID(req.AccountID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_account_id"})
		return
	}
	ch, err := h.bot.UpdateChannel(c.Request.Context(), userID, id, service.NotifyChannelInput{
		Provider: req.Provider, Name: req.Name, WebhookURL: req.WebhookURL,
		Secret: req.Secret, Enabled: req.Enabled,
		EventOrderPaid: req.EventOrderPaid, EventFeeLow: req.EventFeeLow, EventAppPush: req.EventAppPush,
		EventDeviceOffline: req.EventDeviceOffline, AccountID: accountID,
	})
	if err != nil {
		mapConsoleErr(c, err)
		return
	}
	c.JSON(http.StatusOK, notifyChannelJSON(service.ViewNotifyChannel(ch)))
}

func (h *ConsoleMerchantHandler) DeleteNotifyChannel(c *gin.Context) {
	uid, _ := c.Get(middleware.CtxConsoleUserID)
	userID, _ := uid.(int64)
	id, err := parseID(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_id"})
		return
	}
	if err := h.bot.DeleteChannel(c.Request.Context(), userID, id); err != nil {
		mapConsoleErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "id": formatInt64(id)})
}

func (h *ConsoleMerchantHandler) TestNotifyChannel(c *gin.Context) {
	uid, _ := c.Get(middleware.CtxConsoleUserID)
	userID, _ := uid.(int64)
	id, err := parseID(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_id"})
		return
	}
	if err := h.bot.TestChannel(c.Request.Context(), userID, id); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "notify_failed", "detail": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

type consoleTestOrderReq struct {
	AmountCents int    `json:"amount_cents" binding:"required"`
	Channel     string `json:"channel" binding:"required"`
}

// CreateTestOrder 商户自测：用自己的收款码创建 collect 订单，跳转收银台验证到账链路。
func (h *ConsoleMerchantHandler) CreateTestOrder(c *gin.Context) {
	uid, _ := c.Get(middleware.CtxConsoleUserID)
	userID, _ := uid.(int64)
	var req consoleTestOrderReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_body"})
		return
	}
	if req.AmountCents < service.MinRechargeCents || req.AmountCents > service.MaxRechargeCents {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":  "invalid_amount",
			"detail": fmt.Sprintf("金额需在 %d-%d 分之间", service.MinRechargeCents, service.MaxRechargeCents),
		})
		return
	}
	outTradeNo := fmt.Sprintf("TEST%d", time.Now().UnixNano())
	res, err := h.orders.Create(c.Request.Context(), service.CreateOrderInput{
		UserID:      userID,
		OutTradeNo:  outTradeNo,
		AmountCents: req.AmountCents,
		Channel:     req.Channel,
		NotifyURL:   "",
	})
	if err != nil {
		mapOrderErr(c, err)
		return
	}
	view := buildPayOrderView(res.Order, res.PayCode, payOrderOpts{
		PayURL: res.PayURL,
	})
	if res.ExpireIn > 0 {
		view.ExpireIn = res.ExpireIn
	}
	c.JSON(http.StatusOK, view)
}

func formatOptID(v *int64) any {
	if v == nil {
		return nil
	}
	return formatInt64(*v)
}

func mapConsoleErr(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "not_found"})
	case errors.Is(err, service.ErrInvalid):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, service.ErrConflict):
		code, msg := "conflict", "资源已存在"
		switch {
		case strings.Contains(err.Error(), "pay_code_float_exists"):
			code, msg = "pay_code_float_exists", "该设备此渠道已有任意金额码，请先删除后再添加"
		case strings.Contains(err.Error(), "pay_code_fixed_exists"):
			code, msg = "pay_code_fixed_exists", "该设备此渠道已有相同金额的固额码，请先删除后再添加"
		}
		c.JSON(http.StatusConflict, gin.H{"error": code, "message": msg})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "server_error", "detail": err.Error()})
	}
}
