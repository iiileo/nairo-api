package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"nairo/api_service/ent"
	"nairo/api_service/ent/apicredential"
	"nairo/api_service/ent/order"
	"nairo/api_service/ent/walletledger"
	"nairo/api_service/internal/mysql"
	"nairo/api_service/internal/pkg/secretbox"
	"nairo/api_service/internal/pkg/secrethash"
	"nairo/api_service/internal/snowflake"
)

type ConsoleMerchantService struct {
	db    *mysql.Client
	vault *secretbox.Vault
}

func NewConsoleMerchantService(db *mysql.Client, vault *secretbox.Vault) *ConsoleMerchantService {
	return &ConsoleMerchantService{db: db, vault: vault}
}

type AdjustSettingsInput struct {
	Enabled        *bool
	Direction      string
	StepCents      *int
	MaxOffsetCents *int
}

func (s *ConsoleMerchantService) UpdateAdjust(ctx context.Context, userID int64, in AdjustSettingsInput) (*ent.User, error) {
	upd := s.db.Ent().User.UpdateOneID(userID)
	if in.Enabled != nil {
		upd.SetAdjustEnabled(*in.Enabled)
	}
	if d := strings.ToLower(strings.TrimSpace(in.Direction)); d != "" {
		if d != "inc" && d != "dec" {
			return nil, fmt.Errorf("%w: adjust_direction", ErrInvalid)
		}
		upd.SetAdjustDirection(d)
	}
	if in.StepCents != nil {
		if *in.StepCents < 1 {
			return nil, fmt.Errorf("%w: adjust_step_cents", ErrInvalid)
		}
		upd.SetAdjustStepCents(*in.StepCents)
	}
	if in.MaxOffsetCents != nil {
		if *in.MaxOffsetCents < 0 {
			return nil, fmt.Errorf("%w: adjust_max_offset_cents", ErrInvalid)
		}
		upd.SetAdjustMaxOffsetCents(*in.MaxOffsetCents)
	}
	u, err := upd.Save(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return u, nil
}

func (s *ConsoleMerchantService) ListLedgers(ctx context.Context, userID int64, limit int) ([]*ent.WalletLedger, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	return s.db.Ent().WalletLedger.Query().
		Where(walletledger.UserIDEQ(userID)).
		Order(ent.Desc(walletledger.FieldID)).
		Limit(limit).
		All(ctx)
}

func (s *ConsoleMerchantService) GetCredential(ctx context.Context, userID int64) (*ent.ApiCredential, error) {
	cred, err := s.db.Ent().ApiCredential.Query().Where(apicredential.UserIDEQ(userID)).Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	// 历史数据可能用 secret 前缀拼出非标准 key，统一纠正为 nk_ + 16 hex
	if !secrethash.ValidAccessKey(cred.AccessKey) {
		key, genErr := secrethash.NewAccessKey()
		if genErr != nil {
			return nil, genErr
		}
		cred, err = s.db.Ent().ApiCredential.UpdateOneID(cred.ID).SetAccessKey(key).Save(ctx)
		if err != nil {
			return nil, err
		}
	}
	return cred, nil
}

func (s *ConsoleMerchantService) ResetAPISecret(ctx context.Context, userID int64) (accessKey, apiSecret string, err error) {
	cred, err := s.GetCredential(ctx, userID)
	if err != nil {
		return "", "", err
	}
	apiSecret, err = secrethash.RandomToken(24)
	if err != nil {
		return "", "", err
	}
	// Access Key 保持稳定，仅轮换 Secret（与设计文档一致）
	accessKey = cred.AccessKey
	if accessKey == "" || !strings.HasPrefix(accessKey, "nk_") {
		accessKey, err = secrethash.NewAccessKey()
		if err != nil {
			return "", "", err
		}
	}
	storedSecret, err := s.vault.Seal(apiSecret)
	if err != nil {
		return "", "", err
	}
	upd := s.db.Ent().ApiCredential.UpdateOneID(cred.ID).SetSecretHash(storedSecret)
	if accessKey != cred.AccessKey {
		upd.SetAccessKey(accessKey)
	}
	_, err = upd.Save(ctx)
	if err != nil {
		return "", "", err
	}
	return accessKey, apiSecret, nil
}

// EnsureCredential 若无凭证则创建（兼容老数据）。
func (s *ConsoleMerchantService) EnsureCredential(ctx context.Context, userID int64) (*ent.ApiCredential, error) {
	cred, err := s.GetCredential(ctx, userID)
	if err == nil {
		return cred, nil
	}
	if err != ErrNotFound {
		return nil, err
	}
	apiSecret, err := secrethash.RandomToken(24)
	if err != nil {
		return nil, err
	}
	accessKey, err := secrethash.NewAccessKey()
	if err != nil {
		return nil, err
	}
	id, err := snowflake.NextID()
	if err != nil {
		return nil, err
	}
	storedSecret, err := s.vault.Seal(apiSecret)
	if err != nil {
		return nil, err
	}
	return s.db.Ent().ApiCredential.Create().
		SetID(id).
		SetUserID(userID).
		SetAccessKey(accessKey).
		SetSecretHash(storedSecret).
		Save(ctx)
}

func (s *ConsoleMerchantService) CountPaidToday(ctx context.Context, userID int64) (count int, amountCents int64, err error) {
	now := time.Now()
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	list, err := s.db.Ent().Order.Query().
		Where(
			order.UserIDEQ(userID),
			order.StatusEQ("paid"),
			order.PaidAtGTE(start),
		).
		All(ctx)
	if err != nil {
		return 0, 0, err
	}
	for _, o := range list {
		count++
		amountCents += int64(o.AmountCents)
	}
	return count, amountCents, nil
}
