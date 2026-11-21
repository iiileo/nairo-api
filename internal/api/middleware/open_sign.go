package middleware

import (
	"bytes"
	"io"
	"net/http"
	"strconv"
	"time"

	"nairo/api_service/internal/pkg/sign"
	"nairo/api_service/internal/service"

	"github.com/gin-gonic/gin"
)

const CtxOpenUserID = "open_user_id"
const CtxOpenAccessKey = "open_access_key"

func OpenSign(orders *service.OrderService) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := c.GetHeader("X-Nairo-Key")
		tsStr := c.GetHeader("X-Nairo-Timestamp")
		sig := c.GetHeader("X-Nairo-Sign")
		if key == "" || tsStr == "" || sig == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing_sign_headers"})
			return
		}
		ts, err := strconv.ParseInt(tsStr, 10, 64)
		if err != nil || !sign.WithinSkew(ts, 5*time.Minute) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "bad_timestamp"})
			return
		}
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "bad_body"})
			return
		}
		c.Request.Body = io.NopCloser(bytes.NewReader(body))

		userID, accessKey, secret, err := orders.ResolveOpenSecret(c.Request.Context(), key)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid_key"})
			return
		}
		canon := sign.OpenAPICanonical(c.Request.Method, c.Request.URL.Path, ts, sign.BodySHA256Hex(body))
		expect := sign.HMACSHA256Hex(secret, canon)
		if !hmacEqual(expect, sig) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "bad_sign"})
			return
		}
		c.Set(CtxOpenUserID, userID)
		c.Set(CtxOpenAccessKey, accessKey)
		c.Next()
	}
}

func hmacEqual(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	var v byte
	for i := 0; i < len(a); i++ {
		v |= a[i] ^ b[i]
	}
	return v == 0
}
