package auth

import (
	"context"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

// AdminChecker reports whether a user has admin rights (users.is_admin).
type AdminChecker func(ctx context.Context, userID string) (bool, error)

// AdminMiddleware requires AuthMiddleware and checks the user's is_admin flag in the
// database on every request, so revoking admin takes effect without new tokens.
// Admins are granted by hand: UPDATE users SET is_admin = true WHERE email = '...'.
func AdminMiddleware(isAdmin AdminChecker) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := GetUserID(c)
		if userID == "" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Admin access required"})
			return
		}
		ok, err := isAdmin(c.Request.Context(), userID)
		if err != nil {
			log.Printf("admin check for %s: %v", userID, err)
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "Failed to check admin access"})
			return
		}
		if !ok {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Admin access required"})
			return
		}
		c.Next()
	}
}
