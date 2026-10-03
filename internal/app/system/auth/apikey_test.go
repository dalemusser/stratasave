package auth

import (
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
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

const (
	testSentinel = "000000000000000000000001"
	testStudent  = "69b4449ec6006ac370dad9df"
)

// A restricted key is accepted only for requests about its own user ids,
// whatever shape the request takes.
func TestAPIKeyAuthRestricted(t *testing.T) {
	restricted := RestrictedKey{Key: "public-key", UserIDs: []string{testSentinel}, Enforce: true}

	tests := []struct {
		name   string
		method string
		target string
		key    string
		body   string
		want   int
	}{
		// The full key is not restricted.
		{"full key, any user", "POST", "/x", "full-key", `{"game":"mhs","user_id":"` + testStudent + `"}`, http.StatusOK},
		{"full key, no body", "GET", "/x?game=mhs", "full-key", "", http.StatusOK},

		// The restricted key, for its own user.
		{"own user", "POST", "/x", "public-key", `{"game":"mhs","user_id":"` + testSentinel + `","eventType":"e"}`, http.StatusOK},
		{"own user, batch", "POST", "/x", "public-key", `{"game":"mhs","entries":[{"user_id":"` + testSentinel + `"},{"user_id":"` + testSentinel + `"}]}`, http.StatusOK},
		{"own user, list", "GET", "/x?game=mhs&user_id=" + testSentinel, "public-key", "", http.StatusOK},

		// The restricted key, for anyone else.
		{"another user", "POST", "/x", "public-key", `{"game":"mhs","user_id":"` + testStudent + `"}`, http.StatusForbidden},
		{"batch with one other user", "POST", "/x", "public-key", `{"game":"mhs","entries":[{"user_id":"` + testSentinel + `"},{"user_id":"` + testStudent + `"}]}`, http.StatusForbidden},
		{"own user on top, another in the batch", "POST", "/x", "public-key", `{"user_id":"` + testSentinel + `","entries":[{"user_id":"` + testStudent + `"}]}`, http.StatusForbidden},
		{"list of another user", "GET", "/x?game=mhs&user_id=" + testStudent, "public-key", "", http.StatusForbidden},
		{"list of every user", "GET", "/x?game=mhs", "public-key", "", http.StatusForbidden},
		{"list, two user_id parameters", "GET", "/x?user_id=" + testSentinel + "&user_id=" + testStudent, "public-key", "", http.StatusForbidden},
		{"list, parameter in another case", "GET", "/x?game=mhs&USER_ID=" + testSentinel, "public-key", "", http.StatusForbidden},

		// Shapes that could make this check and a handler disagree.
		{"repeated key, other user last", "POST", "/x", "public-key", `{"user_id":"` + testSentinel + `","user_id":"` + testStudent + `"}`, http.StatusForbidden},
		{"key in another case", "POST", "/x", "public-key", `{"user_id":"` + testSentinel + `","USER_ID":"` + testStudent + `"}`, http.StatusForbidden},
		{"key in another case first", "POST", "/x", "public-key", `{"User_Id":"` + testStudent + `","user_id":"` + testSentinel + `"}`, http.StatusForbidden},
		{"key with a folding letter", "POST", "/x", "public-key", `{"user_id":"` + testSentinel + `","uſer_id":"` + testStudent + `"}`, http.StatusForbidden},
		{"entries in another case", "POST", "/x", "public-key", `{"user_id":"` + testSentinel + `","ENTRIES":[{"user_id":"` + testStudent + `"}]}`, http.StatusForbidden},
		{"trailing data after the object", "POST", "/x", "public-key", `{"user_id":"` + testStudent + `"} {"user_id":"` + testSentinel + `"}`, http.StatusForbidden},
		{"user id is not a string", "POST", "/x", "public-key", `{"user_id":1}`, http.StatusForbidden},
		{"user id is null", "POST", "/x", "public-key", `{"user_id":null}`, http.StatusForbidden},
		{"no user id", "POST", "/x", "public-key", `{"game":"mhs"}`, http.StatusForbidden},
		{"not an object", "POST", "/x", "public-key", `["` + testSentinel + `"]`, http.StatusForbidden},
		{"not JSON", "POST", "/x", "public-key", `user_id=` + testSentinel, http.StatusForbidden},
		{"empty body", "POST", "/x", "public-key", ``, http.StatusForbidden},
		{"entry is not an object", "POST", "/x", "public-key", `{"entries":["` + testSentinel + `"]}`, http.StatusForbidden},

		// Neither key.
		{"unknown key", "POST", "/x", "nope", `{"user_id":"` + testSentinel + `"}`, http.StatusUnauthorized},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var seen string
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				b, _ := io.ReadAll(r.Body)
				seen = string(b)
				w.WriteHeader(http.StatusOK)
			})
			h := APIKeyAuthRestricted([]string{"full-key"}, restricted, zap.NewNop())(next)
			req := httptest.NewRequest(tt.method, tt.target, strings.NewReader(tt.body))
			req.Header.Set("Authorization", "Bearer "+tt.key)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != tt.want {
				t.Fatalf("status = %d, want %d", rec.Code, tt.want)
			}
			if rec.Code == http.StatusOK && seen != tt.body {
				t.Errorf("the handler received %q, want the body unchanged %q", seen, tt.body)
			}
		})
	}
}

// With Enforce off, a request outside the key's users is let through
// unchanged (and logged), so use of the key can be seen before it is refused.
func TestAPIKeyAuthRestricted_ObserveOnly(t *testing.T) {
	restricted := RestrictedKey{Key: "public-key", UserIDs: []string{testSentinel}, Enforce: false}
	body := `{"game":"mhs","user_id":"` + testStudent + `","save_data":{"pad":"` + strings.Repeat("x", 4096) + `"}}`

	var seen string
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		seen = string(b)
		w.WriteHeader(http.StatusCreated)
	})
	h := APIKeyAuthRestricted([]string{"full-key"}, restricted, zap.NewNop())(next)

	req := httptest.NewRequest(http.MethodPost, "/api/state/save", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer public-key")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusCreated)
	}
	if seen != body {
		t.Errorf("the handler did not receive the body unchanged")
	}

	// An unknown key is still refused.
	req = httptest.NewRequest(http.MethodPost, "/api/state/save", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer nope")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("unknown key: status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

// A body larger than the check reads reaches the handler whole when the
// request is let through.
func TestAPIKeyAuthRestricted_LargeBodyUnchanged(t *testing.T) {
	restricted := RestrictedKey{Key: "public-key", UserIDs: []string{testSentinel}, Enforce: false}
	body := `{"user_id":"` + testSentinel + `","save_data":{"pad":"` + strings.Repeat("x", restrictedMaxBody+1000) + `"}}`

	var got int
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		got = len(b)
		w.WriteHeader(http.StatusOK)
	})
	h := APIKeyAuthRestricted(nil, restricted, zap.NewNop())(next)
	req := httptest.NewRequest(http.MethodPost, "/api/state/save", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer public-key")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || got != len(body) {
		t.Errorf("status = %d, handler read %d bytes; want 200 and %d bytes", rec.Code, got, len(body))
	}
}

func TestSplitList(t *testing.T) {
	if got := SplitList(" a, ,b,"); !reflect.DeepEqual(got, []string{"a", "b"}) {
		t.Errorf("SplitList = %v", got)
	}
	if got := SplitList(""); got != nil {
		t.Errorf("SplitList(\"\") = %v, want nil", got)
	}
}
