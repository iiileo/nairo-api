package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"nairo/api_service/ent"
	"nairo/api_service/ent/amountlock"
	"nairo/api_service/ent/order"
	"nairo/api_service/ent/paymentorphan"
	"nairo/api_service/internal/mysql"
	"nairo/api_service/internal/snowflake"
)

type ManualService struct {
	db     *mysql.Client
	notify *NotifyService
	bot    *BotNotifyService
}

func NewManualService(db *mysql.Client, notify *NotifyService, bot *BotNotifyService) *ManualService {
	return &ManualService{db: db, notify: notify, bot: bot}
}

type ManualPayInput struct {
	OrderID     int64
	UserID      int64 // 0 = admin 跨用户
	ActorType   string
	ActorID     int64
	PayAmount   string // yuan, optional
	Channel     string
	PaidAt      *time.Time
	Remark      string
}

func (s *ManualService) ManualPay(ctx context.Context, in ManualPayInput) (*ent.Order, error) {
	if strings.TrimSpace(in.Remark) == "" {
		return nil, fmt.Errorf("%w: remark required", ErrInvalid)
	}
	var paid *ent.Order
	err := s.db.WithTx(ctx, func(tx *ent.Tx) error {
		o, err := tx.Order.Get(ctx, in.OrderID)
		if err != nil {
			if ent.IsNotFound(err) {
				return ErrNotFound
			}
			return err
		}
		if in.UserID > 0 && o.UserID != in.UserID {
			return ErrNotFound
		}
		opts := SettleOpts{
			SettleType: "manual",
			Actor:      SettleActor{Type: in.ActorType, ID: in.ActorID},
			Remark:     in.Remark,
		}
		if in.PaidAt != nil {
			opts.PaidAt = *in.PaidAt
		}
		if in.Channel != "" {
			opts.Channel = strings.ToLower(in.Channel)
		}
		if strings.TrimSpace(in.PayAmount) != "" {
			cents, err := parseYuanToCents(in.PayAmount)
			if err != nil {
				return fmt.Errorf("%w: pay_amount", ErrInvalid)
			}
			opts.PayAmount = &cents
		}
		paid, err = markPaidInTx(ctx, tx, o, opts)
		return err
	})
	if err != nil {
		return nil, err
	}
	if ShouldDispatchOrderNotify(paid.Purpose) {
		s.notify.EnqueueOrderPaid(paid)
		s.bot.EnqueueOrderPaid(paid)
	}
	return paid, nil
}

type BindOrphanInput struct {
	OrphanID  int64
	OrderID   int64
	UserID    int64 // 0 = admin
	ActorType string
	ActorID   int64
	Remark    string
}

func (s *ManualService) BindOrphan(ctx context.Context, in BindOrphanInput) (*ent.Order, error) {
	if strings.TrimSpace(in.Remark) == "" {
		return nil, fmt.Errorf("%w: remark required", ErrInvalid)
	}
	var paid *ent.Order
	err := s.db.WithTx(ctx, func(tx *ent.Tx) error {
		orp, err := tx.PaymentOrphan.Get(ctx, in.OrphanID)
		if err != nil {
			if ent.IsNotFound(err) {
				return ErrNotFound
			}
			return err
		}
		if orp.Status != "open" {
			return fmt.Errorf("%w: orphan not open", ErrConflict)
		}
		if in.UserID > 0 && orp.UserID != in.UserID {
			return ErrNotFound
		}
		o, err := tx.Order.Get(ctx, in.OrderID)
		if err != nil {
			if ent.IsNotFound(err) {
				return ErrNotFound
			}
			return err
		}
		if o.UserID != orp.UserID {
			return fmt.Errorf("%w: user mismatch", ErrInvalid)
		}
		if in.UserID > 0 && o.UserID != in.UserID {
			return ErrNotFound
		}
		payAmt := orp.PayAmountCents
		paid, err = markPaidInTx(ctx, tx, o, SettleOpts{
			SettleType: "manual",
			Actor:      SettleActor{Type: in.ActorType, ID: in.ActorID},
			Remark:     in.Remark,
			OrphanID:   &orp.ID,
			PayAmount:  &payAmt,
			Channel:    orp.Channel,
		})
		if err != nil {
			return err
		}
		_, err = tx.PaymentOrphan.UpdateOneID(orp.ID).
			SetStatus("bound").
			SetBoundOrderID(o.ID).
			Save(ctx)
		return err
	})
	if err != nil {
		return nil, err
	}
	if ShouldDispatchOrderNotify(paid.Purpose) {
		s.notify.EnqueueOrderPaid(paid)
		s.bot.EnqueueOrderPaid(paid)
	}
	return paid, nil
}

type ListOrphansFilter struct {
	Status  string // open|bound|ignored；空=全部
	Channel string // wechat|alipay
	Limit   int
}

