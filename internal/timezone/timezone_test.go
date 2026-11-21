package timezone

import (
	"testing"
	"time"
)

func TestInitShanghai(t *testing.T) {
	t.Setenv("APP_TIMEZONE", "Asia/Shanghai")
	t.Setenv("TZ", "")
	Init()
	if time.Local.String() != "Asia/Shanghai" {
		t.Fatalf("got %s", time.Local.String())
	}
	now := time.Now()
	_, offset := now.Zone()
	if offset != 8*3600 {
		t.Fatalf("offset=%d", offset)
	}
}
