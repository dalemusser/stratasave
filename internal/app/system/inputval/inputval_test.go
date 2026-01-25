package inputval

import (
	"testing"
)

func TestIsValidEmail(t *testing.T) {
	tests := []struct {
		email string
		want  bool
	}{
		// Valid emails
		{"user@example.com", true},
		{"user.name@example.com", true},
		{"user+tag@example.com", true},
		{"user@subdomain.example.com", true},
		{"user123@example.co.uk", true},

		// Invalid emails
		{"", false},
		{"   ", false},
		{"notanemail", false},
		{"@example.com", false},
		{"user@", false},
		{"user@.com", false},
		{"user example.com", false},
		{"user@@example.com", false},
		{"Name <user@example.com>", false}, // ParseAddress accepts this but we want bare email
	}

	for _, tt := range tests {
		t.Run(tt.email, func(t *testing.T) {
			got := IsValidEmail(tt.email)
			if got != tt.want {
				t.Errorf("IsValidEmail(%q) = %v, want %v", tt.email, got, tt.want)
			}
		})
	}
}

func TestIsValidHTTPURL(t *testing.T) {
	tests := []struct {
		url  string
		want bool
	}{
		// Valid URLs
		{"http://example.com", true},
		{"https://example.com", true},
		{"https://example.com/path", true},
		{"https://example.com/path?query=value", true},
		{"https://subdomain.example.com", true},
		{"http://localhost:8080", true},

		// Invalid URLs
		{"", false},
		{"   ", false},
		{"example.com", false},          // No scheme
		{"ftp://example.com", false},    // Wrong scheme
		{"file:///path/to/file", false}, // Wrong scheme
		{"javascript:alert(1)", false},  // Wrong scheme
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			got := IsValidHTTPURL(tt.url)
			if got != tt.want {
				t.Errorf("IsValidHTTPURL(%q) = %v, want %v", tt.url, got, tt.want)
			}
		})
	}
}

func TestIsValidObjectID(t *testing.T) {
	tests := []struct {
		id   string
		want bool
	}{
		// Valid ObjectIDs (24 hex characters)
		{"507f1f77bcf86cd799439011", true},
		{"000000000000000000000000", true},
		{"ffffffffffffffffffffffff", true},

		// Invalid ObjectIDs
		{"", false},
		{"   ", false},
		{"507f1f77bcf86cd79943901", false},  // Too short (23 chars)
		{"507f1f77bcf86cd7994390111", false}, // Too long (25 chars)
		{"507f1f77bcf86cd79943901g", false},  // Invalid hex char
		{"not-an-object-id", false},
	}

	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			got := IsValidObjectID(tt.id)
			if got != tt.want {
				t.Errorf("IsValidObjectID(%q) = %v, want %v", tt.id, got, tt.want)
			}
		})
	}
}

func TestIsValidAuthMethod(t *testing.T) {
	tests := []struct {
		method string
		want   bool
	}{
		// Valid auth methods
		{"password", true},
		{"email", true},
		{"google", true},
		{"trust", true},
		// Case insensitive
		{"PASSWORD", true},
		{"Email", true},
		{"GOOGLE", true},
		// With whitespace (should be trimmed)
		{"  password  ", true},

		// Invalid auth methods
		{"", false},
		{"invalid", false},
		{"facebook", false},
		{"oauth", false},
	}

	for _, tt := range tests {
		t.Run(tt.method, func(t *testing.T) {
			got := IsValidAuthMethod(tt.method)
			if got != tt.want {
				t.Errorf("IsValidAuthMethod(%q) = %v, want %v", tt.method, got, tt.want)
			}
		})
	}
}

func TestValidate(t *testing.T) {
	type TestInput struct {
		Name  string `validate:"required" label:"Name"`
		Email string `validate:"required,email" label:"Email"`
	}

	tests := []struct {
		name      string
		input     TestInput
		wantError bool
	}{
		{
			name:      "valid input",
			input:     TestInput{Name: "John", Email: "john@example.com"},
			wantError: false,
		},
		{
			name:      "missing name",
			input:     TestInput{Name: "", Email: "john@example.com"},
			wantError: true,
		},
		{
			name:      "missing email",
			input:     TestInput{Name: "John", Email: ""},
			wantError: true,
		},
		{
			name:      "invalid email",
			input:     TestInput{Name: "John", Email: "notanemail"},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Validate(tt.input)
			if tt.wantError && !result.HasErrors() {
				t.Errorf("Validate() expected errors, got none")
			}
			if !tt.wantError && result.HasErrors() {
				t.Errorf("Validate() expected no errors, got: %s", result.First())
			}
		})
	}
}

func TestResult_First(t *testing.T) {
	// Empty result
	r := &Result{}
	if got := r.First(); got != "" {
		t.Errorf("First() on empty result = %q, want empty string", got)
	}

	// Result with errors
	r = &Result{
		Errors: []FieldError{
			{Field: "name", Label: "Name", Message: "Name is required."},
			{Field: "email", Label: "Email", Message: "Email is required."},
		},
	}
	if got := r.First(); got != "Name is required." {
		t.Errorf("First() = %q, want %q", got, "Name is required.")
	}
}

