package v1

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"nairo/api_service/internal/api/middleware"
	"nairo/api_service/internal/service"

	"github.com/gin-gonic/gin"
)

type ManualHandler struct {
	manual *service.ManualService
}

func NewManualHandler(manual *service.ManualService) *ManualHandler {
	return &ManualHandler{manual: manual}
}

func (h *ManualHandler) ListOrphansAdmin(c *gin.Context) {
	h.listOrphans(c, 0)
}

func (h *ManualHandler) ListOrphansConsole(c *gin.Context) {
	uid, _ := c.Get(middleware.CtxConsoleUserID)
	userID, _ := uid.(int64)
	h.listOrphans(c, userID)
}

func (h *ManualHandler) listOrphans(c *gin.Context, userID int64) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	list, err := h.manual.ListOrphans(c.Request.Context(), userID, service.ListOrphansFilter{
		Status:  c.Query("status"),
		Channel: c.Query("channel"),
		Limit:   limit,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "server_error"})
		return
	}
	items := make([]gin.H, 0, len(list))
	for _, o := range list {
		items = append(items, gin.H{
			"id": formatInt64(o.ID),
			"user_id": formatInt64(o.UserID),
			"account_id": formatInt64(o.AccountID),
			"channel": o.Channel,
			"pay_amount_cents": o.PayAmountCents,
			"reported_at": o.ReportedAt,
			"status": o.Status,
		})
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

type bindOrphanReq struct {
	OrderID string `json:"order_id" binding:"required"`
	Remark  string `json:"remark" binding:"required"`
}

func (h *ManualHandler) BindOrphanAdmin(c *gin.Context) {
	aid, _ := c.Get(middleware.CtxAdminID)
	adminID, _ := aid.(int64)
	h.bindOrphan(c, 0, "admin", adminID)
}

func (h *ManualHandler) BindOrphanConsole(c *gin.Context) {
	uid, _ := c.Get(middleware.CtxConsoleUserID)
	userID, _ := uid.(int64)
	h.bindOrphan(c, userID, "console", userID)
}

func (h *ManualHandler) bindOrphan(c *gin.Context, userID int64, actorType string, actorID int64) {
	oid, err := parseID(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_id"})
		return
	}
	var req bindOrphanReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_body"})
		return
	}
	orderID, err := parseID(req.OrderID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_order_id"})
		return
	}
	o, err := h.manual.BindOrphan(c.Request.Context(), service.BindOrphanInput{
		OrphanID: oid, OrderID: orderID, UserID: userID,
		ActorType: actorType, ActorID: actorID, Remark: req.Remark,
	})
	if err != nil {
		mapManualErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"order_id": formatInt64(o.ID),
		"status": o.Status,
		"settle_type": o.SettleType,
		"pay_amount_cents": o.PayAmountCents,
	})
}

type manualPayReq struct {
	PayAmount string `json:"pay_amount"`
	Channel   string `json:"channel"`
	PaidAt    string `json:"paid_at"`
	Remark    string `json:"remark" binding:"required"`
}

func (h *ManualHandler) ManualPayAdmin(c *gin.Context) {
	aid, _ := c.Get(middleware.CtxAdminID)
	adminID, _ := aid.(int64)
	h.manualPay(c, 0, "admin", adminID)
}

func (h *ManualHandler) ManualPayConsole(c *gin.Context) {
	uid, _ := c.Get(middleware.CtxConsoleUserID)
	userID, _ := uid.(int64)
	h.manualPay(c, userID, "console", userID)
}

func (h *ManualHandler) manualPay(c *gin.Context, userID int64, actorType string, actorID int64) {
	oid, err := parseID(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_id"})
		return
	}
	var req manualPayReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_body"})
		return
	}
	in := service.ManualPayInput{
		OrderID: oid, UserID: userID, ActorType: actorType, ActorID: actorID,
		PayAmount: req.PayAmount, Channel: req.Channel, Remark: req.Remark,
	}
	if req.PaidAt != "" {
		t, err := time.Parse(time.RFC3339, req.PaidAt)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_paid_at"})
			return
		}
		in.PaidAt = &t
	}
	o, err := h.manual.ManualPay(c.Request.Context(), in)
	if err != nil {
		mapManualErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"order_id": formatInt64(o.ID),
		"status": o.Status,
		"settle_type": o.SettleType,
		"pay_amount_cents": o.PayAmountCents,
	})
}