func (s *ManualService) ListOrphans(ctx context.Context, userID int64, f ListOrphansFilter) ([]*ent.PaymentOrphan, error) {
	if f.Limit <= 0 || f.Limit > 100 {
		f.Limit = 50
	}
	q := s.db.Ent().PaymentOrphan.Query().
		Order(ent.Desc(paymentorphan.FieldReportedAt)).
		Limit(f.Limit)
	if userID > 0 {
		q = q.Where(paymentorphan.UserIDEQ(userID))
	}
	st := strings.ToLower(strings.TrimSpace(f.Status))
	if st == "" {
		st = "open"
	}
	if st != "all" {
		q = q.Where(paymentorphan.StatusEQ(st))
	}
	ch := strings.ToLower(strings.TrimSpace(f.Channel))
	if ch == "wechat" || ch == "alipay" {
		q = q.Where(paymentorphan.ChannelEQ(ch))
	}
	return q.All(ctx)
}

type ListOrdersFilter struct {
	Status     string // pending|paid|expired|closed
	Channel    string // wechat|alipay
	SettleType string // auto|manual
	Query      string // out_trade_no 模糊
	Limit      int
}

func (s *ManualService) ListOrders(ctx context.Context, userID int64, f ListOrdersFilter) ([]*ent.Order, error) {
	if f.Limit <= 0 || f.Limit > 100 {
		f.Limit = 50
	}
	q := s.db.Ent().Order.Query().Order(ent.Desc(order.FieldID)).Limit(f.Limit)
	if userID > 0 {
		q = q.Where(order.UserIDEQ(userID))
	}
	if st := strings.ToLower(strings.TrimSpace(f.Status)); st != "" && st != "all" {
		q = q.Where(order.StatusEQ(st))
	}
	if ch := strings.ToLower(strings.TrimSpace(f.Channel)); ch == "wechat" || ch == "alipay" {
		q = q.Where(order.ChannelEQ(ch))
	}
	if st := strings.ToLower(strings.TrimSpace(f.SettleType)); st == "auto" || st == "manual" {
		q = q.Where(order.SettleTypeEQ(st))
	}
	if kw := strings.TrimSpace(f.Query); kw != "" {
		q = q.Where(order.OutTradeNoContains(kw))
	}
	return q.All(ctx)
}

type SetOrderStatusInput struct {
	OrderID   int64
	UserID    int64 // 0 = admin
	ActorType string
	ActorID   int64
	Status    string // paid|closed|expired
	Remark    string
	Channel   string
	PayAmount string
}

// SetOrderStatus 手动改订单状态：用于 App 未上报到账时人工确认，或关闭无效单。
func (s *ManualService) SetOrderStatus(ctx context.Context, in SetOrderStatusInput) (*ent.Order, error) {
	st := strings.ToLower(strings.TrimSpace(in.Status))
	if st != "paid" && st != "closed" && st != "expired" {
		return nil, fmt.Errorf("%w: status", ErrInvalid)
	}
	if strings.TrimSpace(in.Remark) == "" {
		return nil, fmt.Errorf("%w: remark required", ErrInvalid)
	}
	if st == "paid" {
		return s.ManualPay(ctx, ManualPayInput{
			OrderID: in.OrderID, UserID: in.UserID,
			ActorType: in.ActorType, ActorID: in.ActorID,
			Remark: in.Remark, Channel: in.Channel, PayAmount: in.PayAmount,
		})
	}

	var out *ent.Order
	err := s.db.WithTx(ctx, func(tx *ent.Tx) error {
		o, err := tx.Order.Get(ctx, in.OrderID)
		if err != nil {
			if ent.IsNotFound(err) {
				return ErrNotFound
			}
			return err
		}
		if in.UserID > 0 && o.UserID != in.UserID {
			return ErrNotFound
		}
		if o.Status == "paid" {
			return ErrAlreadyPaid
		}
		if o.Status != "pending" && o.Status != "expired" {
			return fmt.Errorf("%w: cannot change from %s", ErrInvalid, o.Status)
		}
		before := o.Status
		now := time.Now()
		upd := tx.Order.UpdateOneID(o.ID).
			SetStatus(st).
			SetManualRemark(in.Remark).
			SetManualByType(in.ActorType).
			SetManualByID(in.ActorID).
			SetManualAt(now)
		o2, err := upd.Save(ctx)
		if err != nil {
			return err
		}
		// 释放金额锁
		_, _ = tx.AmountLock.Delete().Where(amountlock.OrderIDEQ(o.ID)).Exec(ctx)

		lid, err := snowflake.NextID()
		if err != nil {
			return err
		}
		if _, err := tx.ManualSettleLog.Create().
			SetID(lid).
			SetOrderID(o.ID).
			SetUserID(o.UserID).
			SetActorType(in.ActorType).
			SetActorID(in.ActorID).
			SetAction("set_status").
			SetRemark(in.Remark).
			SetBeforeStatus(before).
			SetAfterStatus(st).
			Save(ctx); err != nil {
			return err
		}
		out = o2
		return nil
	})
	return out, err
}

