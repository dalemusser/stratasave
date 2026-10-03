package auth

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"go.uber.org/zap"
)

func TestAPIKeys(t *testing.T) {
	tests := []struct {
		name    string
		primary string
		extra   string
		want    []string
	}{
		{"nothing configured", "", "", nil},
		{"primary only", "alpha", "", []string{"alpha"}},
		{"primary and one extra", "alpha", "beta", []string{"alpha", "beta"}},
		{"several extras, spaces and blanks", "alpha", " beta , ,gamma,", []string{"alpha", "beta", "gamma"}},
		{"repeat of the primary is dropped", "alpha", "alpha,beta", []string{"alpha", "beta"}},
		{"extras without a primary", "", "beta,gamma", []string{"beta", "gamma"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := APIKeys(tt.primary, tt.extra); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("APIKeys(%q, %q) = %v, want %v", tt.primary, tt.extra, got, tt.want)
			}
		})
	}
}

func TestAPIKeyAuth(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	tests := []struct {
		name   string
		keys   []string
		header string
		want   int
	}{
		{"primary key", []string{"alpha", "beta"}, "Bearer alpha", http.StatusOK},
		{"extra key", []string{"alpha", "beta"}, "Bearer beta", http.StatusOK},
		{"scheme is case-insensitive", []string{"alpha"}, "bearer alpha", http.StatusOK},
		{"unknown key", []string{"alpha", "beta"}, "Bearer gamma", http.StatusUnauthorized},
		{"prefix of a key", []string{"alpha"}, "Bearer alph", http.StatusUnauthorized},
		{"key with extra characters", []string{"alpha"}, "Bearer alphabet", http.StatusUnauthorized},
		{"empty key", []string{"alpha"}, "Bearer ", http.StatusUnauthorized},
		{"no header", []string{"alpha"}, "", http.StatusUnauthorized},
		{"wrong scheme", []string{"alpha"}, "Basic alpha", http.StatusUnauthorized},
		{"no keys configured", nil, "Bearer alpha", http.StatusUnauthorized},
		{"only blank keys configured", []string{""}, "Bearer ", http.StatusUnauthorized},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := APIKeyAuth(tt.keys, zap.NewNop())(next)
			req := httptest.NewRequest(http.MethodPost, "/api/state/save", nil)
			if tt.header != "" {
				req.Header.Set("Authorization", tt.header)
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != tt.want {
				t.Errorf("status = %d, want %d", rec.Code, tt.want)
			}
		})
	}
}
