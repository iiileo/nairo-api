package service

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"nairo/api_service/ent"
	"nairo/api_service/ent/notifychannel"
	"nairo/api_service/internal/logger"
	"nairo/api_service/internal/mysql"
	"nairo/api_service/internal/snowflake"
)

const (
	NotifyProviderWecom    = "wecom"
	NotifyProviderFeishu   = "feishu"
	NotifyProviderDingtalk = "dingtalk"

	NotifyEventOrderPaid      = "order_paid"
	NotifyEventFeeLow         = "fee_low"
	NotifyEventAppPush        = "app_push"
	NotifyEventDeviceOffline  = "device_offline"
)

type BotNotifyService struct {
	db     *mysql.Client
	log    *logger.Logger
	client *http.Client
}

func NewBotNotifyService(db *mysql.Client, log *logger.Logger) *BotNotifyService {
	return &BotNotifyService{
		db:     db,
		log:    log,
		client: &http.Client{Timeout: 8 * time.Second},
	}
}

type BotEvent struct {
	Kind    string
	Title   string
	Summary string
	Fields  []BotField
	Level   string // info | warning | success
	// IdemKey 非空时按「幂等键 + 通知渠道」去重，避免重复推送。
	IdemKey string
	// AccountID >0：设备相关事件，推送给「商户级 + 该设备」通道；=0：仅商户级通道。
	AccountID int64
}

type BotField struct {
	Label string
	Value string
}

func ValidNotifyProvider(p string) bool {
	switch strings.ToLower(strings.TrimSpace(p)) {
	case NotifyProviderWecom, NotifyProviderFeishu, NotifyProviderDingtalk:
		return true
	default:
		return false
	}
}

func (s *BotNotifyService) EnqueueOrderPaid(order *ent.Order) {
	if order == nil {
		return
	}
	accountID := int64(0)
	if order.AccountID != nil {
		accountID = *order.AccountID
	}
	ev := BotEvent{
		Kind:      NotifyEventOrderPaid,
		Title:     "订单入账",
		Summary:   fmt.Sprintf("订单 %s 已支付", order.OutTradeNo),
		Level:     "success",
		IdemKey:   fmt.Sprintf("order_paid:%d", order.ID),
		AccountID: accountID,
		Fields: []BotField{
			{Label: "商户单号", Value: order.OutTradeNo},
			{Label: "订单 ID", Value: fmt.Sprintf("%d", order.ID)},
			{Label: "渠道", Value: channelLabel(order.Channel)},
			{Label: "标价", Value: formatYuan(order.AmountCents)},
			{Label: "实付", Value: formatYuan(order.PayAmountCents)},
			{Label: "入账", Value: settleLabel(order.SettleType)},
			{Label: "状态", Value: order.Status},
		},
	}
	if order.PaidAt != nil {
		ev.Fields = append(ev.Fields, BotField{Label: "支付时间", Value: order.PaidAt.Local().Format("2006-01-02 15:04:05")})
	}
	go s.dispatch(order.UserID, NotifyEventOrderPaid, ev)
}

func (s *BotNotifyService) EnqueueFeeInsufficient(userID int64, outTradeNo string, amountCents int, feeCents, balanceCents int64) {
	ev := BotEvent{
		Kind:    NotifyEventFeeLow,
		Title:   "手续费不足",
		Summary: "下单失败：钱包余额不足以支付本单手续费",
		Level:   "warning",
		IdemKey: fmt.Sprintf("fee_low:%d:%s", userID, strings.TrimSpace(outTradeNo)),
		Fields: []BotField{
			{Label: "商户单号", Value: emptyDash(outTradeNo)},
			{Label: "订单金额", Value: formatYuan(amountCents)},
			{Label: "所需手续费", Value: formatYuan(int(feeCents))},
			{Label: "当前余额", Value: formatYuan(int(balanceCents))},
		},
	}
	go s.dispatch(userID, NotifyEventFeeLow, ev)
}

func (s *BotNotifyService) EnqueueAppPush(userID, accountID int64, accountName, channel, priceYuan string, matched bool, outTradeNo, idemKey string) {
	title := "收到 App 支付推送"
	summary := "已匹配订单并入账"
	level := "success"
	if !matched {
		summary = "未匹配到待支付订单（已记入未匹配到账）"
		level = "warning"
	}
	fields := []BotField{
		{Label: "设备", Value: emptyDash(accountName)},
		{Label: "渠道", Value: channelLabel(channel)},
		{Label: "金额", Value: "¥" + strings.TrimSpace(priceYuan)},
		{Label: "结果", Value: summary},
	}
	if matched && outTradeNo != "" {
		fields = append(fields, BotField{Label: "商户单号", Value: outTradeNo})
	}
	ev := BotEvent{
		Kind:      NotifyEventAppPush,
		Title:     title,
		Summary:   summary,
		Level:     level,
		IdemKey:   idemKey,
		AccountID: accountID,
		Fields:    fields,
	}
	go s.dispatch(userID, NotifyEventAppPush, ev)
}