func (s *ManualService) IgnoreOrphan(ctx context.Context, orphanID, userID, actorID int64, actorType, remark string) error {
	if strings.TrimSpace(remark) == "" {
		remark = "ignored"
	}
	return s.db.WithTx(ctx, func(tx *ent.Tx) error {
		orp, err := tx.PaymentOrphan.Get(ctx, orphanID)
		if err != nil {
			if ent.IsNotFound(err) {
				return ErrNotFound
			}
			return err
		}
		if userID > 0 && orp.UserID != userID {
			return ErrNotFound
		}
		if orp.Status != "open" {
			return fmt.Errorf("%w: orphan not open", ErrConflict)
		}
		if _, err := tx.PaymentOrphan.UpdateOneID(orphanID).SetStatus("ignored").Save(ctx); err != nil {
			return err
		}
		lid, err := snowflake.NextID()
		if err != nil {
			return err
		}
		_, err = tx.ManualSettleLog.Create().
			SetID(lid).
			SetOrderID(0).
			SetUserID(orp.UserID).
			SetActorType(actorType).
			SetActorID(actorID).
			SetAction("ignore_orphan").
			SetOrphanID(orphanID).
			SetRemark(remark).
			SetBeforeStatus("open").
			SetAfterStatus("ignored").
			Save(ctx)
		return err
	})
}

const MaxBatchManualIDs = 50

type BatchItemResult struct {
	ID    int64
	OK    bool
	Error string
}

type BatchResult struct {
	OK    int
	Fail  int
	Items []BatchItemResult
}

type BatchSetOrderStatusInput struct {
	OrderIDs  []int64
	UserID    int64
	ActorType string
	ActorID   int64
	Status    string
	Remark    string
	Channel   string
	PayAmount string
}

// BatchSetOrderStatus 批量改订单状态；逐条处理，允许部分成功。
func (s *ManualService) BatchSetOrderStatus(ctx context.Context, in BatchSetOrderStatusInput) (*BatchResult, error) {
	ids := dedupePositiveIDs(in.OrderIDs)
	if len(ids) == 0 {
		return nil, fmt.Errorf("%w: ids required", ErrInvalid)
	}
	if len(ids) > MaxBatchManualIDs {
		return nil, fmt.Errorf("%w: too many ids (max %d)", ErrInvalid, MaxBatchManualIDs)
	}
	st := strings.ToLower(strings.TrimSpace(in.Status))
	if st != "paid" && st != "closed" && st != "expired" {
		return nil, fmt.Errorf("%w: status", ErrInvalid)
	}
	if strings.TrimSpace(in.Remark) == "" {
		return nil, fmt.Errorf("%w: remark required", ErrInvalid)
	}
	out := &BatchResult{Items: make([]BatchItemResult, 0, len(ids))}
	for _, id := range ids {
		_, err := s.SetOrderStatus(ctx, SetOrderStatusInput{
			OrderID: id, UserID: in.UserID, ActorType: in.ActorType, ActorID: in.ActorID,
			Status: st, Remark: in.Remark, Channel: in.Channel, PayAmount: in.PayAmount,
		})
		item := BatchItemResult{ID: id, OK: err == nil}
		if err != nil {
			item.Error = manualErrCode(err)
			out.Fail++
		} else {
			out.OK++
		}
		out.Items = append(out.Items, item)
	}
	return out, nil
}

type BatchIgnoreOrphansInput struct {
	OrphanIDs []int64
	UserID    int64
	ActorType string
	ActorID   int64
	Remark    string
}

// BatchIgnoreOrphans 批量忽略未匹配到账；逐条处理，允许部分成功。
func (s *ManualService) BatchIgnoreOrphans(ctx context.Context, in BatchIgnoreOrphansInput) (*BatchResult, error) {
	ids := dedupePositiveIDs(in.OrphanIDs)
	if len(ids) == 0 {
		return nil, fmt.Errorf("%w: ids required", ErrInvalid)
	}
	if len(ids) > MaxBatchManualIDs {
		return nil, fmt.Errorf("%w: too many ids (max %d)", ErrInvalid, MaxBatchManualIDs)
	}
	remark := strings.TrimSpace(in.Remark)
	if remark == "" {
		remark = "ignored"
	}
	out := &BatchResult{Items: make([]BatchItemResult, 0, len(ids))}
	for _, id := range ids {
		err := s.IgnoreOrphan(ctx, id, in.UserID, in.ActorID, in.ActorType, remark)
		item := BatchItemResult{ID: id, OK: err == nil}
		if err != nil {
			item.Error = manualErrCode(err)
			out.Fail++
		} else {
			out.OK++
		}
		out.Items = append(out.Items, item)
	}
	return out, nil
}

func dedupePositiveIDs(ids []int64) []int64 {
	seen := make(map[int64]struct{}, len(ids))
	out := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

func manualErrCode(err error) string {
	switch {
	case err == nil:
		return ""
	case errors.Is(err, ErrNotFound):
		return "not_found"
	case errors.Is(err, ErrAlreadyPaid):
		return "already_paid"
	case errors.Is(err, ErrConflict):
		return "conflict"
	case errors.Is(err, ErrInvalid):
		return "invalid"
	default:
		return "server_error"
	}
}
