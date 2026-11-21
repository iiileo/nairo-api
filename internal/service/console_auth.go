package service

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"net/mail"
	"strings"
	"time"
	"unicode/utf8"

	"nairo/api_service/ent"
	"nairo/api_service/ent/emailverification"
	"nairo/api_service/ent/refreshtoken"
	"nairo/api_service/ent/user"
	"nairo/api_service/ent/wallet"
	"nairo/api_service/internal/console"
	"nairo/api_service/internal/geetest"
	"nairo/api_service/internal/mailer"
	"nairo/api_service/internal/mysql"
	"nairo/api_service/internal/pkg/secrethash"
	"nairo/api_service/internal/runtimeconfig"
	"nairo/api_service/internal/snowflake"
)

const (
	purposeRegister     = "register"
	minPasswordLen      = 8
	sendCodeCooldownSec = 60
)

type ConsoleAuthService struct {
	db     *mysql.Client
	rt     *runtimeconfig.RuntimeConfig
	tm     *console.TokenManager
	mailer *mailer.Mailer
	users  *UserAdminService
}

func NewConsoleAuthService(
	db *mysql.Client,
	rt *runtimeconfig.RuntimeConfig,
	tm *console.TokenManager,
	mailer *mailer.Mailer,
	users *UserAdminService,
) *ConsoleAuthService {
	return &ConsoleAuthService{db: db, rt: rt, tm: tm, mailer: mailer, users: users}
}

type ConsoleSession struct {
	User *ent.User
	Pair *console.TokenPair
}

type ConsoleProfile struct {
	User         *ent.User
	BalanceCents int64
}

func (s *ConsoleAuthService) Login(ctx context.Context, login, password, userAgent string) (*ConsoleSession, error) {
	login = strings.TrimSpace(login)
	q := s.db.Ent().User.Query().Where(user.StatusEQ("active"))
	u, err := q.Where(user.Or(user.EmailEQ(login), user.PhoneEQ(login))).Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, ErrUnauthorized
		}
		return nil, err
	}
	if !secrethash.CheckPassword(u.PasswordHash, password) {
		return nil, ErrUnauthorized
	}
	return s.issueSession(ctx, u, userAgent)
}

func (s *ConsoleAuthService) SendRegisterCode(ctx context.Context, email string, proof geetest.Proof) error {
	if !s.rt.RegisterEnabled() {
		return ErrRegisterDisabled
	}
	email, err := normalizeEmail(email)
	if err != nil {
		return fmt.Errorf("%w: email", ErrInvalid)
	}
	if s.rt.GeetestEnabled() {
		g := s.rt.Geetest()
		if err := geetest.Validate(ctx, g.CaptchaID, g.CaptchaKey, proof); err != nil {
			return ErrCaptchaFailed
		}
	}
	exists, err := s.db.Ent().User.Query().Where(user.EmailEQ(email)).Exist(ctx)
	if err != nil {
		return err
	}
	if exists {
		return ErrConflict
	}
	latest, err := s.db.Ent().EmailVerification.Query().
		Where(
			emailverification.EmailEQ(email),
			emailverification.PurposeEQ(purposeRegister),
		).
		Order(ent.Desc(emailverification.FieldCreatedAt)).
		First(ctx)
	if err != nil && !ent.IsNotFound(err) {
		return err
	}
	if err == nil && time.Since(latest.CreatedAt) < sendCodeCooldownSec*time.Second {
		return ErrRateLimited
	}

	code, err := randomDigits(6)
	if err != nil {
		return err
	}
	ttl := s.rt.ConsoleCodeTTLSeconds()
	id, err := snowflake.NextID()
	if err != nil {
		return err
	}
	_, err = s.db.Ent().EmailVerification.Create().
		SetID(id).
		SetEmail(email).
		SetCodeHash(secrethash.HashSigningSecret(code)).
		SetPurpose(purposeRegister).
		SetExpiresAt(time.Now().UTC().Add(time.Duration(ttl) * time.Second)).
		Save(ctx)
	if err != nil {
		return err
	}
	subject := "Nairo 注册验证码"
	body := fmt.Sprintf("您的注册验证码是 %s，%d 分钟内有效。如非本人操作请忽略。", code, ttl/60)
	if err := s.mailer.Send(ctx, email, subject, body); err != nil {
		return fmt.Errorf("send mail: %w", err)
	}
	return nil
}

type RegisterInput struct {
	Email           string
	Code            string
	Password        string
	ConfirmPassword string
	UserAgent       string
}

