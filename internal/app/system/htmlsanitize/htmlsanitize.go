// Package htmlsanitize provides HTML sanitization for user-generated rich text content.
// It uses bluemonday to strip potentially dangerous HTML while preserving safe formatting.
package htmlsanitize

import (
	"html/template"
	"strings"
	"sync"

	"github.com/microcosm-cc/bluemonday"
)

var (
	// policy is the shared bluemonday policy for sanitizing rich text.
	policy     *bluemonday.Policy
	policyOnce sync.Once
)

// getPolicy returns the shared sanitization policy, creating it on first use.
func getPolicy() *bluemonday.Policy {
	policyOnce.Do(func() {
		// Start with UGC (User Generated Content) policy as base
		policy = bluemonday.UGCPolicy()

		// Allow tables (for TipTap table extension)
		policy.AllowElements("table", "thead", "tbody", "tfoot", "tr", "th", "td")
		policy.AllowAttrs("colspan", "rowspan").OnElements("th", "td")
		policy.AllowAttrs("class").OnElements("table", "th", "td", "tr")

		// Allow common text formatting
		policy.AllowElements("u", "s", "sub", "sup", "mark")

		// Allow data attributes used by TipTap
		policy.AllowDataAttributes()

		// Allow style attribute on specific elements for tables
		policy.AllowAttrs("style").OnElements("table", "th", "td")
	})
	return policy
}

// Sanitize cleans HTML input, removing potentially dangerous elements and attributes.
// It preserves safe formatting like bold, italic, lists, links, and tables.
// Returns the sanitized HTML string.
func Sanitize(html string) string {
	if html == "" {
		return ""
	}
	return getPolicy().Sanitize(html)
}

// SanitizeToHTML sanitizes HTML input and returns it as template.HTML,
// which is safe to render directly in Go templates without escaping.
func SanitizeToHTML(html string) template.HTML {
	return template.HTML(Sanitize(html))
}

// IsPlainText checks if content appears to be plain text (no HTML tags).
// This can be used to handle legacy plain-text content.
func IsPlainText(content string) bool {
	if content == "" {
		return true
	}
	// Simple check: if it contains both < and >, it's likely HTML
	// Valid HTML tags require both characters, so if either is missing, treat as plain text
	return !strings.Contains(content, "<") || !strings.Contains(content, ">")
}

// PlainTextToHTML converts plain text to minimal HTML by:
// - Escaping HTML entities
// - Converting newlines to <br> tags
// - Wrapping in a <p> tag if it doesn't start with one
func PlainTextToHTML(text string) string {
	if text == "" {
		return ""
	}
	// Escape HTML entities
	escaped := template.HTMLEscapeString(text)
	// Convert newlines to <br>
	escaped = strings.ReplaceAll(escaped, "\n", "<br>")
	return "<p>" + escaped + "</p>"
}

// PrepareForDisplay takes content (which may be plain text or HTML) and
// returns sanitized template.HTML ready for rendering.
// If the content appears to be plain text, it's converted to HTML first.
func PrepareForDisplay(content string) template.HTML {
	if content == "" {
		return ""
	}
	if IsPlainText(content) {
		return template.HTML(PlainTextToHTML(content))
	}
	return SanitizeToHTML(content)
}
