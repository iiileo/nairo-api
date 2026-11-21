package v1

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"nairo/api_service/ent"
	"nairo/api_service/internal/api/middleware"
	"nairo/api_service/internal/runtimeconfig"
	"nairo/api_service/internal/service"

	"github.com/gin-gonic/gin"
)

type OpenOrderHandler struct {
	orders *service.OrderService
	rt     *runtimeconfig.RuntimeConfig
}

func NewOpenOrderHandler(orders *service.OrderService, rt *runtimeconfig.RuntimeConfig) *OpenOrderHandler {
	return &OpenOrderHandler{orders: orders, rt: rt}
}

type createOrderReq struct {
	OutTradeNo  string `json:"out_trade_no" binding:"required"`
	AmountCents int    `json:"amount_cents" binding:"required"`
	Channel     string `json:"channel" binding:"required"`
	NotifyURL   string `json:"notify_url"`
	ReturnURL   string `json:"return_url"`
}

func (h *OpenOrderHandler) Create(c *gin.Context) {
	var req createOrderReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_body"})
		return
	}
	returnURL, ok := normalizeReturnURL(req.ReturnURL)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_return_url", "message": "return_url 必须是 http/https 地址，且不超过 512 字符"})
		return
	}
	uid, _ := c.Get(middleware.CtxOpenUserID)
	userID, _ := uid.(int64)
	res, err := h.orders.Create(c.Request.Context(), service.CreateOrderInput{
		UserID: userID, OutTradeNo: req.OutTradeNo, AmountCents: req.AmountCents,
		Channel: req.Channel, NotifyURL: req.NotifyURL, ReturnURL: returnURL,
	})
	if err != nil {
		mapOrderErr(c, err)
		return
	}
	view := buildPayOrderView(res.Order, res.PayCode, payOrderOpts{
		PublicBase: h.publicBase(),
		PayURL:     res.PayURL,
		AccountID:  formatInt64(res.Account.ID),
	})
	if res.ExpireIn > 0 {
		view.ExpireIn = res.ExpireIn
	}
	c.JSON(http.StatusOK, view)
}

// normalizeReturnURL 校验支付成功跳转地址：空值合法；非空必须是带主机名的 http/https URL。
func normalizeReturnURL(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", true
	}
	if len(raw) > 512 {
		return "", false
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return "", false
	}
	return raw, true
}

func (h *OpenOrderHandler) Get(c *gin.Context) {
	uid, _ := c.Get(middleware.CtxOpenUserID)
	userID, _ := uid.(int64)
	o, err := h.orders.GetByOutTradeNo(c.Request.Context(), userID, c.Param("out_trade_no"))
	if err != nil {
		mapOrderErr(c, err)
		return
	}
	var code *ent.PayCode
	if o.PayCodeID != nil {
		code, _ = h.orders.GetPayCode(c.Request.Context(), *o.PayCodeID)
	}
	base := h.publicBase()
	c.JSON(http.StatusOK, buildPayOrderView(o, code, payOrderOpts{
		PublicBase: base,
		PayURL:     base + "/checkout/" + formatInt64(o.ID),
	}))
}

func (h *OpenOrderHandler) publicBase() string {
	return trimRightSlash(h.rt.PayPublicBaseURL())
}

func absolutizeURL(base, u string) string {
	u = strings.TrimSpace(u)
	if u == "" {
		return ""
	}
	if strings.HasPrefix(u, "http://") || strings.HasPrefix(u, "https://") || strings.HasPrefix(u, "data:") {
		return u
	}
	base = trimRightSlash(base)
	if base == "" {
		return u
	}
	if strings.HasPrefix(u, "/") {
		return base + u
	}
	return base + "/" + u
}

func trimRightSlash(s string) string {
	for len(s) > 0 && s[len(s)-1] == '/' {
		s = s[:len(s)-1]
	}
	return s
}

func mapOrderErr(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrBusy):
		c.JSON(http.StatusConflict, gin.H{"error": "channel_busy", "message": "渠道繁忙，请稍后重试"})
	case errors.Is(err, service.ErrNoChannelDevice):
		ch := channelFromWrappedErr(err)
		chLabel := channelCN(ch)
		c.JSON(http.StatusConflict, gin.H{
			"error":   "no_channel_device",
			"channel": ch,
			"message": fmt.Sprintf("未开启%s收款设备：请先在「设备」中添加监控设备并开启%s通道", chLabel, chLabel),
		})
	case errors.Is(err, service.ErrNoPayCode):
		c.JSON(http.StatusConflict, gin.H{
			"error":   "no_pay_code",
			"message": "暂无收款码：请上传该渠道同金额固额码，或任意金额收款码",
		})
	case errors.Is(err, service.ErrConflict):
		c.JSON(http.StatusConflict, gin.H{"error": "duplicate_out_trade_no", "message": "out_trade_no 已存在"})
	case errors.Is(err, service.ErrInsufficient):
		c.JSON(http.StatusPaymentRequired, gin.H{"error": "insufficient_balance", "message": "手续费余额不足"})
	case errors.Is(err, service.ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "not_found", "message": "订单不存在"})
	case errors.Is(err, service.ErrInvalid):
		msg := err.Error()
		if strings.Contains(msg, "platform_pay_user_self") {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "platform_pay_user_self",
				"message": "当前账号是平台收款商户，不能通过本通道给自己充值或试付；请用管理后台入账，或使用其他商户账号",
			})
			return
		}
		if strings.Contains(msg, "platform_pay_user") {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "platform_pay_user_unset",
				"message": "平台尚未配置收款商户，请联系管理员",
			})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid", "message": msg})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "server_error", "detail": err.Error()})
	}
}

func channelFromWrappedErr(err error) string {
	msg := err.Error()
	if i := strings.LastIndex(msg, ":"); i >= 0 && i+1 < len(msg) {
		ch := strings.ToLower(strings.TrimSpace(msg[i+1:]))
		if ch == "wechat" || ch == "alipay" {
			return ch
		}
	}
	return ""
}

func channelCN(ch string) string {
	switch ch {
	case "wechat":
		return "微信"
	case "alipay":
		return "支付宝"
	default:
		return "该"
	}
}
