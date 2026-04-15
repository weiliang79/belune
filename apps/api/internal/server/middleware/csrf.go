package middleware

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"net/http"
	"strings"
)

const (
	CSRFCookieName = "csrf_token"
	CSRFHeaderName = "X-CSRF-Token"
)

// GenerateCSRFToken returns a 32-byte random token encoded as hex.
func GenerateCSRFToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// CSRF enforces the double-submit cookie pattern for cookie-authenticated
// mutating requests.
//
//   - Safe methods (GET/HEAD/OPTIONS) pass through.
//   - Requests with an Authorization: Bearer header bypass the check: there is
//     no ambient credential, so CSRF is not a risk.
//   - Cookie-authenticated mutating requests must send X-CSRF-Token matching
//     the csrf_token cookie (constant-time compare).
func CSRF() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.Method {
			case http.MethodGet, http.MethodHead, http.MethodOptions:
				next.ServeHTTP(w, r)
				return
			}

			if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
				next.ServeHTTP(w, r)
				return
			}

			cookie, err := r.Cookie(CSRFCookieName)
			if err != nil || cookie.Value == "" {
				http.Error(w, `{"error":"missing csrf token"}`, http.StatusForbidden)
				return
			}
			header := r.Header.Get(CSRFHeaderName)
			if header == "" {
				http.Error(w, `{"error":"missing csrf token"}`, http.StatusForbidden)
				return
			}
			if subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(header)) != 1 {
				http.Error(w, `{"error":"invalid csrf token"}`, http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
