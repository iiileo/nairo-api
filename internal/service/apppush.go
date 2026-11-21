package service

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"nairo/api_service/ent"
	"nairo/api_service/ent/order"
	"nairo/api_service/ent/paymentaccount"
	"nairo/api_service/ent/paymentorphan"
	"nairo/api_service/internal/mysql"
	"nairo/api_service/internal/pkg/channels"
	"nairo/api_service/internal/pkg/secretbox"
	"nairo/api_service/internal/pkg/sign"
	"nairo/api_service/internal/snowflake"
)

// appPushDedupeWindow：同设备同渠道同金额短窗内视为重复推送（微信常连发多条通知）。
const appPushDedupeWindow = 2 * time.Minute

type AppPushService struct {
	db     *mysql.Client
	notify *NotifyService
	bot    *BotNotifyService
	vault  *secretbox.Vault
}

func NewAppPushService(db *mysql.Client, notify *NotifyService, bot *BotNotifyService, vault *secretbox.Vault) *AppPushService {
	return &AppPushService{db: db, notify: notify, bot: bot, vault: vault}
}

type AppPushInput struct {
	CallbackID string
	T          int64
	Channel    string
	Price      string
	Nonce      string
	Sign       string
	IdemKey    string
	DeviceID   string
	RawJSON    string
}

