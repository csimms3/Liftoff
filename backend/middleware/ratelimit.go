package middleware

import (
	"fmt"
	"math"
	"net/http"
	"net/netip"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// RateLimiter allows at most limit requests per key within a fixed window.
type RateLimiter struct {
	limit  int
	window time.Duration
	now    func() time.Time

	mu        sync.Mutex
	hits      map[string]*bucket
	lastSweep time.Time
}

type bucket struct {
	start time.Time
	count int
}

// NewRateLimiter returns a limiter of limit requests per window per key.
func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	return &RateLimiter{limit: limit, window: window, now: time.Now, hits: map[string]*bucket{}}
}

// Allow records a request for key and reports whether it is within the limit,
// and if not, how long until the window resets.
func (l *RateLimiter) Allow(key string) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()

	if now.Sub(l.lastSweep) > l.window {
		for k, b := range l.hits {
			if now.Sub(b.start) >= l.window {
				delete(l.hits, k)
			}
		}
		l.lastSweep = now
	}

	b, ok := l.hits[key]
	if !ok || now.Sub(b.start) >= l.window {
		l.hits[key] = &bucket{start: now, count: 1}
		return true, 0
	}
	if b.count >= l.limit {
		return false, b.start.Add(l.window).Sub(now)
	}
	b.count++
	return true, 0
}

// clientKey is the client's IPv4 address, or its IPv6 /64: one host usually owns a
// whole /64, so keying on the full address would let it rotate to fresh budgets.
func clientKey(ip string) string {
	addr, err := netip.ParseAddr(ip)
	if err != nil || addr.Unmap().Is4() {
		return ip
	}
	prefix, _ := addr.Prefix(64)
	return prefix.String()
}

// Middleware limits by client IP. gin's ClientIP only honours X-Forwarded-For
// from trusted proxies (see Engine.SetTrustedProxies), so clients can't spoof it.
func (l *RateLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		ok, retry := l.Allow(clientKey(c.ClientIP()))
		if !ok {
			c.Header("Retry-After", fmt.Sprint(int(math.Ceil(retry.Seconds()))))
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "Too many attempts. Try again later."})
			return
		}
		c.Next()
	}
}
