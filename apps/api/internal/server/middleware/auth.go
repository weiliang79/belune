package middleware

import (
	"net/http"
)

// Auth returns a middleware that validates JWT tokens or session cookies.
func Auth(jwtSecret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// TODO: Implement JWT/session validation
			// 1. Extract token from Authorization header or cookie
			// 2. Validate and decode token
			// 3. Set user context
			next.ServeHTTP(w, r)
		})
	}
}
