// internal/app/system/authutil/password.go
package authutil

import (
	"errors"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

// Password validation constants
const (
	MinPasswordLength = 6
	MaxPasswordLength = 128
	BcryptCost        = 12
)

// Password validation errors
var (
	ErrPasswordTooShort = errors.New("Password must be at least 6 characters.")
	ErrPasswordTooLong  = errors.New("Password must be less than 128 characters.")
	ErrPasswordCommon   = errors.New("This password is too common. Please choose a different one.")
)

// commonPasswords is a list of very common passwords that are blocked.
var commonPasswords = map[string]bool{
	"123456":    true,
	"1234567":   true,
	"12345678":  true,
	"123456789": true,
	"password":  true,
	"password1": true,
	"qwerty":    true,
	"qwerty123": true,
	"abc123":    true,
	"abcdef":    true,
	"111111":    true,
	"000000":    true,
	"123123":    true,
	"654321":    true,
	"iloveyou":  true,
	"monkey":    true,
	"dragon":    true,
	"master":    true,
	"letmein":   true,
	"welcome":   true,
	"login":     true,
	"admin":     true,
	"princess":  true,
	"sunshine":  true,
	"football":  true,
	"baseball":  true,
	"soccer":    true,
	"hockey":    true,
	"batman":    true,
	"superman":  true,
}

// PasswordRules returns a human-readable description of the password rules.
// This can be displayed on password change forms.
func PasswordRules() string {
	return "Password must be at least 6 characters and cannot be a common password like \"123456\" or \"password\"."
}

// ValidatePassword checks if a password meets the requirements.
// Returns nil if valid, or an error describing the issue.
func ValidatePassword(password string) error {
	if len(password) < MinPasswordLength {
		return ErrPasswordTooShort
	}
	if len(password) > MaxPasswordLength {
		return ErrPasswordTooLong
	}

	// Check against common passwords (case-insensitive)
	if commonPasswords[strings.ToLower(password)] {
		return ErrPasswordCommon
	}

	return nil
}

// HashPassword hashes a password using bcrypt.
// The password should be validated with ValidatePassword first.
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), BcryptCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// CheckPassword compares a plain-text password with a bcrypt hash.
// Returns true if the password matches, false otherwise.
func CheckPassword(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}
