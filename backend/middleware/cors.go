// Package middleware holds HTTP middleware that is not specific to auth.
package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// CORS allows cross-origin requests only from the listed origins. With no origins
// it sends no CORS headers, which is right when the SPA is same-origin (in dev,
// Vite proxies /api, so the browser never makes a cross-origin call).
func CORS(allowedOrigins []string) gin.HandlerFunc {
	allowed := map[string]bool{}
	for _, o := range allowedOrigins {
		if o = strings.TrimRight(strings.TrimSpace(o), "/"); o != "" {
			allowed[o] = true
		}
	}
	return func(c *gin.Context) {
		if len(allowed) > 0 {
			// Responses differ by Origin, so shared caches must key on it.
			c.Writer.Header().Add("Vary", "Origin")
		}
		origin := c.GetHeader("Origin")
		if origin == "" || !allowed[origin] {
			c.Next()
			return
		}
		h := c.Writer.Header()
		h.Set("Access-Control-Allow-Origin", origin)
		h.Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
		h.Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, Authorization")
		h.Set("Access-Control-Max-Age", "600")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

// SplitList parses a comma-separated env value, dropping empty entries.
func SplitList(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
