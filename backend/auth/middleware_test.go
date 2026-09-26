package auth

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gin-gonic/gin"
)

func setupMiddlewareRouter(middleware ...gin.HandlerFunc) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	handlers := append(middleware, func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
	r.GET("/test", handlers...)
	return r
}

// --- AuthMiddleware ---

func TestAuthMiddleware_MissingHeader(t *testing.T) {
	r := setupMiddlewareRouter(AuthMiddleware())
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("got %d, want 401", w.Code)
	}
}

func TestAuthMiddleware_BadFormat(t *testing.T) {
	r := setupMiddlewareRouter(AuthMiddleware())

	cases := []string{
		"notbearer token123",
		"Bearer",
		"token123",
		"",
	}
	for _, h := range cases {
		req := httptest.NewRequest("GET", "/test", nil)
		if h != "" {
			req.Header.Set("Authorization", h)
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusUnauthorized {
			t.Errorf("header %q: got %d, want 401", h, w.Code)
		}
	}
}

func TestAuthMiddleware_InvalidToken(t *testing.T) {
	os.Setenv("JWT_SECRET", "test-secret")
	defer os.Unsetenv("JWT_SECRET")

	r := setupMiddlewareRouter(AuthMiddleware())
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer this.is.invalid")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("got %d, want 401", w.Code)
	}
}

func TestAuthMiddleware_ValidToken(t *testing.T) {
	os.Setenv("JWT_SECRET", "test-secret")
	defer os.Unsetenv("JWT_SECRET")

	token, _, err := GenerateToken("user-123", "test@example.com", false)
	if err != nil {
		t.Fatal(err)
	}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/test", AuthMiddleware(), func(c *gin.Context) {
		userID := GetUserID(c)
		c.JSON(http.StatusOK, gin.H{"user_id": userID})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("got %d, want 200", w.Code)
	}
	if !contains(w.Body.String(), "user-123") {
		t.Errorf("response %q should contain user_id", w.Body.String())
	}
}

// --- AdminMiddleware ---

// adminRouter sets user_id in context (simulating AuthMiddleware) then runs AdminMiddleware
func adminRouter(userID string, check AdminChecker) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/admin", func(c *gin.Context) {
		if userID != "" {
			c.Set(UserIDKey, userID)
		}
		c.Next()
	}, AdminMiddleware(check), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
	return r
}

func TestAdminMiddleware(t *testing.T) {
	admins := map[string]bool{"u-admin": true}
	check := func(_ context.Context, id string) (bool, error) { return admins[id], nil }

	cases := []struct {
		name   string
		userID string
		check  AdminChecker
		want   int
	}{
		{"admin flag set", "u-admin", check, http.StatusOK},
		{"admin flag unset", "u-user", check, http.StatusForbidden},
		{"no user in context", "", check, http.StatusForbidden},
		{"lookup error", "u-admin", func(context.Context, string) (bool, error) {
			return false, errors.New("db down")
		}, http.StatusInternalServerError},
	}
	for _, c := range cases {
		r := adminRouter(c.userID, c.check)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("GET", "/admin", nil))
		if w.Code != c.want {
			t.Errorf("%s: got %d, want %d", c.name, w.Code, c.want)
		}
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 ||
		func() bool {
			for i := 0; i <= len(s)-len(sub); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
			return false
		}())
}
