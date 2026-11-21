package runtimeconfig

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"nairo/api_service/ent"
	"nairo/api_service/ent/platformsetting"
	"nairo/api_service/internal/config"
	"nairo/api_service/internal/logger"
	"nairo/api_service/internal/mysql"
	"nairo/api_service/internal/snowflake"

	"go.uber.org/fx"
)

const SettingRuntimeConfig = "runtime_config"

// RuntimeSMTP 可后台热更新的 SMTP。
type RuntimeSMTP struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	User     string `json:"user"`
	Password string `json:"password"`
	From     string `json:"from"`
	DevPrint bool   `json:"dev_print"`
}

// RuntimePay 可后台热更新的支付公开配置。
type RuntimePay struct {
	PublicBaseURL  string `json:"public_base_url"`
	LockTTLSeconds int64  `json:"lock_ttl_seconds"`
}

// RuntimeConsole 可后台热更新的 Console 业务参数。
type RuntimeConsole struct {
	DefaultFeeRateBps int   `json:"default_fee_rate_bps"`
	CodeTTLSeconds    int64 `json:"code_ttl_seconds"`
	// RegisterEnabled 用指针区分「旧 DB 未写入」与显式关闭。
	RegisterEnabled *bool `json:"register_enabled,omitempty"`
}

// RuntimeGeetest 极验 GT4；Key 仅服务端使用。
type RuntimeGeetest struct {
	CaptchaID  string `json:"captcha_id"`
	CaptchaKey string `json:"captcha_key"`
}

type RuntimeSnapshot struct {
	SMTP    RuntimeSMTP    `json:"smtp"`
	Pay     RuntimePay     `json:"pay"`
	Console RuntimeConsole `json:"console"`
	Geetest RuntimeGeetest `json:"geetest"`
}

// RuntimeConfigView 给管理后台：密码不明文回传。
type RuntimeConfigView struct {
	SMTP struct {
		Host        string `json:"host"`
		Port        int    `json:"port"`
		User        string `json:"user"`
		From        string `json:"from"`
		DevPrint    bool   `json:"dev_print"`
		PasswordSet bool   `json:"password_set"`
	} `json:"smtp"`
	Pay     RuntimePay `json:"pay"`
	Console struct {
		DefaultFeeRateBps int   `json:"default_fee_rate_bps"`
		CodeTTLSeconds    int64 `json:"code_ttl_seconds"`
		RegisterEnabled   bool  `json:"register_enabled"`
	} `json:"console"`
	Geetest struct {
		CaptchaID string `json:"captcha_id"`
		KeySet    bool   `json:"key_set"`
	} `json:"geetest"`
	Source string `json:"source"` // env | db
}

type RuntimeConfigPatch struct {
	SMTP *struct {
		Host     *string `json:"host"`
		Port     *int    `json:"port"`
		User     *string `json:"user"`
		Password *string `json:"password"` // 空字符串表示不改；有值则更新
		From     *string `json:"from"`
		DevPrint *bool   `json:"dev_print"`
	} `json:"smtp"`
	Pay *struct {
		PublicBaseURL  *string `json:"public_base_url"`
		LockTTLSeconds *int64  `json:"lock_ttl_seconds"`
	} `json:"pay"`
	Console *struct {
		DefaultFeeRateBps *int   `json:"default_fee_rate_bps"`
		CodeTTLSeconds    *int64 `json:"code_ttl_seconds"`
		RegisterEnabled   *bool  `json:"register_enabled"`
	} `json:"console"`
	Geetest *struct {
		CaptchaID  *string `json:"captcha_id"`
		CaptchaKey *string `json:"captcha_key"` // 空字符串表示不改
	} `json:"geetest"`
}

// RuntimeConfig 环境默认 + DB 覆盖，支持 Reload。
type RuntimeConfig struct {
	db  *mysql.Client
	cfg *config.Config
	log *logger.Logger

	mu     sync.RWMutex
	snap   RuntimeSnapshot
	source string
}

