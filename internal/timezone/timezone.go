package timezone

import (
	"os"
	"strings"
	"time"

	// distroless 无系统 zoneinfo，嵌入 IANA 时区数据。
	_ "time/tzdata"
)

const Default = "Asia/Shanghai"

// Init 将进程本地时区固定为 Asia/Shanghai（可用 APP_TIMEZONE / TZ 覆盖）。
// 必须在连库、打日志、业务逻辑之前调用。
func Init() {
	name := strings.TrimSpace(os.Getenv("APP_TIMEZONE"))
	if name == "" {
		name = strings.TrimSpace(os.Getenv("TZ"))
	}
	if name == "" {
		name = Default
	}
	loc, err := time.LoadLocation(name)
	if err != nil {
		loc = time.FixedZone("CST", 8*3600)
	}
	time.Local = loc
}

// Location 返回当前业务时区。
func Location() *time.Location {
	if time.Local != nil {
		return time.Local
	}
	loc, err := time.LoadLocation(Default)
	if err != nil {
		return time.FixedZone("CST", 8*3600)
	}
	return loc
}
