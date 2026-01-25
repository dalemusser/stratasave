package authutil

import (
	"strings"
	"testing"
)

func TestValidatePassword(t *testing.T) {
	tests := []struct {
		name     string
		password string
		wantErr  error
	}{
		// Valid passwords
		{"valid short", "abc123x", nil},
		{"valid medium", "mySecurePassword", nil},
		{"valid long", strings.Repeat("a", 128), nil},
		{"valid with special chars", "P@ssw0rd!123", nil},
		{"valid with spaces", "my secret password", nil},

		// Too short
		{"too short 5 chars", "abcde", ErrPasswordTooShort},
		{"too short 1 char", "a", ErrPasswordTooShort},
		{"too short empty", "", ErrPasswordTooShort},

		// Too long
		{"too long", strings.Repeat("a", 129), ErrPasswordTooLong},

		// Common passwords
		{"common 123456", "123456", ErrPasswordCommon},
		{"common password", "password", ErrPasswordCommon},
		{"common PASSWORD uppercase", "PASSWORD", ErrPasswordCommon},
		{"common qwerty", "qwerty", ErrPasswordCommon},
		{"common iloveyou", "iloveyou", ErrPasswordCommon},
		{"common welcome", "welcome", ErrPasswordCommon},
		{"common football", "football", ErrPasswordCommon},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePassword(tt.password)
			if err != tt.wantErr {
				t.Errorf("ValidatePassword(%q) = %v, want %v", tt.password, err, tt.wantErr)
			}
		})
	}
}

func TestHashPassword(t *testing.T) {
	password := "mySecurePassword123"

	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}

	// Hash should be non-empty
	if hash == "" {
		t.Error("HashPassword() returned empty hash")
	}

	// Hash should not equal the original password
	if hash == password {
		t.Error("HashPassword() returned unhashed password")
	}

	// Hash should start with bcrypt prefix
	if !strings.HasPrefix(hash, "$2") {
		t.Errorf("HashPassword() hash does not appear to be bcrypt: %s", hash)
	}

	// Same password should produce different hashes (bcrypt uses salt)
	hash2, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword() second call error = %v", err)
	}
	if hash == hash2 {
		t.Error("HashPassword() should produce different hashes for same password (due to salt)")
	}
}

func TestCheckPassword(t *testing.T) {
	password := "mySecurePassword123"
	wrongPassword := "wrongPassword456"

	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}

	tests := []struct {
		name     string
		password string
		hash     string
		want     bool
	}{
		{"correct password", password, hash, true},
		{"wrong password", wrongPassword, hash, false},
		{"empty password", "", hash, false},
		{"empty hash", password, "", false},
		{"invalid hash format", password, "not-a-valid-hash", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CheckPassword(tt.password, tt.hash)
			if got != tt.want {
				t.Errorf("CheckPassword(%q, hash) = %v, want %v", tt.password, got, tt.want)
			}
		})
	}
}

func TestPasswordRoundTrip(t *testing.T) {
	// Test that a password can be hashed and then verified
	// Note: bcrypt has a 72-byte limit, so we avoid testing near that limit
	// for the "wrong password" check (since truncation would make them match)
	passwords := []string{
		"simple123",
		"Complex!P@ssw0rd#123",
		"with spaces in it",
		"unicode: \u00e9\u00e0\u00fc",
		strings.Repeat("a", 50), // well under bcrypt limit
	}

	for _, password := range passwords {
		t.Run(password[:min(20, len(password))], func(t *testing.T) {
			hash, err := HashPassword(password)
			if err != nil {
				t.Fatalf("HashPassword() error = %v", err)
			}

			if !CheckPassword(password, hash) {
				t.Error("CheckPassword() failed to verify correct password")
			}

			if CheckPassword(password+"x", hash) {
				t.Error("CheckPassword() incorrectly verified wrong password")
			}
		})
	}
}

func TestPasswordRules(t *testing.T) {
	rules := PasswordRules()
	if rules == "" {
		t.Error("PasswordRules() returned empty string")
	}
	if !strings.Contains(rules, "6") {
		t.Error("PasswordRules() should mention minimum length of 6")
	}
}

func TestValidatePassword_AllCommonPasswords(t *testing.T) {
	// Test common passwords that are >= 6 characters (min length)
	// Shorter common passwords like "login" (5 chars) and "admin" (5 chars)
	// will fail the length check first, which is correct behavior
	commonPasswords := []string{
		"123456", "1234567", "12345678", "123456789",
		"password", "password1", "qwerty", "qwerty123",
		"abc123", "abcdef", "111111", "000000",
		"123123", "654321", "iloveyou", "monkey",
		"dragon", "master", "letmein", "welcome",
		"princess", "sunshine",
		"football", "baseball", "soccer", "hockey",
		"batman", "superman",
	}

	for _, pwd := range commonPasswords {
		t.Run(pwd, func(t *testing.T) {
			err := ValidatePassword(pwd)
			if err != ErrPasswordCommon {
				t.Errorf("ValidatePassword(%q) = %v, want ErrPasswordCommon", pwd, err)
			}
		})
	}
}