func NewRuntimeConfig(db *mysql.Client, cfg *config.Config, log *logger.Logger) *RuntimeConfig {
	rt := &RuntimeConfig{db: db, cfg: cfg, log: log, source: "env"}
	rt.snap = rt.fromEnv()
	return rt
}

func (r *RuntimeConfig) fromEnv() RuntimeSnapshot {
	c := r.cfg
	port := c.SMTP.Port
	if port <= 0 {
		port = 587
	}
	lock := c.Pay.LockTTLSeconds
	if lock <= 0 {
		lock = 300
	}
	fee := c.Console.DefaultFeeRateBps
	if fee < 0 {
		fee = 100
	}
	codeTTL := c.Console.CodeTTLSeconds
	if codeTTL <= 0 {
		codeTTL = 600
	}
	return RuntimeSnapshot{
		SMTP: RuntimeSMTP{
			Host:     c.SMTP.Host,
			Port:     port,
			User:     c.SMTP.User,
			Password: c.SMTP.Password,
			From:     c.SMTP.From,
			DevPrint: c.SMTP.DevPrint,
		},
		Pay: RuntimePay{
			PublicBaseURL:  c.Pay.PublicBaseURL,
			LockTTLSeconds: lock,
		},
		Console: RuntimeConsole{
			DefaultFeeRateBps: fee,
			CodeTTLSeconds:    codeTTL,
			RegisterEnabled:   boolPtr(c.Console.RegisterEnabled),
		},
		Geetest: RuntimeGeetest{
			CaptchaID:  c.Geetest.CaptchaID,
			CaptchaKey: c.Geetest.CaptchaKey,
		},
	}
}

func boolPtr(v bool) *bool { return &v }

func derefBool(p *bool, fallback bool) bool {
	if p == nil {
		return fallback
	}
	return *p
}

// Bootstrap 注册启动钩子：MySQL 就绪后从 DB 热加载。
func Bootstrap(lc fx.Lifecycle, rt *RuntimeConfig) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			return rt.Reload(ctx)
		},
	})
}

func (r *RuntimeConfig) Reload(ctx context.Context) error {
	base := r.fromEnv()
	row, err := r.db.Ent().PlatformSetting.Query().
		Where(platformsetting.KeyEQ(SettingRuntimeConfig)).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			r.mu.Lock()
			r.snap = base
			r.source = "env"
			r.mu.Unlock()
			return nil
		}
		return err
	}
	var overlay RuntimeSnapshot
	if err := json.Unmarshal([]byte(row.Value), &overlay); err != nil {
		return fmt.Errorf("runtime_config json: %w", err)
	}
	merged := mergeRuntime(base, overlay)
	r.mu.Lock()
	r.snap = merged
	r.source = "db"
	r.mu.Unlock()
	if r.log != nil {
		r.log.Info(ctx, "runtime config reloaded", "source", "db")
	}
	return nil
}

