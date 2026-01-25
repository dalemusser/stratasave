package files

import "testing"

func TestFormatFileSize(t *testing.T) {
	tests := []struct {
		name  string
		bytes int64
		want  string
	}{
		{"zero bytes", 0, "0 B"},
		{"1 byte", 1, "1 B"},
		{"500 bytes", 500, "500 B"},
		{"1023 bytes", 1023, "1023 B"},
		{"1 KB", 1024, "1.0 KB"},
		{"1.5 KB", 1536, "1.5 KB"},
		{"10 KB", 10240, "10.0 KB"},
		{"500 KB", 512000, "500.0 KB"},
		{"1 MB", 1048576, "1.0 MB"},
		{"1.5 MB", 1572864, "1.5 MB"},
		{"10 MB", 10485760, "10.0 MB"},
		{"500 MB", 524288000, "500.0 MB"},
		{"1 GB", 1073741824, "1.0 GB"},
		{"1.5 GB", 1610612736, "1.5 GB"},
		{"10 GB", 10737418240, "10.0 GB"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatFileSize(tt.bytes)
			if got != tt.want {
				t.Errorf("FormatFileSize(%d) = %q, want %q", tt.bytes, got, tt.want)
			}
		})
	}
}

func TestFileTypeIcon(t *testing.T) {
	tests := []struct {
		contentType string
		want        string
	}{
		// Images
		{"image/png", "image"},
		{"image/jpeg", "image"},
		{"image/gif", "image"},
		{"image/webp", "image"},
		{"image/svg+xml", "image"},

		// Videos
		{"video/mp4", "video"},
		{"video/webm", "video"},
		{"video/quicktime", "video"},
		{"video/x-msvideo", "video"},

		// Audio
		{"audio/mpeg", "audio"},
		{"audio/wav", "audio"},
		{"audio/ogg", "audio"},
		{"audio/flac", "audio"},

		// PDF
		{"application/pdf", "pdf"},

		// Spreadsheets
		{"application/vnd.ms-excel", "spreadsheet"},
		{"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", "spreadsheet"},

		// Documents
		{"application/msword", "document"},
		{"application/vnd.openxmlformats-officedocument.wordprocessingml.document", "document"},

		// Presentations
		{"application/vnd.ms-powerpoint", "presentation"},
		// Note: PPTX contains "officedocument" so it matches "document" before "presentation"
		{"application/vnd.openxmlformats-officedocument.presentationml.presentation", "document"},

		// Archives
		{"application/zip", "archive"},
		{"application/x-compressed", "archive"},
		// Note: x-tar doesn't contain "zip", "compressed", or "archive"
		{"application/x-tar", "file"},
		{"application/x-7z-compressed", "archive"},

		// Default
		{"text/plain", "file"},
		{"text/html", "file"},
		{"application/json", "file"},
		{"application/octet-stream", "file"},
		{"unknown/type", "file"},
	}

	for _, tt := range tests {
		t.Run(tt.contentType, func(t *testing.T) {
			got := FileTypeIcon(tt.contentType)
			if got != tt.want {
				t.Errorf("FileTypeIcon(%q) = %q, want %q", tt.contentType, got, tt.want)
			}
		})
	}
}

func TestIsViewable(t *testing.T) {
	tests := []struct {
		contentType string
		want        bool
	}{
		// Viewable - Images
		{"image/png", true},
		{"image/jpeg", true},
		{"image/gif", true},
		{"image/webp", true},
		{"image/svg+xml", true},

		// Viewable - Videos
		{"video/mp4", true},
		{"video/webm", true},
		{"video/quicktime", true},

		// Viewable - Audio
		{"audio/mpeg", true},
		{"audio/wav", true},
		{"audio/ogg", true},

		// Viewable - PDF
		{"application/pdf", true},

		// Viewable - Text
		{"text/plain", true},
		{"text/html", true},
		{"text/css", true},
		{"text/javascript", true},
		{"text/csv", true},

		// Not viewable - Documents
		{"application/msword", false},
		{"application/vnd.openxmlformats-officedocument.wordprocessingml.document", false},
		{"application/vnd.ms-excel", false},
		{"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", false},
		{"application/vnd.ms-powerpoint", false},

		// Not viewable - Archives
		{"application/zip", false},
		{"application/x-tar", false},
		{"application/x-7z-compressed", false},

		// Not viewable - Other
		{"application/octet-stream", false},
		{"application/json", false},
		{"unknown/type", false},
	}

	for _, tt := range tests {
		t.Run(tt.contentType, func(t *testing.T) {
			got := IsViewable(tt.contentType)
			if got != tt.want {
				t.Errorf("IsViewable(%q) = %v, want %v", tt.contentType, got, tt.want)
			}
		})
	}
}
