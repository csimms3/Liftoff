package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"liftoff/backend/auth"
	"liftoff/backend/internal/testdb"
	"liftoff/backend/repository"

	"github.com/gin-gonic/gin"
)

func TestForgotPassword_LockedAccountGetsNoToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := testdb.SQLite(t).GetSQLite()
	if _, err := db.Exec(`INSERT INTO users (id, email, password_hash) VALUES ('locked', 'old@example.com', ?), ('open', 'me@example.com', 'x')`,
		auth.LockedPasswordHash); err != nil {
		t.Fatal(err)
	}
	h := NewAuthHandler(repository.NewUserRepository(nil, db, true))
	r := gin.New()
	r.POST("/forgot", h.ForgotPassword)

	for _, email := range []string{"old@example.com", "me@example.com"} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/forgot", strings.NewReader(`{"email":"`+email+`"}`)))
		if w.Code != http.StatusOK {
			t.Fatalf("%s: got %d, want 200 (same response either way)", email, w.Code)
		}
	}
	var lockedTokens, openTokens int
	db.QueryRow("SELECT COUNT(*) FROM password_reset_tokens WHERE user_id = 'locked'").Scan(&lockedTokens)
	db.QueryRow("SELECT COUNT(*) FROM password_reset_tokens WHERE user_id = 'open'").Scan(&openTokens)
	if lockedTokens != 0 {
		t.Errorf("locked account got %d reset tokens, want 0", lockedTokens)
	}
	if openTokens != 1 {
		t.Errorf("normal account got %d reset tokens, want 1", openTokens)
	}
}
