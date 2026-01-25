package network

import (
	"net/http/httptest"
	"testing"
)

func TestGetClientIP(t *testing.T) {
	tests := []struct {
		name           string
		xForwardedFor  string
		xRealIP        string
		remoteAddr     string
		expectedIP     string
	}{
		{
			name:       "X-Forwarded-For single IP",
			xForwardedFor: "192.168.1.1",
			remoteAddr:    "10.0.0.1:12345",
			expectedIP:    "192.168.1.1",
		},
		{
			name:       "X-Forwarded-For multiple IPs",
			xForwardedFor: "192.168.1.1, 10.0.0.2, 172.16.0.1",
			remoteAddr:    "10.0.0.1:12345",
			expectedIP:    "192.168.1.1", // First IP in chain
		},
		{
			name:       "X-Forwarded-For with spaces",
			xForwardedFor: "  192.168.1.1  ",
			remoteAddr:    "10.0.0.1:12345",
			expectedIP:    "192.168.1.1",
		},
		{
			name:       "X-Real-IP",
			xRealIP:    "192.168.1.1",
			remoteAddr: "10.0.0.1:12345",
			expectedIP: "192.168.1.1",
		},
		{
			name:       "X-Forwarded-For takes precedence over X-Real-IP",
			xForwardedFor: "192.168.1.1",
			xRealIP:       "10.0.0.2",
			remoteAddr:    "10.0.0.1:12345",
			expectedIP:    "192.168.1.1",
		},
		{
			name:       "RemoteAddr with port",
			remoteAddr: "192.168.1.1:12345",
			expectedIP: "192.168.1.1",
		},
		{
			name:       "RemoteAddr without port",
			remoteAddr: "192.168.1.1",
			expectedIP: "192.168.1.1",
		},
		{
			name:       "IPv6 RemoteAddr with port",
			remoteAddr: "[::1]:12345",
			expectedIP: "[::1]", // IPv6 address with brackets preserved
		},
		{
			name:       "Empty headers fallback to RemoteAddr",
			remoteAddr: "10.0.0.1:8080",
			expectedIP: "10.0.0.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			if tt.xForwardedFor != "" {
				req.Header.Set("X-Forwarded-For", tt.xForwardedFor)
			}
			if tt.xRealIP != "" {
				req.Header.Set("X-Real-IP", tt.xRealIP)
			}
			req.RemoteAddr = tt.remoteAddr

			got := GetClientIP(req)
			if got != tt.expectedIP {
				t.Errorf("GetClientIP() = %q, want %q", got, tt.expectedIP)
			}
		})
	}
}

func TestGetClientIP_NoHeaders(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "127.0.0.1:54321"

	ip := GetClientIP(req)
	if ip != "127.0.0.1" {
		t.Errorf("GetClientIP() = %q, want %q", ip, "127.0.0.1")
	}
}

func TestGetClientIP_ProxyChain(t *testing.T) {
	// Simulate a request that went through multiple proxies
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Forwarded-For", "203.0.113.195, 70.41.3.18, 150.172.238.178")
	req.RemoteAddr = "127.0.0.1:8080"

	// Should return the original client IP (first in chain)
	ip := GetClientIP(req)
	if ip != "203.0.113.195" {
		t.Errorf("GetClientIP() = %q, want %q", ip, "203.0.113.195")
	}
}
