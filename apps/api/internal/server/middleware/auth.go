package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/weiliang79/belune/internal/service"
)

type contextKey string

const (
	ctxUserID  contextKey = "user_id"
	ctxEmail   contextKey = "email"
	ctxRole    contextKey = "role"
	ctxTokenID contextKey = "token_id"
	ctxScopes  contextKey = "scopes"
)

// Auth returns a middleware that authenticates a session JWT or a personal
// access token, both read from the same Authorization header. The PAT branch
// is a prefix check on the already-extracted value, not a new route group —
// every route already behind Auth gains PAT support with no further wiring.
func Auth(authService *service.AuthService, tokenService *service.TokenService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tokenString := extractToken(r)
			if tokenString == "" {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}

			if service.HasTokenPrefix(tokenString) {
				tok, err := tokenService.Authenticate(r.Context(), tokenString)
				if err != nil {
					http.Error(w, `{"error":"invalid or expired token"}`, http.StatusUnauthorized)
					return
				}

				ctx := context.WithValue(r.Context(), ctxUserID, uuid.UUID(tok.UserID.Bytes).String())
				ctx = context.WithValue(ctx, ctxRole, tok.EffectiveRole)
				ctx = context.WithValue(ctx, ctxTokenID, uuid.UUID(tok.TokenID.Bytes).String())
				ctx = context.WithValue(ctx, ctxScopes, tok.Scopes)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			claims, err := authService.ValidateToken(r.Context(), tokenString)
			if err != nil {
				http.Error(w, `{"error":"invalid or expired token"}`, http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), ctxUserID, claims.UserID)
			ctx = context.WithValue(ctx, ctxEmail, claims.Email)
			ctx = context.WithValue(ctx, ctxRole, claims.Role)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func extractToken(r *http.Request) string {
	// Check Authorization header first
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}

	// Fall back to cookie
	cookie, err := r.Cookie("token")
	if err == nil {
		return cookie.Value
	}

	return ""
}

// RequireRole returns a middleware that checks the user's role against the allowed roles.
// Must be used after Auth middleware.
func RequireRole(roles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			role := RoleFromContext(r.Context())
			for _, allowed := range roles {
				if role == allowed {
					next.ServeHTTP(w, r)
					return
				}
			}
			http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		})
	}
}

// UserIDFromContext returns the authenticated user's ID from the request context.
func UserIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(ctxUserID).(string)
	return v
}

// EmailFromContext returns the authenticated user's email from the request context.
// Empty for a PAT-authenticated request — nothing reads it today, and a token
// lookup does not carry the owner's email along for a value nothing uses.
func EmailFromContext(ctx context.Context) string {
	v, _ := ctx.Value(ctxEmail).(string)
	return v
}

// RoleFromContext returns the authenticated user's role from the request
// context — for a PAT this is already the effective role (min(role at issue,
// the owner's role now)), not the owner's raw current role.
func RoleFromContext(ctx context.Context) string {
	v, _ := ctx.Value(ctxRole).(string)
	return v
}

// TokenIDFromContext returns the authenticating PAT's id, or "" for a session
// JWT. h.audit reads this to attribute a mutation to the token that made it
// rather than silently reading as a human action.
func TokenIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(ctxTokenID).(string)
	return v
}

// ScopesFromContext returns the authenticating PAT's scopes, or nil for a
// session JWT (a session implies every scope — callers should treat a nil
// slice from a JWT request as "unrestricted", not "no scopes").
func ScopesFromContext(ctx context.Context) []string {
	v, _ := ctx.Value(ctxScopes).([]string)
	return v
}
