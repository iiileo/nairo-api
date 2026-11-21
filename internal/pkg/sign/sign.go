package sign

import (
	"crypto/hmac"
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"
)

func HMACSHA256Hex(secret, message string) string {
	m := hmac.New(sha256.New, []byte(secret))
	_, _ = m.Write([]byte(message))
	return hex.EncodeToString(m.Sum(nil))
}

func OpenAPICanonical(method, path string, ts int64, bodySHA256Hex string) string {
	return strings.ToUpper(method) + "\n" + path + "\n" + strconv.FormatInt(ts, 10) + "\n" + bodySHA256Hex
}

func BodySHA256Hex(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

func WithinSkew(ts int64, skew time.Duration) bool {
	now := time.Now().Unix()
	diff := now - ts
	if diff < 0 {
		diff = -diff
	}
	return time.Duration(diff)*time.Second <= skew
}

// AppPushSign 兼容常见免签：md5(callback_id + t + channel + price + nonce + app_secret)
func AppPushSign(callbackID string, t int64, channel, price, nonce, appSecret string) string {
	raw := fmt.Sprintf("%s%d%s%s%s%s", callbackID, t, channel, price, nonce, appSecret)
	sum := md5.Sum([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// AppHeartbeatSign：md5(callback_id + t + nonce + app_secret)
func AppHeartbeatSign(callbackID string, t int64, nonce, appSecret string) string {
	raw := fmt.Sprintf("%s%d%s%s", callbackID, t, nonce, appSecret)
	sum := md5.Sum([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// AppBindSign：md5(callback_id + t + nonce + device_id + app_secret)，用于手机绑定/解绑。
func AppBindSign(callbackID string, t int64, nonce, deviceID, appSecret string) string {
	raw := fmt.Sprintf("%s%d%s%s%s", callbackID, t, nonce, deviceID, appSecret)
	sum := md5.Sum([]byte(raw))
	return hex.EncodeToString(sum[:])
}
