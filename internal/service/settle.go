package service

import (
	"context"
	"fmt"
	"time"

	"nairo/api_service/ent"
	"nairo/api_service/ent/amountlock"
	"nairo/api_service/ent/wallet"
	"nairo/api_service/internal/snowflake"
)

type SettleActor struct {
	Type string // auto|console|admin
	ID   int64
}

type SettleOpts struct {
	SettleType string // auto|manual
	Actor      SettleActor
	Remark     string
	OrphanID   *int64
	PaidAt     time.Time
	PayAmount  *int // optional override for manual
	Channel    string
}

// markPaidInTx 将 pending/expired 订单标为 paid。
// collect：扣收款商户手续费；recharge：给 payer 钱包入账；test（试付）：只改状态，不动余额。
func markPaidInTx(ctx context.Context, tx *ent.Tx, o *ent.Order, opts SettleOpts) (*ent.Order, error) {
	if o.Status == "paid" {
		return nil, ErrAlreadyPaid
	}
	if o.Status != "pending" && o.Status != "expired" {
		return nil, ErrInvalid
	}
	paidAt := opts.PaidAt
	if paidAt.IsZero() {
		paidAt = time.Now()
	}

	switch {
	case IsDemoPayPurpose(o.Purpose):
		// 公开试付：不入账、不扣手续费
	case IsWalletCreditPurpose(o.Purpose):
		if err := creditPayerWalletInTx(ctx, tx, o, opts.Remark); err != nil {
			return nil, err
		}
	default:
		if err := deductMerchantFeeInTx(ctx, tx, o, opts.Remark, opts.SettleType); err != nil {
			return nil, err
		}
	}

	upd := tx.Order.UpdateOneID(o.ID).
		SetStatus("paid").
		SetPaidAt(paidAt).
		SetSettleType(opts.SettleType)
	if opts.PayAmount != nil {
		upd.SetPayAmountCents(*opts.PayAmount)
	}
	if opts.Channel != "" {
		upd.SetChannel(opts.Channel)
	}
	if opts.SettleType == "manual" {
		upd.SetManualRemark(opts.Remark).
			SetManualByType(opts.Actor.Type).
			SetManualByID(opts.Actor.ID).
			SetManualAt(paidAt)
	}
	o2, err := upd.Save(ctx)
	if err != nil {
		return nil, err
	}

	// 已支付即可释放金额锁，避免后续同金额订单被不必要微调。
	if _, err := tx.AmountLock.Delete().Where(amountlock.OrderIDEQ(o.ID)).Exec(ctx); err != nil {
		return nil, err
	}

	if opts.SettleType == "manual" || opts.Actor.Type != "auto" {
		lid, err := snowflake.NextID()
		if err != nil {
			return nil, err
		}
		log := tx.ManualSettleLog.Create().
			SetID(lid).
			SetOrderID(o.ID).
			SetUserID(o.UserID).
			SetActorType(opts.Actor.Type).
			SetActorID(opts.Actor.ID).
			SetAction("manual_pay").
			SetRemark(opts.Remark).
			SetBeforeStatus(o.Status).
			SetAfterStatus("paid")
		if opts.OrphanID != nil {
			log.SetOrphanID(*opts.OrphanID).SetAction("bind_orphan")
		}
		if _, err := log.Save(ctx); err != nil {
			return nil, err
		}
	}
	return o2, nil
}

func deductMerchantFeeInTx(ctx context.Context, tx *ent.Tx, o *ent.Order, remark, settleType string) error {
	u, err := tx.User.Get(ctx, o.UserID)
	if err != nil {
		return err
	}
	fee := FeeCents(o.AmountCents, u.FeeRateBps)
	w, err := tx.Wallet.Query().Where(wallet.UserIDEQ(u.ID)).Only(ctx)
	if err != nil {
		return err
	}
	newBal := w.BalanceCents - fee
	if _, err := tx.Wallet.UpdateOneID(w.ID).SetBalanceCents(newBal).Save(ctx); err != nil {
		return err
	}
	if fee == 0 {
		return nil
	}
	lid, err := snowflake.NextID()
	if err != nil {
		return err
	}
	if remark == "" {
		remark = settleType + " settle"
	}
	_, err = tx.WalletLedger.Create().
		SetID(lid).
		SetUserID(u.ID).
		SetType("fee_deduct").
		SetAmountCents(-fee).
		SetBalanceAfterCents(newBal).
		SetOrderID(o.ID).
		SetRemark(remark).
		Save(ctx)
	return err
}

func creditPayerWalletInTx(ctx context.Context, tx *ent.Tx, o *ent.Order, remark string) error {
	if o.PayerUserID == nil || *o.PayerUserID <= 0 {
		return fmt.Errorf("%w: payer_user_id required", ErrInvalid)
	}
	payerID := *o.PayerUserID
	w, err := tx.Wallet.Query().Where(wallet.UserIDEQ(payerID)).Only(ctx)
	if err != nil {
		return err
	}
	credit := int64(o.AmountCents)
	if credit <= 0 {
		return fmt.Errorf("%w: credit amount", ErrInvalid)
	}
	newBal := w.BalanceCents + credit
	if _, err := tx.Wallet.UpdateOneID(w.ID).SetBalanceCents(newBal).Save(ctx); err != nil {
		return err
	}
	lid, err := snowflake.NextID()
	if err != nil {
		return err
	}
	if remark == "" {
		remark = "在线充值手续费"
	}
	_, err = tx.WalletLedger.Create().
		SetID(lid).
		SetUserID(payerID).
		SetType("recharge").
		SetAmountCents(credit).
		SetBalanceAfterCents(newBal).
		SetOrderID(o.ID).
		SetRemark(remark).
		Save(ctx)
	return err
}