func (s *AppPushService) Handle(ctx context.Context, in AppPushInput) error {
	in.Channel = strings.ToLower(strings.TrimSpace(in.Channel))
	if in.Channel != "wechat" && in.Channel != "alipay" {
		return fmt.Errorf("%w: channel", ErrInvalid)
	}
	if !sign.WithinSkew(in.T, 5*time.Minute) {
		return fmt.Errorf("%w: timestamp", ErrInvalid)
	}

	acc, err := s.db.Ent().PaymentAccount.Query().
		Where(paymentaccount.CallbackIDEQ(in.CallbackID), paymentaccount.StatusEQ("active")).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return ErrNotFound
		}
		return err
	}
	plain, err := s.vault.Open(acc.AppSecretHash)
	if err != nil {
		return err
	}
	expect := sign.AppPushSign(in.CallbackID, in.T, in.Channel, in.Price, in.Nonce, plain)
	if !strings.EqualFold(expect, strings.TrimSpace(in.Sign)) {
		return ErrUnauthorized
	}
	if err := checkBoundDevice(ctx, s.db, acc, in.DeviceID); err != nil {
		return err
	}
	if !channels.Supports(acc.Channel, in.Channel) {
		return fmt.Errorf("%w: channel not enabled on device", ErrInvalid)
	}

	payCents, err := parseYuanToCents(in.Price)
	if err != nil {
		return fmt.Errorf("%w: price", ErrInvalid)
	}
	if payCents <= 0 {
		// 金额为 0 的通知（如支付宝营销文案「¥0」）不是收款，直接忽略；返回成功避免旧版 App 反复重试
		return nil
	}

	now := time.Now()
	_ = s.db.Ent().PaymentAccount.UpdateOneID(acc.ID).
		SetAppLastSeenAt(now).
		SetAppConnType("http").
		Exec(ctx)

	idemKey := normalizePushIdemKey(in.IdemKey, in.Nonce)

	var matched *ent.Order
	var outcome string // matched | orphan | duplicate
	err = s.db.WithTx(ctx, func(tx *ent.Tx) error {
		// 1) 请求级幂等（同一 idem_key / nonce 重试直接成功）
		ok, err := claimIdempotencyTx(ctx, tx, IdemScopeAppPush, fmt.Sprintf("req:%d:%s", acc.ID, idemKey), "")
		if err != nil {
			return err
		}
		if !ok {
			outcome = "duplicate"
			return nil
		}

		o, err := tx.Order.Query().
			Where(
				order.UserIDEQ(acc.UserID),
				order.AccountIDEQ(acc.ID),
				order.ChannelEQ(in.Channel),
				order.PayAmountCentsEQ(payCents),
				order.StatusEQ("pending"),
				order.ExpireAtGT(now),
			).
			Order(ent.Asc(order.FieldCreatedAt)).
			First(ctx)
		if err != nil {
			if !ent.IsNotFound(err) {
				return err
			}
			// 2) 短窗内已入账 → 微信重复通知，吞掉
			since := now.Add(-appPushDedupeWindow)
			existsPaid, err := tx.Order.Query().
				Where(
					order.AccountIDEQ(acc.ID),
					order.ChannelEQ(in.Channel),
					order.PayAmountCentsEQ(payCents),
					order.StatusEQ("paid"),
					order.PaidAtNotNil(),
					order.PaidAtGTE(since),
				).
				Exist(ctx)
			if err != nil {
				return err
			}
			if existsPaid {
				outcome = "duplicate"
				return nil
			}
			// 3) 短窗内已有孤儿单 → 重复上报
			n, err := tx.PaymentOrphan.Query().
				Where(
					paymentorphan.AccountIDEQ(acc.ID),
					paymentorphan.ChannelEQ(in.Channel),
					paymentorphan.PayAmountCentsEQ(payCents),
					paymentorphan.ReportedAtGTE(since),
				).
				Count(ctx)
			if err != nil {
				return err
			}
			if n > 0 {
				outcome = "duplicate"
				return nil
			}

			oid, err := snowflake.NextID()
			if err != nil {
				return err
			}
			_, err = tx.PaymentOrphan.Create().
				SetID(oid).
				SetUserID(acc.UserID).
				SetAccountID(acc.ID).
				SetChannel(in.Channel).
				SetPayAmountCents(payCents).
				SetReportedAt(now).
				SetRawPayload(in.RawJSON).
				SetStatus("open").
				Save(ctx)
			if err != nil {
				return err
			}
			outcome = "orphan"
			return nil
		}
		matched, err = markPaidInTx(ctx, tx, o, SettleOpts{
			SettleType: "auto",
			Actor:      SettleActor{Type: "auto", ID: 0},
			Remark:     "auto settle",
			PaidAt:     now,
		})
		if err != nil {
			return err
		}
		outcome = "matched"
		return nil
	})
	if err != nil {
		return err
	}
	if outcome == "duplicate" {
		return nil
	}

	outNo := ""
	silentDemo := matched != nil && IsDemoPayPurpose(matched.Purpose)
	if matched != nil {
		outNo = matched.OutTradeNo
		if outcome == "matched" && ShouldDispatchOrderNotify(matched.Purpose) {
			s.notify.EnqueueOrderPaid(matched)
			s.bot.EnqueueOrderPaid(matched)
		}
	}
	// 试付匹配成功：不推送机器人「收到 App 支付推送」
	if !silentDemo {
		s.bot.EnqueueAppPush(acc.UserID, acc.ID, acc.Name, in.Channel, in.Price, matched != nil && outcome == "matched", outNo,
			fmt.Sprintf("app_push:%d:%s:%s", acc.ID, idemKey, outcome))
	}
	return nil
}

func normalizePushIdemKey(idemKey, nonce string) string {
	idemKey = strings.TrimSpace(idemKey)
	if idemKey != "" {
		return truncateUTF8(idemKey, 96)
	}
	nonce = strings.TrimSpace(nonce)
	if nonce == "" {
		return "empty"
	}
	return truncateUTF8(nonce, 96)
}

// truncateUTF8 按字节截断但不切断多字节字符（中文通知文案会被切坏，写库报 Incorrect string value）。
func truncateUTF8(s string, maxBytes int) string {
	if len(s) <= maxBytes {
		return s
	}
	cut := maxBytes
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut]
}

func parseYuanToCents(price string) (int, error) {
	price = strings.TrimSpace(price)
	f, err := strconv.ParseFloat(price, 64)
	if err != nil {
		return 0, err
	}
	return int(f*100 + 0.5), nil
}
