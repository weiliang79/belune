package middleware

import (
	"context"
	"errors"
	"log/slog"
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
// access token. The PAT branch is a prefix check, not a new route group —
// every route already behind Auth gains PAT support with no further wiring.
//
// A PAT is only ever accepted from the Authorization header, never the
// session cookie extractToken also falls back to — unlike a JWT, a PAT is
// never meant to be an ambient browser credential, so it must come from
// somewhere a script explicitly put it.
func Auth(authService *service.AuthService, tokenService *service.TokenService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if bearer, ok := bearerToken(r); ok && service.HasTokenPrefix(bearer) {
				tok, err := tokenService.Authenticate(r.Context(), bearer)
				if err != nil {
					if errors.Is(err, service.ErrInvalidAPIToken) {
						http.Error(w, `{"error":"invalid or expired token"}`, http.StatusUnauthorized)
						return
					}
					// A lookup failure (DB down, pool exhausted) is not the
					// token's fault — reporting it as 401 tells a CI/CLI
					// client its credential was rejected, and such clients
					// are built to react by rotating or deleting it instead
					// of retrying a transient failure.
					slog.Error("auth: token lookup failed", "error", err)
					http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
					return
				}

				ctx := context.WithValue(r.Context(), ctxUserID, uuid.UUID(tok.UserID.Bytes).String())
				ctx = context.WithValue(ctx, ctxRole, tok.EffectiveRole)
				ctx = context.WithValue(ctx, ctxTokenID, uuid.UUID(tok.TokenID.Bytes).String())
				ctx = context.WithValue(ctx, ctxScopes, tok.Scopes)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			tokenString := extractToken(r)
			if tokenString == "" {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
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

// bearerToken returns the Authorization header's Bearer value only — no
// cookie fallback. Used by the PAT branch, which must never accept a token
// from an ambient credential.
func bearerToken(r *http.Request) (string, bool) {
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer "), true
	}
	return "", false
}

func extractToken(r *http.Request) string {
	// Check Authorization header first
	if v, ok := bearerToken(r); ok {
		return v
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
