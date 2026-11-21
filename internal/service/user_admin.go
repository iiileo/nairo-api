package service

import (
	"context"
	"fmt"
	"math"
	"strings"

	"nairo/api_service/ent"
	"nairo/api_service/ent/user"
	"nairo/api_service/ent/wallet"
	"nairo/api_service/internal/mysql"
	"nairo/api_service/internal/pkg/secretbox"
	"nairo/api_service/internal/pkg/secrethash"
	"nairo/api_service/internal/snowflake"
)

type UserAdminService struct {
	db    *mysql.Client
	vault *secretbox.Vault
}

func NewUserAdminService(db *mysql.Client, vault *secretbox.Vault) *UserAdminService {
	return &UserAdminService{db: db, vault: vault}
}

type CreateUserInput struct {
	Email    string
	Phone    string
	Password string
	FeeRateBps int
}

type CreateUserResult struct {
	User      *ent.User
	AccessKey string
	APISecret string // 仅创建时回显
}

func (s *UserAdminService) CreateUser(ctx context.Context, in CreateUserInput) (*CreateUserResult, error) {
	if strings.TrimSpace(in.Password) == "" {
		return nil, fmt.Errorf("%w: password required", ErrInvalid)
	}
	if in.FeeRateBps < 0 {
		in.FeeRateBps = 100
	}
	passHash, err := secrethash.HashPassword(in.Password)
	if err != nil {
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
	userID, err := snowflake.NextID()
	if err != nil {
		return nil, err
	}
	credID, err := snowflake.NextID()
	if err != nil {
		return nil, err
	}
	walletID, err := snowflake.NextID()
	if err != nil {
		return nil, err
	}

	var out *CreateUserResult
	err = s.db.WithTx(ctx, func(tx *ent.Tx) error {
		uc := tx.User.Create().
			SetID(userID).
			SetPasswordHash(passHash).
			SetStatus("active").
			SetFeeRateBps(in.FeeRateBps).
			SetAdjustEnabled(true).
			SetAdjustDirection("inc").
			SetAdjustStepCents(1).
			SetAdjustMaxOffsetCents(50)
		if e := strings.TrimSpace(in.Email); e != "" {
			uc.SetEmail(e)
		}
		if p := strings.TrimSpace(in.Phone); p != "" {
			uc.SetPhone(p)
		}
		u, err := uc.Save(ctx)
		if err != nil {
			return err
		}
		storedSecret, sealErr := s.vault.Seal(apiSecret)
		if sealErr != nil {
			return sealErr
		}
		_, err = tx.ApiCredential.Create().
			SetID(credID).
			SetUserID(u.ID).
			SetAccessKey(accessKey).
			SetSecretHash(storedSecret).
			Save(ctx)
		if err != nil {
			return err
		}
		_, err = tx.Wallet.Create().
			SetID(walletID).
			SetUserID(u.ID).
			SetBalanceCents(0).
			Save(ctx)
		if err != nil {
			return err
		}
		out = &CreateUserResult{User: u, AccessKey: accessKey, APISecret: apiSecret}
		return nil
	})
	return out, err
}

func (s *UserAdminService) SetFeeRate(ctx context.Context, userID int64, bps int) (*ent.User, error) {
	if bps < 0 {
		return nil, fmt.Errorf("%w: fee_rate_bps", ErrInvalid)
	}
	return s.db.Ent().User.UpdateOneID(userID).SetFeeRateBps(bps).Save(ctx)
}

type UpdateUserInput struct {
	Email    *string
	Phone    *string
	Password *string
	Status   *string
}

func (s *UserAdminService) UpdateUser(ctx context.Context, userID int64, in UpdateUserInput) (*ent.User, error) {
	u, err := s.db.Ent().User.Get(ctx, userID)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	upd := s.db.Ent().User.UpdateOneID(userID)
	changed := false
	if in.Email != nil {
		e := strings.ToLower(strings.TrimSpace(*in.Email))
		if e == "" {
			upd.ClearEmail()
		} else {
			exists, err := s.db.Ent().User.Query().
				Where(user.EmailEQ(e), user.IDNEQ(userID)).
				Exist(ctx)
			if err != nil {
				return nil, err
			}
			if exists {
				return nil, ErrConflict
			}
			upd.SetEmail(e)
		}
		changed = true
	}
	if in.Phone != nil {
		p := strings.TrimSpace(*in.Phone)
		if p == "" {
			upd.ClearPhone()
		} else {
			upd.SetPhone(p)
		}
		changed = true
	}
	if in.Password != nil {
		pass := strings.TrimSpace(*in.Password)
		if len(pass) < 6 {
			return nil, fmt.Errorf("%w: password too short", ErrInvalid)
		}
		hash, err := secrethash.HashPassword(pass)
		if err != nil {
			return nil, err
		}
		upd.SetPasswordHash(hash)
		changed = true
	}
	if in.Status != nil {
		st := strings.ToLower(strings.TrimSpace(*in.Status))
		if st != "active" && st != "disabled" && st != "pending" {
			return nil, fmt.Errorf("%w: status", ErrInvalid)
		}
		upd.SetStatus(st)
		changed = true
	}
	if !changed {
		return u, nil
	}
	return upd.Save(ctx)
}

func (s *UserAdminService) SetPassword(ctx context.Context, userID int64, password string) (*ent.User, error) {
	return s.UpdateUser(ctx, userID, UpdateUserInput{Password: &password})
}

func (s *UserAdminService) Recharge(ctx context.Context, userID, adminID, amountCents int64, remark string) (*ent.Wallet, error) {
	if amountCents <= 0 {
		return nil, fmt.Errorf("%w: amount", ErrInvalid)
	}
	var w *ent.Wallet
	err := s.db.WithTx(ctx, func(tx *ent.Tx) error {
		cur, err := tx.Wallet.Query().Where(wallet.UserIDEQ(userID)).Only(ctx)
		if err != nil {
			return err
		}
		newBal := cur.BalanceCents + amountCents
		cur, err = tx.Wallet.UpdateOneID(cur.ID).SetBalanceCents(newBal).Save(ctx)
		if err != nil {
			return err
		}
		lid, err := snowflake.NextID()
		if err != nil {
			return err
		}
		_, err = tx.WalletLedger.Create().
			SetID(lid).
			SetUserID(userID).
			SetType("recharge").
			SetAmountCents(amountCents).
			SetBalanceAfterCents(newBal).
			SetAdminID(adminID).
			SetRemark(remark).
			Save(ctx)
		if err != nil {
			return err
		}
		w = cur
		return nil
	})
	return w, err
}

func (s *UserAdminService) ListUsers(ctx context.Context, limit, offset int) ([]*ent.User, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	return s.db.Ent().User.Query().Order(ent.Desc(user.FieldID)).Limit(limit).Offset(offset).All(ctx)
}

func FeeCents(amountCents int, feeRateBps int) int64 {
	if amountCents <= 0 || feeRateBps <= 0 {
		return 0
	}
	return int64(math.Ceil(float64(amountCents) * float64(feeRateBps) / 10000.0))
}
