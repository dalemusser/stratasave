package normalize

import "testing"

func TestEmail(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"user@example.com", "user@example.com"},
		{"USER@EXAMPLE.COM", "user@example.com"},
		{"User@Example.Com", "user@example.com"},
		{"  user@example.com  ", "user@example.com"},
		{"\tuser@example.com\n", "user@example.com"},
		{"", ""},
		{"   ", ""},
		{" UPPER@CASE.COM ", "upper@case.com"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := Email(tt.input); got != tt.want {
				t.Errorf("Email(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"John Doe", "John Doe"},
		{"  John Doe  ", "John Doe"},
		{"\tJohn Doe\n", "John Doe"},
		{"john doe", "john doe"},
		{"JOHN DOE", "JOHN DOE"},
		{"", ""},
		{"   ", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := Name(tt.input); got != tt.want {
				t.Errorf("Name(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestAuthMethod(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"password", "password"},
		{"PASSWORD", "password"},
		{"Password", "password"},
		{"  password  ", "password"},
		{"email", "email"},
		{"EMAIL", "email"},
		{"google", "google"},
		{"GOOGLE", "google"},
		{"trust", "trust"},
		{"", ""},
		{"   ", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := AuthMethod(tt.input); got != tt.want {
				t.Errorf("AuthMethod(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestStatus(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"active", "active"},
		{"ACTIVE", "active"},
		{"Active", "active"},
		{"  active  ", "active"},
		{"disabled", "disabled"},
		{"DISABLED", "disabled"},
		{"pending", "pending"},
		{"", ""},
		{"   ", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := Status(tt.input); got != tt.want {
				t.Errorf("Status(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestRole(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"admin", "admin"},
		{"ADMIN", "admin"},
		{"Admin", "admin"},
		{"  admin  ", "admin"},
		{"user", "user"},
		{"USER", "user"},
		{"moderator", "moderator"},
		{"", ""},
		{"   ", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := Role(tt.input); got != tt.want {
				t.Errorf("Role(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestQueryParam(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"search term", "search term"},
		{"  search term  ", "search term"},
		{"\tsearch term\n", "search term"},
		{"SEARCH TERM", "SEARCH TERM"},
		{"", ""},
		{"   ", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := QueryParam(tt.input); got != tt.want {
				t.Errorf("QueryParam(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
