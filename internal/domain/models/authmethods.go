// internal/domain/models/authmethods.go
package models

// AuthMethod represents an authentication method option for the UI.
type AuthMethod struct {
	Value string // The value stored in the database
	Label string // The display label in the UI
}

// AllAuthMethods contains all supported auth methods with their display labels.
// This is used for validation and as a reference for all possible values.
var AllAuthMethods = []AuthMethod{
	{Value: "trust", Label: "Trust"},
	{Value: "password", Label: "Password"},
	{Value: "email", Label: "Email Verification"},
	{Value: "google", Label: "Google"},
	// Add more auth methods as they are implemented:
	// {Value: "microsoft", Label: "Microsoft"},
	// {Value: "clever", Label: "Clever"},
	// {Value: "classlink", Label: "Classlink"},
}

// IsValidAuthMethod checks if a value is a valid auth method.
func IsValidAuthMethod(value string) bool {
	for _, m := range AllAuthMethods {
		if m.Value == value {
			return true
		}
	}
	return false
}

// AllAuthMethodValues returns all auth method values as a slice.
func AllAuthMethodValues() []string {
	values := make([]string, len(AllAuthMethods))
	for i, m := range AllAuthMethods {
		values[i] = m.Value
	}
	return values
}
