package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"nairo/api_service/ent"
	"nairo/api_service/ent/amountlock"
	"nairo/api_service/ent/apicredential"
	"nairo/api_service/ent/order"
	"nairo/api_service/ent/paycode"
	"nairo/api_service/ent/paymentaccount"
	"nairo/api_service/ent/user"
	"nairo/api_service/ent/wallet"
	"nairo/api_service/internal/logger"
	"nairo/api_service/internal/mysql"
	"nairo/api_service/internal/pkg/secretbox"
	"nairo/api_service/internal/runtimeconfig"
	"nairo/api_service/internal/snowflake"

	"go.uber.org/fx"
)

type OrderService struct {
	db    *mysql.Client
	rt    *runtimeconfig.RuntimeConfig
	bot   *BotNotifyService
	vault *secretbox.Vault
}

func NewOrderService(db *mysql.Client, rt *runtimeconfig.RuntimeConfig, bot *BotNotifyService, vault *secretbox.Vault) *OrderService {
	return &OrderService{db: db, rt: rt, bot: bot, vault: vault}
}

func (s *OrderService) FindCredential(ctx context.Context, accessKey string) (*ent.ApiCredential, error) {
	c, err := s.db.Ent().ApiCredential.Query().Where(apicredential.AccessKeyEQ(accessKey)).Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, ErrUnauthorized
		}
		return nil, err
	}
	return c, nil
}

// ResolveOpenSecret 返回可用于 HMAC 的明文 API Secret（兼容加密落库与历史明文）。
func (s *OrderService) ResolveOpenSecret(ctx context.Context, accessKey string) (userID int64, key, secret string, err error) {
	c, err := s.FindCredential(ctx, accessKey)
	if err != nil {
		return 0, "", "", err
	}
	plain, err := s.vault.Open(c.SecretHash)
	if err != nil {
		return 0, "", "", err
	}
	return c.UserID, c.AccessKey, plain, nil
}

type CreateOrderInput struct {
	UserID      int64
	OutTradeNo  string
	AmountCents int
	Channel     string
	NotifyURL   string
	ReturnURL   string // 支付成功后收银台页面跳转地址（选填，仅 http/https）
}

type CreateOrderResult struct {
	Order       *ent.Order
	PayCode     *ent.PayCode
	Account     *ent.PaymentAccount
	PayAmount   int
	CallbackURL string
	PayURL      string // 收银台页，可直接打开展示二维码
	ExpireIn    int64  // 距过期剩余秒数
}

func (s *OrderService) Create(ctx context.Context, in CreateOrderInput) (*CreateOrderResult, error) {
	in.OutTradeNo = strings.TrimSpace(in.OutTradeNo)
	in.Channel = strings.ToLower(strings.TrimSpace(in.Channel))
	if in.OutTradeNo == "" || in.AmountCents <= 0 {
		return nil, fmt.Errorf("%w: out_trade_no/amount", ErrInvalid)
	}
	if in.Channel != "wechat" && in.Channel != "alipay" {
		return nil, fmt.Errorf("%w: channel", ErrInvalid)
	}

	u, err := s.db.Ent().User.Get(ctx, in.UserID)
	if err != nil {
		return nil, err
	}
	if u.Status != "active" {
		return nil, fmt.Errorf("%w: user disabled", ErrInvalid)
	}
	fee := FeeCents(in.AmountCents, u.FeeRateBps)
	w, err := s.db.Ent().Wallet.Query().Where(wallet.UserIDEQ(u.ID)).Only(ctx)
	if err != nil {
		return nil, err
	}
	if w.BalanceCents < fee {
		s.bot.EnqueueFeeInsufficient(u.ID, in.OutTradeNo, in.AmountCents, fee, w.BalanceCents)
		return nil, ErrInsufficient
	}

	exists, err := s.db.Ent().Order.Query().
		Where(order.UserIDEQ(u.ID), order.OutTradeNoEQ(in.OutTradeNo)).
		Exist(ctx)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrConflict
	}

	ttl := s.rt.PayLockTTLSeconds()
	if ttl <= 0 {
		ttl = 300
	}
	expireAt := time.Now().Add(time.Duration(ttl) * time.Second)

	var result *CreateOrderResult
	err = s.db.WithTx(ctx, func(tx *ent.Tx) error {
		allAcc, err := tx.PaymentAccount.Query().
			Where(
				paymentaccount.UserIDEQ(u.ID),
				paymentaccount.StatusEQ("active"),
			).
			All(ctx)
		if err != nil {
			return err
		}
		accounts := FilterAccountsByChannel(allAcc, in.Channel)
		if len(accounts) == 0 {
			return fmt.Errorf("%w:%s", ErrNoChannelDevice, in.Channel)
		}

		// 优先 App 在线账户
		sorted := prioritizeOnline(accounts)

		var chosenAcc *ent.PaymentAccount
		var chosenCode *ent.PayCode
		var payAmount int
		var offset int

		for _, acc := range sorted {
			code, amt, off, ok, err := pickCodeAndAmount(ctx, tx, u, acc.ID, in.Channel, in.AmountCents, expireAt)
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
		o, err := tx.Order.Create().
			SetID(oid).
			SetUserID(u.ID).
			SetPurpose(PurposeCollect).
			SetOutTradeNo(in.OutTradeNo).
			SetAmountCents(in.AmountCents).
			SetPayAmountCents(payAmount).
			SetAdjustOffsetCents(offset).
			SetChannel(in.Channel).
			SetStatus("pending").
			SetNotifyURL(in.NotifyURL).
			SetReturnURL(in.ReturnURL).
			SetPayCodeID(chosenCode.ID).
			SetAccountID(chosenAcc.ID).
			SetSettleType("auto").
			SetExpireAt(expireAt).
			Save(ctx)
		if err != nil {
			return err
		}
		lid, err := snowflake.NextID()
		if err != nil {
			return err
		}
		_, err = tx.AmountLock.Create().
			SetID(lid).
			SetUserID(u.ID).
			SetChannel(in.Channel).
			SetPayAmountCents(payAmount).
			SetOrderID(o.ID).
			SetLockedUntil(expireAt).
			Save(ctx)
		if err != nil {
			return err
		}

		base := s.rt.PayPublicBaseURL()
		expireIn := int64(0)
		if o.ExpireAt != nil {
			expireIn = int64(time.Until(*o.ExpireAt).Seconds())
			if expireIn < 0 {
				expireIn = 0
			}
		}
		result = &CreateOrderResult{
			Order:       o,
			PayCode:     chosenCode,
			Account:     chosenAcc,
			PayAmount:   payAmount,
			CallbackURL: fmt.Sprintf("%s/app/push/%s", base, chosenAcc.CallbackID),
			PayURL:      fmt.Sprintf("%s/checkout/%d", base, o.ID),
			ExpireIn:    expireIn,
		}
		return nil
	})
	return result, err
}