func mergeRuntime(base, over RuntimeSnapshot) RuntimeSnapshot {
	out := base
	// DB 存完整快照；空字段回落 env
	out.SMTP.Host = firstNonEmpty(over.SMTP.Host, base.SMTP.Host)
	if over.SMTP.Port > 0 {
		out.SMTP.Port = over.SMTP.Port
	}
	out.SMTP.User = firstNonEmpty(over.SMTP.User, base.SMTP.User)
	out.SMTP.Password = over.SMTP.Password // 允许存空（表示无密码）
	if over.SMTP.Password == "" && base.SMTP.Password != "" && over.SMTP.Host == "" && over.SMTP.User == "" {
		// 旧残缺记录：保留 env 密码
		out.SMTP.Password = base.SMTP.Password
	}
	out.SMTP.From = firstNonEmpty(over.SMTP.From, base.SMTP.From)
	out.SMTP.DevPrint = over.SMTP.DevPrint
	if over.SMTP.Host == "" && over.SMTP.User == "" && over.SMTP.From == "" && over.SMTP.Port == 0 {
		out.SMTP.DevPrint = base.SMTP.DevPrint
	}
	out.Pay.PublicBaseURL = firstNonEmpty(over.Pay.PublicBaseURL, base.Pay.PublicBaseURL)
	if over.Pay.LockTTLSeconds > 0 {
		out.Pay.LockTTLSeconds = over.Pay.LockTTLSeconds
	}
	if over.Console.DefaultFeeRateBps > 0 {
		out.Console.DefaultFeeRateBps = over.Console.DefaultFeeRateBps
	}
	if over.Console.CodeTTLSeconds > 0 {
		out.Console.CodeTTLSeconds = over.Console.CodeTTLSeconds
	}
	if over.Console.RegisterEnabled != nil {
		out.Console.RegisterEnabled = over.Console.RegisterEnabled
	} else {
		out.Console.RegisterEnabled = base.Console.RegisterEnabled
	}
	out.Geetest.CaptchaID = firstNonEmpty(over.Geetest.CaptchaID, base.Geetest.CaptchaID)
	out.Geetest.CaptchaKey = firstNonEmpty(over.Geetest.CaptchaKey, base.Geetest.CaptchaKey)
	return out
}

func firstNonEmpty(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return a
	}
	return b
}

func (r *RuntimeConfig) Snapshot() RuntimeSnapshot {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.snap
}

func (r *RuntimeConfig) View() RuntimeConfigView {
	s := r.Snapshot()
	r.mu.RLock()
	src := r.source
	r.mu.RUnlock()
	var v RuntimeConfigView
	v.Source = src
	v.SMTP.Host = s.SMTP.Host
	v.SMTP.Port = s.SMTP.Port
	v.SMTP.User = s.SMTP.User
	v.SMTP.From = s.SMTP.From
	v.SMTP.DevPrint = s.SMTP.DevPrint
	v.SMTP.PasswordSet = strings.TrimSpace(s.SMTP.Password) != ""
	v.Pay = s.Pay
	v.Console.DefaultFeeRateBps = s.Console.DefaultFeeRateBps
	v.Console.CodeTTLSeconds = s.Console.CodeTTLSeconds
	v.Console.RegisterEnabled = derefBool(s.Console.RegisterEnabled, true)
	v.Geetest.CaptchaID = s.Geetest.CaptchaID
	v.Geetest.KeySet = strings.TrimSpace(s.Geetest.CaptchaKey) != ""
	return v
}

func (r *RuntimeConfig) SMTP() RuntimeSMTP {
	return r.Snapshot().SMTP
}

func (r *RuntimeConfig) PayPublicBaseURL() string {
	return strings.TrimRight(strings.TrimSpace(r.Snapshot().Pay.PublicBaseURL), "/")
}

func (r *RuntimeConfig) PayLockTTLSeconds() int64 {
	ttl := r.Snapshot().Pay.LockTTLSeconds
	if ttl <= 0 {
		return 300
	}
	return ttl
}

func (r *RuntimeConfig) ConsoleCodeTTLSeconds() int64 {
	ttl := r.Snapshot().Console.CodeTTLSeconds
	if ttl <= 0 {
		return 600
	}
	return ttl
}

func (r *RuntimeConfig) ConsoleDefaultFeeRateBps() int {
	fee := r.Snapshot().Console.DefaultFeeRateBps
	if fee < 0 {
		return 100
	}
	return fee
}

// RegisterEnabled 是否开放商户自助注册；默认 true。
func (r *RuntimeConfig) RegisterEnabled() bool {
	return derefBool(r.Snapshot().Console.RegisterEnabled, true)
}

func (r *RuntimeConfig) Geetest() RuntimeGeetest {
	return r.Snapshot().Geetest
}

