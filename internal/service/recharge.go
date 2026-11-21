package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"nairo/api_service/ent"
	"nairo/api_service/ent/order"
	"nairo/api_service/ent/paymentaccount"
	"nairo/api_service/internal/mysql"
	"nairo/api_service/internal/runtimeconfig"
	"nairo/api_service/internal/snowflake"
)

const (
	PurposeCollect  = "collect"
	PurposeRecharge = "recharge"
	PurposeTest     = "test"

	MinRechargeCents = 10     // ¥0.10
	MaxRechargeCents = 500000 // ¥5000
)

type RechargeService struct {
	db       *mysql.Client
	rt       *runtimeconfig.RuntimeConfig
	platform *PlatformService
}

func NewRechargeService(db *mysql.Client, rt *runtimeconfig.RuntimeConfig, platform *PlatformService) *RechargeService {
	return &RechargeService{db: db, rt: rt, platform: platform}
}

type CreateWalletPayInput struct {
	PayerUserID int64
	AmountCents int
	Channel     string
	Purpose     string // recharge | test
}

type CreateWalletPayResult struct {
	Order       *ent.Order
	PayCode     *ent.PayCode
	Account     *ent.PaymentAccount
	PayAmount   int
	PayPagePath string
}

func (s *RechargeService) Create(ctx context.Context, in CreateWalletPayInput) (*CreateWalletPayResult, error) {
	purpose := strings.ToLower(strings.TrimSpace(in.Purpose))
	// 控制台仅手续费充值；公开演示测试见 CreatePublicDemo
	if purpose != PurposeRecharge {
		if purpose == PurposeTest {
			return nil, fmt.Errorf("%w: use public demo for test", ErrInvalid)
		}
		return nil, fmt.Errorf("%w: purpose", ErrInvalid)
	}
	if in.PayerUserID <= 0 {
		return nil, fmt.Errorf("%w: payer", ErrInvalid)
	}

	payUser, err := s.platform.GetPlatformPayUser(ctx)
	if err != nil {
		return nil, err
	}
	if payUser.ID == in.PayerUserID {
		return nil, fmt.Errorf("%w: platform_pay_user_self", ErrInvalid)
	}

	payer, err := s.db.Ent().User.Get(ctx, in.PayerUserID)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if payer.Status != "active" {
		return nil, fmt.Errorf("%w: payer disabled", ErrInvalid)
	}

	return s.createPlatformPayOrder(ctx, createPlatformPayParams{
		PayUser:     payUser,
		PayerUserID: &in.PayerUserID,
		AmountCents: in.AmountCents,
		Channel:     in.Channel,
		Purpose:     PurposeRecharge,
		OutPrefix:   "RCH",
		PayPagePath: func(oid int64) string { return fmt.Sprintf("/console/pay/%d", oid) },
	})
}

// CreatePublicDemo 首页公开试付：不登录、不入账，仅验证真实收款与到账匹配。
func (s *RechargeService) CreatePublicDemo(ctx context.Context, amountCents int, channel string) (*CreateWalletPayResult, error) {
	payUser, err := s.platform.GetPlatformPayUser(ctx)
	if err != nil {
		return nil, err
	}
	return s.createPlatformPayOrder(ctx, createPlatformPayParams{
		PayUser:     payUser,
		PayerUserID: nil,
		AmountCents: amountCents,
		Channel:     channel,
		Purpose:     PurposeTest,
		OutPrefix:   "DEMO",
		PayPagePath: func(oid int64) string { return fmt.Sprintf("/demo/%d", oid) },
	})
}

type createPlatformPayParams struct {
	PayUser     *ent.User
	PayerUserID *int64
	AmountCents int
	Channel     string
	Purpose     string
	OutPrefix   string
	PayPagePath func(orderID int64) string
}

