package auth

import (
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
)

// AdminMiddleware requires AuthMiddleware and checks if user is admin.
// Admin emails are from ADMIN_EMAILS env (comma-separated) or default admin@liftoff.local
func AdminMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		email, _ := c.Get(UserEmailKey)
		emailStr, ok := email.(string)
		if !ok || !IsAdminEmail(emailStr) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Admin access required"})
			return
		}
		c.Next()
	}
}

// IsAdminEmail returns true if the email is an allowed admin email
func IsAdminEmail(email string) bool {
	allowlist := os.Getenv("ADMIN_EMAILS")
	if allowlist == "" {
		allowlist = "admin@liftoff.local"
	}
	allowed := strings.Split(allowlist, ",")
	emailLower := strings.ToLower(strings.TrimSpace(email))
	for _, a := range allowed {
		if strings.ToLower(strings.TrimSpace(a)) == emailLower {
			return true
		}
	}
	return false
}
