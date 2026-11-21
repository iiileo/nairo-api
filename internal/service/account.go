package service

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"nairo/api_service/ent"
	"nairo/api_service/ent/paymentaccount"
	"nairo/api_service/ent/paycode"
	"nairo/api_service/internal/mysql"
	"nairo/api_service/internal/pkg/channels"
	"nairo/api_service/internal/pkg/secretbox"
	"nairo/api_service/internal/pkg/secrethash"
	"nairo/api_service/internal/pkg/sign"
	"nairo/api_service/internal/runtimeconfig"
	"nairo/api_service/internal/snowflake"
)

type AccountService struct {
	db    *mysql.Client
	rt    *runtimeconfig.RuntimeConfig
	vault *secretbox.Vault
}

func NewAccountService(db *mysql.Client, rt *runtimeconfig.RuntimeConfig, vault *secretbox.Vault) *AccountService {
	return &AccountService{db: db, rt: rt, vault: vault}
}

type CreateAccountInput struct {
	UserID   int64
	Channels []string
	// Channel 兼容旧入参（单渠道）；与 Channels 二选一
	Channel string
	Name    string
}

// AppBindConfig App 扫码导入用的完整配置（含密钥，仅创建/重置时下发一次）。
type AppBindConfig struct {
	V              int      `json:"v"`
	Name           string   `json:"name"`
	BaseURL        string   `json:"base_url"`
	PushURL        string   `json:"push_url"`
	HeartbeatURL   string   `json:"heartbeat_url"`
	HeartbeatWSURL string   `json:"heartbeat_ws_url"`
	CallbackID     string   `json:"callback_id"`
	Secret         string   `json:"secret"`
	Channels       []string `json:"channels"`
}

type CreateAccountResult struct {
	Account     *ent.PaymentAccount
	AppSecret   string
	CallbackURL string
	Channels    []string
	Bind        AppBindConfig
}

func (s *AccountService) CreateAccount(ctx context.Context, in CreateAccountInput) (*CreateAccountResult, error) {
	chs := in.Channels
	if len(chs) == 0 && strings.TrimSpace(in.Channel) != "" {
		chs = []string{in.Channel}
	}
	stored, list, err := channels.Normalize(chs)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalid, err)
	}
	secret, err := secrethash.RandomToken(24)
	if err != nil {
		return nil, err
	}
	storedSecret, err := s.vault.Seal(secret)
	if err != nil {
		return nil, err
	}
	callbackID, err := secrethash.RandomToken(6)
	if err != nil {
		return nil, err
	}
	id, err := snowflake.NextID()
	if err != nil {
		return nil, err
	}
	name := strings.TrimSpace(in.Name)
	a, err := s.db.Ent().PaymentAccount.Create().
		SetID(id).
		SetUserID(in.UserID).
		SetChannel(stored).
		SetName(name).
		SetCallbackID(callbackID).
		SetAppSecretHash(storedSecret).
		SetStatus("active").
		Save(ctx)
	if err != nil {
		return nil, err
	}
	bind := s.buildBind(a, secret, list)
	return &CreateAccountResult{
		Account:     a,
		AppSecret:   secret,
		CallbackURL: bind.PushURL,
		Channels:    list,
		Bind:        bind,
	}, nil
}

const payCodeDeleted = "deleted"

// payCodeMu 串行化收款码的唯一性检查与创建，避免并发重复添加。
var payCodeMu sync.Mutex

type CreatePayCodeInput struct {
	AccountID   int64
	Channel     string // wechat|alipay
	CodeType    string // float|fixed
	AmountCents *int
	ImageURL    string
	Payload     string
}

func (s *AccountService) CreatePayCode(ctx context.Context, in CreatePayCodeInput) (*ent.PayCode, error) {
	return s.CreatePayCodeForUser(ctx, 0, in)
}

