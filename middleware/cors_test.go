package middleware

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestParseAllowedOrigins(t *testing.T) {
	cases := map[string][]string{
		"":                            nil,
		"*":                           nil,
		" * ":                         nil,
		"https://a.com, *":            nil,
		"https://A.com/":              {"https://a.com"},
		"https://a.com, ,http://b.io": {"https://a.com", "http://b.io"},
	}
	for in, want := range cases {
		if got := ParseAllowedOrigins(in); !reflect.DeepEqual(got, want) {
			t.Errorf("ParseAllowedOrigins(%q) = %v, want %v", in, got, want)
		}
	}
}

func serve(allowed []string, method, origin string, preflight bool) *httptest.ResponseRecorder {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	req := httptest.NewRequest(method, "/", nil)
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	if preflight {
		req.Header.Set("Access-Control-Request-Method", "POST")
		req.Header.Set("Access-Control-Request-Headers", "content-type")
	}
	rec := httptest.NewRecorder()
	CORS(allowed)(next).ServeHTTP(rec, req)
	return rec
}

func TestCORSAllowAll(t *testing.T) {
	rec := serve(nil, http.MethodPost, "https://anything.com", false)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Fatalf("ACAO = %q, want *", got)
	}
}

func TestCORSAllowedOrigin(t *testing.T) {
	rec := serve([]string{"https://app.com"}, http.MethodPost, "https://App.com", false)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://App.com" {
		t.Fatalf("ACAO = %q", got)
	}
	if rec.Header().Get("Vary") != "Origin" {
		t.Fatalf("missing Vary: Origin")
	}
}

func TestCORSBlockedOrigin(t *testing.T) {
	rec := serve([]string{"https://app.com"}, http.MethodPost, "https://evil.com", false)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
	if rec.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatalf("blocked origin must not get ACAO")
	}
}

func TestCORSNoOriginPassesThrough(t *testing.T) {
	rec := serve([]string{"https://app.com"}, http.MethodPost, "", false)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

func TestCORSPreflight(t *testing.T) {
	rec := serve([]string{"https://app.com"}, http.MethodOptions, "https://app.com", true)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
	if rec.Header().Get("Access-Control-Allow-Headers") != "content-type" {
		t.Fatalf("Allow-Headers = %q", rec.Header().Get("Access-Control-Allow-Headers"))
	}

	rec = serve([]string{"https://app.com"}, http.MethodOptions, "https://evil.com", true)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("blocked preflight status = %d, want 403", rec.Code)
	}
}
