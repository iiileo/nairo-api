package v1

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"nairo/api_service/ent"
	"nairo/api_service/ent/order"
	"nairo/api_service/ent/paymentorphan"
	"nairo/api_service/ent/walletledger"
	"nairo/api_service/internal/mysql"

	"github.com/gin-gonic/gin"
)

type ExportHandler struct {
	db *mysql.Client
}

func NewExportHandler(db *mysql.Client) *ExportHandler {
	return &ExportHandler{db: db}
}

// OrdersCSVConsole 商户订单对账导出。
func (h *ExportHandler) OrdersCSVConsole(c *gin.Context) {
	uid := consoleUserID(c)
	h.writeOrdersCSV(c, &uid)
}

func (h *ExportHandler) OrdersCSVAdmin(c *gin.Context) {
	var uidPtr *int64
	if u := c.Query("user_id"); u != "" {
		id, err := parseID(u)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_user_id"})
			return
		}
		uidPtr = &id
	}
	h.writeOrdersCSV(c, uidPtr)
}

func (h *ExportHandler) writeOrdersCSV(c *gin.Context, userID *int64) {
	q := h.db.Ent().Order.Query().Order(ent.Desc(order.FieldCreatedAt)).Limit(5000)
	if userID != nil {
		q = q.Where(order.UserIDEQ(*userID))
	}
	if st := c.Query("status"); st != "" {
		q = q.Where(order.StatusEQ(st))
	}
	items, err := q.All(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "server_error"})
		return
	}
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=orders_%s.csv", time.Now().Format("20060102")))
	w := csv.NewWriter(c.Writer)
	_ = w.Write([]string{"order_id", "user_id", "out_trade_no", "purpose", "channel", "amount_cents", "pay_amount_cents", "status", "settle_type", "paid_at", "created_at"})
	for _, o := range items {
		paid := ""
		if o.PaidAt != nil {
			paid = o.PaidAt.Format(time.RFC3339)
		}
		_ = w.Write([]string{
			strconv.FormatInt(o.ID, 10),
			strconv.FormatInt(o.UserID, 10),
			o.OutTradeNo,
			o.Purpose,
			o.Channel,
			strconv.Itoa(o.AmountCents),
			strconv.Itoa(o.PayAmountCents),
			o.Status,
			o.SettleType,
			paid,
			o.CreatedAt.Format(time.RFC3339),
		})
	}
	w.Flush()
}

func (h *ExportHandler) OrphansCSVConsole(c *gin.Context) {
	uid := consoleUserID(c)
	h.writeOrphansCSV(c, &uid)
}

func (h *ExportHandler) OrphansCSVAdmin(c *gin.Context) {
	h.writeOrphansCSV(c, nil)
}

func (h *ExportHandler) writeOrphansCSV(c *gin.Context, userID *int64) {
	q := h.db.Ent().PaymentOrphan.Query().Order(ent.Desc(paymentorphan.FieldReportedAt)).Limit(5000)
	if userID != nil {
		q = q.Where(paymentorphan.UserIDEQ(*userID))
	}
	items, err := q.All(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "server_error"})
		return
	}
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=orphans_%s.csv", time.Now().Format("20060102")))
	w := csv.NewWriter(c.Writer)
	_ = w.Write([]string{"orphan_id", "user_id", "account_id", "channel", "pay_amount_cents", "status", "reported_at"})
	for _, o := range items {
		_ = w.Write([]string{
			strconv.FormatInt(o.ID, 10),
			strconv.FormatInt(o.UserID, 10),
			strconv.FormatInt(o.AccountID, 10),
			o.Channel,
			strconv.Itoa(o.PayAmountCents),
			o.Status,
			o.ReportedAt.Format(time.RFC3339),
		})
	}
	w.Flush()
}

func (h *ExportHandler) LedgersCSVConsole(c *gin.Context) {
	uid := consoleUserID(c)
	items, err := h.db.Ent().WalletLedger.Query().
		Where(walletledger.UserIDEQ(uid)).
		Order(ent.Desc(walletledger.FieldID)).
		Limit(5000).
		All(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "server_error"})
		return
	}
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=wallet_ledgers_%s.csv", time.Now().Format("20060102")))
	w := csv.NewWriter(c.Writer)
	_ = w.Write([]string{"id", "type", "amount_cents", "balance_after_cents", "order_id", "remark", "created_at"})
	for _, l := range items {
		oid := ""
		if l.OrderID != nil {
			oid = strconv.FormatInt(*l.OrderID, 10)
		}
		_ = w.Write([]string{
			strconv.FormatInt(l.ID, 10),
			l.Type,
			strconv.FormatInt(l.AmountCents, 10),
			strconv.FormatInt(l.BalanceAfterCents, 10),
			oid,
			l.Remark,
			l.CreatedAt.Format(time.RFC3339),
		})
	}
	w.Flush()
}