func (h *ManualHandler) ListOrdersConsole(c *gin.Context) {
	uid, _ := c.Get(middleware.CtxConsoleUserID)
	userID, _ := uid.(int64)
	h.listOrders(c, userID)
}

func (h *ManualHandler) ListOrdersAdmin(c *gin.Context) {
	h.listOrders(c, 0)
}

func (h *ManualHandler) listOrders(c *gin.Context, userID int64) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	list, err := h.manual.ListOrders(c.Request.Context(), userID, service.ListOrdersFilter{
		Status:     c.Query("status"),
		Channel:    c.Query("channel"),
		SettleType: c.Query("settle_type"),
		Query:      c.Query("q"),
		Limit:      limit,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "server_error"})
		return
	}
	items := make([]gin.H, 0, len(list))
	for _, o := range list {
		item := gin.H{
			"id": formatInt64(o.ID),
			"out_trade_no": o.OutTradeNo,
			"amount_cents": o.AmountCents,
			"pay_amount_cents": o.PayAmountCents,
			"channel": o.Channel,
			"status": o.Status,
			"settle_type": o.SettleType,
			"paid_at": o.PaidAt,
		}
		if userID == 0 {
			item["user_id"] = formatInt64(o.UserID)
		}
		items = append(items, item)
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

type setStatusReq struct {
	Status    string `json:"status" binding:"required"` // paid|closed|expired
	Remark    string `json:"remark" binding:"required"`
	Channel   string `json:"channel"`
	PayAmount string `json:"pay_amount"`
}

func (h *ManualHandler) SetStatusAdmin(c *gin.Context) {
	aid, _ := c.Get(middleware.CtxAdminID)
	adminID, _ := aid.(int64)
	h.setStatus(c, 0, "admin", adminID)
}

func (h *ManualHandler) SetStatusConsole(c *gin.Context) {
	uid, _ := c.Get(middleware.CtxConsoleUserID)
	userID, _ := uid.(int64)
	h.setStatus(c, userID, "console", userID)
}

func (h *ManualHandler) setStatus(c *gin.Context, userID int64, actorType string, actorID int64) {
	oid, err := parseID(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_id"})
		return
	}
	var req setStatusReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_body"})
		return
	}
	o, err := h.manual.SetOrderStatus(c.Request.Context(), service.SetOrderStatusInput{
		OrderID: oid, UserID: userID, ActorType: actorType, ActorID: actorID,
		Status: req.Status, Remark: req.Remark, Channel: req.Channel, PayAmount: req.PayAmount,
	})
	if err != nil {
		mapManualErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"order_id": formatInt64(o.ID),
		"status":   o.Status,
		"settle_type": o.SettleType,
		"pay_amount_cents": o.PayAmountCents,
	})
}

type ignoreOrphanReq struct {
	Remark string `json:"remark"`
}

func (h *ManualHandler) IgnoreOrphanAdmin(c *gin.Context) {
	aid, _ := c.Get(middleware.CtxAdminID)
	adminID, _ := aid.(int64)
	h.ignoreOrphan(c, 0, "admin", adminID)
}

func (h *ManualHandler) IgnoreOrphanConsole(c *gin.Context) {
	uid, _ := c.Get(middleware.CtxConsoleUserID)
	userID, _ := uid.(int64)
	h.ignoreOrphan(c, userID, "console", userID)
}

func (h *ManualHandler) ignoreOrphan(c *gin.Context, userID int64, actorType string, actorID int64) {
	oid, err := parseID(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_id"})
		return
	}
	var req ignoreOrphanReq
	_ = c.ShouldBindJSON(&req)
	if err := h.manual.IgnoreOrphan(c.Request.Context(), oid, userID, actorID, actorType, req.Remark); err != nil {
		mapManualErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "id": formatInt64(oid)})
}

