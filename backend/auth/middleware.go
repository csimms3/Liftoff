package auth

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

const UserIDKey = "user_id"
const UserEmailKey = "user_email"

// AuthMiddleware validates JWT and sets user context
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Authorization header required"})
			return
		}

		// Support "Bearer <token>" format
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid authorization format"})
			return
		}

		tokenString := parts[1]
		claims, err := ValidateToken(tokenString)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
			return
		}

		c.Set(UserIDKey, claims.UserID)
		c.Set(UserEmailKey, claims.Email)
		c.Next()
	}
}

// GetUserID extracts user ID from gin context (call after AuthMiddleware)
func GetUserID(c *gin.Context) string {
	userID, _ := c.Get(UserIDKey)
	if id, ok := userID.(string); ok {
		return id
	}
	return ""
}