func (s *AccountService) CreatePayCodeForUser(ctx context.Context, userID int64, in CreatePayCodeInput) (*ent.PayCode, error) {
	ct := strings.ToLower(strings.TrimSpace(in.CodeType))
	if ct == "" {
		ct = "float"
	}
	if ct != "float" && ct != "fixed" {
		return nil, fmt.Errorf("%w: code_type", ErrInvalid)
	}
	if ct == "fixed" && (in.AmountCents == nil || *in.AmountCents <= 0) {
		return nil, fmt.Errorf("%w: amount_cents required for fixed", ErrInvalid)
	}
	ch := strings.ToLower(strings.TrimSpace(in.Channel))
	if ch != "wechat" && ch != "alipay" {
		return nil, fmt.Errorf("%w: channel", ErrInvalid)
	}
	payload := strings.TrimSpace(in.Payload)
	imageURL := strings.TrimSpace(in.ImageURL)
	if payload == "" && imageURL == "" {
		return nil, fmt.Errorf("%w: payload required", ErrInvalid)
	}
	acc, err := s.db.Ent().PaymentAccount.Get(ctx, in.AccountID)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if userID > 0 && acc.UserID != userID {
		return nil, ErrNotFound
	}
	if !channels.Supports(acc.Channel, ch) {
		return nil, fmt.Errorf("%w: channel not enabled on device", ErrInvalid)
	}
	// 同设备同渠道：任意金额码仅 1 个；固额码同金额仅 1 个。
	payCodeMu.Lock()
	defer payCodeMu.Unlock()
	dup := s.db.Ent().PayCode.Query().Where(
		paycode.AccountIDEQ(in.AccountID),
		paycode.ChannelEQ(ch),
		paycode.CodeTypeEQ(ct),
		paycode.StatusNEQ(payCodeDeleted),
	)
	if ct == "fixed" {
		dup = dup.Where(paycode.AmountCentsEQ(*in.AmountCents))
	}
	exists, err := dup.Exist(ctx)
	if err != nil {
		return nil, err
	}
	if exists {
		if ct == "fixed" {
			return nil, fmt.Errorf("%w: pay_code_fixed_exists", ErrConflict)
		}
		return nil, fmt.Errorf("%w: pay_code_float_exists", ErrConflict)
	}
	id, err := snowflake.NextID()
	if err != nil {
		return nil, err
	}
	c := s.db.Ent().PayCode.Create().
		SetID(id).
		SetAccountID(in.AccountID).
		SetChannel(ch).
		SetCodeType(ct).
		SetImageURL(imageURL).
		SetStatus("active")
	if ct == "fixed" {
		c.SetAmountCents(*in.AmountCents)
	}
	if payload != "" {
		c.SetPayload(payload)
	}
	return c.Save(ctx)
}

type AppHeartbeatInput struct {
	CallbackID string
	T          int64
	Nonce      string
	Sign       string
	ConnType   string
	DeviceID   string // 手机安装 ID；已被手机绑定的设备必须匹配
}

// TouchHeartbeat 校验签名并刷新在线状态；成功时返回设备（供心跳响应附带最新配置）。
func (s *AccountService) TouchHeartbeat(ctx context.Context, in AppHeartbeatInput) (*ent.PaymentAccount, error) {
	in.CallbackID = strings.TrimSpace(in.CallbackID)
	in.Nonce = strings.TrimSpace(in.Nonce)
	in.Sign = strings.TrimSpace(in.Sign)
	if in.CallbackID == "" || in.Nonce == "" || in.Sign == "" || in.T <= 0 {
		return nil, fmt.Errorf("%w: heartbeat fields", ErrInvalid)
	}
	if !sign.WithinSkew(in.T, 5*time.Minute) {
		return nil, fmt.Errorf("%w: timestamp", ErrInvalid)
	}
	acc, err := s.db.Ent().PaymentAccount.Query().
		Where(paymentaccount.CallbackIDEQ(in.CallbackID), paymentaccount.StatusEQ("active")).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	plain, err := s.vault.Open(acc.AppSecretHash)
	if err != nil {
		return nil, err
	}
	expect := sign.AppHeartbeatSign(in.CallbackID, in.T, in.Nonce, plain)
	if !strings.EqualFold(expect, in.Sign) {
		return nil, ErrUnauthorized
	}
	if err := checkBoundDevice(ctx, s.db, acc, in.DeviceID); err != nil {
		return acc, err
	}
	ct := normalizeAppConnType(in.ConnType)
	now := time.Now()
	if err := s.db.Ent().PaymentAccount.UpdateOneID(acc.ID).
		SetAppLastSeenAt(now).
		SetAppConnType(ct).
		Exec(ctx); err != nil {
		return nil, err
	}
	return acc, nil
}

