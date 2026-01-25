package htmlsanitize

import (
	"html/template"
	"strings"
	"testing"
)

func TestSanitize(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		contains []string    // Strings that should be in output
		excludes []string    // Strings that should NOT be in output
	}{
		{
			name:     "empty string",
			input:    "",
			contains: []string{},
			excludes: []string{},
		},
		{
			name:     "plain text",
			input:    "Hello World",
			contains: []string{"Hello World"},
			excludes: []string{},
		},
		{
			name:     "safe HTML preserved",
			input:    "<p>Hello <strong>World</strong></p>",
			contains: []string{"<p>", "<strong>", "Hello", "World"},
			excludes: []string{},
		},
		{
			name:     "script tag removed",
			input:    "<p>Hello</p><script>alert('xss')</script>",
			contains: []string{"<p>Hello</p>"},
			excludes: []string{"<script>", "alert", "xss"},
		},
		{
			name:     "onclick removed",
			input:    `<p onclick="alert('xss')">Click me</p>`,
			contains: []string{"<p>", "Click me"},
			excludes: []string{"onclick", "alert"},
		},
		{
			name:     "javascript URL removed",
			input:    `<a href="javascript:alert('xss')">Link</a>`,
			contains: []string{"Link"},
			excludes: []string{"javascript:", "alert"},
		},
		{
			name:     "safe link preserved",
			input:    `<a href="https://example.com">Link</a>`,
			contains: []string{"<a", "href", "https://example.com", "Link"},
			excludes: []string{},
		},
		{
			name:     "table elements preserved",
			input:    "<table><tr><td>Cell</td></tr></table>",
			contains: []string{"<table>", "<tr>", "<td>", "Cell"},
			excludes: []string{},
		},
		{
			name:     "iframe removed",
			input:    `<iframe src="https://evil.com"></iframe><p>Content</p>`,
			contains: []string{"<p>Content</p>"},
			excludes: []string{"<iframe", "evil.com"},
		},
		{
			name:     "style tag removed",
			input:    "<style>body{display:none}</style><p>Content</p>",
			contains: []string{"<p>Content</p>"},
			excludes: []string{"<style>", "display:none"},
		},
		{
			name:     "onerror removed",
			input:    `<img src="x" onerror="alert('xss')">`,
			contains: []string{"<img"},
			excludes: []string{"onerror", "alert"},
		},
		{
			name:     "data attributes preserved",
			input:    `<div data-id="123">Content</div>`,
			contains: []string{"data-id", "123", "Content"},
			excludes: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Sanitize(tt.input)

			for _, s := range tt.contains {
				if !strings.Contains(result, s) {
					t.Errorf("Sanitize() result should contain %q, got %q", s, result)
				}
			}

			for _, s := range tt.excludes {
				if strings.Contains(result, s) {
					t.Errorf("Sanitize() result should NOT contain %q, got %q", s, result)
				}
			}
		})
	}
}

func TestSanitizeToHTML(t *testing.T) {
	input := "<p>Hello <script>alert('xss')</script></p>"
	result := SanitizeToHTML(input)

	// Should return template.HTML type
	if _, ok := interface{}(result).(template.HTML); !ok {
		t.Error("SanitizeToHTML() should return template.HTML type")
	}

	// Should be sanitized
	resultStr := string(result)
	if strings.Contains(resultStr, "<script>") {
		t.Error("SanitizeToHTML() should remove script tags")
	}
	if !strings.Contains(resultStr, "<p>Hello") {
		t.Error("SanitizeToHTML() should preserve safe HTML")
	}
}

func TestIsPlainText(t *testing.T) {
	tests := []struct {
		content string
		want    bool
	}{
		{"", true},
		{"Hello World", true},
		{"No tags here", true},
		{"<p>Has tags</p>", false},                // Has both < and >
		{"<strong>Bold</strong>", false},          // Has both < and >
		{"Has < but no closing", true},            // Has < but no > = plain text
		{"Has > but no opening", true},            // Has > but no < = plain text
		{"Plain text with symbols: & < >", false}, // Has both < and >
	}

	for _, tt := range tests {
		t.Run(tt.content, func(t *testing.T) {
			got := IsPlainText(tt.content)
			if got != tt.want {
				t.Errorf("IsPlainText(%q) = %v, want %v", tt.content, got, tt.want)
			}
		})
	}
}

