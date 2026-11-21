package service

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"nairo/api_service/ent"
	"nairo/api_service/ent/notifychannel"
	"nairo/api_service/internal/pkg/channels"
)

// AppNotifyChannel 心跳下发给 App 的 IM Webhook 配置（商户级 + 本设备专属）。
type AppNotifyChannel struct {
	ID                 string `json:"id"`
	Scope              string `json:"scope"` // merchant | device
	Provider           string `json:"provider"`
	Name               string `json:"name"`
	WebhookURL         string `json:"webhook_url"`
	Secret             string `json:"secret,omitempty"`
	EventOrderPaid     bool   `json:"event_order_paid"`
	EventFeeLow        bool   `json:"event_fee_low"`
	EventAppPush       bool   `json:"event_app_push"`
	EventDeviceOffline bool   `json:"event_device_offline"`
}

// AppAccountConfig 设备自身配置（App 本地缓存可能过期，需随心跳校准）。
type AppAccountConfig struct {
	Name     string   `json:"name"`
	Channels []string `json:"channels"`
}

// AppRuntimeConfig 心跳响应中的运行时配置。
type AppRuntimeConfig struct {
	Version string `json:"version"`
	// Channels 设备当前启用的支付渠道（与 account.channels 相同，兼容 App 1.2.9 读取顶层字段）。
	Channels       []string           `json:"channels"`
	Account        AppAccountConfig   `json:"account"`
	NotifyChannels []AppNotifyChannel `json:"notify_channels"`
}

// BuildAppRuntimeConfig 构建指定设备的最新配置（仅 enabled 通道）。
func (s *BotNotifyService) BuildAppRuntimeConfig(ctx context.Context, acc *ent.PaymentAccount) (*AppRuntimeConfig, error) {
	list, err := s.db.Ent().NotifyChannel.Query().
		Where(notifychannel.UserIDEQ(acc.UserID), notifychannel.EnabledEQ(true)).
		Order(ent.Asc(notifychannel.FieldID)).
		All(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]AppNotifyChannel, 0, len(list))
	for _, ch := range list {
		scope := "merchant"
		if ch.AccountID != nil {
			if *ch.AccountID != acc.ID {
				continue
			}
			scope = "device"
		}
		item := AppNotifyChannel{
			ID:                 fmt.Sprintf("%d", ch.ID),
			Scope:              scope,
			Provider:           ch.Provider,
			Name:               ch.Name,
			WebhookURL:         ch.WebhookURL,
			EventOrderPaid:     ch.EventOrderPaid,
			EventFeeLow:        ch.EventFeeLow,
			EventAppPush:       ch.EventAppPush,
			EventDeviceOffline: ch.EventDeviceOffline,
		}
		if ch.Secret != nil {
			item.Secret = *ch.Secret
		}
		out = append(out, item)
	}
	chs := channels.Parse(acc.Channel)
	if chs == nil {
		chs = []string{}
	}
	cfg := &AppRuntimeConfig{
		Channels:       chs,
		Account:        AppAccountConfig{Name: acc.Name, Channels: chs},
		NotifyChannels: out,
	}
	raw, _ := json.Marshal(cfg)
	sum := sha1.Sum(raw)
	cfg.Version = hex.EncodeToString(sum[:])[:12]
	return cfg, nil
}
