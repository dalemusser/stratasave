// Package status provides canonical status values used throughout the application.
//
// Using these constants instead of string literals ensures consistency and
// makes refactoring easier. The constants are plain strings (not a custom type)
// for compatibility with existing code and MongoDB queries.
package status

// Entity status values used for users and other entities.
const (
	Active   = "active"
	Disabled = "disabled"
)

// IsValid returns true if s is a recognized status value.
func IsValid(s string) bool {
	return s == Active || s == Disabled
}

// Default returns the default status for new entities.
func Default() string {
	return Active
}
