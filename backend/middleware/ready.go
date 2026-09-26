package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// DatabaseUnavailable is the error shown to users while the database is down.
const DatabaseUnavailable = "Liftoff can't reach its database right now. Please try again in a minute."

// RequireReady answers 503 until ready reports true, instead of letting requests
// fail deep inside a handler.
func RequireReady(ready func() bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !ready() {
			c.Header("Retry-After", "30")
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": DatabaseUnavailable})
			return
		}
		c.Next()
	}
}