// checkBoundDevice 校验请求所属手机：
//   - 设备已被手机绑定：device_id 必须一致，否则 ErrBoundElsewhere；
//   - 设备尚未绑定且请求带 device_id：自动认领（兼容升级前已在使用的设备）；
//   - 设备尚未绑定且无 device_id（旧版 App）：放行。
func checkBoundDevice(ctx context.Context, db *mysql.Client, acc *ent.PaymentAccount, deviceID string) error {
	deviceID = strings.TrimSpace(deviceID)
	if acc.BoundDeviceID != "" {
		if acc.BoundDeviceID == deviceID {
			return nil
		}
		return ErrBoundElsewhere
	}
	if deviceID == "" {
		return nil
	}
	n, err := db.Ent().PaymentAccount.Update().
		Where(paymentaccount.IDEQ(acc.ID), paymentaccount.BoundDeviceIDEQ("")).
		SetBoundDeviceID(deviceID).
		SetBoundAt(time.Now()).
		Save(ctx)
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrBoundElsewhere
	}
	return nil
}

type AppBindInput struct {
	CallbackID string
	T          int64
	Nonce      string
	Sign       string
	DeviceID   string
	DeviceName string
}

// authBind 校验手机绑定/解绑请求的签名，返回设备及明文密钥校验结果。
func (s *AccountService) authBind(ctx context.Context, in *AppBindInput) (*ent.PaymentAccount, error) {
	in.CallbackID = strings.TrimSpace(in.CallbackID)
	in.Nonce = strings.TrimSpace(in.Nonce)
	in.Sign = strings.TrimSpace(in.Sign)
	in.DeviceID = strings.TrimSpace(in.DeviceID)
	in.DeviceName = strings.TrimSpace(in.DeviceName)
	if len([]rune(in.DeviceName)) > 40 {
		in.DeviceName = string([]rune(in.DeviceName)[:40])
	}
	if in.CallbackID == "" || in.Nonce == "" || in.Sign == "" || in.T <= 0 || in.DeviceID == "" || len(in.DeviceID) > 64 {
		return nil, fmt.Errorf("%w: bind fields", ErrInvalid)
	}
	if !sign.WithinSkew(in.T, 5*time.Minute) {
		return nil, fmt.Errorf("%w: timestamp", ErrInvalid)
	}
	acc, err := s.db.Ent().PaymentAccount.Query().
		Where(paymentaccount.CallbackIDEQ(in.CallbackID), paymentaccount.StatusEQ("active")).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	plain, err := s.vault.Open(acc.AppSecretHash)
	if err != nil {
		return nil, err
	}
	expect := sign.AppBindSign(in.CallbackID, in.T, in.Nonce, in.DeviceID, plain)
	if !strings.EqualFold(expect, in.Sign) {
		return nil, ErrUnauthorized
	}
	return acc, nil
}

// BindDevice 手机绑定监控设备：已被其他手机绑定时返回 ErrBoundElsewhere（并带回设备以便展示占用者型号）。
func (s *AccountService) BindDevice(ctx context.Context, in AppBindInput) (*ent.PaymentAccount, error) {
	acc, err := s.authBind(ctx, &in)
	if err != nil {
		return nil, err
	}
	n, err := s.db.Ent().PaymentAccount.Update().
		Where(
			paymentaccount.IDEQ(acc.ID),
			paymentaccount.Or(paymentaccount.BoundDeviceIDEQ(""), paymentaccount.BoundDeviceIDEQ(in.DeviceID)),
		).
		SetBoundDeviceID(in.DeviceID).
		SetBoundDeviceName(in.DeviceName).
		SetBoundAt(time.Now()).
		Save(ctx)
	if err != nil {
		return nil, err
	}
	if n == 0 {
		// MySQL 在值未变化时也返回 0 行：本机重复绑定同一设备视为成功。
		cur, gerr := s.db.Ent().PaymentAccount.Get(ctx, acc.ID)
		if gerr != nil {
			return nil, gerr
		}
		if cur.BoundDeviceID == in.DeviceID {
			return cur, nil
		}
		return cur, ErrBoundElsewhere
	}
	return acc, nil
}

// UnbindDevice 手机主动解除绑定（仅绑定者本人可操作；未绑定时幂等成功）。
func (s *AccountService) UnbindDevice(ctx context.Context, in AppBindInput) error {
	acc, err := s.authBind(ctx, &in)
	if err != nil {
		return err
	}
	if acc.BoundDeviceID != "" && acc.BoundDeviceID != in.DeviceID {
		return ErrBoundElsewhere
	}
	return s.db.Ent().PaymentAccount.UpdateOneID(acc.ID).
		SetBoundDeviceID("").
		SetBoundDeviceName("").
		ClearBoundAt().
		Exec(ctx)
}

