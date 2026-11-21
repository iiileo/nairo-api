package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type tokenBucket struct {
	mu         sync.Mutex
	tokens     float64
	last       time.Time
	ratePerSec float64
	burst      float64
}

func (b *tokenBucket) allow() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	now := time.Now()
	if b.last.IsZero() {
		b.last = now
		b.tokens = b.burst
	}
	elapsed := now.Sub(b.last).Seconds()
	b.last = now
	b.tokens += elapsed * b.ratePerSec
	if b.tokens > b.burst {
		b.tokens = b.burst
	}
	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

// RateLimit 按 client IP 的简易令牌桶限流。
func RateLimit(ratePerSec, burst float64) gin.HandlerFunc {
	var mu sync.Mutex
	buckets := map[string]*tokenBucket{}
	return func(c *gin.Context) {
		ip := c.ClientIP()
		mu.Lock()
		b, ok := buckets[ip]
		if !ok {
			b = &tokenBucket{ratePerSec: ratePerSec, burst: burst, tokens: burst, last: time.Now()}
			buckets[ip] = b
		}
		mu.Unlock()
		if !b.allow() {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "rate_limited"})
			return
		}
		c.Next()
	}
}