func TestValidatePassword_ShortCommonPasswords(t *testing.T) {
	// Common passwords that are < 6 chars should fail length check first
	shortPasswords := []string{"login", "admin"}

	for _, pwd := range shortPasswords {
		t.Run(pwd, func(t *testing.T) {
			err := ValidatePassword(pwd)
			// These fail the length check before the common password check
			if err != ErrPasswordTooShort {
				t.Errorf("ValidatePassword(%q) = %v, want ErrPasswordTooShort", pwd, err)
			}
		})
	}
}

func TestValidatePassword_CaseInsensitive(t *testing.T) {
	// Common passwords should be blocked regardless of case
	tests := []string{
		"PASSWORD",
		"Password",
		"pAsSwOrD",
		"QWERTY",
		"Qwerty",
		"ILOVEYOU",
		"ILoveYou",
	}

	for _, pwd := range tests {
		t.Run(pwd, func(t *testing.T) {
			err := ValidatePassword(pwd)
			if err != ErrPasswordCommon {
				t.Errorf("ValidatePassword(%q) = %v, want ErrPasswordCommon", pwd, err)
			}
		})
	}
}

func TestValidatePassword_BoundaryLengths(t *testing.T) {
	// Test exactly at boundaries
	tests := []struct {
		name    string
		length  int
		wantErr error
	}{
		{"exactly min-1", MinPasswordLength - 1, ErrPasswordTooShort},
		{"exactly min", MinPasswordLength, nil},
		{"exactly min+1", MinPasswordLength + 1, nil},
		{"exactly max-1", MaxPasswordLength - 1, nil},
		{"exactly max", MaxPasswordLength, nil},
		{"exactly max+1", MaxPasswordLength + 1, ErrPasswordTooLong},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use 'x' to avoid common password detection
			pwd := strings.Repeat("x", tt.length)
			err := ValidatePassword(pwd)
			if err != tt.wantErr {
				t.Errorf("ValidatePassword(len=%d) = %v, want %v", tt.length, err, tt.wantErr)
			}
		})
	}
}

func TestHashPassword_DifferentPasswords(t *testing.T) {
	passwords := []string{
		"simple123456",
		"WithSpecial!@#$%",
		"1234567890abc",
		"unicode\u00e9\u00e0\u00fc123",
	}

	for _, pwd := range passwords {
		t.Run(pwd[:min(10, len(pwd))], func(t *testing.T) {
			hash, err := HashPassword(pwd)
			if err != nil {
				t.Fatalf("HashPassword() error = %v", err)
			}
			if hash == "" {
				t.Error("HashPassword() returned empty hash")
			}
			if !CheckPassword(pwd, hash) {
				t.Error("CheckPassword() failed to verify")
			}
		})
	}
}

func TestCheckPassword_EmptyInputs(t *testing.T) {
	hash, _ := HashPassword("validpassword123")

	tests := []struct {
		name     string
		password string
		hash     string
		want     bool
	}{
		{"both empty", "", "", false},
		{"empty password", "", hash, false},
		{"empty hash", "validpassword123", "", false},
		{"both valid", "validpassword123", hash, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CheckPassword(tt.password, tt.hash)
			if got != tt.want {
				t.Errorf("CheckPassword() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestConstants(t *testing.T) {
	// Verify constants have expected values
	if MinPasswordLength != 6 {
		t.Errorf("MinPasswordLength = %d, want 6", MinPasswordLength)
	}
	if MaxPasswordLength != 128 {
		t.Errorf("MaxPasswordLength = %d, want 128", MaxPasswordLength)
	}
	if BcryptCost != 12 {
		t.Errorf("BcryptCost = %d, want 12", BcryptCost)
	}
}

func TestErrorMessages(t *testing.T) {
	// Verify error messages are user-friendly
	if !strings.Contains(ErrPasswordTooShort.Error(), "6") {
		t.Error("ErrPasswordTooShort should mention minimum length")
	}
	if !strings.Contains(ErrPasswordTooLong.Error(), "128") {
		t.Error("ErrPasswordTooLong should mention maximum length")
	}
	if !strings.Contains(ErrPasswordCommon.Error(), "common") {
		t.Error("ErrPasswordCommon should mention 'common'")
	}
}
