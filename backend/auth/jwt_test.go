package auth

import (
	"os"
	"testing"
	"time"
)

func TestGenerateAndValidateToken(t *testing.T) {
	// Use a fixed secret for testing
	os.Setenv("JWT_SECRET", "test-secret-for-unit-tests")
	defer os.Unsetenv("JWT_SECRET")

	userID := "user-123"
	email := "test@example.com"

	tokenString, expiry, err := GenerateToken(userID, email, false)
	if err != nil {
		t.Fatalf("GenerateToken() error = %v", err)
	}
	if tokenString == "" {
		t.Error("GenerateToken() returned empty token")
	}
	if expiry.Before(time.Now()) {
		t.Error("GenerateToken() expiry should be in the future")
	}

	claims, err := ValidateToken(tokenString)
	if err != nil {
		t.Fatalf("ValidateToken() error = %v", err)
	}
	if claims.UserID != userID {
		t.Errorf("ValidateToken() UserID = %q, want %q", claims.UserID, userID)
	}
	if claims.Email != email {
		t.Errorf("ValidateToken() Email = %q, want %q", claims.Email, email)
	}
}

func TestValidateToken_Invalid(t *testing.T) {
	os.Setenv("JWT_SECRET", "test-secret")
	defer os.Unsetenv("JWT_SECRET")

	_, err := ValidateToken("invalid-token")
	if err != ErrInvalidToken {
		t.Errorf("ValidateToken() error = %v, want ErrInvalidToken", err)
	}

	_, err = ValidateToken("")
	if err != ErrInvalidToken {
		t.Errorf("ValidateToken() error = %v, want ErrInvalidToken for empty", err)
	}
}

func TestGenerateToken_RememberMe(t *testing.T) {
	os.Setenv("JWT_SECRET", "test-secret")
	defer os.Unsetenv("JWT_SECRET")

	_, expiryShort, _ := GenerateToken("u1", "e@e.com", false)
	_, expiryLong, _ := GenerateToken("u1", "e@e.com", true)

	// Remember me should have much longer expiry (default 30 days vs 15 min)
	diffShort := expiryShort.Sub(time.Now())
	diffLong := expiryLong.Sub(time.Now())

	if diffLong <= diffShort {
		t.Errorf("RememberMe token should have longer expiry: short=%v, long=%v", diffShort, diffLong)
	}
}