func (s *BotNotifyService) dispatch(userID int64, event string, ev BotEvent) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	chs, err := s.listEnabled(ctx, userID, event, ev.AccountID)
	if err != nil {
		s.log.Warn(ctx, "bot notify list failed", "user_id", userID, "event", event, "err", err)
		return
	}
	for _, ch := range chs {
		if key := strings.TrimSpace(ev.IdemKey); key != "" {
			ok, err := claimIdempotency(ctx, s.db, IdemScopeBotNotify, fmt.Sprintf("%s:ch:%d", key, ch.ID), event)
			if err != nil {
				s.log.Warn(ctx, "bot notify idem claim failed",
					"user_id", userID, "channel_id", ch.ID, "err", err)
				continue
			}
			if !ok {
				s.log.Info(ctx, "bot notify skipped duplicate",
					"user_id", userID, "channel_id", ch.ID, "provider", ch.Provider, "event", event, "idem", key)
				continue
			}
		}
		if err := s.sendOnce(ctx, ch, ev); err != nil {
			s.log.Warn(ctx, "bot notify send failed",
				"user_id", userID, "channel_id", ch.ID, "provider", ch.Provider, "err", err)
			continue
		}
		s.log.Info(ctx, "bot notify ok",
			"user_id", userID, "channel_id", ch.ID, "provider", ch.Provider, "event", event)
	}
}

func (s *BotNotifyService) listEnabled(ctx context.Context, userID int64, event string, accountID int64) ([]*ent.NotifyChannel, error) {
	q := s.db.Ent().NotifyChannel.Query().
		Where(notifychannel.UserIDEQ(userID), notifychannel.EnabledEQ(true))
	switch event {
	case NotifyEventOrderPaid:
		q = q.Where(notifychannel.EventOrderPaidEQ(true))
	case NotifyEventFeeLow:
		q = q.Where(notifychannel.EventFeeLowEQ(true))
	case NotifyEventAppPush:
		q = q.Where(notifychannel.EventAppPushEQ(true))
	case NotifyEventDeviceOffline:
		q = q.Where(notifychannel.EventDeviceOfflineEQ(true))
	}
	all, err := q.All(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]*ent.NotifyChannel, 0, len(all))
	for _, ch := range all {
		if accountID <= 0 {
			// 商户级事件（如手续费不足）：只走未绑定设备的通道
			if ch.AccountID == nil {
				out = append(out, ch)
			}
			continue
		}
		// 设备事件：商户级（全部设备）+ 该设备专属
		if ch.AccountID == nil || *ch.AccountID == accountID {
			out = append(out, ch)
		}
	}
	return out, nil
}

func (s *BotNotifyService) SendTest(ctx context.Context, ch *ent.NotifyChannel) error {
	ev := BotEvent{
		Kind:    "test",
		Title:   "Nairo 通知测试",
		Summary: "这是一条测试消息，配置正常即可收到。",
		Level:   "info",
		Fields: []BotField{
			{Label: "渠道", Value: providerLabel(ch.Provider)},
			{Label: "名称", Value: emptyDash(ch.Name)},
			{Label: "时间", Value: time.Now().Local().Format("2006-01-02 15:04:05")},
		},
	}
	return s.sendOnce(ctx, ch, ev)
}

func (s *BotNotifyService) sendOnce(ctx context.Context, ch *ent.NotifyChannel, ev BotEvent) error {
	webhook := strings.TrimSpace(ch.WebhookURL)
	if webhook == "" {
		return fmt.Errorf("empty webhook")
	}
	secret := ""
	if ch.Secret != nil {
		secret = strings.TrimSpace(*ch.Secret)
	}

	var (
		body []byte
		err  error
		u    = webhook
	)
	switch ch.Provider {
	case NotifyProviderWecom:
		body, err = buildWecomPayload(ev)
	case NotifyProviderFeishu:
		body, err = buildFeishuPayload(ev, secret)
	case NotifyProviderDingtalk:
		u, err = dingtalkSignedURL(webhook, secret)
		if err != nil {
			return err
		}
		body, err = buildDingtalkPayload(ev)
	default:
		return fmt.Errorf("unknown provider %s", ch.Provider)
	}
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("http %d: %s", resp.StatusCode, string(raw))
	}
	// 各厂商成功仍可能返回业务错误码
	var probe map[string]any
	if json.Unmarshal(raw, &probe) == nil {
		if code, ok := asInt(probe["code"]); ok && code != 0 {
			return fmt.Errorf("provider code %v: %s", probe["code"], string(raw))
		}
		if code, ok := asInt(probe["StatusCode"]); ok && code != 0 {
			return fmt.Errorf("provider StatusCode %v: %s", probe["StatusCode"], string(raw))
		}
		if code, ok := asInt(probe["errcode"]); ok && code != 0 {
			return fmt.Errorf("provider errcode %v: %s", probe["errcode"], string(raw))
		}
	}
	return nil
}

