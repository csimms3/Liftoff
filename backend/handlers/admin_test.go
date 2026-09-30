package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"liftoff/backend/internal/testdb"
	"liftoff/backend/repository"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

// newTestDB returns a fresh, migrated Postgres schema.
func newTestDB(t *testing.T) *pgxpool.Pool {
	return testdb.Postgres(t)
}

func setupAdminRouter(db *pgxpool.Pool) (*gin.Engine, *AdminHandler) {
	gin.SetMode(gin.TestMode)
	userRepo := repository.NewUserRepository(db)
	adminRepo := repository.NewAdminRepository(db)
	handler := NewAdminHandler(userRepo, adminRepo)
	r := gin.New()
	r.GET("/admin/users", handler.ListUsers)
	r.GET("/admin/stats", handler.GetStats)
	return r, handler
}

func TestListUsers_Empty(t *testing.T) {
	db := newTestDB(t)
	r, _ := setupAdminRouter(db)

	req := httptest.NewRequest(http.MethodGet, "/admin/users", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("got %d, want 200. body: %s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	users, ok := resp["users"]
	if !ok {
		t.Fatal("response missing 'users' key")
	}
	list, ok := users.([]interface{})
	if !ok {
		t.Fatalf("expected users to be an array, got %T: %v", users, users)
	}
	if len(list) != 0 {
		t.Errorf("expected empty users list, got %d items", len(list))
	}
}

func TestListUsers_WithData(t *testing.T) {
	db := newTestDB(t)
	_, err := db.Exec(context.Background(), `INSERT INTO users (id, email, password_hash) VALUES ($1,$2,$3)`,
		"u1", "alice@example.com", "hash1")
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(context.Background(), `INSERT INTO users (id, email, password_hash) VALUES ($1,$2,$3)`,
		"u2", "bob@example.com", "hash2")
	if err != nil {
		t.Fatal(err)
	}

	r, _ := setupAdminRouter(db)
	req := httptest.NewRequest(http.MethodGet, "/admin/users", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("got %d, want 200. body: %s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	list := resp["users"].([]interface{})
	if len(list) != 2 {
		t.Errorf("expected 2 users, got %d", len(list))
	}
}

func TestGetStats_Empty(t *testing.T) {
	db := newTestDB(t)
	r, _ := setupAdminRouter(db)

	req := httptest.NewRequest(http.MethodGet, "/admin/stats", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("got %d, want 200. body: %s", w.Code, w.Body.String())
	}
	var stats map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &stats); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"total_users", "total_workouts", "total_sessions", "new_users_7d"} {
		if _, ok := stats[key]; !ok {
			t.Errorf("response missing key %q", key)
		}
	}
	if stats["total_users"].(float64) != 0 {
		t.Errorf("expected 0 total_users, got %v", stats["total_users"])
	}
}

func TestGetStats_WithData(t *testing.T) {
	db := newTestDB(t)
	testdb.Exec(t, db,
		`INSERT INTO users (id, email, password_hash) VALUES ('u1','a@b.com','h')`,
		`INSERT INTO routines (id, user_id, name) VALUES ('r1','u1','R')`,
		`INSERT INTO workouts (id, name, user_id, routine_id) VALUES ('w1','Workout A','u1','r1')`,
		`INSERT INTO workouts (id, name, user_id, routine_id) VALUES ('w2','Workout B','u1','r1')`,
		`INSERT INTO workout_sessions (id, workout_id, user_id) VALUES ('s1','w1','u1')`,
	)

	r, _ := setupAdminRouter(db)
	req := httptest.NewRequest(http.MethodGet, "/admin/stats", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("got %d, want 200", w.Code)
	}
	var stats map[string]float64
	json.Unmarshal(w.Body.Bytes(), &stats)
	if stats["total_users"] != 1 {
		t.Errorf("total_users = %v, want 1", stats["total_users"])
	}
	if stats["total_workouts"] != 2 {
		t.Errorf("total_workouts = %v, want 2", stats["total_workouts"])
	}
	if stats["total_sessions"] != 1 {
		t.Errorf("total_sessions = %v, want 1", stats["total_sessions"])
	}
}
