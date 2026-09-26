package auth

import (
	"os"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
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
	diffShort := time.Until(expiryShort)
	diffLong := time.Until(expiryLong)

	if diffLong <= diffShort {
		t.Errorf("RememberMe token should have longer expiry: short=%v, long=%v", diffShort, diffLong)
	}
}

func TestCheckConfig(t *testing.T) {
	cases := []struct {
		secret string
		ok     bool
	}{
		{"", false},
		{"short", false},
		{"0123456789abcdef0123456789abcdef", true},
	}
	for _, c := range cases {
		t.Setenv("JWT_SECRET", c.secret)
		if err := CheckConfig(); (err == nil) != c.ok {
			t.Errorf("CheckConfig() with %d-char secret: err = %v, want ok=%v", len(c.secret), err, c.ok)
		}
	}
}

func TestGenerateToken_NoSecret(t *testing.T) {
	t.Setenv("JWT_SECRET", "")
	if _, _, err := GenerateToken("u1", "e@e.com", false); err != ErrNoSecret {
		t.Errorf("GenerateToken() without secret: err = %v, want ErrNoSecret", err)
	}
}

func TestValidateToken_RejectsOtherAlgorithms(t *testing.T) {
	t.Setenv("JWT_SECRET", "test-secret")
	claims := Claims{UserID: "u1", RegisteredClaims: jwt.RegisteredClaims{
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
	}}
	for _, method := range []jwt.SigningMethod{jwt.SigningMethodHS384, jwt.SigningMethodHS512} {
		s, err := jwt.NewWithClaims(method, claims).SignedString([]byte("test-secret"))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := ValidateToken(s); err != ErrInvalidToken {
			t.Errorf("%s token accepted, want ErrInvalidToken", method.Alg())
		}
	}
	none, _ := jwt.NewWithClaims(jwt.SigningMethodNone, claims).SignedString(jwt.UnsafeAllowNoneSignatureType)
	if _, err := ValidateToken(none); err != ErrInvalidToken {
		t.Error("alg=none token accepted")
	}
}