func buildWecomPayload(ev BotEvent) ([]byte, error) {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("### %s\n", ev.Title))
	b.WriteString(fmt.Sprintf("%s\n", coloredWecom(ev.Summary, ev.Level)))
	for _, f := range ev.Fields {
		b.WriteString(fmt.Sprintf("> **%s**：%s\n", f.Label, f.Value))
	}
	b.WriteString("\n<font color=\"comment\">Nairo 通知</font>")
	return json.Marshal(map[string]any{
		"msgtype": "markdown",
		"markdown": map[string]string{
			"content": b.String(),
		},
	})
}

func buildDingtalkPayload(ev BotEvent) ([]byte, error) {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("### %s\n\n", ev.Title))
	b.WriteString(fmt.Sprintf("%s\n\n", ev.Summary))
	for _, f := range ev.Fields {
		b.WriteString(fmt.Sprintf("- **%s**：%s\n", f.Label, f.Value))
	}
	b.WriteString("\n> Nairo 通知")
	return json.Marshal(map[string]any{
		"msgtype": "markdown",
		"markdown": map[string]string{
			"title": ev.Title,
			"text":  b.String(),
		},
	})
}

func buildFeishuPayload(ev BotEvent, secret string) ([]byte, error) {
	template := "blue"
	switch ev.Level {
	case "success":
		template = "green"
	case "warning":
		template = "orange"
	}
	var md strings.Builder
	md.WriteString(ev.Summary)
	md.WriteString("\n")
	for _, f := range ev.Fields {
		md.WriteString(fmt.Sprintf("\n**%s** %s", f.Label, f.Value))
	}
	card := map[string]any{
		"header": map[string]any{
			"template": template,
			"title": map[string]string{
				"tag":     "plain_text",
				"content": ev.Title,
			},
		},
		"elements": []any{
			map[string]any{
				"tag": "div",
				"text": map[string]string{
					"tag":     "lark_md",
					"content": md.String(),
				},
			},
			map[string]any{
				"tag": "note",
				"elements": []any{
					map[string]string{"tag": "plain_text", "content": "Nairo 通知"},
				},
			},
		},
	}
	payload := map[string]any{
		"msg_type": "interactive",
		"card":     card,
	}
	if secret != "" {
		ts := strconv.FormatInt(time.Now().Unix(), 10)
		sign, err := feishuSign(ts, secret)
		if err != nil {
			return nil, err
		}
		payload["timestamp"] = ts
		payload["sign"] = sign
	}
	return json.Marshal(payload)
}

func feishuSign(timestamp, secret string) (string, error) {
	// 官方：以 timestamp+"\n"+secret 为 key，对空消息做 HmacSHA256，再 Base64。
	stringToSign := timestamp + "\n" + secret
	mac := hmac.New(sha256.New, []byte(stringToSign))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil)), nil
}

func dingtalkSignedURL(webhook, secret string) (string, error) {
	if secret == "" {
		return webhook, nil
	}
	ts := strconv.FormatInt(time.Now().UnixMilli(), 10)
	stringToSign := ts + "\n" + secret
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(stringToSign))
	sign := url.QueryEscape(base64.StdEncoding.EncodeToString(mac.Sum(nil)))
	sep := "?"
	if strings.Contains(webhook, "?") {
		sep = "&"
	}
	return webhook + sep + "timestamp=" + ts + "&sign=" + sign, nil
}

func coloredWecom(text, level string) string {
	switch level {
	case "success":
		return fmt.Sprintf("<font color=\"info\">%s</font>", text)
	case "warning":
		return fmt.Sprintf("<font color=\"warning\">%s</font>", text)
	default:
		return text
	}
}

func formatYuan(cents int) string {
	neg := cents < 0
	if neg {
		cents = -cents
	}
	s := fmt.Sprintf("¥%d.%02d", cents/100, cents%100)
	if neg {
		return "-" + s
	}
	return s
}

