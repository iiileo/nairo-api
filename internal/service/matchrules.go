package service

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"nairo/api_service/ent"
	"nairo/api_service/ent/platformsetting"
	"nairo/api_service/internal/snowflake"
)

func compileRE(pat string) (*regexp.Regexp, error) {
	return regexp.Compile(pat)
}

const SettingAppMatchRules = "app_payment_match_rules"

// AppMatchRules App 通知匹配规则（服务端下发，后台可改）。
type AppMatchRules struct {
	Version           int               `json:"version"`
	UpdatedAt         string            `json:"updated_at,omitempty"`
	WatchPackages     map[string]string `json:"watch_packages"`
	ReceiveKeywords   []string          `json:"receive_keywords"`
	OutboundKeywords  []string          `json:"outbound_keywords"`
	IgnoreKeywords    []string          `json:"ignore_keywords"`
	SanitizePatterns  []string          `json:"sanitize_patterns"`
	AmountPatterns    []AmountPattern   `json:"amount_patterns"`
}

type AmountPattern struct {
	Name  string `json:"name"`
	Regex string `json:"regex"`
	Group int    `json:"group"` // 捕获组，默认 1
}

func DefaultAppMatchRules() AppMatchRules {
	return AppMatchRules{
		Version: 1,
		WatchPackages: map[string]string{
			"wechat": "com.tencent.mm",
			"alipay": "com.eg.android.AlipayGphone",
		},
		ReceiveKeywords: []string{
			"收款助手", "收款商业版", "成功收款", "已收款", "收款到账",
			"个人收款码", "二维码收款", "收钱码",
			"向你付款", "付款给你", "扫码向你付款", "通过扫码向你付款",
			"收到一笔", "收款", "到账", "收钱", "入账",
		},
		OutboundKeywords: []string{
			"你已支付", "你支付了", "付款成功", "支付成功", "已付款",
			"转账成功", "向商家付款", "付款给",
		},
		IgnoreKeywords: []string{
			"提现", "已打款", "转入零钱", "转入银行卡",
			"经营账户提现", "结算到账", "佣金到账",
			"服务通知", "账单", "对账单", "月度", "活动",
			"碰一下", "0元领", "限量发放", "恢复原价",
			"昨日收款", "今日收款", "笔，总计", "笔总计",
			"红包封面", "好友推荐", "新的朋友", "语音通话", "视频通话",
			"正在通话", "表情包", "拍了拍",
		},
		SanitizePatterns: []string{
			`\[\s*\d+\s*条\s*]`,
			`\d+\s*条消息`,
			`\d+\s*条新消息`,
		},
		AmountPatterns: []AmountPattern{
			{
				Name:  "yuan",
				Regex: `(?<![0-9.])([0-9]+(?:\.[0-9]{1,2})?)\s*元(?!\s*条)`,
				Group: 1,
			},
			{
				Name:  "currency",
				Regex: `[¥￥]\s*([0-9]+(?:\.[0-9]{1,2})?)`,
				Group: 1,
			},
			{
				Name:  "plain",
				Regex: `(?:成功收款|已收款|收款到账|个人收款码到账|二维码收款|收钱码到账|到账|收钱|入账)[^0-9¥￥元\[\]]{0,8}([0-9]+(?:\.[0-9]{1,2})?)(?!\s*条)`,
				Group: 1,
			},
		},
	}
}

func (s *PlatformService) GetAppMatchRules(ctx context.Context) (AppMatchRules, error) {
	row, err := s.db.Ent().PlatformSetting.Query().
		Where(platformsetting.KeyEQ(SettingAppMatchRules)).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return DefaultAppMatchRules(), nil
		}
		return AppMatchRules{}, err
	}
	raw := strings.TrimSpace(row.Value)
	if raw == "" {
		return DefaultAppMatchRules(), nil
	}
	var rules AppMatchRules
	if err := json.Unmarshal([]byte(raw), &rules); err != nil {
		return DefaultAppMatchRules(), nil
	}
	return normalizeMatchRules(rules), nil
}

