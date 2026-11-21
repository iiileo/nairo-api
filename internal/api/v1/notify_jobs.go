package v1

import (
	"net/http"
	"time"

	"nairo/api_service/ent"
	"nairo/api_service/internal/service"

	"github.com/gin-gonic/gin"
)

type NotifyJobHandler struct {
	notify *service.NotifyService
}

func NewNotifyJobHandler(notify *service.NotifyService) *NotifyJobHandler {
	return &NotifyJobHandler{notify: notify}
}

func (h *NotifyJobHandler) ListConsole(c *gin.Context) {
	uid := consoleUserID(c)
	items, err := h.notify.ListJobs(c.Request.Context(), service.NotifyJobFilter{
		UserID: &uid,
		Status: c.Query("status"),
		Limit:  100,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "server_error"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": mapNotifyJobs(items)})
}

func (h *NotifyJobHandler) ReplayConsole(c *gin.Context) {
	uid := consoleUserID(c)
	id, err := parseID(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_id"})
		return
	}
	j, err := h.notify.ReplayJob(c.Request.Context(), id, uid)
	if err != nil {
		mapConsoleErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "job": mapNotifyJob(j)})
}

func (h *NotifyJobHandler) ListAdmin(c *gin.Context) {
	var uidPtr *int64
	if u := c.Query("user_id"); u != "" {
		id, err := parseID(u)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_user_id"})
			return
		}
		uidPtr = &id
	}
	items, err := h.notify.ListJobs(c.Request.Context(), service.NotifyJobFilter{
		UserID: uidPtr,
		Status: c.Query("status"),
		Limit:  100,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "server_error"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": mapNotifyJobs(items)})
}

func (h *NotifyJobHandler) ReplayAdmin(c *gin.Context) {
	id, err := parseID(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_id"})
		return
	}
	j, err := h.notify.ReplayJob(c.Request.Context(), id, 0)
	if err != nil {
		mapConsoleErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "job": mapNotifyJob(j)})
}

func mapNotifyJobs(items []*ent.MerchantNotifyJob) []gin.H {
	out := make([]gin.H, 0, len(items))
	for _, j := range items {
		out = append(out, mapNotifyJob(j))
	}
	return out
}

func mapNotifyJob(j *ent.MerchantNotifyJob) gin.H {
	if j == nil {
		return gin.H{}
	}
	var next any
	if j.NextRetryAt != nil {
		next = j.NextRetryAt.Format(time.RFC3339)
	}
	return gin.H{
		"id":            formatInt64(j.ID),
		"user_id":       formatInt64(j.UserID),
		"order_id":      formatInt64(j.OrderID),
		"out_trade_no":  j.OutTradeNo,
		"notify_url":    j.NotifyURL,
		"status":        j.Status,
		"attempts":      j.Attempts,
		"max_attempts":  j.MaxAttempts,
		"next_retry_at": next,
		"last_error":    j.LastError,
		"created_at":    j.CreatedAt.Format(time.RFC3339),
		"updated_at":    j.UpdatedAt.Format(time.RFC3339),
	}
}
