package files

import (
	"fmt"
	"strings"
)

// FormatFileSize formats a file size in bytes to a human-readable string.
func FormatFileSize(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)

	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.1f GB", float64(bytes)/float64(GB))
	case bytes >= MB:
		return fmt.Sprintf("%.1f MB", float64(bytes)/float64(MB))
	case bytes >= KB:
		return fmt.Sprintf("%.1f KB", float64(bytes)/float64(KB))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

// FileTypeIcon returns an SVG icon name for a content type.
func FileTypeIcon(contentType string) string {
	switch {
	case strings.HasPrefix(contentType, "image/"):
		return "image"
	case strings.HasPrefix(contentType, "video/"):
		return "video"
	case strings.HasPrefix(contentType, "audio/"):
		return "audio"
	case contentType == "application/pdf":
		return "pdf"
	case strings.Contains(contentType, "spreadsheet") || strings.Contains(contentType, "excel"):
		return "spreadsheet"
	case strings.Contains(contentType, "document") || strings.Contains(contentType, "word"):
		return "document"
	case strings.Contains(contentType, "presentation") || strings.Contains(contentType, "powerpoint"):
		return "presentation"
	case strings.Contains(contentType, "zip") || strings.Contains(contentType, "compressed") || strings.Contains(contentType, "archive"):
		return "archive"
	default:
		return "file"
	}
}

// IsViewable returns true if the content type can be displayed inline in a browser.
func IsViewable(contentType string) bool {
	switch {
	case strings.HasPrefix(contentType, "image/"):
		return true
	case strings.HasPrefix(contentType, "video/"):
		return true
	case strings.HasPrefix(contentType, "audio/"):
		return true
	case contentType == "application/pdf":
		return true
	case strings.HasPrefix(contentType, "text/"):
		return true
	default:
		return false
	}
}