func channelLabel(ch string) string {
	switch strings.ToLower(ch) {
	case "wechat":
		return "微信"
	case "alipay":
		return "支付宝"
	default:
		return ch
	}
}

func settleLabel(t string) string {
	switch t {
	case "auto":
		return "自动匹配"
	case "manual":
		return "人工补单"
	default:
		return emptyDash(t)
	}
}

func providerLabel(p string) string {
	switch p {
	case NotifyProviderWecom:
		return "企业微信"
	case NotifyProviderFeishu:
		return "飞书"
	case NotifyProviderDingtalk:
		return "钉钉"
	default:
		return p
	}
}

func emptyDash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "—"
	}
	return s
}

func asInt(v any) (int, bool) {
	switch n := v.(type) {
	case float64:
		return int(n), true
	case int:
		return n, true
	case int64:
		return int(n), true
	case json.Number:
		i, err := n.Int64()
		return int(i), err == nil
	default:
		return 0, false
	}
}

// --- Console CRUD ---

type NotifyChannelInput struct {
	Provider           string
	Name               string
	WebhookURL         string
	Secret             *string // nil=不改；""=清空
	Enabled            *bool
	EventOrderPaid     *bool
	EventFeeLow        *bool
	EventAppPush       *bool
	EventDeviceOffline *bool
	// AccountID：nil=不改（更新时）；创建时 nil/空=商户级；非空指针且 >0=绑定设备；指针且 <=0=清空为商户级
	AccountID *int64
}

func (s *BotNotifyService) ListChannels(ctx context.Context, userID int64) ([]*ent.NotifyChannel, error) {
	return s.db.Ent().NotifyChannel.Query().
		Where(notifychannel.UserIDEQ(userID)).
		Order(ent.Desc(notifychannel.FieldCreatedAt)).
		All(ctx)
}

func (s *BotNotifyService) CreateChannel(ctx context.Context, userID int64, in NotifyChannelInput) (*ent.NotifyChannel, error) {
	provider := strings.ToLower(strings.TrimSpace(in.Provider))
	if !ValidNotifyProvider(provider) {
		return nil, fmt.Errorf("%w: provider", ErrInvalid)
	}
	webhook := strings.TrimSpace(in.WebhookURL)
	if webhook == "" || !strings.HasPrefix(webhook, "http") {
		return nil, fmt.Errorf("%w: webhook_url", ErrInvalid)
	}
	accountID, err := s.resolveAccountID(ctx, userID, in.AccountID)
	if err != nil {
		return nil, err
	}
	id, err := snowflake.NextID()
	if err != nil {
		return nil, err
	}
	b := s.db.Ent().NotifyChannel.Create().
		SetID(id).
		SetUserID(userID).
		SetProvider(provider).
		SetName(strings.TrimSpace(in.Name)).
		SetWebhookURL(webhook).
		SetEnabled(true).
		SetEventOrderPaid(true).
		SetEventFeeLow(true).
		SetEventAppPush(true).
		SetEventDeviceOffline(true)
	if accountID != nil {
		b.SetAccountID(*accountID)
		// 设备专属通道不接收商户级「手续费不足」
		b.SetEventFeeLow(false)
	}
	if in.Secret != nil && strings.TrimSpace(*in.Secret) != "" {
		b.SetSecret(strings.TrimSpace(*in.Secret))
	}
	if in.Enabled != nil {
		b.SetEnabled(*in.Enabled)
	}
	if in.EventOrderPaid != nil {
		b.SetEventOrderPaid(*in.EventOrderPaid)
	}
	if in.EventFeeLow != nil && accountID == nil {
		b.SetEventFeeLow(*in.EventFeeLow)
	}
	if in.EventAppPush != nil {
		b.SetEventAppPush(*in.EventAppPush)
	}
	if in.EventDeviceOffline != nil {
		b.SetEventDeviceOffline(*in.EventDeviceOffline)
	}
	return b.Save(ctx)
}

