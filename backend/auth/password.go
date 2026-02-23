package auth

import (
	"errors"
	"regexp"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

var (
	ErrPasswordTooShort       = errors.New("password must be at least 8 characters")
	ErrPasswordNoNumber       = errors.New("password must contain at least one number")
	ErrPasswordNoCapital      = errors.New("password must contain at least one capital letter")
	ErrPasswordNoSpecialChar  = errors.New("password must contain at least one special character")
)

var (
	hasNumber      = regexp.MustCompile(`[0-9]`)
	hasCapital     = regexp.MustCompile(`[A-Z]`)
	hasSpecialChar = regexp.MustCompile("[!@#$%^&*()_+\\-=\\[\\]{};':\"\\\\|,.<>/?~`]")
)

// ValidatePassword checks if password meets requirements:
// - At least 8 characters
// - At least one number
// - At least one capital letter
// - At least one special character
func ValidatePassword(password string) error {
	if len(password) < 8 {
		return ErrPasswordTooShort
	}
	if !hasNumber.MatchString(password) {
		return ErrPasswordNoNumber
	}
	if !hasCapital.MatchString(password) {
		return ErrPasswordNoCapital
	}
	if !hasSpecialChar.MatchString(password) {
		return ErrPasswordNoSpecialChar
	}
	return nil
}

// HashPassword hashes a password using bcrypt
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// CheckPassword verifies a password against a hash
func CheckPassword(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// NormalizeEmail converts email to lowercase for case-insensitive comparison
func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}
