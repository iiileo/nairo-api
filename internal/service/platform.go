package service

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"nairo/api_service/ent"
	"nairo/api_service/ent/platformsetting"
	"nairo/api_service/ent/user"
	"nairo/api_service/internal/mysql"
	"nairo/api_service/internal/snowflake"
)

const SettingPlatformPayUserID = "platform_pay_user_id"

type PlatformService struct {
	db *mysql.Client
}

func NewPlatformService(db *mysql.Client) *PlatformService {
	return &PlatformService{db: db}
}

func (s *PlatformService) GetPlatformPayUserID(ctx context.Context) (int64, error) {
	row, err := s.db.Ent().PlatformSetting.Query().
		Where(platformsetting.KeyEQ(SettingPlatformPayUserID)).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return 0, nil
		}
		return 0, err
	}
	v := strings.TrimSpace(row.Value)
	if v == "" {
		return 0, nil
	}
	id, err := strconv.ParseInt(v, 10, 64)
	if err != nil || id <= 0 {
		return 0, nil
	}
	return id, nil
}

func (s *PlatformService) GetPlatformPayUser(ctx context.Context) (*ent.User, error) {
	id, err := s.GetPlatformPayUserID(ctx)
	if err != nil {
		return nil, err
	}
	if id == 0 {
		return nil, fmt.Errorf("%w: platform_pay_user unset", ErrInvalid)
	}
	u, err := s.db.Ent().User.Get(ctx, id)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, fmt.Errorf("%w: platform_pay_user missing", ErrNotFound)
		}
		return nil, err
	}
	if u.Status != "active" {
		return nil, fmt.Errorf("%w: platform_pay_user disabled", ErrInvalid)
	}
	return u, nil
}

type PlatformSettingsView struct {
	PayUserID    string     `json:"pay_user_id"`
	PayUser      *ent.User  `json:"-"`
	PayUserEmail *string    `json:"pay_user_email,omitempty"`
	PayUserPhone *string    `json:"pay_user_phone,omitempty"`
	PayUserOK    bool       `json:"pay_user_ok"`
}

func (s *PlatformService) GetSettings(ctx context.Context) (*PlatformSettingsView, error) {
	id, err := s.GetPlatformPayUserID(ctx)
	if err != nil {
		return nil, err
	}
	view := &PlatformSettingsView{}
	if id == 0 {
		return view, nil
	}
	view.PayUserID = fmt.Sprintf("%d", id)
	u, err := s.db.Ent().User.Get(ctx, id)
	if err != nil {
		if ent.IsNotFound(err) {
			return view, nil
		}
		return nil, err
	}
	view.PayUser = u
	view.PayUserEmail = u.Email
	view.PayUserPhone = u.Phone
	view.PayUserOK = u.Status == "active"
	return view, nil
}

func (s *PlatformService) SetPlatformPayUserID(ctx context.Context, userID int64) error {
	if userID < 0 {
		return fmt.Errorf("%w: user_id", ErrInvalid)
	}
	if userID > 0 {
		u, err := s.db.Ent().User.Query().Where(user.IDEQ(userID)).Only(ctx)
		if err != nil {
			if ent.IsNotFound(err) {
				return ErrNotFound
			}
			return err
		}
		if u.Status != "active" {
			return fmt.Errorf("%w: user disabled", ErrInvalid)
		}
	}
	val := ""
	if userID > 0 {
		val = strconv.FormatInt(userID, 10)
	}
	row, err := s.db.Ent().PlatformSetting.Query().
		Where(platformsetting.KeyEQ(SettingPlatformPayUserID)).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			id, err := snowflake.NextID()
			if err != nil {
				return err
			}
			_, err = s.db.Ent().PlatformSetting.Create().
				SetID(id).
				SetKey(SettingPlatformPayUserID).
				SetValue(val).
				Save(ctx)
			return err
		}
		return err
	}
	_, err = s.db.Ent().PlatformSetting.UpdateOneID(row.ID).SetValue(val).Save(ctx)
	return err
}
