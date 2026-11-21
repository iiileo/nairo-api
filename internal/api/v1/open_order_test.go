package v1

import (
	"strings"
	"testing"
)

func TestNormalizeReturnURL(t *testing.T) {
	cases := []struct {
		in   string
		want string
		ok   bool
	}{
		{"", "", true},
		{"   ", "", true},
		{"https://shop.example.com/orders/1?x=1", "https://shop.example.com/orders/1?x=1", true},
		{" http://localhost:3000/done ", "http://localhost:3000/done", true},
		{"javascript:alert(1)", "", false},
		{"data:text/html,hi", "", false},
		{"//evil.com/x", "", false},
		{"/relative/path", "", false},
		{"ftp://example.com/a", "", false},
		{"https:///nohost", "", false},
		{"https://" + strings.Repeat("a", 520) + ".com", "", false},
	}
	for _, c := range cases {
		got, ok := normalizeReturnURL(c.in)
		if got != c.want || ok != c.ok {
			t.Errorf("normalizeReturnURL(%q) = (%q,%v), want (%q,%v)", c.in, got, ok, c.want, c.ok)
		}
	}
}
