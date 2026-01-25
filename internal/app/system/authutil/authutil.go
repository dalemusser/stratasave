// internal/app/system/authutil/authutil.go
// Package authutil provides centralized authentication field handling
// for user creation and editing forms.
package authutil

// Terminology: User Identifiers
//   - UserID / userID / user_id: The MongoDB ObjectID (_id) that uniquely identifies a user record
//   - LoginID / loginID / login_id: The human-readable string users type to log in

import (
	"errors"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

// Auth method behavior groups - single source of truth
var (
	// EmailIsLoginMethods are auth methods where email IS the login identity
	// (no separate Login ID field needed)
	EmailIsLoginMethods = map[string]bool{
		"email":  true,
		"google": true,
	}
)

// EmailIsLogin returns true if the given auth method uses email as the login identity.
func EmailIsLogin(method string) bool {
	return EmailIsLoginMethods[method]
}

// AuthInput holds the raw form values for auth-related fields.
type AuthInput struct {
	Method       string
	LoginID      string
	Email        string
	TempPassword string
	IsEdit       bool // If true, password is optional (leave blank to keep existing)
}

// AuthResult holds the validated and processed auth fields ready for storage.
type AuthResult struct {
	EffectiveLoginID string  // The login_id to store (either LoginID or Email depending on method)
	Email            *string // Optional email (set if provided)
	PasswordHash     *string // bcrypt hash (set if password provided)
	PasswordTemp     *bool   // true if password is temporary (set if password provided)
}

// Common validation errors
var (
	ErrEmailRequired    = errors.New("Email is required for this authentication method.")
	ErrInvalidEmail     = errors.New("Please enter a valid email address.")
	ErrLoginIDRequired  = errors.New("Login ID is required.")
	ErrPasswordRequired = errors.New("Temporary password is required for password authentication.")
)

// isValidEmail performs a basic email format validation.
// It checks for the presence of @ and at least one character on each side.
func isValidEmail(email string) bool {
	// Basic validation: must have @ with text on both sides
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return false
	}
	// Local part must not be empty
	if len(parts[0]) == 0 {
		return false
	}
	// Domain must contain at least one dot after @
	domain := parts[1]
	dotIdx := strings.LastIndex(domain, ".")
	if dotIdx < 1 || dotIdx >= len(domain)-1 {
		return false
	}
	return true
}

// ValidateAndResolve validates the auth input based on the selected method
// and returns the resolved fields ready for storage.
func ValidateAndResolve(input AuthInput) (*AuthResult, error) {
	result := &AuthResult{}

	// Determine effective login ID based on auth method
	if EmailIsLogin(input.Method) {
		// For email/google: email is required and becomes login_id
		if input.Email == "" {
			return nil, ErrEmailRequired
		}
		// Validate email format
		if !isValidEmail(input.Email) {
			return nil, ErrInvalidEmail
		}
		result.EffectiveLoginID = input.Email
	} else {
		// For other methods: login_id is required
		if input.LoginID == "" {
			return nil, ErrLoginIDRequired
		}
		result.EffectiveLoginID = input.LoginID
	}

	// Validate password for password auth method
	if input.Method == "password" && input.TempPassword == "" && !input.IsEdit {
		return nil, ErrPasswordRequired
	}

	// Set optional email if provided (for non-email-login methods)
	if input.Email != "" {
		result.Email = &input.Email
	}

	// Hash password if provided (for password auth method)
	if input.Method == "password" && input.TempPassword != "" {
		hash, err := bcrypt.GenerateFromPassword([]byte(input.TempPassword), 12)
		if err != nil {
			return nil, err
		}
		hashStr := string(hash)
		result.PasswordHash = &hashStr
		tempFlag := true
		result.PasswordTemp = &tempFlag
	}

	return result, nil
}

// TemplateData holds data needed for rendering auth fields in templates.
// Embed this in your form data struct.
type TemplateData struct {
	LoginID      string
	Email        string
	TempPassword string
	Auth         string // current auth method value
}

// EmailIsLoginMethod returns true if the current auth method uses email as login.
// This is a template helper method.
func (d TemplateData) EmailIsLoginMethod() bool {
	return EmailIsLogin(d.Auth)
}

// IsPasswordMethod returns true if the current auth method is password.
// This is a template helper method.
func (d TemplateData) IsPasswordMethod() bool {
	return d.Auth == "password"
}