func (s *BotNotifyService) UpdateChannel(ctx context.Context, userID, id int64, in NotifyChannelInput) (*ent.NotifyChannel, error) {
	ch, err := s.getOwned(ctx, userID, id)
	if err != nil {
		return nil, err
	}
	u := s.db.Ent().NotifyChannel.UpdateOneID(ch.ID)
	u.SetName(strings.TrimSpace(in.Name))
	if in.WebhookURL != "" {
		wh := strings.TrimSpace(in.WebhookURL)
		if !strings.HasPrefix(wh, "http") {
			return nil, fmt.Errorf("%w: webhook_url", ErrInvalid)
		}
		u.SetWebhookURL(wh)
	}
	boundAccount := ch.AccountID
	if in.AccountID != nil {
		aid, err := s.resolveAccountID(ctx, userID, in.AccountID)
		if err != nil {
			return nil, err
		}
		if aid == nil {
			u.ClearAccountID()
			boundAccount = nil
		} else {
			u.SetAccountID(*aid)
			boundAccount = aid
		}
	}
	if in.Secret != nil {
		sec := strings.TrimSpace(*in.Secret)
		if sec == "" {
			u.ClearSecret()
		} else {
			u.SetSecret(sec)
		}
	}
	if in.Enabled != nil {
		u.SetEnabled(*in.Enabled)
	}
	if in.EventOrderPaid != nil {
		u.SetEventOrderPaid(*in.EventOrderPaid)
	}
	if boundAccount != nil {
		u.SetEventFeeLow(false)
	} else if in.EventFeeLow != nil {
		u.SetEventFeeLow(*in.EventFeeLow)
	}
	if in.EventAppPush != nil {
		u.SetEventAppPush(*in.EventAppPush)
	}
	if in.EventDeviceOffline != nil {
		u.SetEventDeviceOffline(*in.EventDeviceOffline)
	}
	return u.Save(ctx)
}

func (s *BotNotifyService) DeleteChannel(ctx context.Context, userID, id int64) error {
	ch, err := s.getOwned(ctx, userID, id)
	if err != nil {
		return err
	}
	return s.db.Ent().NotifyChannel.DeleteOneID(ch.ID).Exec(ctx)
}

func (s *BotNotifyService) TestChannel(ctx context.Context, userID, id int64) error {
	ch, err := s.getOwned(ctx, userID, id)
	if err != nil {
		return err
	}
	return s.SendTest(ctx, ch)
}

func (s *BotNotifyService) getOwned(ctx context.Context, userID, id int64) (*ent.NotifyChannel, error) {
	ch, err := s.db.Ent().NotifyChannel.Get(ctx, id)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if ch.UserID != userID {
		return nil, ErrNotFound
	}
	return ch, nil
}

type NotifyChannelView struct {
	ID                 string    `json:"id"`
	AccountID          string    `json:"account_id,omitempty"`
	Provider           string    `json:"provider"`
	ProviderLabel      string    `json:"provider_label"`
	Name               string    `json:"name"`
	WebhookURL         string    `json:"webhook_url"`
	HasSecret          bool      `json:"has_secret"`
	Enabled            bool      `json:"enabled"`
	EventOrderPaid     bool      `json:"event_order_paid"`
	EventFeeLow        bool      `json:"event_fee_low"`
	EventAppPush       bool      `json:"event_app_push"`
	EventDeviceOffline bool      `json:"event_device_offline"`
	CreatedAt          time.Time `json:"created_at"`
}

func ViewNotifyChannel(ch *ent.NotifyChannel) *NotifyChannelView {
	hasSecret := ch.Secret != nil && strings.TrimSpace(*ch.Secret) != ""
	v := &NotifyChannelView{
		ID:                 fmt.Sprintf("%d", ch.ID),
		Provider:           ch.Provider,
		ProviderLabel:      providerLabel(ch.Provider),
		Name:               ch.Name,
		WebhookURL:         ch.WebhookURL,
		HasSecret:          hasSecret,
		Enabled:            ch.Enabled,
		EventOrderPaid:     ch.EventOrderPaid,
		EventFeeLow:        ch.EventFeeLow,
		EventAppPush:       ch.EventAppPush,
		EventDeviceOffline: ch.EventDeviceOffline,
		CreatedAt:          ch.CreatedAt,
	}
	if ch.AccountID != nil && *ch.AccountID > 0 {
		v.AccountID = fmt.Sprintf("%d", *ch.AccountID)
	}
	return v
}

// resolveAccountID 校验设备归属；in 为 nil 或 <=0 表示商户级（不绑定设备）。
func (s *BotNotifyService) resolveAccountID(ctx context.Context, userID int64, in *int64) (*int64, error) {
	if in == nil || *in <= 0 {
		return nil, nil
	}
	acc, err := s.db.Ent().PaymentAccount.Get(ctx, *in)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, fmt.Errorf("%w: account not found", ErrInvalid)
		}
		return nil, err
	}
	if acc.UserID != userID {
		return nil, fmt.Errorf("%w: account not found", ErrInvalid)
	}
	id := acc.ID
	return &id, nil
}
