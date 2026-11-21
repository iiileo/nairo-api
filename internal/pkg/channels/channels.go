package channels

import (
	"fmt"
	"sort"
	"strings"
)

var allowed = map[string]struct{}{
	"wechat": {},
	"alipay": {},
}

// Normalize 去重、校验、排序后以逗号拼接存储，如 "alipay,wechat"。
func Normalize(in []string) (string, []string, error) {
	seen := map[string]struct{}{}
	out := make([]string, 0, 2)
	for _, raw := range in {
		ch := strings.ToLower(strings.TrimSpace(raw))
		if ch == "" {
			continue
		}
		if _, ok := allowed[ch]; !ok {
			return "", nil, fmt.Errorf("invalid channel: %s", ch)
		}
		if _, ok := seen[ch]; ok {
			continue
		}
		seen[ch] = struct{}{}
		out = append(out, ch)
	}
	if len(out) == 0 {
		return "", nil, fmt.Errorf("channels required")
	}
	sort.Strings(out)
	return strings.Join(out, ","), out, nil
}

// Parse 解析存储值（兼容旧单渠道）。
func Parse(stored string) []string {
	stored = strings.TrimSpace(stored)
	if stored == "" {
		return nil
	}
	parts := strings.Split(stored, ",")
	out := make([]string, 0, len(parts))
	seen := map[string]struct{}{}
	for _, p := range parts {
		ch := strings.ToLower(strings.TrimSpace(p))
		if _, ok := allowed[ch]; !ok {
			continue
		}
		if _, ok := seen[ch]; ok {
			continue
		}
		seen[ch] = struct{}{}
		out = append(out, ch)
	}
	sort.Strings(out)
	return out
}

func Supports(stored, want string) bool {
	want = strings.ToLower(strings.TrimSpace(want))
	for _, ch := range Parse(stored) {
		if ch == want {
			return true
		}
	}
	return false
}

func Label(stored string) string {
	list := Parse(stored)
	names := make([]string, 0, len(list))
	for _, ch := range list {
		switch ch {
		case "wechat":
			names = append(names, "微信")
		case "alipay":
			names = append(names, "支付宝")
		default:
			names = append(names, ch)
		}
	}
	if len(names) == 0 {
		return "—"
	}
	return strings.Join(names, " + ")
}
