package service

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestTruncateUTF8KeepsRunes(t *testing.T) {
	raw := "com.tencent.mm|-759047995|1795089323773|微信收款助手|[6条]微信收款助手: 微信支付收款0.10元(老顾客第22次消费)"
	for _, n := range []int{96, 128} {
		got := truncateUTF8(raw, n)
		if !utf8.ValidString(got) {
			t.Fatalf("n=%d: invalid utf8 after truncation: %q", n, got)
		}
		if len(got) > n {
			t.Fatalf("n=%d: too long: %d", n, len(got))
		}
	}
	if got := truncateUTF8("abc", 96); got != "abc" {
		t.Fatalf("short string changed: %q", got)
	}
}

func TestNormalizePushIdemKeyWithChinese(t *testing.T) {
	raw := strings.Repeat("收款", 60)
	got := normalizePushIdemKey(raw, "")
	if !utf8.ValidString(got) || len(got) > 96 {
		t.Fatalf("bad key: valid=%v len=%d", utf8.ValidString(got), len(got))
	}
}
