// Package normalize provides helper functions for consistent string normalization
// across the application. Use these helpers instead of scattered strings.ToLower
// and strings.TrimSpace calls to ensure consistent behavior.
package normalize

import "strings"

// Email normalizes an email address by trimming whitespace and converting to lowercase.
// This is the canonical way to normalize emails before storage or comparison.
func Email(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

// Name normalizes a name by trimming whitespace.
// Use text.Fold() for case-insensitive comparison keys.
func Name(s string) string {
	return strings.TrimSpace(s)
}

// AuthMethod normalizes an auth method by trimming whitespace and converting to lowercase.
func AuthMethod(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

// Status normalizes a status value by trimming whitespace and converting to lowercase.
func Status(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

// Role normalizes a role value by trimming whitespace and converting to lowercase.
func Role(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

// QueryParam normalizes a query parameter by trimming whitespace.
func QueryParam(s string) string {
	return strings.TrimSpace(s)
}
