package auth

import (
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

// adminRouter sets user_email in context (simulating AuthMiddleware) then runs AdminMiddleware
func adminRouter(email string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/admin", func(c *gin.Context) {
		c.Set(UserEmailKey, email)
		c.Next()
	}, AdminMiddleware(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
	return r
}

func TestAdminMiddleware_NonAdmin(t *testing.T) {
	r := adminRouter("user@example.com")
	req := httptest.NewRequest("GET", "/admin", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Errorf("got %d, want 403", w.Code)
	}
}

func TestAdminMiddleware_DefaultAdmin(t *testing.T) {
	os.Unsetenv("ADMIN_EMAILS")
	r := adminRouter("admin@liftoff.local")
	req := httptest.NewRequest("GET", "/admin", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("got %d, want 200", w.Code)
	}
}

func TestAdminMiddleware_EnvOverride(t *testing.T) {
	os.Setenv("ADMIN_EMAILS", "boss@company.com,ops@company.com")
	defer os.Unsetenv("ADMIN_EMAILS")

	cases := []struct {
		email string
		want  int
	}{
		{"boss@company.com", http.StatusOK},
		{"ops@company.com", http.StatusOK},
		{"admin@liftoff.local", http.StatusForbidden}, // default no longer applies
		{"other@company.com", http.StatusForbidden},
	}
	for _, c := range cases {
		r := adminRouter(c.email)
		req := httptest.NewRequest("GET", "/admin", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != c.want {
			t.Errorf("email %q: got %d, want %d", c.email, w.Code, c.want)
		}
	}
}

// --- IsAdminEmail ---

func TestIsAdminEmail(t *testing.T) {
	os.Unsetenv("ADMIN_EMAILS")

	cases := []struct {
		email string
		want  bool
	}{
		{"admin@liftoff.local", true},
		{"ADMIN@LIFTOFF.LOCAL", true},
		{"  admin@liftoff.local  ", true},
		{"user@example.com", false},
		{"", false},
	}
	for _, c := range cases {
		got := IsAdminEmail(c.email)
		if got != c.want {
			t.Errorf("IsAdminEmail(%q) = %v, want %v", c.email, got, c.want)
		}
	}
}

func TestIsAdminEmail_EnvList(t *testing.T) {
	os.Setenv("ADMIN_EMAILS", "alice@co.com, Bob@Co.Com ,charlie@co.com")
	defer os.Unsetenv("ADMIN_EMAILS")

	cases := []struct {
		email string
		want  bool
	}{
		{"alice@co.com", true},
		{"bob@co.com", true},   // case-insensitive
		{"charlie@co.com", true},
		{"dave@co.com", false},
		{"admin@liftoff.local", false}, // default not active when env is set
	}
	for _, c := range cases {
		got := IsAdminEmail(c.email)
		if got != c.want {
			t.Errorf("IsAdminEmail(%q) = %v, want %v", c.email, got, c.want)
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
