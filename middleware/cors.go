package middleware

import (
	"net/http"
	"strings"
)

// ParseAllowedOrigins turns the ALLOWED_ORIGINS value into a list of origins.
// An empty value or "*" means every origin is allowed and returns nil.
// Entries are trimmed, lowercased and stripped of a trailing slash so that
// "https://App.example.com/" matches the browser's "https://app.example.com".
func ParseAllowedOrigins(value string) []string {
	var origins []string
	for _, p := range strings.Split(value, ",") {
		o := strings.ToLower(strings.TrimRight(strings.TrimSpace(p), "/"))
		if o == "" {
			continue
		}
		if o == "*" {
			return nil
		}
		origins = append(origins, o)
	}
	return origins
}

// CORS sets the CORS headers and answers preflight requests.
//
// With a nil/empty allowed list every origin is accepted and
// Access-Control-Allow-Origin is "*". Otherwise only the listed origins get
// CORS headers, and requests carrying any other Origin header are rejected
// with 403. Requests without an Origin header (curl, server-to-server,
// health probes) are not browser cross-origin requests and are let through.
func CORS(allowed []string) func(http.Handler) http.Handler {
	allowAll := len(allowed) == 0
	set := make(map[string]struct{}, len(allowed))
	for _, o := range allowed {
		set[o] = struct{}{}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			h := w.Header()

			if allowAll {
				h.Set("Access-Control-Allow-Origin", "*")
			} else {
				h.Add("Vary", "Origin")
				if origin != "" {
					if _, ok := set[strings.ToLower(origin)]; !ok {
						http.Error(w, `{"jsonrpc":"2.0","error":{"code":-32600,"message":"Origin not allowed"},"id":null}`, http.StatusForbidden)
						return
					}
					h.Set("Access-Control-Allow-Origin", origin)
				}
			}

			if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != "" {
				h.Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
				if reqHeaders := r.Header.Get("Access-Control-Request-Headers"); reqHeaders != "" {
					h.Set("Access-Control-Allow-Headers", reqHeaders)
				} else {
					h.Set("Access-Control-Allow-Headers", "Content-Type")
				}
				h.Set("Access-Control-Max-Age", "86400")
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
