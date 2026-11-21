package secrethash

import (
	"fmt"
	"strings"
)

// NewAccessKey 生成公开 Access Key，形如 nk_ + 16 位 hex（与 API Secret 独立）。
func NewAccessKey() (string, error) {
	token, err := RandomToken(8)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("nk_%s", token), nil
}

// ValidAccessKey 校验是否为设计约定的 nk_ + 16 hex。
func ValidAccessKey(key string) bool {
	if !strings.HasPrefix(key, "nk_") || len(key) != 19 {
		return false
	}
	for _, c := range key[3:] {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return false
		}
	}
	return true
}