func prioritizeOnline(accounts []*ent.PaymentAccount) []*ent.PaymentAccount {
	online := make([]*ent.PaymentAccount, 0, len(accounts))
	offline := make([]*ent.PaymentAccount, 0, len(accounts))
	threshold := time.Now().Add(-AppOnlineWindow)
	for _, a := range accounts {
		if a.AppLastSeenAt != nil && a.AppLastSeenAt.After(threshold) {
			online = append(online, a)
		} else {
			offline = append(offline, a)
		}
	}
	return append(online, offline...)
}

// pickCodeAndAmount 优先同金额固额码；没有再用任意金额码。
func pickCodeAndAmount(
	ctx context.Context,
	tx *ent.Tx,
	u *ent.User,
	accountID int64,
	channel string,
	amountCents int,
	until time.Time,
) (code *ent.PayCode, payAmount, offset int, ok bool, err error) {
	fixedCode, err := queryActivePayCode(ctx, tx, accountID, channel, "fixed", &amountCents)
	if err != nil {
		return nil, 0, 0, false, err
	}
	if fixedCode != nil {
		busy, err := hasActiveLock(ctx, tx, u.ID, channel, amountCents)
		if err != nil {
			return nil, 0, 0, false, err
		}
		if !busy {
			return fixedCode, amountCents, 0, true, nil
		}
	}

	floatCode, err := queryActivePayCode(ctx, tx, accountID, channel, "float", nil)
	if err != nil {
		return nil, 0, 0, false, err
	}
	if floatCode == nil {
		return nil, 0, 0, false, nil
	}
	amt, off, allocated, err := allocateAmount(ctx, tx, u, channel, amountCents, until)
	if err != nil {
		return nil, 0, 0, false, err
	}
	if !allocated {
		return nil, 0, 0, false, nil
	}
	return floatCode, amt, off, true, nil
}

func queryActivePayCode(
	ctx context.Context,
	tx *ent.Tx,
	accountID int64,
	channel, codeType string,
	fixedAmount *int,
) (*ent.PayCode, error) {
	q := tx.PayCode.Query().Where(
		paycode.AccountIDEQ(accountID),
		paycode.StatusEQ("active"),
		paycode.CodeTypeEQ(codeType),
		paycode.Or(paycode.ChannelEQ(channel), paycode.ChannelEQ("")),
	)
	if codeType == "fixed" {
		if fixedAmount == nil || *fixedAmount <= 0 {
			return nil, nil
		}
		q = q.Where(paycode.AmountCentsEQ(*fixedAmount))
	}
	codes, err := q.Limit(1).All(ctx)
	if err != nil {
		return nil, err
	}
	if len(codes) == 0 {
		return nil, nil
	}
	return codes[0], nil
}