// ReleaseBoundDevice 商户在控制台强制释放手机绑定（手机丢失/卸载后使用）。
func (s *AccountService) ReleaseBoundDevice(ctx context.Context, userID, accountID int64) error {
	acc, err := s.db.Ent().PaymentAccount.Get(ctx, accountID)
	if err != nil {
		if ent.IsNotFound(err) {
			return ErrNotFound
		}
		return err
	}
	if acc.UserID != userID {
		return ErrNotFound
	}
	return s.db.Ent().PaymentAccount.UpdateOneID(accountID).
		SetBoundDeviceID("").
		SetBoundDeviceName("").
		ClearBoundAt().
		Exec(ctx)
}

func normalizeAppConnType(raw string) string {
	ct := strings.ToLower(strings.TrimSpace(raw))
	switch ct {
	case "ws", "websocket":
		return "ws"
	case "tcp":
		return "tcp"
	default:
		return "http"
	}
}

func (s *AccountService) ListAccounts(ctx context.Context, userID int64) ([]*ent.PaymentAccount, error) {
	return s.db.Ent().PaymentAccount.Query().
		Where(
			paymentaccount.UserIDEQ(userID),
			paymentaccount.StatusEQ("active"),
		).
		Order(ent.Desc(paymentaccount.FieldID)).
		All(ctx)
}

func (s *AccountService) ListPayCodes(ctx context.Context, accountID int64) ([]*ent.PayCode, error) {
	return s.ListPayCodesForUser(ctx, 0, accountID)
}

func (s *AccountService) ListPayCodesForUser(ctx context.Context, userID, accountID int64) ([]*ent.PayCode, error) {
	if userID > 0 {
		acc, err := s.db.Ent().PaymentAccount.Get(ctx, accountID)
		if err != nil {
			if ent.IsNotFound(err) {
				return nil, ErrNotFound
			}
			return nil, err
		}
		if acc.UserID != userID {
			return nil, ErrNotFound
		}
	}
	return s.db.Ent().PayCode.Query().
		Where(paycode.AccountIDEQ(accountID), paycode.StatusNEQ(payCodeDeleted)).
		Order(ent.Desc(paycode.FieldID)).
		All(ctx)
}

// DeletePayCodeForUser 删除收款码（软删除：保留历史订单引用，不再参与分配与展示）。
// userID>0 时校验设备归属。
func (s *AccountService) DeletePayCodeForUser(ctx context.Context, userID, codeID int64) error {
	code, err := s.db.Ent().PayCode.Get(ctx, codeID)
	if err != nil {
		if ent.IsNotFound(err) {
			return ErrNotFound
		}
		return err
	}
	if code.Status == payCodeDeleted {
		return ErrNotFound
	}
	if userID > 0 {
		acc, err := s.db.Ent().PaymentAccount.Get(ctx, code.AccountID)
		if err != nil {
			if ent.IsNotFound(err) {
				return ErrNotFound
			}
			return err
		}
		if acc.UserID != userID {
			return ErrNotFound
		}
	}
	return s.db.Ent().PayCode.UpdateOneID(codeID).SetStatus(payCodeDeleted).Exec(ctx)
}

func (s *AccountService) CallbackURL(callbackID string) string {
	base := s.rt.PayPublicBaseURL()
	return fmt.Sprintf("%s/app/push/%s", base, callbackID)
}

func (s *AccountService) HeartbeatURL() string {
	base := s.rt.PayPublicBaseURL()
	return fmt.Sprintf("%s/app/heartbeat", base)
}

func (s *AccountService) HeartbeatWSURL() string {
	base := s.rt.PayPublicBaseURL()
	switch {
	case strings.HasPrefix(base, "https://"):
		base = "wss://" + strings.TrimPrefix(base, "https://")
	case strings.HasPrefix(base, "http://"):
		base = "ws://" + strings.TrimPrefix(base, "http://")
	}
	return fmt.Sprintf("%s/app/heartbeat/ws", base)
}

func (s *AccountService) BaseURL() string {
	return s.rt.PayPublicBaseURL()
}

func (s *AccountService) buildBind(acc *ent.PaymentAccount, secret string, list []string) AppBindConfig {
	if list == nil {
		list = channels.Parse(acc.Channel)
	}
	name := acc.Name
	if name == "" {
		name = channels.Label(acc.Channel)
	}
	return AppBindConfig{
		V:              1,
		Name:           name,
		BaseURL:        s.BaseURL(),
		PushURL:        s.CallbackURL(acc.CallbackID),
		HeartbeatURL:   s.HeartbeatURL(),
		HeartbeatWSURL: s.HeartbeatWSURL(),
		CallbackID:     acc.CallbackID,
		Secret:         secret,
		Channels:       list,
	}
}

