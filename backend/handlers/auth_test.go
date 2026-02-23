package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"liftoff/backend/auth"
	"liftoff/backend/repository"

	"github.com/gin-gonic/gin"
)

func setupAuthTest(t *testing.T) (*AuthHandler, *gin.Engine) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	userRepo := repository.NewUserRepository(nil, nil, true) // useSQLite true but nil - we'll need a proper test DB
	// For now we test validation logic without DB
	handler := NewAuthHandler(userRepo)
	return handler, r
}

func TestLoginRequest_Validation(t *testing.T) {
	handler, r := setupAuthTest(t)
	r.POST("/login", handler.Login)

	tests := []struct {
		name       string
		body       map[string]interface{}
		wantStatus int
	}{
		{"missing email", map[string]interface{}{"password": "Pass123!"}, http.StatusBadRequest},
		{"missing password", map[string]interface{}{"email": "test@test.com"}, http.StatusBadRequest},
		{"invalid email", map[string]interface{}{"email": "bad", "password": "Pass123!"}, http.StatusBadRequest},
		{"empty body", map[string]interface{}{}, http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.body)
			req := httptest.NewRequest(http.MethodPost, "/login", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != tt.wantStatus {
				t.Errorf("got status %d, want %d", w.Code, tt.wantStatus)
			}
		})
	}
}

func TestRegisterRequest_Validation(t *testing.T) {
	handler, r := setupAuthTest(t)
	r.POST("/register", handler.Register)

	tests := []struct {
		name       string
		body       map[string]interface{}
		wantStatus int
	}{
		{"missing email", map[string]interface{}{"password": "Pass123!"}, http.StatusBadRequest},
		{"missing password", map[string]interface{}{"email": "test@test.com"}, http.StatusBadRequest},
		{"invalid email", map[string]interface{}{"email": "bad", "password": "Pass123!"}, http.StatusBadRequest},
		{"short password", map[string]interface{}{"email": "test@test.com", "password": "Short1!"}, http.StatusBadRequest},
		{"no number", map[string]interface{}{"email": "test@test.com", "password": "Password!"}, http.StatusBadRequest},
		{"no capital", map[string]interface{}{"email": "test@test.com", "password": "password1!"}, http.StatusBadRequest},
		{"no special", map[string]interface{}{"email": "test@test.com", "password": "Password12"}, http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.body)
			req := httptest.NewRequest(http.MethodPost, "/register", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != tt.wantStatus {
				t.Errorf("got status %d, want %d. body: %s", w.Code, tt.wantStatus, w.Body.String())
			}
		})
	}
}

func TestForgotPasswordRequest_Validation(t *testing.T) {
	handler, r := setupAuthTest(t)
	r.POST("/forgot", handler.ForgotPassword)

	tests := []struct {
		name       string
		body       map[string]interface{}
		wantStatus int
	}{
		{"missing email", map[string]interface{}{}, http.StatusBadRequest},
		{"invalid email", map[string]interface{}{"email": "bad"}, http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.body)
			req := httptest.NewRequest(http.MethodPost, "/forgot", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != tt.wantStatus {
				t.Errorf("got status %d, want %d", w.Code, tt.wantStatus)
			}
		})
	}
}

func TestResetPasswordRequest_Validation(t *testing.T) {
	handler, r := setupAuthTest(t)
	r.POST("/reset", handler.ResetPassword)

	tests := []struct {
		name       string
		body       map[string]interface{}
		wantStatus int
	}{
		{"missing token", map[string]interface{}{"newPassword": "Pass123!"}, http.StatusBadRequest},
		{"missing password", map[string]interface{}{"token": "abc123"}, http.StatusBadRequest},
		{"invalid password", map[string]interface{}{"token": "abc", "newPassword": "short"}, http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.body)
			req := httptest.NewRequest(http.MethodPost, "/reset", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != tt.wantStatus {
				t.Errorf("got status %d, want %d", w.Code, tt.wantStatus)
			}
		})
	}
}

func TestHashToken(t *testing.T) {
	hash1 := auth.HashToken("test-token")
	hash2 := auth.HashToken("test-token")
	if hash1 != hash2 {
		t.Error("HashToken should be deterministic")
	}
	if len(hash1) != 64 {
		t.Errorf("SHA256 hash should be 64 hex chars, got %d", len(hash1))
	}
}
