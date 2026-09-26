package auth

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var (
	ErrInvalidToken = errors.New("invalid or expired token")
	ErrNoSecret     = errors.New("JWT_SECRET is not set")
)

// MinSecretLength is the shortest JWT_SECRET the server will start with.
const MinSecretLength = 32

const (
	DefaultTokenExpiryMinutes   = 720 // 12h, so nobody is logged out mid-workout
	DefaultRememberMeExpiryDays = 30
)

type Claims struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
	jwt.RegisteredClaims
}

// TokenConfig holds JWT configuration
type TokenConfig struct {
	Secret               []byte
	ExpiryMinutes        int
	RememberMeExpiryDays int
}

// GetTokenConfig loads JWT config from environment
func GetTokenConfig() TokenConfig {
	secret := os.Getenv("JWT_SECRET")

	expiryMinutes, _ := strconv.Atoi(os.Getenv("JWT_EXPIRY_MINUTES"))
	if expiryMinutes <= 0 {
		expiryMinutes = DefaultTokenExpiryMinutes
	}

	rememberMeDays, _ := strconv.Atoi(os.Getenv("JWT_REMEMBER_ME_DAYS"))
	if rememberMeDays <= 0 {
		rememberMeDays = DefaultRememberMeExpiryDays
	}

	return TokenConfig{
		Secret:               []byte(secret),
		ExpiryMinutes:        expiryMinutes,
		RememberMeExpiryDays: rememberMeDays,
	}
}

// CheckConfig reports a missing or weak JWT_SECRET. The server refuses to start
// without one: a known secret would let anyone mint tokens.
func CheckConfig() error {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		return ErrNoSecret
	}
	if len(secret) < MinSecretLength {
		return fmt.Errorf("JWT_SECRET must be at least %d characters", MinSecretLength)
	}
	return nil
}

// GenerateToken creates a JWT for the user
func GenerateToken(userID, email string, rememberMe bool) (string, time.Time, error) {
	config := GetTokenConfig()
	if len(config.Secret) == 0 {
		return "", time.Time{}, ErrNoSecret
	}

	var expiry time.Time
	if rememberMe {
		expiry = time.Now().Add(time.Duration(config.RememberMeExpiryDays) * 24 * time.Hour)
	} else {
		expiry = time.Now().Add(time.Duration(config.ExpiryMinutes) * time.Minute)
	}

	claims := Claims{
		UserID: userID,
		Email:  email,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiry),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(config.Secret)
	if err != nil {
		return "", time.Time{}, err
	}

	return tokenString, expiry, nil
}

// ValidateToken parses and validates a JWT, returning the claims
func ValidateToken(tokenString string) (*Claims, error) {
	config := GetTokenConfig()
	if len(config.Secret) == 0 {
		return nil, ErrInvalidToken
	}

	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		return config.Secret, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))

	if err != nil {
		return nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, ErrInvalidToken
	}

	return claims, nil
}