func (s *PlatformService) SetAppMatchRules(ctx context.Context, in AppMatchRules) (AppMatchRules, error) {
	rules := normalizeMatchRules(in)
	if err := validateMatchRules(rules); err != nil {
		return AppMatchRules{}, fmt.Errorf("%w: %s", ErrInvalid, err.Error())
	}
	rules.UpdatedAt = time.Now().Format(time.RFC3339)
	if rules.Version <= 0 {
		rules.Version = 1
	}
	b, err := json.Marshal(rules)
	if err != nil {
		return AppMatchRules{}, err
	}
	if err := s.upsertSetting(ctx, SettingAppMatchRules, string(b)); err != nil {
		return AppMatchRules{}, err
	}
	return rules, nil
}

func (s *PlatformService) ResetAppMatchRules(ctx context.Context) (AppMatchRules, error) {
	rules := DefaultAppMatchRules()
	rules.UpdatedAt = time.Now().Format(time.RFC3339)
	return s.SetAppMatchRules(ctx, rules)
}

func (s *PlatformService) upsertSetting(ctx context.Context, key, val string) error {
	row, err := s.db.Ent().PlatformSetting.Query().
		Where(platformsetting.KeyEQ(key)).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			id, err := snowflake.NextID()
			if err != nil {
				return err
			}
			_, err = s.db.Ent().PlatformSetting.Create().
				SetID(id).
				SetKey(key).
				SetValue(val).
				Save(ctx)
			return err
		}
		return err
	}
	_, err = s.db.Ent().PlatformSetting.UpdateOneID(row.ID).SetValue(val).Save(ctx)
	return err
}

func normalizeMatchRules(in AppMatchRules) AppMatchRules {
	def := DefaultAppMatchRules()
	out := in
	if out.Version <= 0 {
		out.Version = def.Version
	}
	if len(out.WatchPackages) == 0 {
		out.WatchPackages = def.WatchPackages
	}
	out.ReceiveKeywords = cleanStrings(out.ReceiveKeywords)
	out.OutboundKeywords = cleanStrings(out.OutboundKeywords)
	out.IgnoreKeywords = cleanStrings(out.IgnoreKeywords)
	out.SanitizePatterns = cleanStrings(out.SanitizePatterns)
	if len(out.AmountPatterns) == 0 {
		out.AmountPatterns = def.AmountPatterns
	} else {
		cleaned := make([]AmountPattern, 0, len(out.AmountPatterns))
		for _, p := range out.AmountPatterns {
			p.Name = strings.TrimSpace(p.Name)
			p.Regex = strings.TrimSpace(p.Regex)
			if p.Regex == "" {
				continue
			}
			if p.Group <= 0 {
				p.Group = 1
			}
			if p.Name == "" {
				p.Name = fmt.Sprintf("p%d", len(cleaned)+1)
			}
			cleaned = append(cleaned, p)
		}
		if len(cleaned) == 0 {
			cleaned = def.AmountPatterns
		}
		out.AmountPatterns = cleaned
	}
	if len(out.ReceiveKeywords) == 0 {
		out.ReceiveKeywords = def.ReceiveKeywords
	}
	return out
}

func validateMatchRules(rules AppMatchRules) error {
	if len(rules.AmountPatterns) == 0 {
		return fmt.Errorf("amount_patterns required")
	}
	for _, p := range rules.AmountPatterns {
		if _, err := compileRE(p.Regex); err != nil {
			return fmt.Errorf("bad amount pattern %q: %v", p.Name, err)
		}
	}
	for _, pat := range rules.SanitizePatterns {
		if _, err := compileRE(pat); err != nil {
			return fmt.Errorf("bad sanitize pattern: %v", err)
		}
	}
	return nil
}

func cleanStrings(in []string) []string {
	out := make([]string, 0, len(in))
	seen := map[string]struct{}{}
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}