// GetBindConfig 返回当前密钥对应的扫码配置（不轮换密钥），用于在不重置的情况下再次扫码添加 App。
func (s *AccountService) GetBindConfig(ctx context.Context, userID, accountID int64) (AppBindConfig, error) {
	acc, err := s.db.Ent().PaymentAccount.Get(ctx, accountID)
	if err != nil {
		if ent.IsNotFound(err) {
			return AppBindConfig{}, ErrNotFound
		}
		return AppBindConfig{}, err
	}
	if acc.UserID != userID || acc.Status == "disabled" {
		return AppBindConfig{}, ErrNotFound
	}
	secret, err := s.vault.Open(acc.AppSecretHash)
	if err != nil {
		return AppBindConfig{}, err
	}
	return s.buildBind(acc, secret, nil), nil
}

func (s *AccountService) ResetAppSecret(ctx context.Context, userID, accountID int64) (string, *ent.PaymentAccount, AppBindConfig, error) {
	acc, err := s.db.Ent().PaymentAccount.Get(ctx, accountID)
	if err != nil {
		if ent.IsNotFound(err) {
			return "", nil, AppBindConfig{}, ErrNotFound
		}
		return "", nil, AppBindConfig{}, err
	}
	if acc.UserID != userID {
		return "", nil, AppBindConfig{}, ErrNotFound
	}
	secret, err := secrethash.RandomToken(24)
	if err != nil {
		return "", nil, AppBindConfig{}, err
	}
	storedSecret, err := s.vault.Seal(secret)
	if err != nil {
		return "", nil, AppBindConfig{}, err
	}
	acc, err = s.db.Ent().PaymentAccount.UpdateOneID(accountID).
		SetAppSecretHash(storedSecret).
		SetBoundDeviceID("").
		SetBoundDeviceName("").
		ClearBoundAt().
		Save(ctx)
	if err != nil {
		return "", nil, AppBindConfig{}, err
	}
	bind := s.buildBind(acc, secret, nil)
	return secret, acc, bind, nil
}

func (s *AccountService) UpdateChannels(ctx context.Context, userID, accountID int64, chs []string) (*ent.PaymentAccount, error) {
	stored, _, err := channels.Normalize(chs)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalid, err)
	}
	acc, err := s.db.Ent().PaymentAccount.Get(ctx, accountID)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if acc.UserID != userID {
		return nil, ErrNotFound
	}
	return s.db.Ent().PaymentAccount.UpdateOneID(accountID).SetChannel(stored).Save(ctx)
}

// DeleteAccount 软删除监控设备：停用并轮换 callback，旧 App 立即失效。
func (s *AccountService) DeleteAccount(ctx context.Context, userID, accountID int64) error {
	acc, err := s.db.Ent().PaymentAccount.Get(ctx, accountID)
	if err != nil {
		if ent.IsNotFound(err) {
			return ErrNotFound
		}
		return err
	}
	if acc.UserID != userID {
		return ErrNotFound
	}
	if acc.Status == "disabled" {
		return nil
	}
	newCB, err := secrethash.RandomToken(6)
	if err != nil {
		return err
	}
	newSecret, err := secrethash.RandomToken(24)
	if err != nil {
		return err
	}
	storedSecret, err := s.vault.Seal(newSecret)
	if err != nil {
		return err
	}
	return s.db.Ent().PaymentAccount.UpdateOneID(accountID).
		SetStatus("disabled").
		SetCallbackID(newCB).
		SetAppSecretHash(storedSecret).
		SetName(strings.TrimSpace(acc.Name + " (已删除)")).
		Exec(ctx)
}

func (s *AccountService) CountOnlineAccounts(ctx context.Context, userID int64, within time.Duration) (int, error) {
	if within <= 0 {
		within = AppOnlineWindow
	}
	since := time.Now().Add(-within)
	return s.db.Ent().PaymentAccount.Query().
		Where(
			paymentaccount.UserIDEQ(userID),
			paymentaccount.StatusEQ("active"),
			paymentaccount.AppLastSeenAtGTE(since),
		).
		Count(ctx)
}

func FilterAccountsByChannel(list []*ent.PaymentAccount, channel string) []*ent.PaymentAccount {
	out := make([]*ent.PaymentAccount, 0, len(list))
	for _, a := range list {
		if channels.Supports(a.Channel, channel) {
			out = append(out, a)
		}
	}
	return out
}
