// Package inputval provides form input validation using waffle/pantry/validate.
//
// This package wraps pantry/validate to provide a convenient interface for
// validating HTTP form inputs with struct tags. Define an input struct with
// validate tags, populate it from form values, and call Validate to get
// user-friendly error messages.
//
// Example:
//
//	type CreateUserInput struct {
//	    FullName string `validate:"required" label:"Full name"`
//	    Email    string `validate:"required,email" label:"Email"`
//	}
//
//	input := CreateUserInput{
//	    FullName: r.FormValue("full_name"),
//	    Email:    r.FormValue("email"),
//	}
//
//	if err := inputval.Validate(input); err != nil {
//	    // err.First() gives the first error message for display
//	    renderWithError(w, r, err.First())
//	    return
//	}
package inputval

import (
	"net/mail"
	"net/url"
	"reflect"
	"strings"
	"sync"

	"github.com/dalemusser/stratasave/internal/domain/models"
	"github.com/dalemusser/waffle/pantry/validate"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Result holds validation results with user-friendly messages.
type Result struct {
	Errors []FieldError
}

// FieldError represents a validation error for a single field.
type FieldError struct {
	Field   string
	Label   string
	Message string
}

// HasErrors returns true if there are any validation errors.
func (r *Result) HasErrors() bool {
	return len(r.Errors) > 0
}

// First returns the first error message, or empty string if no errors.
func (r *Result) First() string {
	if len(r.Errors) > 0 {
		return r.Errors[0].Message
	}
	return ""
}

// All returns all error messages joined with "; ".
func (r *Result) All() string {
	if len(r.Errors) == 0 {
		return ""
	}
	msgs := make([]string, len(r.Errors))
	for i, e := range r.Errors {
		msgs[i] = e.Message
	}
	return strings.Join(msgs, "; ")
}

// customValidator is a singleton validator with custom rules registered.
var (
	customValidator *validate.Validator
	validatorOnce   sync.Once
)

// getValidator returns the singleton validator with custom rules.
func getValidator() *validate.Validator {
	validatorOnce.Do(func() {
		customValidator = validate.New(validate.WithStopOnFirstError())

		// authmethod: validates against AllowedAuthMethods
		customValidator.RegisterRuleFunc("authmethod", func(value any) bool {
			if s, ok := value.(string); ok {
				return IsValidAuthMethod(s)
			}
			return false
		}, "authmethod")

		// httpurl: validates that string is a valid http/https URL
		customValidator.RegisterRuleFunc("httpurl", func(value any) bool {
			if s, ok := value.(string); ok {
				return IsValidHTTPURL(s)
			}
			return false
		}, "httpurl")

		// objectid: validates that string is a valid MongoDB ObjectID hex
		customValidator.RegisterRuleFunc("objectid", func(value any) bool {
			if s, ok := value.(string); ok {
				return IsValidObjectID(s)
			}
			return false
		}, "objectid")
	})
	return customValidator
}

// Validate validates a struct and returns a Result with user-friendly errors.
// The struct should have `validate` tags for rules and optional `label` tags
// for user-friendly field names.
//
// Supported validation rules (from pantry/validate):
//   - required: field must not be empty
//   - email: field must be a valid email address
//   - oneof=a b c: field must be one of the specified values
//   - timezone: field must be a valid IANA time zone
//   - min=N: string length or numeric value must be >= N
//   - max=N: string length or numeric value must be <= N
//
// Custom validation rules (registered by this package):
//   - authmethod: field must be a valid auth method (trust, password, email, google)
//   - httpurl: field must be a valid http:// or https:// URL
//   - objectid: field must be a valid MongoDB ObjectID hex string
//
// Example:
//
//	type Input struct {
//	    Name   string `validate:"required,max=200" label:"Full name"`
//	    Email  string `validate:"required,email,max=254" label:"Email address"`
//	    Role   string `validate:"required,oneof=admin" label:"Role"`
//	    Auth   string `validate:"required,authmethod" label:"Auth method"`
//	}
func Validate(s any) *Result {
	result := &Result{}

	v := getValidator()
	err := v.Struct(s)
	if err == nil {
		return result
	}

	// Get field labels from struct tags
	labels := getFieldLabels(s)

	if errs, ok := err.(validate.Errors); ok {
		for _, e := range errs {
			label := labels[e.Field]
			if label == "" {
				label = e.Field
			}

			msg := formatMessage(label, e.Rule, e.Param)
			result.Errors = append(result.Errors, FieldError{
				Field:   e.Field,
				Label:   label,
				Message: msg,
			})
		}
	}

	return result
}

// getFieldLabels extracts the "label" tag from struct fields.
func getFieldLabels(s any) map[string]string {
	labels := make(map[string]string)

	val := reflect.ValueOf(s)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}
	if val.Kind() != reflect.Struct {
		return labels
	}

	typ := val.Type()
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)

		// Get the field name (use json tag if available)
		fieldName := field.Name
		if jsonTag := field.Tag.Get("json"); jsonTag != "" {
			parts := strings.Split(jsonTag, ",")
			if parts[0] != "" && parts[0] != "-" {
				fieldName = parts[0]
			}
		}

		// Get the label
		if label := field.Tag.Get("label"); label != "" {
			labels[fieldName] = label
		}
	}

	return labels
}

// formatMessage creates a user-friendly message for a validation rule.
func formatMessage(label, rule, param string) string {
	switch rule {
	case "required":
		return label + " is required."
	case "email":
		return "A valid email address is required."
	case "oneof", "enum":
		return label + " must be one of: " + strings.ReplaceAll(param, " ", ", ") + "."
	case "timezone":
		return label + " must be a valid time zone."
	case "min":
		return label + " must be at least " + param + " characters."
	case "max":
		return label + " must be at most " + param + " characters."
	case "authmethod":
		return label + " must be one of: " + strings.Join(AllowedAuthMethodsList(), ", ") + "."
	case "httpurl":
		return label + " must be a valid URL starting with http:// or https://."
	case "objectid":
		return label + " is not a valid ID."
	default:
		return label + " is invalid."
	}
}

// IsValidEmail checks if the given string has a valid email format.
//
// This function uses Go's net/mail.ParseAddress for RFC 5322 compliant validation.
// RFC 5322 defines the Internet Message Format, including email address syntax.
func IsValidEmail(email string) bool {
	email = strings.TrimSpace(email)
	if email == "" {
		return false
	}

	// net/mail.ParseAddress provides RFC 5322 compliant validation.
	addr, err := mail.ParseAddress(email)
	if err != nil {
		return false
	}

	// ParseAddress accepts "Name <email>" format, so verify the address
	// matches what we passed in (just the email part).
	return addr.Address == email
}

// AllowedAuthMethodsList returns all valid auth methods as a slice.
// Useful for displaying in error messages.
func AllowedAuthMethodsList() []string {
	return models.AllAuthMethodValues()
}

// IsValidAuthMethod checks if the given method (case-insensitive) is a valid auth method.
func IsValidAuthMethod(method string) bool {
	return models.IsValidAuthMethod(strings.ToLower(strings.TrimSpace(method)))
}

// IsValidHTTPURL checks if the given string is a valid http:// or https:// URL.
func IsValidHTTPURL(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	u, err := url.Parse(s)
	if err != nil {
		return false
	}
	return u.Scheme == "http" || u.Scheme == "https"
}

// IsValidObjectID checks if the given string is a valid MongoDB ObjectID hex.
func IsValidObjectID(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	_, err := primitive.ObjectIDFromHex(s)
	return err == nil
}