// GeetestEnabled 已配置 ID+Key 时开启注册验证码极验。
func (r *RuntimeConfig) GeetestEnabled() bool {
	g := r.Geetest()
	return strings.TrimSpace(g.CaptchaID) != "" && strings.TrimSpace(g.CaptchaKey) != ""
}

func (r *RuntimeConfig) ServerMode() string {
	if r.cfg == nil {
		return "local"
	}
	return r.cfg.Server.Mode
}

func (r *RuntimeConfig) Update(ctx context.Context, patch RuntimeConfigPatch) (RuntimeConfigView, error) {
	cur := r.Snapshot()
	next := cur

	if patch.SMTP != nil {
		p := patch.SMTP
		if p.Host != nil {
			next.SMTP.Host = strings.TrimSpace(*p.Host)
		}
		if p.Port != nil && *p.Port > 0 {
			next.SMTP.Port = *p.Port
		}
		if p.User != nil {
			next.SMTP.User = strings.TrimSpace(*p.User)
		}
		if p.From != nil {
			next.SMTP.From = strings.TrimSpace(*p.From)
		}
		if p.DevPrint != nil {
			next.SMTP.DevPrint = *p.DevPrint
		}
		if p.Password != nil && *p.Password != "" {
			next.SMTP.Password = *p.Password
		}
	}
	if patch.Pay != nil {
		if patch.Pay.PublicBaseURL != nil {
			next.Pay.PublicBaseURL = strings.TrimSpace(*patch.Pay.PublicBaseURL)
		}
		if patch.Pay.LockTTLSeconds != nil && *patch.Pay.LockTTLSeconds > 0 {
			next.Pay.LockTTLSeconds = *patch.Pay.LockTTLSeconds
		}
	}
	if patch.Console != nil {
		if patch.Console.DefaultFeeRateBps != nil && *patch.Console.DefaultFeeRateBps >= 0 {
			next.Console.DefaultFeeRateBps = *patch.Console.DefaultFeeRateBps
		}
		if patch.Console.CodeTTLSeconds != nil && *patch.Console.CodeTTLSeconds > 0 {
			next.Console.CodeTTLSeconds = *patch.Console.CodeTTLSeconds
		}
		if patch.Console.RegisterEnabled != nil {
			next.Console.RegisterEnabled = boolPtr(*patch.Console.RegisterEnabled)
		}
	}
	if patch.Geetest != nil {
		if patch.Geetest.CaptchaID != nil {
			next.Geetest.CaptchaID = strings.TrimSpace(*patch.Geetest.CaptchaID)
		}
		if patch.Geetest.CaptchaKey != nil && *patch.Geetest.CaptchaKey != "" {
			next.Geetest.CaptchaKey = *patch.Geetest.CaptchaKey
		}
	}
	if next.SMTP.Port <= 0 {
		next.SMTP.Port = 587
	}
	b, err := json.Marshal(next)
	if err != nil {
		return RuntimeConfigView{}, err
	}
	if err := r.upsert(ctx, string(b)); err != nil {
		return RuntimeConfigView{}, err
	}
	r.mu.Lock()
	r.snap = next
	r.source = "db"
	r.mu.Unlock()
	if r.log != nil {
		r.log.Info(ctx, "runtime config updated")
	}
	return r.View(), nil
}

func (r *RuntimeConfig) upsert(ctx context.Context, val string) error {
	row, err := r.db.Ent().PlatformSetting.Query().
		Where(platformsetting.KeyEQ(SettingRuntimeConfig)).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			id, err := snowflake.NextID()
			if err != nil {
				return err
			}
			_, err = r.db.Ent().PlatformSetting.Create().
				SetID(id).
				SetKey(SettingRuntimeConfig).
				SetValue(val).
				Save(ctx)
			return err
		}
		return err
	}
	_, err = r.db.Ent().PlatformSetting.UpdateOneID(row.ID).SetValue(val).Save(ctx)
	return err
}
