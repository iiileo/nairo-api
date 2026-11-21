package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"nairo/api_service/ent"
	"nairo/api_service/ent/merchantnotifyjob"
	"nairo/api_service/internal/logger"
	"nairo/api_service/internal/mysql"
	"nairo/api_service/internal/pkg/sign"
	"nairo/api_service/internal/snowflake"

	"go.uber.org/fx"
)

type NotifyService struct {
	db     *mysql.Client
	log    *logger.Logger
	client *http.Client
}

func NewNotifyService(db *mysql.Client, log *logger.Logger) *NotifyService {
	return &NotifyService{
		db:  db,
		log: log,
		client: &http.Client{Timeout: 8 * time.Second},
	}
}

type NotifyPayload struct {
	OutTradeNo     string `json:"out_trade_no"`
	OrderID        string `json:"order_id"`
	AmountCents    int    `json:"amount_cents"`
	PayAmountCents int    `json:"pay_amount_cents"`
	Channel        string `json:"channel"`
	Status         string `json:"status"`
	SettleType     string `json:"settle_type"`
	PaidAt         int64  `json:"paid_at"`
}

func (s *NotifyService) EnqueueOrderPaid(order *ent.Order) {
	if order == nil || order.NotifyURL == "" || s.db == nil {
		return
	}
	payload := NotifyPayload{
		OutTradeNo:     order.OutTradeNo,
		OrderID:        fmt.Sprintf("%d", order.ID),
		AmountCents:    order.AmountCents,
		PayAmountCents: order.PayAmountCents,
		Channel:        order.Channel,
		Status:         order.Status,
		SettleType:     order.SettleType,
	}
	if order.PaidAt != nil {
		payload.PaidAt = order.PaidAt.Unix()
	}
	body, _ := json.Marshal(payload)
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	id, err := snowflake.NextID()
	if err != nil {
		s.log.Warn(ctx, "merchant notify enqueue id failed", "err", err)
		return
	}
	now := time.Now()
	_, err = s.db.Ent().MerchantNotifyJob.Create().
		SetID(id).
		SetUserID(order.UserID).
		SetOrderID(order.ID).
		SetOutTradeNo(order.OutTradeNo).
		SetNotifyURL(order.NotifyURL).
		SetPayload(string(body)).
		SetStatus("pending").
		SetAttempts(0).
		SetMaxAttempts(8).
		SetNextRetryAt(now).
		Save(ctx)
	if err != nil {
		s.log.Warn(ctx, "merchant notify enqueue failed", "order_id", order.ID, "err", err)
		return
	}
	go s.processOne(id)
}

func RegisterNotifyWorker(lc fx.Lifecycle, s *NotifyService, log *logger.Logger) {
	if s == nil {
		return
	}
	stop := make(chan struct{})
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			go s.loop(stop)
			log.Info(ctx, "merchant notify worker started")
			return nil
		},
		OnStop: func(ctx context.Context) error {
			close(stop)
			return nil
		},
	})
}

func (s *NotifyService) loop(stop <-chan struct{}) {
	t := time.NewTicker(5 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-stop:
			return
		case <-t.C:
			s.drainPending(20)
		}
	}
}

func (s *NotifyService) drainPending(limit int) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	now := time.Now()
	jobs, err := s.db.Ent().MerchantNotifyJob.Query().
		Where(
			merchantnotifyjob.StatusEQ("pending"),
			merchantnotifyjob.Or(
				merchantnotifyjob.NextRetryAtIsNil(),
				merchantnotifyjob.NextRetryAtLTE(now),
			),
		).
		Order(ent.Asc(merchantnotifyjob.FieldCreatedAt)).
		Limit(limit).
		All(ctx)
	if err != nil {
		s.log.Warn(ctx, "merchant notify poll failed", "err", err)
		return
	}
	for _, j := range jobs {
		s.deliverJob(ctx, j)
	}
}