type batchSetStatusReq struct {
	IDs       []string `json:"ids" binding:"required"`
	Status    string   `json:"status" binding:"required"`
	Remark    string   `json:"remark" binding:"required"`
	Channel   string   `json:"channel"`
	PayAmount string   `json:"pay_amount"`
}

func (h *ManualHandler) BatchSetStatusAdmin(c *gin.Context) {
	aid, _ := c.Get(middleware.CtxAdminID)
	adminID, _ := aid.(int64)
	h.batchSetStatus(c, 0, "admin", adminID)
}

func (h *ManualHandler) BatchSetStatusConsole(c *gin.Context) {
	uid, _ := c.Get(middleware.CtxConsoleUserID)
	userID, _ := uid.(int64)
	h.batchSetStatus(c, userID, "console", userID)
}

func (h *ManualHandler) batchSetStatus(c *gin.Context, userID int64, actorType string, actorID int64) {
	var req batchSetStatusReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_body"})
		return
	}
	ids, err := parseIDList(req.IDs)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_ids"})
		return
	}
	res, err := h.manual.BatchSetOrderStatus(c.Request.Context(), service.BatchSetOrderStatusInput{
		OrderIDs: ids, UserID: userID, ActorType: actorType, ActorID: actorID,
		Status: req.Status, Remark: req.Remark, Channel: req.Channel, PayAmount: req.PayAmount,
	})
	if err != nil {
		mapManualErr(c, err)
		return
	}
	c.JSON(http.StatusOK, batchResultJSON(res))
}

type batchIgnoreOrphansReq struct {
	IDs    []string `json:"ids" binding:"required"`
	Remark string   `json:"remark"`
}

func (h *ManualHandler) BatchIgnoreOrphansAdmin(c *gin.Context) {
	aid, _ := c.Get(middleware.CtxAdminID)
	adminID, _ := aid.(int64)
	h.batchIgnoreOrphans(c, 0, "admin", adminID)
}

func (h *ManualHandler) BatchIgnoreOrphansConsole(c *gin.Context) {
	uid, _ := c.Get(middleware.CtxConsoleUserID)
	userID, _ := uid.(int64)
	h.batchIgnoreOrphans(c, userID, "console", userID)
}

func (h *ManualHandler) batchIgnoreOrphans(c *gin.Context, userID int64, actorType string, actorID int64) {
	var req batchIgnoreOrphansReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_body"})
		return
	}
	ids, err := parseIDList(req.IDs)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_ids"})
		return
	}
	res, err := h.manual.BatchIgnoreOrphans(c.Request.Context(), service.BatchIgnoreOrphansInput{
		OrphanIDs: ids, UserID: userID, ActorType: actorType, ActorID: actorID, Remark: req.Remark,
	})
	if err != nil {
		mapManualErr(c, err)
		return
	}
	c.JSON(http.StatusOK, batchResultJSON(res))
}

func parseIDList(raw []string) ([]int64, error) {
	if len(raw) == 0 {
		return nil, errors.New("empty")
	}
	out := make([]int64, 0, len(raw))
	for _, s := range raw {
		id, err := parseID(strings.TrimSpace(s))
		if err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, nil
}

func batchResultJSON(res *service.BatchResult) gin.H {
	items := make([]gin.H, 0, len(res.Items))
	for _, it := range res.Items {
		row := gin.H{"id": formatInt64(it.ID), "ok": it.OK}
		if it.Error != "" {
			row["error"] = it.Error
		}
		items = append(items, row)
	}
	return gin.H{"ok": res.OK, "fail": res.Fail, "items": items}
}

func mapManualErr(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "not_found"})
	case errors.Is(err, service.ErrAlreadyPaid):
		c.JSON(http.StatusConflict, gin.H{"error": "already_paid"})
	case errors.Is(err, service.ErrConflict):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	case errors.Is(err, service.ErrInvalid):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "server_error", "detail": err.Error()})
	}
}
