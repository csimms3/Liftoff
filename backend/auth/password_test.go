package auth

import (
	"testing"
)

func TestValidatePassword(t *testing.T) {
	tests := []struct {
		name    string
		password string
		wantErr bool
		err     error
	}{
		{"valid password", "Password1!", false, nil},
		{"valid with symbols", "MyP@ssw0rd!", false, nil},
		{"too short", "Pass1!", true, ErrPasswordTooShort},
		{"no number", "Password!!", true, ErrPasswordNoNumber},
		{"no capital", "password1!", true, ErrPasswordNoCapital},
		{"no special", "Password12", true, ErrPasswordNoSpecialChar},
		{"empty", "", true, ErrPasswordTooShort},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePassword(tt.password)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePassword() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && err != tt.err {
				t.Errorf("ValidatePassword() error = %v, want %v", err, tt.err)
			}
		})
	}
}

func TestHashPasswordAndCheck(t *testing.T) {
	password := "SecurePass1!"
	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}
	if hash == "" {
		t.Error("HashPassword() returned empty hash")
	}
	if hash == password {
		t.Error("HashPassword() should not return plain password")
	}

	if !CheckPassword(password, hash) {
		t.Error("CheckPassword() should return true for correct password")
	}
	if CheckPassword("WrongPass1!", hash) {
		t.Error("CheckPassword() should return false for wrong password")
	}
}

func TestNormalizeEmail(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"User@Example.COM", "user@example.com"},
		{"  test@test.com  ", "test@test.com"},
		{"ALREADYLOWER", "alreadylower"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := NormalizeEmail(tt.input)
			if got != tt.want {
				t.Errorf("NormalizeEmail() = %q, want %q", got, tt.want)
			}
		})
	}
}