func TestPlainTextToHTML(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		contains []string
	}{
		{
			name:     "empty",
			input:    "",
			contains: []string{},
		},
		{
			name:     "simple text",
			input:    "Hello World",
			contains: []string{"<p>", "Hello World", "</p>"},
		},
		{
			name:     "newlines converted",
			input:    "Line 1\nLine 2",
			contains: []string{"<br>"},
		},
		{
			name:     "HTML entities escaped",
			input:    "<script>alert('xss')</script>",
			contains: []string{"&lt;script&gt;"},
		},
		{
			name:     "ampersand escaped",
			input:    "Tom & Jerry",
			contains: []string{"&amp;"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := PlainTextToHTML(tt.input)

			for _, s := range tt.contains {
				if !strings.Contains(result, s) {
					t.Errorf("PlainTextToHTML(%q) should contain %q, got %q", tt.input, s, result)
				}
			}
		})
	}
}

func TestPrepareForDisplay(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		contains []string
		excludes []string
	}{
		{
			name:     "empty",
			input:    "",
			contains: []string{},
			excludes: []string{},
		},
		{
			name:     "plain text wrapped",
			input:    "Hello World",
			contains: []string{"<p>", "Hello World", "</p>"},
			excludes: []string{},
		},
		{
			name:     "HTML sanitized",
			input:    "<p>Hello</p><script>bad</script>",
			contains: []string{"<p>Hello</p>"},
			excludes: []string{"<script>", "bad"},
		},
		{
			name:     "XSS script tag removed",
			input:    "<script>alert('xss')</script>",
			contains: []string{}, // Script content is stripped
			excludes: []string{"<script>", "alert", "xss"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := PrepareForDisplay(tt.input)
			resultStr := string(result)

			for _, s := range tt.contains {
				if !strings.Contains(resultStr, s) {
					t.Errorf("PrepareForDisplay(%q) should contain %q, got %q", tt.input, s, resultStr)
				}
			}

			for _, s := range tt.excludes {
				if strings.Contains(resultStr, s) {
					t.Errorf("PrepareForDisplay(%q) should NOT contain %q, got %q", tt.input, s, resultStr)
				}
			}
		})
	}
}

func TestSanitize_Idempotent(t *testing.T) {
	// Sanitizing twice should give the same result
	input := "<p>Hello <strong>World</strong></p>"

	result1 := Sanitize(input)
	result2 := Sanitize(result1)

	if result1 != result2 {
		t.Errorf("Sanitize() not idempotent: first=%q, second=%q", result1, result2)
	}
}

func TestSanitize_ListElements(t *testing.T) {
	input := "<ul><li>Item 1</li><li>Item 2</li></ul>"
	result := Sanitize(input)

	if !strings.Contains(result, "<ul>") {
		t.Error("Sanitize() should preserve <ul>")
	}
	if !strings.Contains(result, "<li>") {
		t.Error("Sanitize() should preserve <li>")
	}
}

func TestSanitize_FormattingElements(t *testing.T) {
	tests := []struct {
		tag   string
		input string
	}{
		{"strong", "<strong>Bold</strong>"},
		{"em", "<em>Italic</em>"},
		{"u", "<u>Underline</u>"},
		{"s", "<s>Strikethrough</s>"},
		{"sub", "<sub>Subscript</sub>"},
		{"sup", "<sup>Superscript</sup>"},
		{"mark", "<mark>Highlighted</mark>"},
		{"blockquote", "<blockquote>Quote</blockquote>"},
		{"code", "<code>Code</code>"},
		{"pre", "<pre>Preformatted</pre>"},
	}

	for _, tt := range tests {
		t.Run(tt.tag, func(t *testing.T) {
			result := Sanitize(tt.input)
			if !strings.Contains(result, "<"+tt.tag+">") {
				t.Errorf("Sanitize() should preserve <%s>, got %q", tt.tag, result)
			}
		})
	}
}