func (s *RechargeService) createPlatformPayOrder(ctx context.Context, in createPlatformPayParams) (*CreateWalletPayResult, error) {
	channel := strings.ToLower(strings.TrimSpace(in.Channel))
	if channel != "wechat" && channel != "alipay" {
		return nil, fmt.Errorf("%w: channel", ErrInvalid)
	}
	if in.AmountCents < MinRechargeCents || in.AmountCents > MaxRechargeCents {
		return nil, fmt.Errorf("%w: amount_cents range %d-%d", ErrInvalid, MinRechargeCents, MaxRechargeCents)
	}
	if in.PayUser == nil {
		return nil, fmt.Errorf("%w: platform_pay_user", ErrInvalid)
	}

	ttl := s.rt.PayLockTTLSeconds()
	if ttl <= 0 {
		ttl = 300
	}
	expireAt := time.Now().Add(time.Duration(ttl) * time.Second)
	payerPart := int64(0)
	if in.PayerUserID != nil {
		payerPart = *in.PayerUserID
	}
	outNo := fmt.Sprintf("%s%d_%d", in.OutPrefix, payerPart, time.Now().UnixNano())

	var result *CreateWalletPayResult
	err := s.db.WithTx(ctx, func(tx *ent.Tx) error {
		allAcc, err := tx.PaymentAccount.Query().
			Where(
				paymentaccount.UserIDEQ(in.PayUser.ID),
				paymentaccount.StatusEQ("active"),
			).
			All(ctx)
		if err != nil {
			return err
		}
		accounts := FilterAccountsByChannel(allAcc, channel)
		if len(accounts) == 0 {
			return fmt.Errorf("%w:%s", ErrNoChannelDevice, channel)
		}
		sorted := prioritizeOnline(accounts)

		var chosenAcc *ent.PaymentAccount
		var chosenCode *ent.PayCode
		var payAmount int
		var offset int
		for _, acc := range sorted {
			code, amt, off, ok, err := pickCodeAndAmount(ctx, tx, in.PayUser, acc.ID, channel, in.AmountCents, expireAt)
			if err != nil {
				return err
			}
			if !ok || code == nil {
				continue
			}
			chosenAcc = acc
			chosenCode = code
			payAmount = amt
			offset = off
			break
		}
		if chosenAcc == nil || chosenCode == nil {
			return ErrNoPayCode
		}

		oid, err := snowflake.NextID()
		if err != nil {
			return err
		}
		b := tx.Order.Create().
			SetID(oid).
			SetUserID(in.PayUser.ID).
			SetPurpose(in.Purpose).
			SetOutTradeNo(outNo).
			SetAmountCents(in.AmountCents).
			SetPayAmountCents(payAmount).
			SetAdjustOffsetCents(offset).
			SetChannel(channel).
			SetStatus("pending").
			SetNotifyURL("").
			SetPayCodeID(chosenCode.ID).
			SetAccountID(chosenAcc.ID).
			SetSettleType("auto").
			SetExpireAt(expireAt)
		if in.PayerUserID != nil {
			b.SetPayerUserID(*in.PayerUserID)
		}
		o, err := b.Save(ctx)
		if err != nil {
			return err
		}
		lid, err := snowflake.NextID()
		if err != nil {
			return err
		}
		_, err = tx.AmountLock.Create().
			SetID(lid).
			SetUserID(in.PayUser.ID).
			SetChannel(channel).
			SetPayAmountCents(payAmount).
			SetOrderID(o.ID).
			SetLockedUntil(expireAt).
			Save(ctx)
		if err != nil {
			return err
		}
		result = &CreateWalletPayResult{
			Order:       o,
			PayCode:     chosenCode,
			Account:     chosenAcc,
			PayAmount:   payAmount,
			PayPagePath: in.PayPagePath(o.ID),
		}
		return nil
	})
	return result, err
}

func (s *RechargeService) GetPublicDemo(ctx context.Context, orderID int64) (*ent.Order, *ent.PayCode, error) {
	o, err := s.db.Ent().Order.Get(ctx, orderID)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, nil, ErrNotFound
		}
		return nil, nil, err
	}
	if o.Purpose != PurposeTest || o.PayerUserID != nil {
		return nil, nil, ErrNotFound
	}
	var code *ent.PayCode
	if o.PayCodeID != nil {
		code, err = s.db.Ent().PayCode.Get(ctx, *o.PayCodeID)
		if err != nil && !ent.IsNotFound(err) {
			return nil, nil, err
		}
	}
	return o, code, nil
}

func (s *RechargeService) PublicDemoStatus(ctx context.Context) (configured bool, warning string, err error) {
	view, err := s.platform.GetSettings(ctx)
	if err != nil {
		return false, "", err
	}
	configured = view.PayUserID != "" && view.PayUserOK
	warning = "演示支付走真实收款通道，金额不会退回。"
	return configured, warning, nil
}

func (s *RechargeService) GetForPayer(ctx context.Context, payerUserID, orderID int64) (*ent.Order, *ent.PayCode, error) {
	o, err := s.db.Ent().Order.Get(ctx, orderID)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, nil, ErrNotFound
		}
		return nil, nil, err
	}
	if o.PayerUserID == nil || *o.PayerUserID != payerUserID {
		return nil, nil, ErrNotFound
	}
	if o.Purpose != PurposeRecharge && o.Purpose != PurposeTest {
		return nil, nil, ErrNotFound
	}
	var code *ent.PayCode
	if o.PayCodeID != nil {
		code, err = s.db.Ent().PayCode.Get(ctx, *o.PayCodeID)
		if err != nil && !ent.IsNotFound(err) {
			return nil, nil, err
		}
	}
	return o, code, nil
}

func (s *RechargeService) ListRecentForPayer(ctx context.Context, payerUserID int64, limit int) ([]*ent.Order, error) {
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	return s.db.Ent().Order.Query().
		Where(
			order.PayerUserIDEQ(payerUserID),
			order.PurposeIn(PurposeRecharge, PurposeTest),
		).
		Order(ent.Desc(order.FieldCreatedAt)).
		Limit(limit).
		All(ctx)
}

// IsWalletCreditPurpose：手续费充值到账后给付款方钱包入账。
func IsWalletCreditPurpose(purpose string) bool {
	return purpose == PurposeRecharge
}

// IsDemoPayPurpose：公开试付，只验证匹配，不入账、不扣费、不推送。
func IsDemoPayPurpose(purpose string) bool {
	return purpose == PurposeTest
}

// ShouldDispatchOrderNotify：商户 HTTP/机器人「订单入账」类通知。
func ShouldDispatchOrderNotify(purpose string) bool {
	return !IsWalletCreditPurpose(purpose) && !IsDemoPayPurpose(purpose)
}
