package v1

import (
	"net/http"
	"time"

	"nairo/api_service/ent/merchantnotifyjob"
	"nairo/api_service/ent/order"
	"nairo/api_service/ent/paymentorphan"
	"nairo/api_service/internal/mysql"
	"nairo/api_service/internal/service"

	"github.com/gin-gonic/gin"
)

type OpsHandler struct {
	db *mysql.Client
}

func NewOpsHandler(db *mysql.Client) *OpsHandler {
	return &OpsHandler{db: db}
}

// Summary Admin 运维概览（匹配/孤儿/回调失败）。
func (h *OpsHandler) Summary(c *gin.Context) {
	ctx := c.Request.Context()
	since := time.Now().Add(-24 * time.Hour)
	paid24, _ := h.db.Ent().Order.Query().
		Where(order.StatusEQ("paid"), order.PaidAtGTE(since)).
		Count(ctx)
	orphanOpen, _ := h.db.Ent().PaymentOrphan.Query().
		Where(paymentorphan.StatusEQ("open")).
		Count(ctx)
	notifyPending, _ := h.db.Ent().MerchantNotifyJob.Query().
		Where(merchantnotifyjob.StatusEQ("pending")).
		Count(ctx)
	notifyFailed, _ := h.db.Ent().MerchantNotifyJob.Query().
		Where(merchantnotifyjob.StatusEQ("failed")).
		Count(ctx)
	c.JSON(http.StatusOK, gin.H{
		"paid_orders_24h":     paid24,
		"open_orphans":        orphanOpen,
		"notify_pending":      notifyPending,
		"notify_failed":       notifyFailed,
		"heartbeat_miss_limit": service.AppHeartbeatMissLimit,
		"online_window_sec":   int(service.AppOnlineWindow.Seconds()),
		"generated_at":        time.Now().Format(time.RFC3339),
	})
}