func (s *NotifyService) processOne(id int64) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	j, err := s.db.Ent().MerchantNotifyJob.Get(ctx, id)
	if err != nil {
		return
	}
	s.deliverJob(ctx, j)
}

func (s *NotifyService) deliverJob(ctx context.Context, j *ent.MerchantNotifyJob) {
	if j == nil || j.Status != "pending" {
		return
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, j.NotifyURL, bytes.NewReader([]byte(j.Payload)))
	if err != nil {
		s.failAttempt(ctx, j, err.Error())
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Nairo-Notify", "1")
	req.Header.Set("X-Nairo-Body-SHA256", sign.BodySHA256Hex([]byte(j.Payload)))
	resp, err := s.client.Do(req)
	if err != nil {
		s.failAttempt(ctx, j, err.Error())
		return
	}
	_ = resp.Body.Close()
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		_, _ = s.db.Ent().MerchantNotifyJob.UpdateOneID(j.ID).
			SetStatus("done").
			SetAttempts(j.Attempts + 1).
			SetLastError("").
			ClearNextRetryAt().
			Save(ctx)
		s.log.Info(ctx, "merchant notify ok", "job_id", j.ID, "out_trade_no", j.OutTradeNo)
		return
	}
	s.failAttempt(ctx, j, fmt.Sprintf("status %d", resp.StatusCode))
}

func (s *NotifyService) failAttempt(ctx context.Context, j *ent.MerchantNotifyJob, errMsg string) {
	attempts := j.Attempts + 1
	upd := s.db.Ent().MerchantNotifyJob.UpdateOneID(j.ID).
		SetAttempts(attempts).
		SetLastError(trimErr(errMsg, 500))
	if attempts >= j.MaxAttempts {
		upd.SetStatus("failed").ClearNextRetryAt()
		_, _ = upd.Save(ctx)
		s.log.Warn(ctx, "merchant notify failed permanently",
			"job_id", j.ID, "out_trade_no", j.OutTradeNo, "err", errMsg)
		return
	}
	// 指数退避：1s,2s,4s... 上限 15m
	shift := attempts - 1
	if shift > 9 {
		shift = 9
	}
	delay := time.Duration(1<<uint(shift)) * time.Second
	if delay > 15*time.Minute {
		delay = 15 * time.Minute
	}
	next := time.Now().Add(delay)
	_, _ = upd.SetStatus("pending").SetNextRetryAt(next).Save(ctx)
	s.log.Warn(ctx, "merchant notify retry scheduled",
		"job_id", j.ID, "attempts", attempts, "next", next, "err", errMsg)
}

func trimErr(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

type NotifyJobFilter struct {
	UserID *int64
	Status string
	Limit  int
}

func (s *NotifyService) ListJobs(ctx context.Context, f NotifyJobFilter) ([]*ent.MerchantNotifyJob, error) {
	limit := f.Limit
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	q := s.db.Ent().MerchantNotifyJob.Query().
		Order(ent.Desc(merchantnotifyjob.FieldCreatedAt)).
		Limit(limit)
	if f.UserID != nil {
		q = q.Where(merchantnotifyjob.UserIDEQ(*f.UserID))
	}
	if st := f.Status; st == "pending" || st == "done" || st == "failed" {
		q = q.Where(merchantnotifyjob.StatusEQ(st))
	}
	return q.All(ctx)
}

func (s *NotifyService) ReplayJob(ctx context.Context, jobID int64, userID int64) (*ent.MerchantNotifyJob, error) {
	j, err := s.db.Ent().MerchantNotifyJob.Get(ctx, jobID)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if userID > 0 && j.UserID != userID {
		return nil, ErrNotFound
	}
	now := time.Now()
	j, err = s.db.Ent().MerchantNotifyJob.UpdateOneID(j.ID).
		SetStatus("pending").
		SetNextRetryAt(now).
		SetLastError("").
		Save(ctx)
	if err != nil {
		return nil, err
	}
	go s.processOne(j.ID)
	return j, nil
}