func (s *ConsoleAuthService) Register(ctx context.Context, in RegisterInput) (*ConsoleSession, error) {
	if !s.rt.RegisterEnabled() {
		return nil, ErrRegisterDisabled
	}
	email, err := normalizeEmail(in.Email)
	if err != nil {
		return nil, fmt.Errorf("%w: email", ErrInvalid)
	}
	if utf8.RuneCountInString(in.Password) < minPasswordLen {
		return nil, fmt.Errorf("%w: password too short", ErrInvalid)
	}
	if in.Password != in.ConfirmPassword {
		return nil, fmt.Errorf("%w: password mismatch", ErrInvalid)
	}
	code := strings.TrimSpace(in.Code)
	if len(code) < 4 {
		return nil, ErrCodeInvalid
	}

	exists, err := s.db.Ent().User.Query().Where(user.EmailEQ(email)).Exist(ctx)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrConflict
	}

	now := time.Now().UTC()
	ver, err := s.db.Ent().EmailVerification.Query().
		Where(
			emailverification.EmailEQ(email),
			emailverification.PurposeEQ(purposeRegister),
			emailverification.UsedAtIsNil(),
			emailverification.ExpiresAtGT(now),
			emailverification.CodeHashEQ(secrethash.HashSigningSecret(code)),
		).
		Order(ent.Desc(emailverification.FieldCreatedAt)).
		First(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, ErrCodeInvalid
		}
		return nil, err
	}
	if _, err := s.db.Ent().EmailVerification.UpdateOneID(ver.ID).SetUsedAt(now).Save(ctx); err != nil {
		return nil, err
	}

	fee := s.rt.ConsoleDefaultFeeRateBps()
	if fee < 0 {
		fee = 100
	}
	created, err := s.users.CreateUser(ctx, CreateUserInput{
		Email:      email,
		Password:   in.Password,
		FeeRateBps: fee,
	})
	if err != nil {
		return nil, err
	}
	return s.issueSession(ctx, created.User, in.UserAgent)
}

func (s *ConsoleAuthService) Refresh(ctx context.Context, refreshToken, userAgent string) (*ConsoleSession, error) {
	refreshToken = strings.TrimSpace(refreshToken)
	if refreshToken == "" {
		return nil, ErrUnauthorized
	}
	claims, err := s.tm.ParseRefresh(refreshToken)
	if err != nil {
		return nil, ErrUnauthorized
	}
	hash := secrethash.HashSigningSecret(refreshToken)
	row, err := s.db.Ent().RefreshToken.Query().
		Where(refreshtoken.TokenHashEQ(hash)).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, ErrUnauthorized
		}
		return nil, err
	}
	now := time.Now().UTC()
	if row.RevokedAt != nil || row.ExpiresAt.Before(now) || row.UserID != claims.UserID {
		return nil, ErrUnauthorized
	}
	if _, err := s.db.Ent().RefreshToken.UpdateOneID(row.ID).SetRevokedAt(now).Save(ctx); err != nil {
		return nil, err
	}
	u, err := s.db.Ent().User.Get(ctx, claims.UserID)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, ErrUnauthorized
		}
		return nil, err
	}
	if u.Status != "active" {
		return nil, ErrUnauthorized
	}
	return s.issueSession(ctx, u, userAgent)
}

func (s *ConsoleAuthService) Logout(ctx context.Context, refreshToken string) error {
	refreshToken = strings.TrimSpace(refreshToken)
	if refreshToken == "" {
		return nil
	}
	hash := secrethash.HashSigningSecret(refreshToken)
	_, err := s.db.Ent().RefreshToken.Update().
		Where(
			refreshtoken.TokenHashEQ(hash),
			refreshtoken.RevokedAtIsNil(),
		).
		SetRevokedAt(time.Now().UTC()).
		Save(ctx)
	return err
}

func (s *ConsoleAuthService) TouchHeartbeat(ctx context.Context, userID int64) error {
	return s.db.Ent().User.UpdateOneID(userID).SetConsoleLastSeenAt(time.Now()).Exec(ctx)
}

func (s *ConsoleAuthService) Profile(ctx context.Context, userID int64) (*ConsoleProfile, error) {
	u, err := s.db.Ent().User.Get(ctx, userID)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	w, err := s.db.Ent().Wallet.Query().Where(wallet.UserIDEQ(userID)).Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return &ConsoleProfile{User: u, BalanceCents: 0}, nil
		}
		return nil, err
	}
	return &ConsoleProfile{User: u, BalanceCents: w.BalanceCents}, nil
}

func (s *ConsoleAuthService) issueSession(ctx context.Context, u *ent.User, userAgent string) (*ConsoleSession, error) {
	pair, err := s.tm.IssuePair(u.ID)
	if err != nil {
		return nil, err
	}
	id, err := snowflake.NextID()
	if err != nil {
		return nil, err
	}
	ua := userAgent
	if utf8.RuneCountInString(ua) > 255 {
		ua = string([]rune(ua)[:255])
	}
	_, err = s.db.Ent().RefreshToken.Create().
		SetID(id).
		SetUserID(u.ID).
		SetTokenHash(secrethash.HashSigningSecret(pair.RefreshToken)).
		SetExpiresAt(pair.RefreshExpiresAt).
		SetUserAgent(ua).
		Save(ctx)
	if err != nil {
		return nil, err
	}
	return &ConsoleSession{User: u, Pair: pair}, nil
}

func normalizeEmail(email string) (string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return "", fmt.Errorf("empty")
	}
	addr, err := mail.ParseAddress(email)
	if err != nil {
		return "", err
	}
	return strings.ToLower(addr.Address), nil
}

func randomDigits(n int) (string, error) {
	var b strings.Builder
	b.Grow(n)
	for i := 0; i < n; i++ {
		v, err := rand.Int(rand.Reader, big.NewInt(10))
		if err != nil {
			return "", err
		}
		b.WriteByte(byte('0' + v.Int64()))
	}
	return b.String(), nil
}