// pickFloatCode 保留给仅需任意金额码的场景。
func pickFloatCode(ctx context.Context, tx *ent.Tx, accountID int64, channel string) (*ent.PayCode, error) {
	code, err := queryActivePayCode(ctx, tx, accountID, channel, "float", nil)
	if err != nil {
		return nil, err
	}
	if code == nil {
		return nil, ErrNoPayCode
	}
	return code, nil
}

func allocateAmount(ctx context.Context, tx *ent.Tx, u *ent.User, channel string, base int, until time.Time) (pay int, offset int, ok bool, err error) {
	if !u.AdjustEnabled {
		// 不微调：尝试占用 base 本身
		busy, err := hasActiveLock(ctx, tx, u.ID, channel, base)
		if err != nil {
			return 0, 0, false, err
		}
		if busy {
			return 0, 0, false, nil
		}
		return base, 0, true, nil
	}
	step := u.AdjustStepCents
	if step <= 0 {
		step = 1
	}
	maxOff := u.AdjustMaxOffsetCents
	if maxOff <= 0 {
		maxOff = 50
	}
	dir := strings.ToLower(u.AdjustDirection)
	if dir != "dec" {
		dir = "inc"
	}

	// 含 0 偏移
	candidates := make([]int, 0, maxOff/step+1)
	candidates = append(candidates, 0)
	for o := step; o <= maxOff; o += step {
		candidates = append(candidates, o)
	}

	for _, o := range candidates {
		var amt int
		if dir == "dec" {
			amt = base - o
		} else {
			amt = base + o
		}
		if amt <= 0 {
			continue
		}
		busy, err := hasActiveLock(ctx, tx, u.ID, channel, amt)
		if err != nil {
			return 0, 0, false, err
		}
		if busy {
			continue
		}
		off := o
		if dir == "dec" {
			off = -o
		}
		_ = until
		return amt, off, true, nil
	}
	return 0, 0, false, nil
}

func hasActiveLock(ctx context.Context, tx *ent.Tx, userID int64, channel string, payAmount int) (bool, error) {
	now := time.Now()
	return tx.AmountLock.Query().
		Where(
			amountlock.UserIDEQ(userID),
			amountlock.ChannelEQ(channel),
			amountlock.PayAmountCentsEQ(payAmount),
			amountlock.LockedUntilGT(now),
		).
		Exist(ctx)
}

// CleanupStaleAmountLocks 清理已无占用意义的金额锁：
// 1) 已过期；2) 关联订单已非 pending（含历史已支付未释放）；3) 订单已不存在。
func (s *OrderService) CleanupStaleAmountLocks(ctx context.Context) (int64, error) {
	res, err := s.db.DB().ExecContext(ctx, `
DELETE al FROM amount_locks al
LEFT JOIN orders o ON o.id = al.order_id
WHERE al.locked_until <= ? OR o.id IS NULL OR o.status <> 'pending'`, time.Now())
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return n, nil
}

// RegisterAmountLockCleanup 启动时清一次历史脏锁，并周期性清过期锁。
func RegisterAmountLockCleanup(lc fx.Lifecycle, s *OrderService, log *logger.Logger) {
	if s == nil {
		return
	}
	stop := make(chan struct{})
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			n, err := s.CleanupStaleAmountLocks(ctx)
			if err != nil {
				log.Warn(ctx, "amount lock cleanup failed", "error", err.Error())
			} else if n > 0 {
				log.Info(ctx, "amount lock cleanup", "deleted", n)
			}
			go func() {
				t := time.NewTicker(60 * time.Second)
				defer t.Stop()
				for {
					select {
					case <-stop:
						return
					case <-t.C:
						cctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
						_, _ = s.CleanupStaleAmountLocks(cctx)
						cancel()
					}
				}
			}()
			return nil
		},
		OnStop: func(context.Context) error {
			close(stop)
			return nil
		},
	})
}

func (s *OrderService) GetByID(ctx context.Context, orderID int64) (*ent.Order, error) {
	o, err := s.db.Ent().Order.Get(ctx, orderID)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return o, nil
}

func (s *OrderService) GetPayCode(ctx context.Context, payCodeID int64) (*ent.PayCode, error) {
	c, err := s.db.Ent().PayCode.Get(ctx, payCodeID)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return c, nil
}

func (s *OrderService) GetByOutTradeNo(ctx context.Context, userID int64, outTradeNo string) (*ent.Order, error) {
	o, err := s.db.Ent().Order.Query().
		Where(order.UserIDEQ(userID), order.OutTradeNoEQ(outTradeNo)).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return o, nil
}

func (s *OrderService) LoadUser(ctx context.Context, userID int64) (*ent.User, error) {
	return s.db.Ent().User.Query().Where(user.IDEQ(userID)).Only(ctx)
}
