package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/weiliang79/belune/internal/store/generated"
)

var ErrInvalidAPIToken = errors.New("invalid or expired token")

// TokenPrefix marks a value as a personal access token rather than a session
// JWT — checked before any DB lookup, so a wrong-shaped Authorization header
// never reaches the hash comparison. It also doubles as a secret-scanner
// signature, same reasoning as GitHub's own token prefixes.
const TokenPrefix = "belune_pat_"

// lastUsedCoarsen bounds how often a validated token's last_used_at is
// written. A dashboard held open or a 15s Prometheus scrape would otherwise
// turn every single request into a write; the field only needs to answer "is
// this token still in use", not record exact request timestamps.
const lastUsedCoarsen = 5 * time.Minute

// HasTokenPrefix reports whether s looks like a personal access token.
func HasTokenPrefix(s string) bool {
	return strings.HasPrefix(s, TokenPrefix)
}

// TokenService owns personal access tokens: generation, hashing, and the
// authentication lookup the auth middleware calls on every Bearer request
// whose value has the PAT prefix.
type TokenService struct {
	queries *generated.Queries
}

func NewTokenService(queries *generated.Queries) *TokenService {
	return &TokenService{queries: queries}
}

// AuthenticatedToken is what a valid PAT resolves to — enough for the auth
// middleware to populate the request context exactly like a JWT session.
type AuthenticatedToken struct {
	TokenID       pgtype.UUID
	UserID        pgtype.UUID
	EffectiveRole string
	Scopes        []string
	// ProjectID is invalid (zero) when the token is not pinned — every
	// project the owner can reach, evaluated at use time.
	ProjectID pgtype.UUID
}

// GenerateToken produces a new plaintext token and its SHA-256 hash. The
// plaintext is returned to the caller exactly once by whoever calls this
// (PR3's create endpoint) — only the hash is ever persisted.
func GenerateToken() (plain string, hash []byte, err error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", nil, err
	}
	plain = TokenPrefix + base64.RawURLEncoding.EncodeToString(b)
	sum := sha256.Sum256([]byte(plain))
	return plain, sum[:], nil
}

func hashToken(plain string) []byte {
	sum := sha256.Sum256([]byte(plain))
	return sum[:]
}

// minRole returns the more restrictive of two roles. There are only two
// roles, so this is "both admin, or member" rather than a general ordering —
// it exists to make the shrink-only intent readable at the call site, not to
// generalise past a two-role model that isn't there.
func minRole(a, b string) string {
	if a == "admin" && b == "admin" {
		return "admin"
	}
	return "member"
}

// Authenticate looks up a plaintext PAT by its hash, rejects an unknown or
// expired one, and computes the effective role — min(role_at_issue, the
// owner's role right now). Demotion restricts immediately; promotion does
// NOT retroactively elevate a token minted under lower privilege.
//
// Also coarsens last_used_at (see lastUsedCoarsen): the touch is best-effort
// and its failure must never fail authentication.
func (s *TokenService) Authenticate(ctx context.Context, plain string) (*AuthenticatedToken, error) {
	row, err := s.queries.GetAPITokenByHash(ctx, hashToken(plain))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrInvalidAPIToken
		}
		return nil, err
	}

	if row.ExpiresAt.Valid && row.ExpiresAt.Time.Before(time.Now()) {
		return nil, ErrInvalidAPIToken
	}

	now := time.Now()
	if !row.LastUsedAt.Valid || now.Sub(row.LastUsedAt.Time) > lastUsedCoarsen {
		if err := s.queries.UpdateAPITokenLastUsed(ctx, generated.UpdateAPITokenLastUsedParams{
			ID:         row.ID,
			LastUsedAt: pgtype.Timestamptz{Time: now, Valid: true},
		}); err != nil {
			slog.Warn("token: failed to update last_used_at", "token_id", uuidString(row.ID), "error", err)
		}
	}

	return &AuthenticatedToken{
		TokenID:       row.ID,
		UserID:        row.UserID,
		EffectiveRole: minRole(row.RoleAtIssue, row.UserRole),
		Scopes:        row.Scopes,
		ProjectID:     row.ProjectID,
	}, nil
}
