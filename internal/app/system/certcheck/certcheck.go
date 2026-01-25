// internal/app/system/certcheck/certcheck.go
package certcheck

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"
)

// CertInfo contains information about a TLS certificate.
type CertInfo struct {
	Host      string    `json:"host"`
	ExpiresAt time.Time `json:"expires_at"`
	DaysLeft  int       `json:"days_left"`
	Issuer    string    `json:"issuer"`
	IsValid   bool      `json:"is_valid"`
	Error     string    `json:"error,omitempty"`
}

// Check retrieves certificate information for a given host.
// The host can be a URL (https://example.com) or just a hostname (example.com).
func Check(hostOrURL string) CertInfo {
	host := extractHost(hostOrURL)
	if host == "" {
		return CertInfo{
			Host:    hostOrURL,
			IsValid: false,
			Error:   "invalid host",
		}
	}

	// Skip cert check for localhost/development
	if isLocalhost(host) {
		return CertInfo{
			Host:    host,
			IsValid: true,
			Error:   "localhost - no TLS",
		}
	}

	info := CertInfo{Host: host}

	// Connect with a timeout
	dialer := &net.Dialer{Timeout: 5 * time.Second}
	conn, err := tls.DialWithDialer(dialer, "tcp", host+":443", &tls.Config{
		ServerName: host,
	})
	if err != nil {
		info.IsValid = false
		info.Error = fmt.Sprintf("connection failed: %v", err)
		return info
	}
	defer conn.Close()

	certs := conn.ConnectionState().PeerCertificates
	if len(certs) == 0 {
		info.IsValid = false
		info.Error = "no certificates found"
		return info
	}

	cert := certs[0]
	info.ExpiresAt = cert.NotAfter
	info.DaysLeft = int(time.Until(cert.NotAfter).Hours() / 24)
	info.Issuer = cert.Issuer.CommonName
	info.IsValid = time.Now().Before(cert.NotAfter) && time.Now().After(cert.NotBefore)

	return info
}

// extractHost extracts the hostname from a URL or returns the input if it's already a hostname.
func extractHost(hostOrURL string) string {
	// If it looks like a URL, parse it
	if strings.HasPrefix(hostOrURL, "http://") || strings.HasPrefix(hostOrURL, "https://") {
		u, err := url.Parse(hostOrURL)
		if err != nil {
			return ""
		}
		return u.Hostname()
	}
	// Otherwise assume it's already a hostname
	// Strip any port if present
	if idx := strings.Index(hostOrURL, ":"); idx != -1 {
		return hostOrURL[:idx]
	}
	return hostOrURL
}

// isLocalhost returns true if the host is a localhost address.
func isLocalhost(host string) bool {
	return host == "localhost" || host == "127.0.0.1" || host == "::1" || host == ""
}