func TestResult_All(t *testing.T) {
	// Empty result
	r := &Result{}
	if got := r.All(); got != "" {
		t.Errorf("All() on empty result = %q, want empty string", got)
	}

	// Result with errors
	r = &Result{
		Errors: []FieldError{
			{Field: "name", Label: "Name", Message: "Name is required."},
			{Field: "email", Label: "Email", Message: "Email is required."},
		},
	}
	want := "Name is required.; Email is required."
	if got := r.All(); got != want {
		t.Errorf("All() = %q, want %q", got, want)
	}
}

func TestResult_HasErrors(t *testing.T) {
	// Empty result
	r := &Result{}
	if r.HasErrors() {
		t.Error("HasErrors() on empty result should return false")
	}

	// Result with errors
	r = &Result{
		Errors: []FieldError{
			{Field: "name", Label: "Name", Message: "Name is required."},
		},
	}
	if !r.HasErrors() {
		t.Error("HasErrors() with errors should return true")
	}
}

func TestAllowedAuthMethodsList(t *testing.T) {
	methods := AllowedAuthMethodsList()
	if len(methods) == 0 {
		t.Error("AllowedAuthMethodsList() returned empty list")
	}

	// Should contain known methods
	found := make(map[string]bool)
	for _, m := range methods {
		found[m] = true
	}

	expected := []string{"password", "email", "google", "trust"}
	for _, e := range expected {
		if !found[e] {
			t.Errorf("AllowedAuthMethodsList() missing %q", e)
		}
	}
}

func TestValidate_CustomRules(t *testing.T) {
	// Test authmethod rule
	type AuthInput struct {
		Method string `validate:"required,authmethod" label:"Auth method"`
	}

	result := Validate(AuthInput{Method: "password"})
	if result.HasErrors() {
		t.Errorf("Validate() authmethod=password should be valid, got: %s", result.First())
	}

	result = Validate(AuthInput{Method: "invalid"})
	if !result.HasErrors() {
		t.Error("Validate() authmethod=invalid should fail")
	}

	// Test httpurl rule
	type URLInput struct {
		URL string `validate:"required,httpurl" label:"URL"`
	}

	result = Validate(URLInput{URL: "https://example.com"})
	if result.HasErrors() {
		t.Errorf("Validate() httpurl should be valid, got: %s", result.First())
	}

	result = Validate(URLInput{URL: "ftp://example.com"})
	if !result.HasErrors() {
		t.Error("Validate() httpurl=ftp should fail")
	}

	// Test objectid rule
	type IDInput struct {
		ID string `validate:"required,objectid" label:"ID"`
	}

	result = Validate(IDInput{ID: "507f1f77bcf86cd799439011"})
	if result.HasErrors() {
		t.Errorf("Validate() objectid should be valid, got: %s", result.First())
	}

	result = Validate(IDInput{ID: "invalid-id"})
	if !result.HasErrors() {
		t.Error("Validate() objectid=invalid should fail")
	}
}

func TestValidate_MinMaxRules(t *testing.T) {
	type LengthInput struct {
		Short string `validate:"min=3" label:"Short field"`
		Long  string `validate:"max=5" label:"Long field"`
	}

	// Valid lengths
	result := Validate(LengthInput{Short: "abc", Long: "12345"})
	if result.HasErrors() {
		t.Errorf("Validate() valid lengths should pass, got: %s", result.First())
	}

	// Too short
	result = Validate(LengthInput{Short: "ab", Long: "123"})
	if !result.HasErrors() {
		t.Error("Validate() short=ab should fail min=3")
	}

	// Too long
	result = Validate(LengthInput{Short: "abcd", Long: "123456"})
	if !result.HasErrors() {
		t.Error("Validate() long=123456 should fail max=5")
	}
}

func TestValidate_OneOfRule(t *testing.T) {
	type EnumInput struct {
		Status string `validate:"oneof=active inactive" label:"Status"`
	}

	result := Validate(EnumInput{Status: "active"})
	if result.HasErrors() {
		t.Errorf("Validate() oneof=active should be valid, got: %s", result.First())
	}

	result = Validate(EnumInput{Status: "deleted"})
	if !result.HasErrors() {
		t.Error("Validate() oneof=deleted should fail")
	}
}

func TestValidate_PointerStruct(t *testing.T) {
	type Input struct {
		Name string `validate:"required" label:"Name"`
	}

	input := &Input{Name: "test"}
	result := Validate(input)
	if result.HasErrors() {
		t.Errorf("Validate() pointer struct should work, got: %s", result.First())
	}
}

func TestValidate_NonStruct(t *testing.T) {
	// Validate with non-struct should not panic
	result := Validate("not a struct")
	// Should return empty result (no fields to validate)
	if result == nil {
		t.Error("Validate() non-struct should return non-nil result")
	}
}

func TestValidate_JSONTags(t *testing.T) {
	type Input struct {
		FullName string `json:"full_name" validate:"required" label:"Full name"`
	}

	result := Validate(Input{FullName: ""})
	if !result.HasErrors() {
		t.Error("Validate() empty FullName should fail")
	}
	// The label should be used in the message
	if result.First() != "Full name is required." {
		t.Errorf("Validate() error message = %q, want label-based message", result.First())
	}
}

func TestValidate_NoLabel(t *testing.T) {
	type Input struct {
		Name string `validate:"required"` // No label tag
	}

	result := Validate(Input{Name: ""})
	if !result.HasErrors() {
		t.Error("Validate() empty Name should fail")
	}
	// Should use field name when no label
	if result.First() != "Name is required." {
		t.Errorf("Validate() error message = %q, want field name message", result.First())
	}
}
